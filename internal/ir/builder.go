package ir

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"

	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

type IRBuilder struct {
	instrs                  []*Instr
	currentBlock            *BasicBlock
	insertionIndex          int
	blockIdInsetionIndexMap map[int]int
	blocks                  []*BasicBlock
	tempVarCount            int
	blocksCount             int
	symbolTable             *symbol_table.SymbolTable[zeus_value.Value]
	instrIdCount            int
	usedFuncIRNames         map[string]bool
}

func NewIRBuilder() *IRBuilder {
	symbol_table := symbol_table.NewSymbolTable[zeus_value.Value]()
	symbol_table.EnterScope()

	builder := &IRBuilder{
		currentBlock:            nil,
		tempVarCount:            0,
		blocksCount:             0,
		insertionIndex:          0,
		instrIdCount:            0,
		blockIdInsetionIndexMap: make(map[int]int),
		symbolTable:             symbol_table,
		usedFuncIRNames:         make(map[string]bool),
	}

	// Register all primordial classes from the registry upfront
	// This ensures DECL_CLASS instructions are always at the beginning
	builder.initializePrimordials()

	return builder
}

// initializePrimordials registers all primordial classes and functions from the registry.
// This is called once during IRBuilder creation to ensure all primordials are declared
// at the start of the instruction list.
func (b *IRBuilder) initializePrimordials() {
	registry := zeus_value.Registry

	// Emit DECL_CLASS for all registered primordial classes
	for _, class := range registry.GetAllClasses() {
		b.emitPrimordialClassDecl(class)
	}

	// Register primordial functions in symbol table (DECL_PRIMORDIAL_FUNC is emitted in Generate)
	for _, fn := range registry.GetAllFunctions() {
		b.symbolTable.DeclareGlobalSymbol(fn.Name, fn)
	}
}

// emitPrimordialClassDecl emits a DECL_CLASS instruction for a primordial class
// and registers it in the symbol table
func (b *IRBuilder) emitPrimordialClassDecl(class *zeus_value.Class) {
	// Register in symbol table
	b.symbolTable.DeclareGlobalSymbol(class.Name, class)

	// Emit DECL_CLASS instruction
	result := b.createTempVariable(class.GetSpan())
	b.pushInstr(&Instr{
		Type:   InstrTypeDeclClass,
		Output: result,
		Input:  NewDeclClassInstrInput(class),
		Span:   class.GetSpan(),
	})
}

// EmitClassDeclAtStart emits a DECL_CLASS instruction at the start of the instruction list.
// This is used for dynamically discovered array classes that need to be declared before use.
func (b *IRBuilder) EmitClassDeclAtStart(class *zeus_value.Class) {
	// Save current state
	savedBlock := b.currentBlock
	savedIndex := b.insertionIndex

	// Switch to global insertion at the start
	b.currentBlock = nil
	b.insertionIndex = 0

	// Emit DECL_CLASS
	result := b.createTempVariable(class.GetSpan())
	b.pushInstr(&Instr{
		Type:   InstrTypeDeclClass,
		Output: result,
		Input:  NewDeclClassInstrInput(class),
		Span:   class.GetSpan(),
	})

	// Restore state (account for inserted instruction)
	b.currentBlock = savedBlock
	b.insertionIndex = savedIndex + 1
}

// generateUniqueGlobalName returns a name not yet present in the global registry.
// Used to generate unique IR-level names for globally visible symbols (functions, classes).
func (b *IRBuilder) generateUniqueGlobalName(name string) string {
	unique_name := name
	count := 1

	for {
		if _, ok := b.symbolTable.GetSymbol(unique_name); !ok {
			break
		}
		unique_name = name + strconv.Itoa(count)
		count++
	}

	return unique_name
}

