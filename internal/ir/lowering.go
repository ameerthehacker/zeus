package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/util"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// LowerPass represents a pluggable pass that can be run to lower IR constructs
// Passes receive individual instructions with automatic context management
type LowerPass interface {
	// HandleInstruction processes a single instruction
	// The Lowerer automatically manages currentBlock context
	HandleInstruction(l *Lowerer, instr *Instr)

	// Finalize is called after all instructions have been processed
	// This allows passes to perform the actual lowering transformations
	Finalize(l *Lowerer)

	// GetName returns the name of the pass for debugging/logging
	GetName() string
}

// Lowerer transforms high-level IR constructs into lower-level operations.
// This runs after type checking and before codegen.
type Lowerer struct {
	builder      *IRBuilder
	currentBlock *BasicBlock
	passes       []LowerPass
}

func NewLowerer(builder *IRBuilder) *Lowerer {
	l := &Lowerer{
		builder: builder,
	}

	// Initialize lowering passes
	// Note: IndexLoweringPass should run before StringCastLoweringPass
	// because array operations may involve string arrays
	// StringOperatorLoweringPass should run before StringCastLoweringPass
	// because string operators need to be lowered to method calls first
	// ArrayMethodLoweringPass should run early to lower concat/slice
	// before other passes might interfere
	// FuncTypePropCallLoweringPass must run before FuncPtrNullCheckPass because it
	// converts CALL_METHOD instructions (for fn-type properties) into INDIRECT_FUNC_CALL,
	// which is what the null-check pass watches for.
	l.passes = []LowerPass{
		// Runs first so all later passes see a normalized fixed-arity argument list.
		NewVariadicCallLoweringPass(),
		NewArrayMethodLoweringPass(),
		NewIndexLoweringPass(),
		NewStringOperatorLoweringPass(),
		NewStringCastLoweringPass(),
		NewAccessorLoweringPass(), // must run before FuncTypePropCallLoweringPass
		NewFuncTypePropCallLoweringPass(),
		NewCastLoweringPass(),
		NewFunctorCallLoweringPass(),
		NewFuncPtrNullCheckPass(),
		// Runs last: synthesizes user-class (and functor-wrapper) factory functions, so it must
		// see every class — including the wrappers CastLoweringPass emits.
		NewFactoryLoweringPass(),
	}

	return l
}

// AddPass adds a new pass to the lowerer
func (l *Lowerer) AddPass(pass LowerPass) {
	l.passes = append(l.passes, pass)
}

// Lower runs all registered passes
func (l *Lowerer) Lower() {
	for _, pass := range l.passes {
		l.runPass(pass)
	}
}

// runPass executes a single pass with automatic context management
func (l *Lowerer) runPass(pass LowerPass) {
	// Walk through all instructions including those inside function bodies
	for _, instr := range l.builder.GetInstrs() {
		l.currentBlock = nil
		pass.HandleInstruction(l, instr)

		// If this is a function declaration, walk its body blocks
		if IsFunctionDeclInstr(instr.Type) {
			body := AsDeclFuncInstrInput(instr.Input).Body
			l.walkBlocks(body, pass)
		}
		if IsClassMethodDeclInstr(instr.Type) {
			body := AsDeclClassMethodInstrInput(instr.Input).Body
			l.walkBlocks(body, pass)
		}
	}

	// Call finalize after processing all instructions
	pass.Finalize(l)
}

// walkBlocks walks through a block and its successors
func (l *Lowerer) walkBlocks(block *BasicBlock, pass LowerPass) {
	if block == nil {
		return
	}

	visited := make(map[int]bool)
	worklist := []*BasicBlock{block}

	for len(worklist) > 0 {
		current := worklist[0]
		worklist = worklist[1:]

		if visited[current.Id] {
			continue
		}
		visited[current.Id] = true

		l.currentBlock = current
		for _, instr := range current.Instrs {
			pass.HandleInstruction(l, instr)
		}

		worklist = append(worklist, current.Successors...)
	}
}

// GetBuilder returns the IR builder
func (l *Lowerer) GetBuilder() *IRBuilder {
	return l.builder
}

// GetCurrentBlock returns the current basic block being processed
func (l *Lowerer) GetCurrentBlock() *BasicBlock {
	return l.currentBlock
}

// =============================================================================
// StringCastLoweringPass - Lowers string<->u8[] casts to proper IR sequences
// =============================================================================

type castToLower struct {
	instr             *Instr
	input             *CastInstrInput
	block             *BasicBlock
	isU8ArrayToString bool
}

// StringCastLoweringPass lowers string<->u8[] cast operations
type StringCastLoweringPass struct {
	castsToLower []castToLower
}

func NewStringCastLoweringPass() *StringCastLoweringPass {
	return &StringCastLoweringPass{}
}

func (p *StringCastLoweringPass) GetName() string {
	return "StringCastLowering"
}

// HandleInstruction collects CAST instructions that need lowering
func (p *StringCastLoweringPass) HandleInstruction(l *Lowerer, instr *Instr) {
	if instr.Type != InstrTypeCast {
		return
	}

	castInput := AsCastInstrInput(instr.Input)
	if castInput == nil {
		return
	}

	valueType := zeus_value.GetValueType(castInput.Value)

	// u8[] -> string
	if isU8ArrayType(valueType) {
		if objType, ok := castInput.CastType.(zeus_value.ObjectType); ok && objType.Class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING {
			p.castsToLower = append(p.castsToLower, castToLower{instr, castInput, l.GetCurrentBlock(), true})
			return
		}
	}

	// string -> u8[]
	if objType, ok := valueType.(zeus_value.ObjectType); ok && objType.Class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING {
		if isU8ArrayType(castInput.CastType) {
			p.castsToLower = append(p.castsToLower, castToLower{instr, castInput, l.GetCurrentBlock(), false})
			return
		}
	}
}

// Finalize performs the actual lowering transformations
func (p *StringCastLoweringPass) Finalize(l *Lowerer) {
	for _, cast := range p.castsToLower {
		if cast.isU8ArrayToString {
			p.lowerU8ArrayToStringCast(l, cast.instr, cast.input, cast.block)
		} else {
			p.lowerStringToU8ArrayCast(l, cast.instr, cast.input, cast.block)
		}
	}
}

// lowerU8ArrayToStringCast lowers a u8[] -> string cast
// Generates: get length, new u8[], copy, new string
func (p *StringCastLoweringPass) lowerU8ArrayToStringCast(l *Lowerer, castInstr *Instr, input *CastInstrInput, block *BasicBlock) {
	span := castInstr.Span
	builder := l.GetBuilder()
	setInsertionPoint(builder, block, castInstr)

	u8ArrayClass := getU8ArrayClass(builder)
	stringClass := getStringClass(builder)
	u8ArrayType := zeus_value.NewObjectType(u8ArrayClass)
	stringType := zeus_value.NewObjectType(stringClass)
	i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: span}

	// Get source array length and create new array with copy
	sourceLength := builder.BuildLoadProperty(input.Value, zeus_value.ARRAY_PROPERTY_LENGTH, i32Type, span)
	newArray := createTypedNewObj(builder, u8ArrayClass, []zeus_value.Value{sourceLength}, span)

	builder.BuildMethodCall(newArray, zeus_value.ARRAY_METHOD_COPY,
		[]zeus_value.Value{input.Value},
		zeus_value.VoidType{Span: span},
		[]zeus_value.ValueType{u8ArrayType},
		span)

	// Create new string with the copied array
	result := createTypedNewObj(builder, stringClass, []zeus_value.Value{newArray}, span)
	updateOutputVar(castInstr.Output, result, stringType)
	builder.DeleteInstr(block, castInstr)
}

// lowerStringToU8ArrayCast lowers a string -> u8[] cast
// Generates: get data, get length, new u8[], copy
func (p *StringCastLoweringPass) lowerStringToU8ArrayCast(l *Lowerer, castInstr *Instr, input *CastInstrInput, block *BasicBlock) {
	span := castInstr.Span
	builder := l.GetBuilder()
	setInsertionPoint(builder, block, castInstr)

	u8ArrayClass := getU8ArrayClass(builder)
	u8ArrayType := zeus_value.NewObjectType(u8ArrayClass)
	i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: span}

	// Access string.data and get its length
	sourceData := builder.BuildLoadProperty(input.Value, zeus_value.STRING_PROPERTY_DATA, u8ArrayType, span)
	sourceLength := builder.BuildLoadProperty(sourceData, zeus_value.ARRAY_PROPERTY_LENGTH, i32Type, span)

	// Create new u8[] with copy
	newArray := createTypedNewObj(builder, u8ArrayClass, []zeus_value.Value{sourceLength}, span)

	builder.BuildMethodCall(newArray, zeus_value.ARRAY_METHOD_COPY,
		[]zeus_value.Value{sourceData},
		zeus_value.VoidType{Span: span},
		[]zeus_value.ValueType{u8ArrayType},
		span)

	updateOutputVar(castInstr.Output, newArray, u8ArrayType)
	builder.DeleteInstr(block, castInstr)
}

