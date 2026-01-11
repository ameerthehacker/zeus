package ir

import (
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
	l.passes = []LowerPass{
		NewIndexLoweringPass(),
		NewStringOperatorLoweringPass(),
		NewStringCastLoweringPass(),
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
	sourceArray := input.Value
	span := castInstr.Span
	output := castInstr.Output
	builder := l.GetBuilder()

	// Get classes and types
	u8ArrayClass := getU8ArrayClass(builder)
	stringClass := getStringClass(builder)
	u8ArrayType := zeus_value.NewObjectType(*u8ArrayClass)
	stringType := zeus_value.NewObjectType(*stringClass)
	i32Type := zeus_value.IntType{Size: zeus_value.I32, Span: span}

	// Set builder to insert before the CAST instruction
	setInsertionPoint(builder, block, castInstr)

	// Get source array length
	sourceLength := builder.BuildLoadProperty(sourceArray, zeus_value.ARRAY_PROPERTY_LENGTH, i32Type, span)

	// Create new u8[] with capacity = source length
	newArray := builder.BuildNewObj(u8ArrayClass, []zeus_value.Value{sourceLength}, span)
	zeus_value.AsVar(newArray).ValueType = u8ArrayType

	// Call copy on new array: newArray.copy(source)
	builder.BuildMethodCall(newArray, zeus_value.ARRAY_METHOD_COPY,
		[]zeus_value.Value{sourceArray},
		zeus_value.VoidType{Span: span},
		[]zeus_value.ValueType{u8ArrayType},
		span)

	// Create new string with the copied array
	result := builder.BuildNewObj(stringClass, []zeus_value.Value{newArray}, span)
	resultVar := zeus_value.AsVar(result)
	resultVar.ValueType = stringType

	// Update the output variable
	if output != nil {
		output.Name = resultVar.Name
		output.ValueType = stringType
	}

	// Delete the original CAST instruction
	builder.DeleteInstr(block, castInstr)
}

// lowerStringToU8ArrayCast lowers a string -> u8[] cast
// Generates: get data, get length, new u8[], copy
func (p *StringCastLoweringPass) lowerStringToU8ArrayCast(l *Lowerer, castInstr *Instr, input *CastInstrInput, block *BasicBlock) {
	sourceString := input.Value
	span := castInstr.Span
	output := castInstr.Output
	builder := l.GetBuilder()

	// Get classes and types
	u8ArrayClass := getU8ArrayClass(builder)
	u8ArrayType := zeus_value.NewObjectType(*u8ArrayClass)
	i32Type := zeus_value.IntType{Size: zeus_value.I32, Span: span}

	// Set builder to insert before the CAST instruction
	setInsertionPoint(builder, block, castInstr)

	// Access string.data to get the source u8[]
	sourceData := builder.BuildLoadProperty(sourceString, zeus_value.STRING_PROPERTY_DATA, u8ArrayType, span)

	// Get source data length
	sourceLength := builder.BuildLoadProperty(sourceData, zeus_value.ARRAY_PROPERTY_LENGTH, i32Type, span)

	// Create new u8[] with capacity = source length
	newArray := builder.BuildNewObj(u8ArrayClass, []zeus_value.Value{sourceLength}, span)
	newArrayVar := zeus_value.AsVar(newArray)
	newArrayVar.ValueType = u8ArrayType

	// Call copy on new array: newArray.copy(sourceData)
	builder.BuildMethodCall(newArray, zeus_value.ARRAY_METHOD_COPY,
		[]zeus_value.Value{sourceData},
		zeus_value.VoidType{Span: span},
		[]zeus_value.ValueType{u8ArrayType},
		span)

	// Update the output variable
	if output != nil {
		output.Name = newArrayVar.Name
		output.ValueType = u8ArrayType
	}

	// Delete the original CAST instruction
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

// getU8ArrayClass returns the u8[] class from symbol table
// This class must exist by the time lowering runs (registered during type checking)
func getU8ArrayClass(builder *IRBuilder) *zeus_value.Class {
	existingClass, ok := builder.symbolTable.GetSymbol("u8[]")
	zeus_error.Assert(ok, "u8[] class not found in symbol table - lowering requires type checking to run first")

	class := zeus_value.AsClass(existingClass)
	zeus_error.Assert(class != nil, "u8[] symbol is not a class")

	return class
}

// getStringClass returns the string class from symbol table
// This class must exist by the time lowering runs (registered during IRBuilder initialization)
func getStringClass(builder *IRBuilder) *zeus_value.Class {
	existingClass, ok := builder.symbolTable.GetSymbol(zeus_value.ZEUS_PRIMORDIAL_STRING)
	zeus_error.Assert(ok, "string class not found in symbol table - lowering requires type checking to run first")

	class := zeus_value.AsClass(existingClass)
	zeus_error.Assert(class != nil, "string symbol is not a class")

	return class
}

// =============================================================================
// IndexLoweringPass - Lowers GET_INDEX instructions to .get() method calls
// =============================================================================

type indexToLower struct {
	instr *Instr
	input *GetIndexInstrInput
	block *BasicBlock
}

// IndexLoweringPass lowers GET_INDEX instructions to array.get() method calls
type IndexLoweringPass struct {
	indicesToLower []indexToLower
}

func NewIndexLoweringPass() *IndexLoweringPass {
	return &IndexLoweringPass{}
}

func (p *IndexLoweringPass) GetName() string {
	return "IndexLowering"
}

// HandleInstruction collects GET_INDEX instructions that need lowering
func (p *IndexLoweringPass) HandleInstruction(l *Lowerer, instr *Instr) {
	if instr.Type != InstrTypeGetIndex {
		return
	}

	input := AsGetIndexInstrInput(instr.Input)
	if input == nil {
		return
	}

	p.indicesToLower = append(p.indicesToLower, indexToLower{instr, input, l.GetCurrentBlock()})
}

// Finalize performs the actual lowering transformations
func (p *IndexLoweringPass) Finalize(l *Lowerer) {
	for _, indexOp := range p.indicesToLower {
		p.lowerGetIndex(l, indexOp.instr, indexOp.input, indexOp.block)
	}
}

// lowerGetIndex converts GET_INDEX into a sequence of array.get() method calls
// For array[i][j], generates: temp = array.get(i); result = temp.get(j)
// For string[i], generates: temp = string.data; result = temp.get(i)
func (p *IndexLoweringPass) lowerGetIndex(l *Lowerer, instr *Instr, input *GetIndexInstrInput, block *BasicBlock) {
	span := instr.Span
	output := instr.Output
	builder := l.GetBuilder()

	// Set builder to insert before the GET_INDEX instruction
	setInsertionPoint(builder, block, instr)

	currentValue := input.Array
	i32Type := zeus_value.IntType{Size: zeus_value.I32, Span: span}

	// Check if this is string indexing
	targetType := zeus_value.GetValueType(currentValue)
	if objType := zeus_value.AsObjectType(targetType); objType != nil && objType.Class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING {
		// String indexing: first access the .data property to get the u8[]
		zeus_error.Assert(len(input.Indices) == 1, "string indexing should only have one index - type checking should have caught this")

		u8ArrayClass := getU8ArrayClass(builder)
		u8ArrayType := zeus_value.NewObjectType(*u8ArrayClass)
		u8Type := zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: span}

		// Access string.data to get the u8[]
		dataArray := builder.BuildLoadProperty(currentValue, zeus_value.STRING_PROPERTY_DATA, u8ArrayType, span)

		// Call .get(index) on the u8[] array
		result := builder.BuildMethodCall(dataArray, zeus_value.ARRAY_METHOD_GET,
			[]zeus_value.Value{input.Indices[0]},
			u8Type,
			[]zeus_value.ValueType{i32Type},
			span)

		// Update the output variable
		if output != nil {
			resultVar := zeus_value.AsVar(result)
			output.Name = resultVar.Name
			output.ValueType = u8Type
		}

		builder.DeleteInstr(block, instr)
		return
	}

	// Process each index, calling .get() method for each one
	for _, index := range input.Indices {
		arrayType := zeus_value.GetValueType(currentValue)

		objType := zeus_value.AsObjectType(arrayType)
		zeus_error.Assert(objType != nil && objType.Class.ArrayElementType != nil,
			"GET_INDEX lowering requires array type with element type - type checking should have caught this")
		elementType := objType.Class.ArrayElementType

		// Call array.get(index)
		result := builder.BuildMethodCall(currentValue, zeus_value.ARRAY_METHOD_GET,
			[]zeus_value.Value{index},
			elementType,
			[]zeus_value.ValueType{i32Type},
			span)

		// Set the result type - handle nested arrays specially
		resultVar := zeus_value.AsVar(result)
		if arrayElemType, ok := elementType.(zeus_value.ArrayType); ok {
			// For nested arrays, we need to get the array class
			if existingClass, ok := builder.symbolTable.GetSymbol(arrayElemType.String()); ok {
				if class := zeus_value.AsClass(existingClass); class != nil {
					resultVar.ValueType = zeus_value.NewObjectType(*class)
				}
			}
		}

		currentValue = result
	}

	// Update the output variable
	if output != nil && currentValue != nil {
		if resultVar := zeus_value.AsVar(currentValue); resultVar != nil {
			output.Name = resultVar.Name
			output.ValueType = resultVar.ValueType
		}
	}

	builder.DeleteInstr(block, instr)
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
	output := instr.Output
	builder := l.GetBuilder()

	// Set builder to insert before the binary operation instruction
	setInsertionPoint(builder, block, instr)

	stringClass := getStringClass(builder)
	stringType := zeus_value.NewObjectType(*stringClass)

	switch instr.Type {
	case InstrTypeAdd:
		// string + string -> left.concat(right)
		result := builder.BuildMethodCall(input.Left, zeus_value.STRING_METHOD_CONCAT,
			[]zeus_value.Value{input.Right},
			stringType,
			[]zeus_value.ValueType{stringType},
			span)

		if output != nil {
			resultVar := zeus_value.AsVar(result)
			output.Name = resultVar.Name
			output.ValueType = stringType
		}

	case InstrTypeEqEq:
		// string == string -> left.equals(right)
		boolType := zeus_value.BoolType{Span: span}
		result := builder.BuildMethodCall(input.Left, zeus_value.STRING_METHOD_EQUALS,
			[]zeus_value.Value{input.Right},
			boolType,
			[]zeus_value.ValueType{stringType},
			span)

		if output != nil {
			resultVar := zeus_value.AsVar(result)
			output.Name = resultVar.Name
			output.ValueType = boolType
		}

	case InstrTypeNotEq:
		// string != string -> !left.equals(right)
		boolType := zeus_value.BoolType{Span: span}
		equalsResult := builder.BuildMethodCall(input.Left, zeus_value.STRING_METHOD_EQUALS,
			[]zeus_value.Value{input.Right},
			boolType,
			[]zeus_value.ValueType{stringType},
			span)

		// Negate the result
		result := builder.BuildUnaryOp(equalsResult, InstrTypeNot, span)

		if output != nil {
			resultVar := zeus_value.AsVar(result)
			output.Name = resultVar.Name
			output.ValueType = boolType
		}

	case InstrTypeLessThan, InstrTypeGreaterThan, InstrTypeLessThanEq, InstrTypeGreaterThanEq:
		// string comparison -> left.compare(right) <op> 0
		i8Type := zeus_value.IntType{Size: zeus_value.I8, Signed: true, Span: span}
		compareResult := builder.BuildMethodCall(input.Left, zeus_value.STRING_METHOD_COMPARE,
			[]zeus_value.Value{input.Right},
			i8Type,
			[]zeus_value.ValueType{stringType},
			span)

		// Compare with 0 using the appropriate comparison
		zero := zeus_value.NewConstant("0", i8Type, span)

		// Map the original operator to the comparison against 0
		compOp := instr.Type
		result := builder.BuildBinaryOp(compareResult, zero, compOp, span)

		if output != nil {
			resultVar := zeus_value.AsVar(result)
			output.Name = resultVar.Name
			output.ValueType = zeus_value.BoolType{Span: span}
		}
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