// generateUniqueFuncIRName returns a function IR name that is unique across the entire module.
// Unlike generateUniqueGlobalName, it also checks usedFuncIRNames so names from already-exited
// scopes are still avoided (live symbol-table check alone misses them).
func (b *IRBuilder) generateUniqueFuncIRName(name string) string {
	unique := name
	for count := 1; ; count++ {
		_, inScope := b.symbolTable.GetSymbol(unique)
		if !inScope && !b.usedFuncIRNames[unique] {
			break
		}
		unique = name + strconv.Itoa(count)
	}
	b.usedFuncIRNames[unique] = true
	return unique
}

func (b *IRBuilder) createTempVariable(span *token.Span) *zeus_value.Var {
	temp_variable_name := zeus_value.TEMP_VARIABLE_PREFIX + strconv.Itoa(b.tempVarCount)
	b.tempVarCount++

	return zeus_value.NewVar(temp_variable_name, nil, false, span)
}

func (b *IRBuilder) GetInsertionBlock() *BasicBlock {
	return b.currentBlock
}

func (b *IRBuilder) pushInstr(instr *Instr) {
	instr.Id = b.instrIdCount
	b.instrIdCount += 1
	if b.currentBlock == nil {
		b.instrs = append(b.instrs[:b.insertionIndex], append([]*Instr{instr}, b.instrs[b.insertionIndex:]...)...)
		b.insertionIndex++
	} else {
		blockInsertionIndex, ok := b.blockIdInsetionIndexMap[b.currentBlock.Id]
		zeus_error.Assert(ok, "block id not found in block id insertion index map")
		b.currentBlock.Instrs = append(b.currentBlock.Instrs[:blockInsertionIndex], append([]*Instr{instr}, b.currentBlock.Instrs[blockInsertionIndex:]...)...)
		b.blockIdInsetionIndexMap[b.currentBlock.Id]++
	}
}

func (b *IRBuilder) BuildSuccessorBlock() *BasicBlock {
	new_block := b.BuildBasicBlock()

	if b.currentBlock != nil {
		b.currentBlock.Successors = append(b.currentBlock.Successors, new_block)
	}

	return new_block
}

func (b *IRBuilder) BuildBasicBlock() *BasicBlock {
	new_block := NewBasicBlock(b.blocksCount)
	b.blockIdInsetionIndexMap[b.blocksCount] = 0
	b.blocks = append(b.blocks, new_block)
	b.blocksCount++

	return new_block
}

func (b *IRBuilder) SetInsertionBlock(block *BasicBlock) {
	b.currentBlock = block
}

func (b *IRBuilder) SetInsertionAfter(instr *Instr) {
	instrIndex := slices.Index(b.instrs, instr)
	zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in instructions list", instr.String()))
	b.insertionIndex = instrIndex + 1
}

func (b *IRBuilder) SetInsertionBefore(instr *Instr) {
	instrIndex := slices.Index(b.instrs, instr)
	zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in instructions list", instr.String()))
	b.insertionIndex = instrIndex
}

// ResetBlockInsertionToEnd sets the builder to append instructions at the end of block.
func (b *IRBuilder) ResetBlockInsertionToEnd(block *BasicBlock) {
	b.currentBlock = block
	b.blockIdInsetionIndexMap[block.Id] = len(block.Instrs)
}

// SplitBlockBefore splits block at instr: instructions from instr onwards move into a
// freshly-created tail block, which inherits block's successor list.
// block's successor list is cleared (caller is responsible for adding the right edges).
// Returns the tail block, or nil if instr is not in block.
func (b *IRBuilder) SplitBlockBefore(block *BasicBlock, instr *Instr) *BasicBlock {
	instrIdx := slices.Index(block.Instrs, instr)
	if instrIdx == -1 {
		return nil
	}

	tailBlock := b.BuildBasicBlock()

	// Move instructions from instrIdx onwards into tailBlock
	tailBlock.Instrs = make([]*Instr, len(block.Instrs)-instrIdx)
	copy(tailBlock.Instrs, block.Instrs[instrIdx:])
	b.blockIdInsetionIndexMap[tailBlock.Id] = len(tailBlock.Instrs)

	// Trim the original block
	block.Instrs = block.Instrs[:instrIdx]
	b.blockIdInsetionIndexMap[block.Id] = instrIdx

	// tailBlock inherits block's successors; caller will set block's new successors
	tailBlock.Successors = block.Successors
	block.Successors = nil

	return tailBlock
}