// =============================================================================
// Helper functions for lowering passes
// =============================================================================

// setInsertionPoint sets the builder to insert before the given instruction
func setInsertionPoint(builder *IRBuilder, block *BasicBlock, instr *Instr) {
	if block != nil {
		builder.SetBlockInsertionBefore(block, instr)
	} else {
		builder.SetInsertionBefore(instr)
	}
}

// getClassFromSymbolTable returns a class from symbol table with assertions
func getClassFromSymbolTable(builder *IRBuilder, className string) *zeus_value.Class {
	existingClass, ok := builder.symbolTable.GetSymbol(className)
	zeus_error.Assert(ok, className+" class not found in symbol table - lowering requires type checking to run first")

	class := zeus_value.AsClass(existingClass)
	zeus_error.Assert(class != nil, className+" symbol is not a class")

	return class
}

// getU8ArrayClass returns the u8[] class from symbol table
func getU8ArrayClass(builder *IRBuilder) *zeus_value.Class {
	return getClassFromSymbolTable(builder, "u8[]")
}

// getStringClass returns the string class from symbol table
func getStringClass(builder *IRBuilder) *zeus_value.Class {
	return getClassFromSymbolTable(builder, zeus_value.ZEUS_PRIMORDIAL_STRING)
}

// createTypedNewObj creates a new object and sets its type correctly
func createTypedNewObj(builder *IRBuilder, class *zeus_value.Class, args []zeus_value.Value, span *token.Span) zeus_value.Value {
	objType := zeus_value.NewObjectType(class)
	newObj := builder.BuildNewObj(class, args, span)
	zeus_value.AsVar(newObj).ValueType = objType
	return newObj
}

// deleteInstrIfPresent deletes an instruction if both instr and block are non-nil
func deleteInstrIfPresent(builder *IRBuilder, block *BasicBlock, instr *Instr) {
	if instr != nil && block != nil {
		builder.DeleteInstr(block, instr)
	}
}

// getArrayClassFromValue extracts the array class from a value's type
func getArrayClassFromValue(value zeus_value.Value) *zeus_value.Class {
	objType := zeus_value.AsObjectType(zeus_value.GetValueType(value))
	zeus_error.Assert(objType != nil, "value is not an object type")
	return objType.Class
}

// updateOutputVar updates the output variable to point to the result value
func updateOutputVar(output *zeus_value.Var, result zeus_value.Value, resultType zeus_value.ValueType) {
	if output != nil {
		resultVar := zeus_value.AsVar(result)
		output.Name = resultVar.Name
		output.ValueType = resultType
	}
}

// =============================================================================
// AccessorLoweringPass - Lowers GET_ACCESSOR/SET_ACCESSOR instructions.
// User-defined accessors lower to CALL_METHOD with mangled names (#get_X / #set_X).
// Primordial IsLowered accessors (e.g. arr.length) lower to direct field access.
type accessorOp struct {
	instr *Instr
	block *BasicBlock
	isGet bool
}

type AccessorLoweringPass struct {
	ops []accessorOp
}

func NewAccessorLoweringPass() *AccessorLoweringPass {
	return &AccessorLoweringPass{}
}

func (p *AccessorLoweringPass) GetName() string { return "AccessorLowering" }

func (p *AccessorLoweringPass) HandleInstruction(l *Lowerer, instr *Instr) {
	switch instr.Type {
	case InstrTypeGetAccessor:
		p.ops = append(p.ops, accessorOp{instr, l.GetCurrentBlock(), true})
	case InstrTypeSetAccessor:
		p.ops = append(p.ops, accessorOp{instr, l.GetCurrentBlock(), false})
	}
}

func (p *AccessorLoweringPass) Finalize(l *Lowerer) {
	for _, op := range p.ops {
		if op.isGet {
			p.lowerGetAccessor(l, op.instr, op.block)
		} else {
			p.lowerSetAccessor(l, op.instr, op.block)
		}
	}
}

func (p *AccessorLoweringPass) findAccessor(object zeus_value.Value, accessorName string) *zeus_value.ClassAccessor {
	// Static accessor: object is a *Class
	if class := zeus_value.AsClass(object); class != nil {
		return zeus_value.LookupStaticAccessor(class, accessorName)
	}
	// Instance accessor
	objType := zeus_value.AsObjectType(zeus_value.GetValueType(object))
	if objType == nil {
		return nil
	}
	return zeus_value.LookupAccessor(objType.Class, accessorName)
}

func (p *AccessorLoweringPass) lowerGetAccessor(l *Lowerer, instr *Instr, block *BasicBlock) {
	input := AsGetAccessorInstrInput(instr.Input)
	builder := l.GetBuilder()
	span := instr.Span

	acc := p.findAccessor(input.Object, input.AccessorName)
	if acc == nil || acc.Getter == nil {
		return
	}

	setInsertionPoint(builder, block, instr)

	var result zeus_value.Value
	if acc.IsStatic {
		// Static accessor: lower to CALL_FUNC with mangled getter name (no self param).
		result = builder.BuildCallFunc(acc.Getter, []zeus_value.Value{}, span)
	} else if acc.IsLowered {
		// Primordial accessor: lower to direct field access ("_" + accessorName)
		internalPropName := "_" + input.AccessorName
		propPtr := builder.BuildObjectPropertyAccess(input.Object, internalPropName, false, span)
		result = builder.BuildLoad(zeus_value.AsVar(propPtr), span)
	} else {
		// User-defined accessor: lower to CALL_METHOD with mangled getter name.
		result = builder.BuildMethodCall(input.Object, acc.Getter.Name, []zeus_value.Value{}, acc.Getter.ReturnType, []zeus_value.ValueType{}, span)
	}

	updateOutputVar(instr.Output, result, acc.Getter.ReturnType)
	builder.DeleteInstr(block, instr)
}

func (p *AccessorLoweringPass) lowerSetAccessor(l *Lowerer, instr *Instr, block *BasicBlock) {
	input := AsSetAccessorInstrInput(instr.Input)
	builder := l.GetBuilder()
	span := instr.Span

	acc := p.findAccessor(input.Object, input.AccessorName)
	if acc == nil || acc.Setter == nil {
		return
	}

	setInsertionPoint(builder, block, instr)

	var result zeus_value.Value
	if acc.IsStatic {
		// Static accessor: lower to CALL_FUNC with mangled setter name (no self param).
		result = builder.BuildCallFunc(acc.Setter, []zeus_value.Value{input.Value}, span)
	} else if acc.IsLowered {
		// Lowered primordial setters — not common, but handle gracefully by direct store.
		// Currently only IsLowered getters are used (arr.length is read-only).
		internalPropName := "_" + input.AccessorName
		propPtr := builder.BuildObjectPropertyAccess(input.Object, internalPropName, true, span)
		builder.BuildStore(zeus_value.AsVar(propPtr), input.Value, span)
		result = input.Value
	} else {
		// User-defined setter: lower to CALL_METHOD with mangled setter name.
		result = builder.BuildMethodCall(input.Object, acc.Setter.Name, []zeus_value.Value{input.Value}, zeus_value.VoidType{Span: span}, []zeus_value.ValueType{}, span)
	}

	updateOutputVar(instr.Output, result, zeus_value.GetValueType(input.Value))
	builder.DeleteInstr(block, instr)
}

// IndexLoweringPass - Lowers GET_INDEX/SET_INDEX instructions to method calls
// =============================================================================

type indexToLower struct {
	instr *Instr
	input *GetIndexInstrInput
	block *BasicBlock
}

type setIndexToLower struct {
	instr *Instr
	input *SetIndexInstrInput
	block *BasicBlock
}

// IndexLoweringPass lowers GET_INDEX/SET_INDEX instructions to array.get()/.set() method calls
type IndexLoweringPass struct {
	indicesToLower    []indexToLower
	setIndicesToLower []setIndexToLower
}

func NewIndexLoweringPass() *IndexLoweringPass {
	return &IndexLoweringPass{}
}

func (p *IndexLoweringPass) GetName() string {
	return "IndexLowering"
}

// HandleInstruction collects GET_INDEX/SET_INDEX instructions that need lowering
func (p *IndexLoweringPass) HandleInstruction(l *Lowerer, instr *Instr) {
	if instr.Type == InstrTypeGetIndex {
		input := AsGetIndexInstrInput(instr.Input)
		if input == nil {
			return
		}
		p.indicesToLower = append(p.indicesToLower, indexToLower{instr, input, l.GetCurrentBlock()})
		return
	}

	if instr.Type == InstrTypeSetIndex {
		input := AsSetIndexInstrInput(instr.Input)
		if input == nil {
			return
		}
		p.setIndicesToLower = append(p.setIndicesToLower, setIndexToLower{instr, input, l.GetCurrentBlock()})
	}
}

