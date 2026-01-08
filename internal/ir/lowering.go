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
	l.passes = []LowerPass{
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

	// Get classes from symbol table
	u8ArrayClass := getU8ArrayClass(builder)
	stringClass := getStringClass(builder)
	u8ArrayType := zeus_value.NewObjectType(*u8ArrayClass)
	stringType := zeus_value.NewObjectType(*stringClass)

	// Create copy method function type: (source: u8[]) -> void
	copyMethodType := zeus_value.NewFunctionType(
		zeus_value.VoidType{Span: span},
		[]zeus_value.ValueType{u8ArrayType},
	)

	// Set builder to insert before the CAST instruction in the correct block
	if block != nil {
		builder.SetBlockInsertionBefore(block, castInstr)
	} else {
		builder.SetInsertionBefore(castInstr)
	}

	// Get source array length: source.length
	lengthPtr := builder.BuildObjectPropertyAccess(sourceArray, zeus_value.ARRAY_PROPERTY_LENGTH, false, span)
	lengthPtrVar := zeus_value.AsVar(lengthPtr)
	lengthPtrVar.ValueType = zeus_value.IntType{Size: zeus_value.I32, Span: span}
	sourceLength := builder.BuildLoad(lengthPtrVar, span)
	sourceLengthVar := zeus_value.AsVar(sourceLength)
	sourceLengthVar.ValueType = zeus_value.IntType{Size: zeus_value.I32, Span: span}

	// Create new u8[] with capacity = source length
	newArray := builder.BuildNewObj(u8ArrayClass, []zeus_value.Value{sourceLength}, span)
	newArrayVar := zeus_value.AsVar(newArray)
	newArrayVar.ValueType = u8ArrayType

	// Call copy on new array: newArray.copy(source)
	copyMethodPtr := builder.BuildObjectPropertyAccess(newArray, zeus_value.ARRAY_METHOD_COPY, false, span)
	copyMethodPtrVar := zeus_value.AsVar(copyMethodPtr)
	copyMethodPtrVar.ValueType = copyMethodType
	copyMethod := builder.BuildLoad(copyMethodPtrVar, span)
	copyMethodVar := zeus_value.AsVar(copyMethod)
	copyMethodVar.ValueType = copyMethodType
	builder.BuildIndirectFuncCall(copyMethod, []zeus_value.Value{sourceArray}, span)

	// Create new string with the copied array
	result := builder.BuildNewObj(stringClass, []zeus_value.Value{newArray}, span)
	resultVar := zeus_value.AsVar(result)
	resultVar.ValueType = stringType

	// Update the output variable name to reference the result
	// The CAST instruction remains but codegen will look up the output by name,
	// which now matches the string created by lowering
	if output != nil {
		output.Name = resultVar.Name
		output.ValueType = stringType
	}
}

// lowerStringToU8ArrayCast lowers a string -> u8[] cast
// Generates: get data, get length, new u8[], copy
func (p *StringCastLoweringPass) lowerStringToU8ArrayCast(l *Lowerer, castInstr *Instr, input *CastInstrInput, block *BasicBlock) {
	sourceString := input.Value
	span := castInstr.Span
	output := castInstr.Output
	builder := l.GetBuilder()

	// Get classes from symbol table
	u8ArrayClass := getU8ArrayClass(builder)
	u8ArrayType := zeus_value.NewObjectType(*u8ArrayClass)

	// Create copy method function type: (source: u8[]) -> void
	copyMethodType := zeus_value.NewFunctionType(
		zeus_value.VoidType{Span: span},
		[]zeus_value.ValueType{u8ArrayType},
	)

	// Set builder to insert before the CAST instruction in the correct block
	if block != nil {
		builder.SetBlockInsertionBefore(block, castInstr)
	} else {
		builder.SetInsertionBefore(castInstr)
	}

	// Access string.data to get the source u8[]
	dataPtr := builder.BuildObjectPropertyAccess(sourceString, zeus_value.STRING_PROPERTY_DATA, false, span)
	dataPtrVar := zeus_value.AsVar(dataPtr)
	dataPtrVar.ValueType = u8ArrayType
	sourceData := builder.BuildLoad(dataPtrVar, span)
	sourceDataVar := zeus_value.AsVar(sourceData)
	sourceDataVar.ValueType = u8ArrayType

	// Get source data length: sourceData.length
	lengthPtr := builder.BuildObjectPropertyAccess(sourceData, zeus_value.ARRAY_PROPERTY_LENGTH, false, span)
	lengthPtrVar := zeus_value.AsVar(lengthPtr)
	lengthPtrVar.ValueType = zeus_value.IntType{Size: zeus_value.I32, Span: span}
	sourceLength := builder.BuildLoad(lengthPtrVar, span)
	sourceLengthVar := zeus_value.AsVar(sourceLength)
	sourceLengthVar.ValueType = zeus_value.IntType{Size: zeus_value.I32, Span: span}

	// Create new u8[] with capacity = source length
	newArray := builder.BuildNewObj(u8ArrayClass, []zeus_value.Value{sourceLength}, span)
	newArrayVar := zeus_value.AsVar(newArray)
	newArrayVar.ValueType = u8ArrayType

	// Call copy on new array: newArray.copy(sourceData)
	copyMethodPtr := builder.BuildObjectPropertyAccess(newArray, zeus_value.ARRAY_METHOD_COPY, false, span)
	copyMethodPtrVar := zeus_value.AsVar(copyMethodPtr)
	copyMethodPtrVar.ValueType = copyMethodType
	copyMethod := builder.BuildLoad(copyMethodPtrVar, span)
	copyMethodVar := zeus_value.AsVar(copyMethod)
	copyMethodVar.ValueType = copyMethodType
	builder.BuildIndirectFuncCall(copyMethod, []zeus_value.Value{sourceData}, span)

	// Update the output variable name to reference the new array
	// The CAST instruction remains but codegen will look up the output by name,
	// which now matches the new array created by lowering
	if output != nil {
		output.Name = newArrayVar.Name
		output.ValueType = u8ArrayType
	}
}

// =============================================================================
// Helper functions for lowering passes
// =============================================================================

// getU8ArrayClass returns the u8[] class from symbol table
// This class must exist by the time lowering runs (registered during type checking)
func getU8ArrayClass(builder *IRBuilder) *zeus_value.Class {
	u8ArrayClassName := "u8[]"

	existingClass, ok := builder.symbolTable.GetSymbol(u8ArrayClassName)
	zeus_error.Assert(ok, "u8[] class not found in symbol table - lowering requires type checking to run first")

	class := zeus_value.AsClass(existingClass)
	zeus_error.Assert(class != nil, "u8[] symbol is not a class")

	return class
}

// getStringClass returns the string class from symbol table
// This class must exist by the time lowering runs (registered during IRBuilder initialization)
func getStringClass(builder *IRBuilder) *zeus_value.Class {
	stringClassName := zeus_value.ZEUS_PRIMORDIAL_STRING

	existingClass, ok := builder.symbolTable.GetSymbol(stringClassName)
	zeus_error.Assert(ok, "string class not found in symbol table - lowering requires type checking to run first")

	class := zeus_value.AsClass(existingClass)
	zeus_error.Assert(class != nil, "string symbol is not a class")

	return class
}