func (b *IRBuilder) SetBlockInsertionAfter(block *BasicBlock, instr *Instr) {
	instrIndex := slices.Index(block.Instrs, instr)
	zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in block instructions list", instr.String()))
	b.SetInsertionBlock(block)
	b.blockIdInsetionIndexMap[block.Id] = instrIndex + 1
}

func (b *IRBuilder) SetBlockInsertionBefore(block *BasicBlock, instr *Instr) {
	instrIndex := slices.Index(block.Instrs, instr)
	zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in block instructions list", instr.String()))
	b.SetInsertionBlock(block)
	b.blockIdInsetionIndexMap[block.Id] = instrIndex
}

// SetInsertionBeforeInstr positions the builder to insert just before instr.
// If block is non-nil and contains instr, uses block-scoped insertion;
// otherwise falls back to global-list insertion (for module-level instructions).
func (b *IRBuilder) SetInsertionBeforeInstr(block *BasicBlock, instr *Instr) {
	if block != nil {
		if idx := slices.Index(block.Instrs, instr); idx != -1 {
			b.SetInsertionBlock(block)
			b.blockIdInsetionIndexMap[block.Id] = idx
			return
		}
	}
	idx := slices.Index(b.instrs, instr)
	zeus_error.Assert(idx != -1, fmt.Sprintf("instruction %s not found in any known location", instr.String()))
	b.SetInsertionBlock(nil)
	b.insertionIndex = idx
}

// DeleteInstr removes an instruction from the IR.
// If block is nil, the instruction is removed from the global instruction list.
// If block is provided, the instruction is removed from that block's instruction list.
func (b *IRBuilder) DeleteInstr(block *BasicBlock, instr *Instr) {
	if block == nil {
		// Delete from global instructions
		instrIndex := slices.Index(b.instrs, instr)
		zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in global instructions list", instr.String()))
		b.instrs = slices.Delete(b.instrs, instrIndex, instrIndex+1)
		// Adjust insertion index if needed
		if b.insertionIndex > instrIndex {
			b.insertionIndex--
		}
	} else {
		// Delete from block instructions
		instrIndex := slices.Index(block.Instrs, instr)
		zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in block %d instructions list", instr.String(), block.Id))
		block.Instrs = slices.Delete(block.Instrs, instrIndex, instrIndex+1)
		// Adjust block insertion index if needed
		if blockIdx, ok := b.blockIdInsetionIndexMap[block.Id]; ok && blockIdx > instrIndex {
			b.blockIdInsetionIndexMap[block.Id]--
		}
	}
}

func (b *IRBuilder) BuildBinaryOp(left, right zeus_value.Value, op InstrType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   op,
		Output: result,
		Input:  NewBinaryOpInstrInput(left, right),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildExport(modulePath string, value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeExport,
		Input: NewExportInstrInput(modulePath, value),
		Span:  span,
	})
}

