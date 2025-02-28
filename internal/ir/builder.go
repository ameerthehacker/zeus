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
	instrs      []*Instr
	currentBlock *BasicBlock
	insertionIndex int
	blockIdInsetionIndexMap map[int]int
	blocks []*BasicBlock
	tempVarCount int
	blocksCount int
	symbolTable *symbol_table.SymbolTable[zeus_value.Value]
}

func NewIRBuilder() *IRBuilder {
	symbol_table := symbol_table.NewSymbolTable[zeus_value.Value]()
	symbol_table.EnterScope()

	return &IRBuilder{
		currentBlock: nil,
		tempVarCount: 0,
		blocksCount: 0,
		insertionIndex: 0,
		blockIdInsetionIndexMap: make(map[int]int),
		symbolTable: symbol_table,
	}
}

func (b *IRBuilder) generateUniqueSymbolName(name string) string {
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

func (b *IRBuilder) createTempVariable(span *token.Span) *zeus_value.Var {
	temp_variable_name := zeus_value.TEMP_VARIABLE_PREFIX + strconv.Itoa(b.tempVarCount)
	b.tempVarCount++

	return zeus_value.NewVar(temp_variable_name, nil, false, span)
}

func (b *IRBuilder) GetInsertionBlock() *BasicBlock {
	return b.currentBlock
}

func (b *IRBuilder) pushInstr(instr *Instr) {
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

func (b* IRBuilder) SetBlockInsertionAfter(block *BasicBlock, instr *Instr) {
	instrIndex := slices.Index(block.Instrs, instr)
	zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in block instructions list", instr.String()))
	b.SetInsertionBlock(block)
	b.blockIdInsetionIndexMap[block.Id] = instrIndex + 1
}

func (b* IRBuilder) SetBlockInsertionBefore(block *BasicBlock, instr *Instr) {
	instrIndex := slices.Index(block.Instrs, instr)
	zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in block instructions list", instr.String()))
	b.SetInsertionBlock(block)
	b.blockIdInsetionIndexMap[block.Id] = instrIndex
}

func (b *IRBuilder) BuildBinaryOp(left, right zeus_value.Value, op InstrType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type: op,
		Output: result,
		Input: NewBinaryOpInstrInput(left, right),
		Span: span,
	})

	return result
}

func (b *IRBuilder) BuildExport(value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeExport,
		Input: NewExportInstrInput(value),
		Span: span,
	})
}

func (b *IRBuilder) BuildLoad(addr *zeus_value.Var, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(addr.Span)

	b.pushInstr(&Instr{
		Type: InstrTypeLoad,
		Output: result,
		Input: NewLoadInstrInput(addr),
		Span: span,
	})

	return result
}

func (b *IRBuilder) BuildVarDecl(v *VarDecl) *zeus_value.Var {
	unique_name := b.generateUniqueSymbolName(v.Name)

	variable := zeus_value.NewVar(unique_name, v.ValueType, true, v.Span)

	b.symbolTable.DeclareSymbol(unique_name, variable)

	b.pushInstr(&Instr{
		Type: InstrTypeDeclVar,
		Input: NewDeclareVarInstrInput(variable, v.Initializer, v.IsConst),
		Span: v.Span,
	})

	return variable
}

func (b *IRBuilder) BuildStore(addr *zeus_value.Var, value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeStore,
		Input: NewStoreInstrInput(addr, value),
		Span: span,
	})
}

func (b *IRBuilder) BuildCast(value zeus_value.Value, castType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type: InstrTypeCast,
		Output: result,
		Input: NewCastInstrInput(value, castType),
		Span: span,
	})
	result.ValueType = castType

	return result
}

func (b *IRBuilder) BuildFuncDecl(name string, args []*VarDecl, body *BasicBlock, return_type zeus_value.ValueType, span *token.Span) *zeus_value.Function {
	b.symbolTable.EnterScope()
	params := []*zeus_value.Var{}
	for _, arg := range args {
		variable := zeus_value.NewVar(b.generateUniqueSymbolName(arg.Name), arg.ValueType, false, arg.Span)
		b.symbolTable.DeclareSymbol(arg.Name, variable)

		params = append(params, variable)
	}

	fn := zeus_value.NewFunction(
		name,
		params,
		return_type,
		span,
	)
	// functions are global
	b.symbolTable.DeclareGlobalSymbol(fn.Name, fn)

	b.pushInstr(&Instr{
		Type: InstrTypeDeclFunc,
		Input: NewDeclFuncInstrInput(fn, body),
		Span: span,
	})

	b.symbolTable.ExitScope()

	return fn
}

func (b *IRBuilder) BuildJmp(target *BasicBlock, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeJmp,
		Input: NewJmpInstrInput(target),
		Span: span,
	})
}

func (b *IRBuilder) BuildCondJmp(true_target *BasicBlock, false_target *BasicBlock, condition zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeCondJmp,
		Input: NewCondJmpInstrInput(true_target, false_target, condition),
		Span: span,
	})
}

func (b *IRBuilder) BuildCallFunc(callee *zeus_value.Function, args []zeus_value.Value, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type: InstrTypeCallFunc,
		Output: result,
		Input: NewCallFuncInstrInput(callee, args),
		Span: span,
	})

	return result
}

func (b *IRBuilder) BuildReturn(value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeReturn,
		Input: NewReturnInstrInput(value),
		Span: span,
	})
}

func (b *IRBuilder) BuildUnaryOp(value zeus_value.Value, op InstrType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type: op,
		Output: result,
		Input: NewUnaryOpInstrInput(value),
		Span: span,
	})

	return result
}

func (b *IRBuilder) BuildImport(importedValue zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeImport,
		Input: NewImportInstrInput(importedValue),
		Span: span,
	})
}

func (b *IRBuilder) Walk(fnInstr func(instr *Instr), fnBlock func(block *BasicBlock)) {
	worklist := []*BasicBlock{}
	i := 0

	for i < len(b.instrs) {
		instr := b.instrs[i]
		fnInstr(instr)

		if IsFunctionDeclInstr(instr.Type) {
			worklist = append(worklist, AsDeclFuncInstrInput(instr.Input).Body)
		}

		for len(worklist) > 0 {
			block := worklist[0]
			worklist = worklist[1:]
			fnBlock(block)
			j := 0
			// walk the instructions in the block
			for j < len(block.Instrs) {
				instr := block.Instrs[j]
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
	zeus_error.Assert(blockIndex != -1, "block not found in blocks list")
	b.blocks = slices.Delete(b.blocks, blockIndex, blockIndex + 1)

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
		block.Instrs = slices.Delete(block.Instrs, conctrolFlowInstrIndex + 1, len(block.Instrs))
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

func (b *IRBuilder) GetFunctionBlocks() []*BasicBlock {
	functionBlocks := []*BasicBlock{}

	for _, instr := range b.instrs {
		if IsFunctionDeclInstr(instr.Type) {
			functionBlocks = append(functionBlocks, AsDeclFuncInstrInput(instr.Input).Body)
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

func (b* IRBuilder) GetInstrs() []*Instr {
	return b.instrs
}