// Finalize performs the actual lowering transformations
func (p *IndexLoweringPass) Finalize(l *Lowerer) {
	for _, indexOp := range p.indicesToLower {
		p.lowerGetIndex(l, indexOp.instr, indexOp.input, indexOp.block)
	}
	for _, setOp := range p.setIndicesToLower {
		p.lowerSetIndex(l, setOp.instr, setOp.input, setOp.block)
	}
}

// lowerGetIndex converts GET_INDEX into a sequence of array.get() method calls
// For array[i][j], generates: temp = array.get(i); result = temp.get(j)
// For string[i], generates: temp = string.data; result = temp.get(i)
func (p *IndexLoweringPass) lowerGetIndex(l *Lowerer, instr *Instr, input *GetIndexInstrInput, block *BasicBlock) {
	span := instr.Span
	builder := l.GetBuilder()
	setInsertionPoint(builder, block, instr)

	currentValue := input.Array
	i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: span}

	// Check if this is string indexing
	targetType := zeus_value.GetValueType(currentValue)
	if objType := zeus_value.AsObjectType(targetType); objType != nil && objType.Class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING {
		// String indexing: first access the .data property to get the u8[]
		zeus_error.Assert(len(input.Indices) == 1, "string indexing should only have one index - type checking should have caught this")

		u8ArrayClass := getU8ArrayClass(builder)
		u8ArrayType := zeus_value.NewObjectType(u8ArrayClass)
		u8Type := zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: span}

		dataArray := builder.BuildLoadProperty(currentValue, zeus_value.STRING_PROPERTY_DATA, u8ArrayType, span)
		result := builder.BuildMethodCall(dataArray, zeus_value.ARRAY_METHOD_GET,
			[]zeus_value.Value{input.Indices[0]}, u8Type, []zeus_value.ValueType{i32Type}, span)

		updateOutputVar(instr.Output, result, u8Type)
		builder.DeleteInstr(block, instr)
		return
	}

	// Fast path: a single-index read of a primitive-element array lowers to a direct
	// load from the array's raw `data` buffer, skipping the get() method call, its
	// vtable dispatch and the runtime round-trip. This read is already bounds-checked
	// upstream by emitBoundsCheck (see internal/ir/ir.go, VisitIndex rvalue path), so
	// the index is guaranteed in range here. Non-primitive elements (nested arrays /
	// object elements) and multi-index accesses keep the method-call path below.
	if len(input.Indices) == 1 {
		if objType := zeus_value.AsObjectType(zeus_value.GetValueType(currentValue)); objType != nil &&
			objType.Class.ArrayElementType != nil && zeus_value.IsPrimitiveType(objType.Class.ArrayElementType) {
			elementType := objType.Class.ArrayElementType
			data := builder.BuildLoadProperty(currentValue, zeus_value.ARRAY_PROPERTY_DATA, zeus_value.OpaqueType{Span: span}, span)
			result := builder.BuildElemLoad(data, input.Indices[0], elementType, span)
			updateOutputVar(instr.Output, result, elementType)
			builder.DeleteInstr(block, instr)
			return
		}
	}

	// Process each index, calling .get() method for each one
	for _, index := range input.Indices {
		arrayType := zeus_value.GetValueType(currentValue)

		objType := zeus_value.AsObjectType(arrayType)
		zeus_error.Assert(objType != nil && objType.Class.ArrayElementType != nil,
			"GET_INDEX lowering requires array type with element type - type checking should have caught this")
		elementType := objType.Class.ArrayElementType

		result := builder.BuildMethodCall(currentValue, zeus_value.ARRAY_METHOD_GET,
			[]zeus_value.Value{index}, elementType, []zeus_value.ValueType{i32Type}, span)

		// Set the result type - handle nested arrays specially
		resultVar := zeus_value.AsVar(result)
		if arrayElemType, ok := elementType.(zeus_value.ArrayType); ok {
			if existingClass, ok := builder.symbolTable.GetSymbol(arrayElemType.String()); ok {
				if class := zeus_value.AsClass(existingClass); class != nil {
					resultVar.ValueType = zeus_value.NewObjectType(class)
				}
			}
		}
		currentValue = result
	}

	// Update the output variable
	if instr.Output != nil && currentValue != nil {
		if resultVar := zeus_value.AsVar(currentValue); resultVar != nil {
			instr.Output.Name = resultVar.Name
			instr.Output.ValueType = resultVar.ValueType
		}
	}

	builder.DeleteInstr(block, instr)
}

// lowerSetIndex converts SET_INDEX into either a direct data-buffer store (for
// primitive-element arrays) guarded by a bounds check, or an array.set() method call.
//
// Bracket writes auto-extend the array on an out-of-range index (a documented feature),
// so the fast path only takes the in-bounds case and falls back to the runtime set() —
// which grows the array — for out-of-range or negative indices:
//
//	if (0 <= index < length) data[index] = value      // fast: direct store
//	else                     array.set(index, value)  // runtime: auto-extend
func (p *IndexLoweringPass) lowerSetIndex(l *Lowerer, instr *Instr, input *SetIndexInstrInput, block *BasicBlock) {
	span := instr.Span
	builder := l.GetBuilder()

	i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: span}
	valueType := zeus_value.GetValueType(input.Value)

	arrayType := zeus_value.GetValueType(input.Array)
	objType := zeus_value.AsObjectType(arrayType)
	zeus_error.Assert(objType != nil && objType.Class.ArrayElementType != nil,
		"SET_INDEX lowering requires array type with element type - type checking should have caught this")
	elementType := objType.Class.ArrayElementType

	// An earlier fast-pathed write may have split this instruction into a new tail block,
	// so locate its current block rather than trusting the one captured at collection time.
	curBlock := builder.FindBlockContaining(instr)
	if curBlock == nil {
		curBlock = block
	}

	// Fast path: primitive-element arrays store directly into the `data` buffer, guarded so
	// out-of-range writes still auto-extend via the runtime set().
	if zeus_value.IsPrimitiveType(elementType) {
		if tail := builder.SplitBlockBefore(curBlock, instr); tail != nil {
			inBoundsBlock := builder.BuildBasicBlock()
			fallbackBlock := builder.BuildBasicBlock()
			curBlock.Successors = []*BasicBlock{inBoundsBlock, fallbackBlock}

			// curBlock: outOfBounds = index < 0 || index >= length
			builder.ResetBlockInsertionToEnd(curBlock)
			length := builder.BuildLoadProperty(input.Array, zeus_value.ARRAY_PROPERTY_LENGTH, i32Type, span)
			zero := zeus_value.NewConstant("0", i32Type, span)
			isNegative := builder.BuildBinaryOp(input.Index, zero, InstrTypeLessThan, span)
			isOverflow := builder.BuildBinaryOp(input.Index, length, InstrTypeGreaterThanEq, span)
			outOfBounds := builder.BuildBinaryOp(isNegative, isOverflow, InstrTypeOr, span)
			builder.BuildCondJmp(fallbackBlock, inBoundsBlock, outOfBounds, span)

			// inBoundsBlock: direct store into the data buffer. The value must be converted
			// to the element type first: SET_INDEX values are not pre-cast (the runtime set()
			// normally handles element-width conversion), so storing e.g. an i8 literal into an
			// i32[] slot would otherwise emit a 1-byte store and corrupt the upper bytes.
			builder.ResetBlockInsertionToEnd(inBoundsBlock)
			value := input.Value
			if zeus_value.GetValueType(value).String() != elementType.String() {
				value = builder.BuildCast(value, elementType, span)
			}
			data := builder.BuildLoadProperty(input.Array, zeus_value.ARRAY_PROPERTY_DATA, zeus_value.OpaqueType{Span: span}, span)
			builder.BuildElemStore(data, input.Index, value, elementType, span)
			builder.BuildJmp(tail, span)
			inBoundsBlock.Successors = []*BasicBlock{tail}

			// fallbackBlock: runtime set() (auto-extend / negative indices).
			builder.ResetBlockInsertionToEnd(fallbackBlock)
			builder.BuildMethodCall(input.Array, zeus_value.ARRAY_METHOD_SET,
				[]zeus_value.Value{input.Index, input.Value},
				zeus_value.VoidType{Span: span},
				[]zeus_value.ValueType{i32Type, valueType},
				span)
			builder.BuildJmp(tail, span)
			fallbackBlock.Successors = []*BasicBlock{tail}

			builder.DeleteInstr(tail, instr)
			return
		}
	}

	// Method-call path: non-primitive elements, or the block could not be split.
	setInsertionPoint(builder, curBlock, instr)
	builder.BuildMethodCall(input.Array, zeus_value.ARRAY_METHOD_SET,
		[]zeus_value.Value{input.Index, input.Value},
		zeus_value.VoidType{Span: span},
		[]zeus_value.ValueType{i32Type, valueType},
		span)

	builder.DeleteInstr(curBlock, instr)
}

// =============================================================================
// VariadicCallLoweringPass - collapses the trailing arguments of variadic calls
// into a freshly-created Zeus array, so a variadic callee always receives an array.
// =============================================================================