func (b *IRBuilder) BuildLoad(addr *zeus_value.Var, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(addr.Span)

	b.pushInstr(&Instr{
		Type:   InstrTypeLoad,
		Output: result,
		Input:  NewLoadInstrInput(addr),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildVarDecl(v *VarDecl) *zeus_value.Var {
	unique_name := b.generateUniqueGlobalName(v.Name)

	variable := zeus_value.NewVar(unique_name, v.ValueType, true, v.Span)
	variable.OriginalName = v.Name
	variable.IsConst = v.IsConst

	b.symbolTable.DeclareSymbol(unique_name, variable)

	b.pushInstr(&Instr{
		Type:  InstrTypeDeclVar,
		Input: NewDeclareVarInstrInput(variable, v.Initializer, v.IsConst),
		Span:  v.Span,
	})

	return variable
}

func (b *IRBuilder) BuildGlobalVarDecl(v *VarDecl) *zeus_value.Var {
	unique_name := b.generateUniqueGlobalName(v.Name)

	variable := zeus_value.NewVar(unique_name, v.ValueType, true, v.Span)
	variable.OriginalName = v.Name
	variable.IsConst = v.IsConst

	b.symbolTable.DeclareSymbol(unique_name, variable)

	b.pushInstr(&Instr{
		Type:  InstrTypeDeclGlobalVar,
		Input: NewDeclareVarInstrInput(variable, v.Initializer, v.IsConst),
		Span:  v.Span,
	})

	return variable
}

func (b *IRBuilder) BuildStore(addr *zeus_value.Var, value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeStore,
		Input: NewStoreInstrInput(addr, value),
		Span:  span,
	})
}

func (b *IRBuilder) BuildCast(value zeus_value.Value, castType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeCast,
		Output: result,
		Input:  NewCastInstrInput(value, castType),
		Span:   span,
	})
	result.ValueType = castType

	return result
}

func (b *IRBuilder) BuildCoerce(value zeus_value.Value, targetType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	b.pushInstr(&Instr{
		Type:   InstrTypeCoerce,
		Output: result,
		Input:  NewCoerceInstrInput(value, targetType),
		Span:   span,
	})
	// Output keeps the source ObjectType, not the target FunctionType — this is the key
	// difference from BuildCast: the variable's type becomes the actual functor ObjectType.
	result.ValueType = zeus_value.GetValueType(value)
	return result
}

func (b *IRBuilder) BuildDeclPrimordialFunc(fn *zeus_value.Function, span *token.Span) {
	b.symbolTable.DeclareGlobalSymbol(fn.Name, fn)
	b.pushInstr(&Instr{
		Type:  InstrTypeDeclPrimordialFunc,
		Input: NewDeclPrimordialFuncInstrInput(fn),
		Span:  span,
	})
}

func (b *IRBuilder) BuildFuncDecl(name string, args []*VarDecl, body *BasicBlock, return_type zeus_value.ValueType, class *zeus_value.Class, span *token.Span) *zeus_value.Function {
	var existingStub *zeus_value.Function
	if existing, ok := b.symbolTable.GetSymbolInCurrentScope(name); ok {
		existingStub = zeus_value.AsFunction(existing)
	}

	b.symbolTable.EnterScope()
	params := []*zeus_value.Var{}
	for _, arg := range args {
		variable := zeus_value.NewVar(b.generateUniqueGlobalName(arg.Name), arg.ValueType, false, arg.Span)
		b.symbolTable.DeclareSymbol(arg.Name, variable)

		params = append(params, variable)
	}

	var fn *zeus_value.Function
	if existingStub != nil {
		// Update stub in place so forward-call references remain valid
		existingStub.Params = params
		existingStub.ReturnType = return_type
		fn = existingStub
		b.usedFuncIRNames[fn.Name] = true
	} else {
		irName := b.generateUniqueFuncIRName(name)
		fn = zeus_value.NewFunction(irName, params, return_type, span)
		if irName != name {
			fn.OriginalName = name
		}
	}

	b.symbolTable.ExitScope()
	// Register under the original source name so call-site lookups (GetSymbol(name))
	// resolve correctly regardless of the IR-level unique name.
	if class == nil {
		b.symbolTable.DeclareSymbol(name, fn)
	}

	if class != nil {
		b.pushInstr(&Instr{
			Type:  InstrTypeDeclClassMethod,
			Input: NewDeclClassMethodInstrInput(fn, body, class),
			Span:  span,
		})
	} else {
		b.pushInstr(&Instr{
			Type:  InstrTypeDeclFunc,
			Input: NewDeclFuncInstrInput(fn, body),
			Span:  span,
		})
	}

	return fn
}

func (b *IRBuilder) BuildJmp(target *BasicBlock, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeJmp,
		Input: NewJmpInstrInput(target),
		Span:  span,
	})
}

func (b *IRBuilder) BuildCondJmp(true_target *BasicBlock, false_target *BasicBlock, condition zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeCondJmp,
		Input: NewCondJmpInstrInput(true_target, false_target, condition),
		Span:  span,
	})
}

func (b *IRBuilder) BuildCallFunc(callee *zeus_value.Function, args []zeus_value.Value, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeCallFunc,
		Output: result,
		Input:  NewCallFuncInstrInput(callee, args),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildIndirectFuncCall(callee zeus_value.Value, args []zeus_value.Value, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeIndirectFuncCall,
		Output: result,
		Input:  NewIndirectFuncCallInstrInput(callee, args),
	})

	return result
}

func (b *IRBuilder) BuildReturn(value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeReturn,
		Input: NewReturnInstrInput(value),
		Span:  span,
	})
}

func (b *IRBuilder) BuildUnaryOp(value zeus_value.Value, op InstrType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   op,
		Output: result,
		Input:  NewUnaryOpInstrInput(value),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildImport(modulePath string, name string, importedValue zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeImport,
		Input: NewImportInstrInput(modulePath, importedValue),
		Span:  span,
	})

	b.symbolTable.DeclareGlobalSymbol(name, importedValue)
}

func (b *IRBuilder) BuildNewObj(callee zeus_value.Value, args []zeus_value.Value, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeNewObj,
		Output: result,
		Input:  NewNewObjInstrInput(callee, args),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildGetIndex(array zeus_value.Value, indices []zeus_value.Value, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeGetIndex,
		Output: result,
		Input:  NewGetIndexInstrInput(array, indices),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildSetIndex(array, index, value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeSetIndex,
		Input: NewSetIndexInstrInput(array, index, value),
		Span:  span,
	})
}

func (b *IRBuilder) BuildClassMethodDecl(method *zeus_value.Function, body *BasicBlock, class *zeus_value.Class, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeDeclClassMethod,
		Input: NewDeclClassMethodInstrInput(method, body, class),
		Span:  span,
	})
}

func (b *IRBuilder) BuildClassDecl(class *zeus_value.Class, span *token.Span) string {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeDeclClass,
		Output: result,
		Input:  NewDeclClassInstrInput(class),
		Span:   span,
	})

	return class.Name
}

func (b *IRBuilder) Walk(fnInstr func(instr *Instr), fnBlock func(block *BasicBlock)) {
	worklist := []*BasicBlock{}
	// avoid rewalking the same instruction when new instructions are pushed
	// on top of existing instructions
	visitedInstrs := map[int]bool{}
	i := 0

	for i < len(b.instrs) {
		instr := b.instrs[i]
		_, isVisited := visitedInstrs[instr.Id]
		if isVisited {
			i++
			continue
		}
		visitedInstrs[instr.Id] = true
		fnInstr(instr)

		if IsFunctionDeclInstr(instr.Type) {
			worklist = append(worklist, AsDeclFuncInstrInput(instr.Input).Body)
		}

		if IsClassMethodDeclInstr(instr.Type) {
			worklist = append(worklist, AsDeclClassMethodInstrInput(instr.Input).Body)
		}

		for len(worklist) > 0 {
			block := worklist[0]
			worklist = worklist[1:]
			fnBlock(block)
			j := 0
			// walk the instructions in the block
			for j < len(block.Instrs) {
				instr := block.Instrs[j]
				_, isVisited := visitedInstrs[instr.Id]
				if isVisited {
					j++
					continue
				}
				visitedInstrs[instr.Id] = true
				fnInstr(instr)
				j++
			}
			worklist = append(worklist, block.Successors...)
		}
		i++
	}
}