type variadicCallToLower struct {
	instr      *Instr
	block      *BasicBlock
	fixedCount int
	restType   zeus_value.ValueType
}

// VariadicCallLoweringPass rewrites calls to variadic functions/methods. Trailing
// arguments (those past the fixed parameters) are pushed into a new Zeus array and the
// call's argument list becomes [fixed..., restArray]. It handles direct calls
// (CALL_FUNC), indirect/functor calls (INDIRECT_FUNC_CALL) and method calls
// (METHOD_CALL). It runs first so every later pass observes a fixed-arity argument list.
type VariadicCallLoweringPass struct {
	callsToLower []variadicCallToLower
}

func NewVariadicCallLoweringPass() *VariadicCallLoweringPass {
	return &VariadicCallLoweringPass{}
}

func (p *VariadicCallLoweringPass) GetName() string {
	return "VariadicCallLowering"
}

func (p *VariadicCallLoweringPass) HandleInstruction(l *Lowerer, instr *Instr) {
	fixedCount, restType, ok := variadicCallTarget(instr)
	if !ok {
		return
	}
	p.callsToLower = append(p.callsToLower, variadicCallToLower{instr, l.GetCurrentBlock(), fixedCount, restType})
}

func (p *VariadicCallLoweringPass) Finalize(l *Lowerer) {
	builder := l.GetBuilder()
	for _, call := range p.callsToLower {
		lowerVariadicCall(builder, call)
	}
}

// variadicCallTarget reports the fixed-parameter count and rest parameter type when
// instr is a call to a variadic callee; ok is false otherwise.
func variadicCallTarget(instr *Instr) (int, zeus_value.ValueType, bool) {
	switch instr.Type {
	case InstrTypeCallFunc:
		callee := zeus_value.AsFunction(AsCallFuncInstrInput(instr.Input).Callee)
		if callee == nil || !callee.IsVariadic {
			return 0, nil, false
		}
		return restFromParams(callee.Params)
	case InstrTypeIndirectFuncCall:
		calleeType := zeus_value.GetValueType(AsIndirectFuncCallInstrInput(instr.Input).Function)
		if ft := zeus_value.AsFunctionType(calleeType); ft != nil {
			if !ft.IsVariadic {
				return 0, nil, false
			}
			return restFromParamTypes(ft.ParamTypes)
		}
		// Functor/closure object callee (e.g. `f(args)` where f is a functor instance).
		// The call is still INDIRECT here — FunctorCallLoweringPass rewrites it to a
		// __call__ method call later, after this pass has collapsed the trailing args.
		if objType := zeus_value.AsObjectType(calleeType); objType != nil {
			if params, ok := variadicMethodParams(objType.Class, token.FUNCTOR_CALL_METHOD_NAME); ok {
				return restFromParams(params)
			}
		}
	case InstrTypeMethodCall:
		input := AsMethodCallInstrInput(instr.Input)
		objType := zeus_value.AsObjectType(zeus_value.GetValueType(input.Object))
		if objType == nil {
			return 0, nil, false
		}
		// Real class method.
		if params, ok := variadicMethodParams(objType.Class, input.MethodName); ok {
			return restFromParams(params)
		}
		// Function-typed property invoked as a method: obj.fnProp(args).
		for _, property := range objType.Class.Properties {
			if property.Property.Name == input.MethodName {
				if ft, ok := property.Property.ValueType.(zeus_value.FunctionType); ok && ft.IsVariadic {
					return restFromParamTypes(ft.ParamTypes)
				}
			}
		}
	}
	return 0, nil, false
}

// variadicMethodParams returns the parameters of a class method matched by source name
// when that method is variadic.
func variadicMethodParams(class *zeus_value.Class, name string) ([]*zeus_value.Var, bool) {
	for _, method := range class.Methods {
		if method.Method.SourceName() == name && method.Method.IsVariadic {
			return method.Method.Params, true
		}
	}
	return nil, false
}

func restFromParams(params []*zeus_value.Var) (int, zeus_value.ValueType, bool) {
	if len(params) == 0 {
		return 0, nil, false
	}
	fixedCount := len(params) - 1
	return fixedCount, params[fixedCount].ValueType, true
}

func restFromParamTypes(paramTypes []zeus_value.ValueType) (int, zeus_value.ValueType, bool) {
	if len(paramTypes) == 0 {
		return 0, nil, false
	}
	fixedCount := len(paramTypes) - 1
	return fixedCount, paramTypes[fixedCount], true
}

// arrayClassFromType resolves the array primordial class for a rest parameter's type.
// Rest parameters resolved during IR gen are ObjectTypes; a rest parameter coming from a
// function-type annotation (e.g. `(...xs: i32[]) => i32`) is a raw ArrayType that must be
// looked up (or created) from the array class registry.
func arrayClassFromType(builder *IRBuilder, valueType zeus_value.ValueType) *zeus_value.Class {
	if objType := zeus_value.AsObjectType(valueType); objType != nil && objType.Class.ArrayElementType != nil {
		return objType.Class
	}
	arrayType, ok := valueType.(zeus_value.ArrayType)
	if !ok {
		return nil
	}
	className := arrayType.String()
	if sym, ok := builder.symbolTable.GetSymbol(className); ok {
		return zeus_value.AsClass(sym)
	}
	arrayClass := zeus_value.Registry.GetOrCreateArrayClass(arrayType)
	builder.symbolTable.DeclareGlobalSymbol(className, arrayClass)
	builder.EmitClassDeclAtStart(arrayClass)
	return arrayClass
}

func lowerVariadicCall(builder *IRBuilder, call variadicCallToLower) {
	instr := call.instr
	span := instr.Span
	args := variadicCallArgs(instr)

	arrayClass := arrayClassFromType(builder, call.restType)
	zeus_error.Assert(arrayClass != nil, "variadic rest parameter is not an array type - type checking should have caught this")

	setInsertionPoint(builder, call.block, instr)

	// Build an empty rest array (always created, even with zero trailing args) and push
	// each trailing argument into it. The array is created empty (length 0) and grown by
	// push — the constructor argument is the initial length, so it must start at 0.
	zero := zeus_value.NewConstant("0", zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: span}, span)
	arrayObj := createTypedNewObj(builder, arrayClass, []zeus_value.Value{zero}, span)

	elementType := arrayClass.ArrayElementType
	for _, arg := range args[call.fixedCount:] {
		builder.BuildMethodCall(arrayObj, zeus_value.ARRAY_METHOD_PUSH,
			[]zeus_value.Value{arg},
			zeus_value.VoidType{Span: span},
			[]zeus_value.ValueType{elementType},
			span)
	}

	// Rewrite the call arguments to [fixed..., restArray].
	newArgs := append(append([]zeus_value.Value{}, args[:call.fixedCount]...), arrayObj)
	setVariadicCallArgs(instr, newArgs)
}

// variadicCallArgs returns the argument slice of a call instruction.
func variadicCallArgs(instr *Instr) []zeus_value.Value {
	switch instr.Type {
	case InstrTypeCallFunc:
		return AsCallFuncInstrInput(instr.Input).Args
	case InstrTypeIndirectFuncCall:
		return AsIndirectFuncCallInstrInput(instr.Input).Args
	case InstrTypeMethodCall:
		return AsMethodCallInstrInput(instr.Input).Args
	}
	return nil
}

// setVariadicCallArgs replaces the argument slice of a call instruction in place.
func setVariadicCallArgs(instr *Instr, args []zeus_value.Value) {
	switch instr.Type {
	case InstrTypeCallFunc:
		AsCallFuncInstrInput(instr.Input).Args = args
	case InstrTypeIndirectFuncCall:
		AsIndirectFuncCallInstrInput(instr.Input).Args = args
	case InstrTypeMethodCall:
		AsMethodCallInstrInput(instr.Input).Args = args
	}
}

// =============================================================================
// StringOperatorLoweringPass - Lowers string operators to method calls
// =============================================================================

type stringOpToLower struct {
	instr *Instr
	input *BinaryOpInstrInput
	block *BasicBlock
}

// StringOperatorLoweringPass lowers string operators (+, ==, !=, <, >, <=, >=)
// to method calls (concat, equals, compare)
type StringOperatorLoweringPass struct {
	opsToLower []stringOpToLower
}

func NewStringOperatorLoweringPass() *StringOperatorLoweringPass {
	return &StringOperatorLoweringPass{}
}

func (p *StringOperatorLoweringPass) GetName() string {
	return "StringOperatorLowering"
}

// HandleInstruction collects binary operations on strings that need lowering
func (p *StringOperatorLoweringPass) HandleInstruction(l *Lowerer, instr *Instr) {
	// Check if this is a binary operation
	if !isBinaryOp(instr.Type) {
		return
	}

	input := AsBinaryOpInstrInput(instr.Input)
	if input == nil {
		return
	}

	// Check if both operands are strings
	leftType := zeus_value.GetValueType(input.Left)
	rightType := zeus_value.GetValueType(input.Right)

	if !isStringType(leftType) || !isStringType(rightType) {
		return
	}

	p.opsToLower = append(p.opsToLower, stringOpToLower{instr, input, l.GetCurrentBlock()})
}