func (b *IRBuilder) deleteBlock(block *BasicBlock) {
	blockIndex := slices.Index(b.blocks, block)
	if blockIndex == -1 {
		return
	}
	b.blocks = slices.Delete(b.blocks, blockIndex, blockIndex+1)

	// remove this block as successor in other blocks
	for _, otherBlock := range b.blocks {
		otherBlock.Successors = slices.DeleteFunc(otherBlock.Successors, func(successor *BasicBlock) bool {
			return successor.Id == block.Id
		})
	}
}

func (b *IRBuilder) deleteDeadCode(block *BasicBlock) {
	conctrolFlowInstrIndex := slices.IndexFunc(block.Instrs, func(instr *Instr) bool {
		return IsControlFlowInstr(instr.Type)
	})

	// delete all instructions after the control flow instruction
	if conctrolFlowInstrIndex != -1 {
		block.Instrs = slices.Delete(block.Instrs, conctrolFlowInstrIndex+1, len(block.Instrs))
	}
}

func (b *IRBuilder) GetBranchingBlocks(block *BasicBlock) []*BasicBlock {
	branchingBlocks := []*BasicBlock{}

	for _, instr := range block.Instrs {
		switch instr.Type {
		case InstrTypeJmp:
			branchingBlocks = append(branchingBlocks, AsJmpInstrInput(instr.Input).Target)
		case InstrTypeCondJmp:
			branchingBlocks = append(branchingBlocks, AsCondJmpInstrInput(instr.Input).TrueTarget, AsCondJmpInstrInput(instr.Input).FalseTarget)
		case InstrTypePushHandler:
			// PUSH_HANDLER branches to both try body and handler block
			input := AsPushHandlerInstrInput(instr.Input)
			branchingBlocks = append(branchingBlocks, input.TryBodyBlock, input.HandlerBlock)
		case InstrTypeCheckException:
			// CHECK_EXCEPTION branches to handler or continue block
			input := AsCheckExceptionInstrInput(instr.Input)
			branchingBlocks = append(branchingBlocks, input.HandlerBlock, input.ContinueBlock)
		}
	}

	return branchingBlocks
}

func (b *IRBuilder) optimizeBlocks(blocks []*BasicBlock) {
	optimizedBlocks := map[*BasicBlock]bool{}
	var visitAndOptimize func(block *BasicBlock)

	// we delete the dead code in the block and then visit the branching blocks
	visitAndOptimize = func(block *BasicBlock) {
		_, isOptimized := optimizedBlocks[block]
		if isOptimized {
			return
		}

		b.deleteDeadCode(block)

		optimizedBlocks[block] = true
		branchingBlocks := b.GetBranchingBlocks(block)
		for _, branchingBlock := range branchingBlocks {
			visitAndOptimize(branchingBlock)
		}
	}

	for _, block := range blocks {
		visitAndOptimize(block)
	}

	// delete unreachable blocks
	for _, block := range b.blocks {
		_, isOptimized := optimizedBlocks[block]
		if !isOptimized {
			b.deleteBlock(block)
		}
	}
}

func (b *IRBuilder) BuildObjectPropertyAccess(object zeus_value.Value, property string, isLValue bool, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	result.IsPtr = true

	b.pushInstr(&Instr{
		Type:   InstrTypeObjectPropertyAccess,
		Output: result,
		Input:  NewObjectPropertyAccessInstrInput(object, property, isLValue),
	})

	return result
}

// BuildLoadProperty loads a property value from an object.
// This is a convenience method that combines BuildObjectPropertyAccess and BuildLoad,
// setting the appropriate types on the intermediate variables.
func (b *IRBuilder) BuildLoadProperty(object zeus_value.Value, propertyName string, propertyType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	// Get property pointer
	propPtr := b.BuildObjectPropertyAccess(object, propertyName, false, span)
	propPtrVar := zeus_value.AsVar(propPtr)
	propPtrVar.ValueType = propertyType

	// Load the property value
	propValue := b.BuildLoad(propPtrVar, span)
	propValueVar := zeus_value.AsVar(propValue)
	propValueVar.ValueType = propertyType

	return propValue
}

// BuildMethodCall calls a method on an object, emitting a single CALL_METHOD instruction.
// returnType is set on the result when non-nil (lowering-pass callers supply it;
// TC pass infers it for IR-gen callers where nil is passed).
func (b *IRBuilder) BuildMethodCall(object zeus_value.Value, methodName string, args []zeus_value.Value, returnType zeus_value.ValueType, argTypes []zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	if returnType != nil {
		result.ValueType = returnType
	}

	b.pushInstr(&Instr{
		Type:   InstrTypeMethodCall,
		Output: result,
		Input:  NewMethodCallInstrInput(object, methodName, args),
		Span:   span,
	})

	return result
}

// BuildThrow builds a THROW instruction
func (b *IRBuilder) BuildThrow(classId int, objectPtr zeus_value.Value, sourceFile string, span *token.Span) {
	sourceLine := 0
	if span != nil {
		sourceLine = span.Start.Line
	}
	b.pushInstr(&Instr{
		Type:  InstrTypeThrow,
		Input: NewThrowInstrInput(classId, objectPtr, sourceFile, sourceLine),
		Span:  span,
	})
}

// BuildPushHandler builds a PUSH_HANDLER instruction to register an exception handler
func (b *IRBuilder) BuildPushHandler(handlerBlock *BasicBlock, tryBodyBlock *BasicBlock, classIds []int, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypePushHandler,
		Input: NewPushHandlerInstrInput(handlerBlock, tryBodyBlock, classIds),
		Span:  span,
	})
}

// BuildPopHandler builds a POP_HANDLER instruction to unregister an exception handler
func (b *IRBuilder) BuildPopHandler(span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypePopHandler,
		Span: span,
	})
}

// BuildCheckException builds a CHECK_EXCEPTION instruction that branches based on exception state
func (b *IRBuilder) BuildCheckException(handlerBlock *BasicBlock, continueBlock *BasicBlock, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeCheckException,
		Input: NewCheckExceptionInstrInput(handlerBlock, continueBlock),
		Span:  span,
	})
}

// BuildGetException builds a GET_EXCEPTION instruction to retrieve the current exception
func (b *IRBuilder) BuildGetException(expectedType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	result.ValueType = expectedType
	b.pushInstr(&Instr{
		Type:   InstrTypeGetException,
		Output: result,
		Span:   span,
	})
	return result
}

// BuildClearException builds a CLEAR_EXCEPTION instruction to clear the current exception state
func (b *IRBuilder) BuildClearException(span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeClearException,
		Span: span,
	})
}

func (b *IRBuilder) GetFunctionBlocks() []*BasicBlock {
	functionBlocks := []*BasicBlock{}

	for _, instr := range b.instrs {
		if IsFunctionDeclInstr(instr.Type) {
			functionBlocks = append(functionBlocks, AsDeclFuncInstrInput(instr.Input).Body)
		}

		if IsClassMethodDeclInstr(instr.Type) {
			functionBlocks = append(functionBlocks, AsDeclClassMethodInstrInput(instr.Input).Body)
		}
	}

	return functionBlocks
}

func (b *IRBuilder) Optimize() {
	b.optimizeBlocks(b.GetFunctionBlocks())
}

func (b *IRBuilder) String() string {
	output := []string{}

	b.Walk(func(instr *Instr) {
		output = append(output, instr.String())
	}, func(block *BasicBlock) {
		output = append(output, fmt.Sprintf("%d:", block.Id))
	})

	return strings.Join(output, "\n")
}

func (b *IRBuilder) Print() {
	fmt.Println(b.String())
}

func (b *IRBuilder) GetInstrs() []*Instr {
	return b.instrs
}