// Finalize performs the actual lowering transformations
func (p *StringOperatorLoweringPass) Finalize(l *Lowerer) {
	for _, op := range p.opsToLower {
		p.lowerStringOp(l, op.instr, op.input, op.block)
	}
}

// lowerStringOp converts a string binary operation to method calls
func (p *StringOperatorLoweringPass) lowerStringOp(l *Lowerer, instr *Instr, input *BinaryOpInstrInput, block *BasicBlock) {
	span := instr.Span
	builder := l.GetBuilder()
	setInsertionPoint(builder, block, instr)

	stringClass := getStringClass(builder)
	stringType := zeus_value.NewObjectType(stringClass)
	boolType := zeus_value.BoolType{Span: span}

	switch instr.Type {
	case InstrTypeAdd:
		// string + string -> left.concat(right)
		result := builder.BuildMethodCall(input.Left, zeus_value.STRING_METHOD_CONCAT,
			[]zeus_value.Value{input.Right}, stringType, []zeus_value.ValueType{stringType}, span)
		updateOutputVar(instr.Output, result, stringType)

	case InstrTypeEqEq:
		// string == string -> left.equals(right)
		result := builder.BuildMethodCall(input.Left, zeus_value.STRING_METHOD_EQUALS,
			[]zeus_value.Value{input.Right}, boolType, []zeus_value.ValueType{stringType}, span)
		updateOutputVar(instr.Output, result, boolType)

	case InstrTypeNotEq:
		// string != string -> !left.equals(right)
		equalsResult := builder.BuildMethodCall(input.Left, zeus_value.STRING_METHOD_EQUALS,
			[]zeus_value.Value{input.Right}, boolType, []zeus_value.ValueType{stringType}, span)
		result := builder.BuildUnaryOp(equalsResult, InstrTypeNot, span)
		updateOutputVar(instr.Output, result, boolType)

	case InstrTypeLessThan, InstrTypeGreaterThan, InstrTypeLessThanEq, InstrTypeGreaterThanEq:
		// string comparison -> left.compare(right) <op> 0
		i8Type := zeus_value.IntType{Size: zeus_value.I8, Signed: true, Span: span}
		compareResult := builder.BuildMethodCall(input.Left, zeus_value.STRING_METHOD_COMPARE,
			[]zeus_value.Value{input.Right}, i8Type, []zeus_value.ValueType{stringType}, span)
		zero := zeus_value.NewConstant("0", i8Type, span)
		result := builder.BuildBinaryOp(compareResult, zero, instr.Type, span)
		updateOutputVar(instr.Output, result, boolType)
	}

	builder.DeleteInstr(block, instr)
}

// isBinaryOp checks if an instruction type is a binary operation
func isBinaryOp(instrType InstrType) bool {
	switch instrType {
	case InstrTypeAdd, InstrTypeSub, InstrTypeMul, InstrTypeDiv,
		InstrTypeEqEq, InstrTypeNotEq,
		InstrTypeLessThan, InstrTypeGreaterThan,
		InstrTypeLessThanEq, InstrTypeGreaterThanEq:
		return true
	}
	return false
}

// =============================================================================
// ArrayMethodLoweringPass - Lowers array concat/slice to use factory functions
// =============================================================================

type arrayMethodToLower struct {
	callInstr       *Instr                      // CALL_METHOD or INDIRECT_FUNC_CALL instruction
	callInput       *IndirectFuncCallInstrInput // Its input (nil for CALL_METHOD path)
	args            []zeus_value.Value          // Call arguments (unified for both paths)
	propAccessInstr *Instr                      // OBJECT_PROPERTY_ACCESS instruction (nil for CALL_METHOD path)
	loadInstr       *Instr                      // LOAD instruction (nil for CALL_METHOD path)
	block           *BasicBlock                 // Block for call instruction
	propAccessBlock *BasicBlock                 // Block for property access instruction
	loadBlock       *BasicBlock                 // Block for load instruction
	methodName      string
	arrayObj        zeus_value.Value // The array object the method is called on
}

// deleteAllInstrs deletes all related instructions (call, load, property access)
func (m *arrayMethodToLower) deleteAllInstrs(builder *IRBuilder) {
	builder.DeleteInstr(m.block, m.callInstr)
	deleteInstrIfPresent(builder, m.loadBlock, m.loadInstr)
	deleteInstrIfPresent(builder, m.propAccessBlock, m.propAccessInstr)
}

// ArrayMethodLoweringPass lowers array concat/slice methods to proper IR
// that uses factory functions instead of creating arrays in the runtime
type ArrayMethodLoweringPass struct {
	methodsToLower []arrayMethodToLower
	// Track property accesses on arrays
	propAccesses map[string]*propAccessInfo // Map from output var name to property access info
}

type propAccessInfo struct {
	propAccessInstr *Instr // OBJECT_PROPERTY_ACCESS instruction
	loadInstr       *Instr // LOAD instruction (if any)
	object          zeus_value.Value
	methodName      string
	propAccessBlock *BasicBlock
	loadBlock       *BasicBlock
}

func NewArrayMethodLoweringPass() *ArrayMethodLoweringPass {
	return &ArrayMethodLoweringPass{
		propAccesses: make(map[string]*propAccessInfo),
	}
}

func (p *ArrayMethodLoweringPass) GetName() string {
	return "ArrayMethodLowering"
}

// HandleInstruction collects instructions that need lowering
func (p *ArrayMethodLoweringPass) HandleInstruction(l *Lowerer, instr *Instr) {
	// Track OBJECT_PROPERTY_ACCESS on arrays for concat/slice
	if instr.Type == InstrTypeObjectPropertyAccess {
		input := AsObjectPropertyAccessInstrInput(instr.Input)
		if input == nil || instr.Output == nil {
			return
		}

		// Check if the object is an array type
		objType := zeus_value.AsObjectType(zeus_value.GetValueType(input.Object))
		if objType == nil || objType.Class.PrimordialName != zeus_value.ZEUS_PRIMORDIAL_ARRAY {
			return
		}

		// Check if this is concat, slice, or reverse property access
		if input.Property == zeus_value.ARRAY_METHOD_CONCAT || input.Property == zeus_value.ARRAY_METHOD_SLICE || input.Property == zeus_value.ARRAY_METHOD_REVERSE {
			p.propAccesses[instr.Output.Name] = &propAccessInfo{
				propAccessInstr: instr,
				object:          input.Object,
				methodName:      input.Property,
				propAccessBlock: l.GetCurrentBlock(),
			}
		}
		return
	}

	// Track LOAD instructions that load from our tracked property accesses
	if instr.Type == InstrTypeLoad {
		input := AsLoadInstrInput(instr.Input)
		if input == nil || instr.Output == nil {
			return
		}

		// Check if loading from a tracked property access
		if propInfo, ok := p.propAccesses[input.Addr.Name]; ok {
			// Update the mapping to point to the load result, preserving original info
			p.propAccesses[instr.Output.Name] = &propAccessInfo{
				propAccessInstr: propInfo.propAccessInstr,
				loadInstr:       instr,
				object:          propInfo.object,
				methodName:      propInfo.methodName,
				propAccessBlock: propInfo.propAccessBlock,
				loadBlock:       l.GetCurrentBlock(),
			}
		}
		return
	}

	// Look for INDIRECT_FUNC_CALL that uses our tracked methods (old path, retained for safety)
	if instr.Type == InstrTypeIndirectFuncCall {
		input := AsIndirectFuncCallInstrInput(instr.Input)
		if input == nil {
			return
		}

		funcVar := zeus_value.AsVar(input.Function)
		if funcVar == nil {
			return
		}

		// Check if this call is to a tracked method
		if propInfo, ok := p.propAccesses[funcVar.Name]; ok {
			p.methodsToLower = append(p.methodsToLower, arrayMethodToLower{
				callInstr:       instr,
				callInput:       input,
				args:            input.Args,
				propAccessInstr: propInfo.propAccessInstr,
				loadInstr:       propInfo.loadInstr,
				block:           l.GetCurrentBlock(),
				propAccessBlock: propInfo.propAccessBlock,
				loadBlock:       propInfo.loadBlock,
				methodName:      propInfo.methodName,
				arrayObj:        propInfo.object,
			})
		}
		return
	}

	// CALL_METHOD path: directly detect concat/slice/reverse on array objects
	if instr.Type == InstrTypeMethodCall {
		input := AsMethodCallInstrInput(instr.Input)
		if input == nil {
			return
		}

		if input.MethodName != zeus_value.ARRAY_METHOD_CONCAT && input.MethodName != zeus_value.ARRAY_METHOD_SLICE && input.MethodName != zeus_value.ARRAY_METHOD_REVERSE {
			return
		}

		objType := zeus_value.AsObjectType(zeus_value.GetValueType(input.Object))
		if objType == nil || objType.Class.PrimordialName != zeus_value.ZEUS_PRIMORDIAL_ARRAY {
			return
		}

		p.methodsToLower = append(p.methodsToLower, arrayMethodToLower{
			callInstr:  instr,
			args:       input.Args,
			block:      l.GetCurrentBlock(),
			methodName: input.MethodName,
			arrayObj:   input.Object,
		})
	}
}

// Finalize performs the actual lowering transformations
func (p *ArrayMethodLoweringPass) Finalize(l *Lowerer) {
	for _, method := range p.methodsToLower {
		switch method.methodName {
		case zeus_value.ARRAY_METHOD_CONCAT:
			p.lowerArrayConcat(l, method)
		case zeus_value.ARRAY_METHOD_SLICE:
			p.lowerArraySlice(l, method)
		case zeus_value.ARRAY_METHOD_REVERSE:
			p.lowerArrayReverse(l, method)
		}
	}
}

// lowerArrayConcat lowers arr1.concat(arr2) to use factory function
// Generates:
//
//	len1 = arr1.length
//	len2 = arr2.length
//	totalLen = len1 + len2
//	newArr = new T[totalLen]
//	newArr.copyRange(arr1, 0, 0, len1)
//	newArr.copyRange(arr2, 0, len1, len2)
//	result = newArr
func (p *ArrayMethodLoweringPass) lowerArrayConcat(l *Lowerer, method arrayMethodToLower) {
	span := method.callInstr.Span
	builder := l.GetBuilder()
	setInsertionPoint(builder, method.block, method.callInstr)

	arr1 := method.arrayObj
	arr2 := method.args[0]
	arrayClass := getArrayClassFromValue(arr1)
	i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: span}
	arrayObjType := zeus_value.NewObjectType(arrayClass)

	// Load lengths
	len1 := builder.BuildLoadProperty(arr1, zeus_value.ARRAY_PROPERTY_LENGTH, i32Type, span)
	len2 := builder.BuildLoadProperty(arr2, zeus_value.ARRAY_PROPERTY_LENGTH, i32Type, span)

	// totalLen = len1 + len2
	totalLen := builder.BuildBinaryOp(len1, len2, InstrTypeAdd, span)
	zeus_value.AsVar(totalLen).ValueType = i32Type

	// Create new array and copy elements
	newArr := createTypedNewObj(builder, arrayClass, []zeus_value.Value{totalLen}, span)
	zero := zeus_value.NewConstant("0", i32Type, span)

	builder.BuildMethodCall(newArr, zeus_value.ARRAY_METHOD_COPY_RANGE,
		[]zeus_value.Value{arr1, zero, zero, len1},
		zeus_value.VoidType{Span: span},
		[]zeus_value.ValueType{arrayObjType, i32Type, i32Type, i32Type},
		span)

	builder.BuildMethodCall(newArr, zeus_value.ARRAY_METHOD_COPY_RANGE,
		[]zeus_value.Value{arr2, zero, len1, len2},
		zeus_value.VoidType{Span: span},
		[]zeus_value.ValueType{arrayObjType, i32Type, i32Type, i32Type},
		span)

	updateOutputVar(method.callInstr.Output, newArr, arrayObjType)
	method.deleteAllInstrs(builder)
}

// lowerArraySlice lowers arr.slice(start, end) to use factory function
// Generates:
//
//	sliceLen = end - start (clamped)
//	newArr = new T[sliceLen]
//	newArr.copyRange(arr, start, 0, sliceLen)
//	result = newArr
func (p *ArrayMethodLoweringPass) lowerArraySlice(l *Lowerer, method arrayMethodToLower) {
	span := method.callInstr.Span
	builder := l.GetBuilder()
	setInsertionPoint(builder, method.block, method.callInstr)

	arr := method.arrayObj
	startArg := method.args[0]
	endArg := method.args[1]
	arrayClass := getArrayClassFromValue(arr)
	i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: span}
	arrayObjType := zeus_value.NewObjectType(arrayClass)

	// sliceLen = end - start
	sliceLen := builder.BuildBinaryOp(endArg, startArg, InstrTypeSub, span)
	zeus_value.AsVar(sliceLen).ValueType = i32Type

	// Create new array and copy elements
	newArr := createTypedNewObj(builder, arrayClass, []zeus_value.Value{sliceLen}, span)
	zero := zeus_value.NewConstant("0", i32Type, span)

	builder.BuildMethodCall(newArr, zeus_value.ARRAY_METHOD_COPY_RANGE,
		[]zeus_value.Value{arr, startArg, zero, sliceLen},
		zeus_value.VoidType{Span: span},
		[]zeus_value.ValueType{arrayObjType, i32Type, i32Type, i32Type},
		span)

	updateOutputVar(method.callInstr.Output, newArr, arrayObjType)
	method.deleteAllInstrs(builder)
}

// lowerArrayReverse lowers arr.reverse() to use factory function
// Returns a NEW array with elements in reverse order (non-mutating)
// Generates:
//
//	len = arr.length
//	newArr = new T[len]
//	newArr.copyRangeReversed(arr, 0, 0, len)
//	result = newArr
func (p *ArrayMethodLoweringPass) lowerArrayReverse(l *Lowerer, method arrayMethodToLower) {
	span := method.callInstr.Span
	builder := l.GetBuilder()
	setInsertionPoint(builder, method.block, method.callInstr)

	arr := method.arrayObj
	arrayClass := getArrayClassFromValue(arr)
	i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: span}
	arrayObjType := zeus_value.NewObjectType(arrayClass)

	// Load arr.length
	arrLen := builder.BuildLoadProperty(arr, zeus_value.ARRAY_PROPERTY_LENGTH, i32Type, span)

	// Create new array and copy elements in reverse
	newArr := createTypedNewObj(builder, arrayClass, []zeus_value.Value{arrLen}, span)
	zero := zeus_value.NewConstant("0", i32Type, span)

	builder.BuildMethodCall(newArr, zeus_value.ARRAY_METHOD_COPY_RANGE_REVERSE,
		[]zeus_value.Value{arr, zero, zero, arrLen},
		zeus_value.VoidType{Span: span},
		[]zeus_value.ValueType{arrayObjType, i32Type, i32Type, i32Type},
		span)

	updateOutputVar(method.callInstr.Output, newArr, arrayObjType)
	method.deleteAllInstrs(builder)
}

// =============================================================================
// FuncTypePropCallLoweringPass — rewrites CALL_METHOD instructions where the
// "method" is actually a function-type property (e.g. obj.fnProp(args)).
// The type checker marks these by leaving the output type set to the fn return
// type and NOT erroring. This pass converts them to OBJ_PROP_ACCESS + LOAD +
// INDIRECT_FUNC_CALL so that codegen sees the correct pattern.
// =============================================================================

type funcTypePropCallToLower struct {
	instr    *Instr
	input    *MethodCallInstrInput
	block    *BasicBlock
	funcType zeus_value.FunctionType
}

type FuncTypePropCallLoweringPass struct {
	callsToLower []funcTypePropCallToLower
}

func NewFuncTypePropCallLoweringPass() *FuncTypePropCallLoweringPass {
	return &FuncTypePropCallLoweringPass{}
}

func (p *FuncTypePropCallLoweringPass) GetName() string { return "FuncTypePropCallLowering" }

func (p *FuncTypePropCallLoweringPass) HandleInstruction(l *Lowerer, instr *Instr) {
	if instr.Type != InstrTypeMethodCall {
		return
	}
	input := AsMethodCallInstrInput(instr.Input)
	objectType := zeus_value.GetValueType(input.Object)
	objType := zeus_value.AsObjectType(objectType)
	if objType == nil {
		return
	}
	for _, prop := range objType.Class.Properties {
		if prop.Property.Name == input.MethodName {
			if ft, ok := prop.Property.ValueType.(zeus_value.FunctionType); ok {
				p.callsToLower = append(p.callsToLower, funcTypePropCallToLower{
					instr:    instr,
					input:    input,
					block:    l.GetCurrentBlock(),
					funcType: ft,
				})
				return
			}
		}
	}
}

func (p *FuncTypePropCallLoweringPass) Finalize(l *Lowerer) {
	for _, c := range p.callsToLower {
		p.lowerFuncTypePropCall(l, c)
	}
}

func (p *FuncTypePropCallLoweringPass) lowerFuncTypePropCall(l *Lowerer, c funcTypePropCallToLower) {
	builder := l.GetBuilder()
	span := c.instr.Span

	setInsertionPoint(builder, c.block, c.instr)

	// Load the function pointer from the property
	fnPtr := builder.BuildLoadProperty(c.input.Object, c.input.MethodName, c.funcType, span)
	fnPtrVar := zeus_value.AsVar(fnPtr)
	if fnPtrVar != nil {
		fnPtrVar.ValueType = c.funcType
	}

	// Call it indirectly
	result := builder.BuildIndirectFuncCall(fnPtr, c.input.Args, span)
	resultVar := zeus_value.AsVar(result)
	if resultVar != nil {
		resultVar.ValueType = c.funcType.ReturnType
	}

	updateOutputVar(c.instr.Output, result, c.funcType.ReturnType)
	builder.DeleteInstr(c.block, c.instr)
}

// =============================================================================
// FunctorCallLoweringPass — converts INDIRECT_FUNC_CALL on an object with a
// public __call__ method into a CALL_METHOD(token.FUNCTOR_CALL_METHOD_NAME) instruction so codegen
// sees the standard method-call pattern.
// =============================================================================

type functorCallToLower struct {
	instr *Instr
	input *IndirectFuncCallInstrInput
	block *BasicBlock
}

type FunctorCallLoweringPass struct {
	callsToLower []functorCallToLower
}

func NewFunctorCallLoweringPass() *FunctorCallLoweringPass {
	return &FunctorCallLoweringPass{}
}

func (p *FunctorCallLoweringPass) GetName() string { return "FunctorCallLowering" }

func (p *FunctorCallLoweringPass) HandleInstruction(l *Lowerer, instr *Instr) {
	if instr.Type != InstrTypeIndirectFuncCall {
		return
	}
	input := AsIndirectFuncCallInstrInput(instr.Input)
	if input == nil {
		return
	}
	objType := zeus_value.AsObjectType(zeus_value.GetValueType(input.Function))
	if objType == nil {
		return
	}
	for _, method := range objType.Class.Methods {
		if method.Method.SourceName() == token.FUNCTOR_CALL_METHOD_NAME && method.AccessModifier.Type == token.TokenTypePublic {
			p.callsToLower = append(p.callsToLower, functorCallToLower{instr, input, l.GetCurrentBlock()})
			return
		}
	}
}

func (p *FunctorCallLoweringPass) Finalize(l *Lowerer) {
	for _, c := range p.callsToLower {
		p.lowerFunctorCall(l, c)
	}
}

func (p *FunctorCallLoweringPass) lowerFunctorCall(l *Lowerer, c functorCallToLower) {
	builder := l.GetBuilder()
	span := c.instr.Span
	setInsertionPoint(builder, c.block, c.instr)

	objType := zeus_value.AsObjectType(zeus_value.GetValueType(c.input.Function))
	var returnType zeus_value.ValueType
	var paramTypes []zeus_value.ValueType
	for _, method := range objType.Class.Methods {
		if method.Method.SourceName() == token.FUNCTOR_CALL_METHOD_NAME {
			if ft, ok := zeus_value.GetValueType(method.Method).(zeus_value.FunctionType); ok {
				returnType = ft.ReturnType
				paramTypes = ft.ParamTypes
			}
			break
		}
	}

	result := builder.BuildMethodCall(c.input.Function, token.FUNCTOR_CALL_METHOD_NAME, c.input.Args, returnType, paramTypes, span)
	updateOutputVar(c.instr.Output, result, returnType)
	builder.DeleteInstr(c.block, c.instr)
}

// =============================================================================
// FuncPtrNullCheckPass — inserts a runtime null check before every
// INDIRECT_FUNC_CALL. If the function pointer is null at runtime a
// NullPointerException is thrown instead of invoking undefined behaviour.
// =============================================================================

type funcPtrCallToCheck struct {
	instr *Instr
	block *BasicBlock
}

type FuncPtrNullCheckPass struct {
	callsToCheck []funcPtrCallToCheck
}

func NewFuncPtrNullCheckPass() *FuncPtrNullCheckPass {
	return &FuncPtrNullCheckPass{}
}

func (p *FuncPtrNullCheckPass) GetName() string { return "FuncPtrNullCheck" }

func (p *FuncPtrNullCheckPass) HandleInstruction(l *Lowerer, instr *Instr) {
	if instr.Type == InstrTypeIndirectFuncCall && l.GetCurrentBlock() != nil {
		p.callsToCheck = append(p.callsToCheck, funcPtrCallToCheck{instr, l.GetCurrentBlock()})
	}
}

func (p *FuncPtrNullCheckPass) Finalize(l *Lowerer) {
	builder := l.GetBuilder()
	for _, c := range p.callsToCheck {
		p.lowerNullCheck(builder, c.instr, c.block)
	}
}

func (p *FuncPtrNullCheckPass) lowerNullCheck(builder *IRBuilder, callInstr *Instr, block *BasicBlock) {
	span := callInstr.Span
	input := AsIndirectFuncCallInstrInput(callInstr.Input)

	// Split block so that callInstr and everything after it lives in continueBlock
	continueBlock := builder.SplitBlockBefore(block, callInstr)
	if continueBlock == nil {
		return
	}

	// Create a dedicated block for the null throw
	throwBlock := builder.BuildBasicBlock()

	// Original block branches to throwBlock (if null) or continueBlock (if not null)
	block.Successors = []*BasicBlock{throwBlock, continueBlock}

	// Emit null comparison + conditional jump into the original block
	builder.ResetBlockInsertionToEnd(block)
	nullConst := zeus_value.NewConstant(zeus_value.NULL_CONSTANT_VALUE, zeus_value.NullType{Span: span}, span)
	isNull := builder.BuildBinaryOp(input.Function, nullConst, InstrTypeEqEq, span)
	builder.BuildCondJmp(throwBlock, continueBlock, isNull, span)

	// Emit throw into throwBlock
	builder.ResetBlockInsertionToEnd(throwBlock)
	emitNullFuncPtrThrow(builder, span)
}

// emitNullFuncPtrThrow builds IR that creates a NullPointerException and throws it.
func emitNullFuncPtrThrow(builder *IRBuilder, span *token.Span) {
	errorClass := getClassFromSymbolTable(builder, "Error")
	stringClass := getStringClass(builder)
	u8ArrayClass := getU8ArrayClass(builder)
	i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: span}

	nameString := buildLiteralString(builder, "NullPointerException", stringClass, u8ArrayClass, i32Type, span)
	msgString := buildLiteralString(builder, "Cannot call null function pointer", stringClass, u8ArrayClass, i32Type, span)

	errorObj := builder.BuildNewObj(errorClass, []zeus_value.Value{nameString, msgString}, span)
	zeus_value.AsVar(errorObj).ValueType = zeus_value.NewObjectType(errorClass)

	builder.BuildThrow(errorClass.Id, errorObj, "", span)
}

// buildLiteralString creates a Zeus string object from a Go string using IR instructions.
func buildLiteralString(builder *IRBuilder, str string, stringClass *zeus_value.Class, u8ArrayClass *zeus_value.Class, i32Type zeus_value.IntType, span *token.Span) zeus_value.Value {
	strBytes := []byte(str)
	u8Array := builder.BuildNewObj(u8ArrayClass, []zeus_value.Value{
		zeus_value.NewConstant(fmt.Sprintf("%d", len(strBytes)), i32Type, span),
	}, span)
	zeus_value.AsVar(u8Array).ValueType = zeus_value.NewObjectType(u8ArrayClass)

	for i, b := range strBytes {
		idx := zeus_value.NewConstant(fmt.Sprintf("%d", i), i32Type, span)
		byteVal := zeus_value.NewConstant(fmt.Sprintf("%d", b), zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: span}, span)
		builder.BuildMethodCall(u8Array, zeus_value.ARRAY_METHOD_SET, []zeus_value.Value{idx, byteVal}, nil, nil, span)
	}

	strObj := builder.BuildNewObj(stringClass, []zeus_value.Value{u8Array}, span)
	zeus_value.AsVar(strObj).ValueType = zeus_value.NewObjectType(stringClass)
	return strObj
}

// =============================================================================
// CastLoweringPass — for *Function → FunctionType casts, generates a wrapper
// functor class and replaces the CAST instruction in-place with NEW_OBJ.
// ObjectType → FunctionType casts (both are ptr addrspace(1)) are no-ops handled
// by genCast in codegen.
// =============================================================================

type funcCastInfo struct {
	instr  *Instr
	fn     *zeus_value.Function
	fnType zeus_value.FunctionType
}

type CastLoweringPass struct {
	castsToLower   []funcCastInfo
	wrappersSeen   map[string]bool
	wrapperClasses map[string]*zeus_value.Class
}

func NewCastLoweringPass() *CastLoweringPass {
	return &CastLoweringPass{
		wrappersSeen:   make(map[string]bool),
		wrapperClasses: make(map[string]*zeus_value.Class),
	}
}

func (p *CastLoweringPass) GetName() string { return "CastLowering" }

func (p *CastLoweringPass) HandleInstruction(l *Lowerer, instr *Instr) {
	if instr.Type != InstrTypeCast {
		return
	}
	input := AsCastInstrInput(instr.Input)
	asFunc := zeus_value.AsFunction(input.Value)
	if asFunc == nil {
		return
	}
	fnType := zeus_value.AsFunctionType(input.CastType)
	if fnType == nil {
		return
	}
	p.castsToLower = append(p.castsToLower, funcCastInfo{
		instr:  instr,
		fn:     asFunc,
		fnType: *fnType,
	})
}

func (p *CastLoweringPass) Finalize(l *Lowerer) {
	builder := l.GetBuilder()
	for _, c := range p.castsToLower {
		p.generateFuncWrapper(builder, c)
	}
}

func (p *CastLoweringPass) generateFuncWrapper(builder *IRBuilder, c funcCastInfo) {
	wrapperClassName := c.fn.SourceName() + "Functor"

	var wrapperClass *zeus_value.Class
	if !p.wrappersSeen[wrapperClassName] {
		p.wrappersSeen[wrapperClassName] = true
		wrapperClass = p.createFuncWrapperClass(builder, c.fn, c.fnType, wrapperClassName, c.instr.Span)
		p.wrapperClasses[wrapperClassName] = wrapperClass
	} else {
		wrapperClass = p.wrapperClasses[wrapperClassName]
	}

	// Replace the CAST instruction in-place with NEW_OBJ
	c.instr.Type = InstrTypeNewObj
	c.instr.Input = NewNewObjInstrInput(wrapperClass, []zeus_value.Value{})
	c.instr.Output.ValueType = zeus_value.NewObjectType(wrapperClass)
}

func (p *CastLoweringPass) createFuncWrapperClass(builder *IRBuilder, fn *zeus_value.Function, fnType zeus_value.FunctionType, className string, span *token.Span) *zeus_value.Class {
	constructorStub := zeus_value.NewFunction(token.CONSTRUCTOR_METHOD_NAME, []*zeus_value.Var{}, zeus_value.VoidType{Span: span}, span)

	callParams := make([]*zeus_value.Var, len(fnType.ParamTypes))
	for i, pt := range fnType.ParamTypes {
		callParams[i] = zeus_value.NewVar(fmt.Sprintf("p%d", i), pt, false, span)
	}
	callStub := zeus_value.NewFunction(token.FUNCTOR_CALL_METHOD_NAME, callParams, fnType.ReturnType, span)

	wrapperClass := zeus_value.NewClass(
		className,
		[]*zeus_value.ClassProperty{},
		[]*zeus_value.ClassMethod{
			zeus_value.NewClassMethod(constructorStub, &token.Token{Type: token.TokenTypePublic, Span: span}),
			zeus_value.NewClassMethod(callStub, &token.Token{Type: token.TokenTypePublic, Span: span}),
		},
		nil, "", nil, span,
	)
	builder.EmitClassDeclAtStart(wrapperClass)

	saved := builder.SaveInsertionPoint()
	builder.ResetToGlobalEnd()

	// Constructor: empty body
	constructorBlock := builder.BuildBasicBlock()
	constructorFn := builder.BuildFuncDecl(className+"_constructor", []*VarDecl{}, constructorBlock, zeus_value.VoidType{Span: span}, wrapperClass, span)
	constructorFn.OriginalName = token.CONSTRUCTOR_METHOD_NAME
	constructorStub.Name = constructorFn.Name
	constructorStub.OriginalName = token.CONSTRUCTOR_METHOD_NAME
	builder.SetInsertionBlock(constructorBlock)
	builder.BuildReturn(nil, span)

	// Reset to top-level insertion so the __call__ DECL_CLASS_METHOD lands in b.instrs,
	// not inside constructorBlock (which would prevent Walk from seeing its body).
	builder.ResetToGlobalEnd()

	// __call__: delegates to the original function
	callParamDecls := make([]*VarDecl, len(fnType.ParamTypes))
	for i, pt := range fnType.ParamTypes {
		callParamDecls[i] = NewVarDecl(fmt.Sprintf("p%d", i), pt, false, nil, span)
	}
	callBlock := builder.BuildBasicBlock()
	callFn := builder.BuildFuncDecl(className+"___call__", callParamDecls, callBlock, fnType.ReturnType, wrapperClass, span)
	callFn.OriginalName = token.FUNCTOR_CALL_METHOD_NAME
	callStub.Name = callFn.Name
	callStub.OriginalName = token.FUNCTOR_CALL_METHOD_NAME
	builder.SetInsertionBlock(callBlock)

	callArgs := make([]zeus_value.Value, len(callFn.Params))
	for i, param := range callFn.Params {
		callArgs[i] = param
	}
	if zeus_value.IsVoidType(fnType.ReturnType) {
		builder.BuildCallFunc(fn, callArgs, span)
		builder.BuildReturn(nil, span)
	} else {
		callResult := builder.BuildCallFunc(fn, callArgs, span)
		builder.BuildReturn(callResult, span)
	}

	builder.RestoreInsertionPoint(saved)

	return wrapperClass
}

// =============================================================================
// FactoryLoweringPass — synthesizes each user class's zeus_new_<Class> factory as
// real IR (ALLOC_OBJ + optional constructor call + RETURN), so object-construction
// policy lives in the IR rather than being re-derived in codegen. Primordial and
// array factories keep their codegen-synthesized bodies (genFactoryFunctionBody).
// =============================================================================

type FactoryLoweringPass struct {
	classes []*zeus_value.Class
	seen    map[string]bool
	// ctorEmitted holds classes that have an actually-emitted constructor (a DECL_CLASS_METHOD
	// with a body). Some synthesized classes — e.g. ref cells — carry a no-op constructor
	// descriptor with no emitted body; the factory must not try to call one.
	ctorEmitted map[string]bool
}

func NewFactoryLoweringPass() *FactoryLoweringPass {
	return &FactoryLoweringPass{seen: make(map[string]bool), ctorEmitted: make(map[string]bool)}
}

func (p *FactoryLoweringPass) GetName() string { return "FactoryLowering" }

func (p *FactoryLoweringPass) HandleInstruction(l *Lowerer, instr *Instr) {
	switch instr.Type {
	case InstrTypeDeclClassMethod:
		// Record classes whose constructor is actually compiled, mirroring codegen's old
		// "getLLVMConstructorMethod != nil" guard so the factory only calls a real constructor.
		in := AsDeclClassMethodInstrInput(instr.Input)
		if in.Method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
			p.ctorEmitted[in.Class.Name] = true
		}
	case InstrTypeDeclClass:
		class := AsDeclClassInstrInput(instr.Input).Class
		// Primordials (string, Error) and arrays keep their codegen-synthesized factory for now;
		// they are runtime-called by name and have finicky internals (a follow-up PR moves them here).
		if class.PrimordialName != "" || p.seen[class.Name] {
			return
		}
		p.seen[class.Name] = true
		p.classes = append(p.classes, class)
	}
}

func (p *FactoryLoweringPass) Finalize(l *Lowerer) {
	builder := l.GetBuilder()
	for _, class := range p.classes {
		p.synthesizeFactory(builder, class)
	}
}

// synthesizeFactory emits, at top level, a standalone function
//
//	zeus_new_<Class>(p0, p1, …):
//	    %obj = ALLOC_OBJ <Class>                    // GC-zeroed alloc + header; fields default to zero
//	    SUPER_CONSTRUCTOR_CALL(<ctorClass>, %obj, [p0, p1, …])   // only if the class has an effective ctor
//	    return %obj
//
// It reuses SUPER_CONSTRUCTOR_CALL (the exact "call class C's constructor on an object with args"
// mechanism used by super(...)) so no new constructor-call codegen is needed. Positioning mirrors
// createFuncWrapperClass.
func (p *FactoryLoweringPass) synthesizeFactory(builder *IRBuilder, class *zeus_value.Class) {
	span := class.Span
	layout := class.Layout()
	factoryName := util.GetFactoryFunctionName(class.Name)

	// Factory params mirror the effective constructor's params (none when the class has no ctor).
	var paramDecls []*VarDecl
	if layout.Constructor != nil {
		paramDecls = make([]*VarDecl, len(layout.Constructor.Params))
		for i, param := range layout.Constructor.Params {
			paramDecls[i] = NewVarDecl(fmt.Sprintf("p%d", i), param.ValueType, false, nil, span)
		}
	}

	saved := builder.SaveInsertionPoint()
	builder.ResetToGlobalEnd()

	body := builder.BuildBasicBlock()
	factoryFn := builder.BuildFuncDecl(factoryName, paramDecls, body, zeus_value.NewObjectType(class), nil, span)
	builder.SetInsertionBlock(body)

	obj := builder.BuildAllocObj(class, span)
	// Call the effective constructor only when it is actually compiled. A class may carry a no-op
	// constructor descriptor with no emitted body (e.g. ref cells); calling it would reference a
	// non-existent function, so we skip it — matching the old codegen factory's leniency.
	if layout.Constructor != nil && layout.ConstructorClass != nil && p.ctorEmitted[layout.ConstructorClass.Name] {
		args := make([]zeus_value.Value, len(factoryFn.Params))
		for i, param := range factoryFn.Params {
			args[i] = param
		}
		builder.BuildSuperConstructorCall(layout.ConstructorClass, obj, args, span)
	}
	builder.BuildReturn(obj, span)

	builder.RestoreInsertionPoint(saved)
}
