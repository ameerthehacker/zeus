package ir

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ameerthehacker/zeus/internal/constant"
	"github.com/ameerthehacker/zeus/internal/token"
)

type IRBuilder struct {
	instrs      []*Instr
	current_block *BasicBlock
	temp_var_count int
	blocks_count int
}

func NewIRBuilder() *IRBuilder {
	return &IRBuilder{
		current_block: nil,
		temp_var_count: 0,
		blocks_count: 0,
	}
}

func (b *IRBuilder) CreateTempVariable() *TempVariable {
	temp_variable_name := "%" + strconv.Itoa(b.temp_var_count)
	b.temp_var_count++

	return &TempVariable{
		Name: temp_variable_name,
	}
}

func (b *IRBuilder) pushInstr(instr *Instr) {
	if b.current_block == nil {
		b.instrs = append(b.instrs, instr)
	} else {
		b.current_block.Instrs = append(b.current_block.Instrs, instr)
	}
}

func (b *IRBuilder) BuildBasicBlock() *BasicBlock {
	new_block := NewBasicBlock(b.blocks_count)
	b.blocks_count++

	if b.current_block != nil {
		b.current_block.Successors = append(b.current_block.Successors, new_block)
	}

	return new_block
}

func (b *IRBuilder) BuildEnterScope() {
	b.pushInstr(&Instr{
		Type: InstrTypeEnterScope,
	})
}

func (b *IRBuilder) BuildExitScope() {
	b.pushInstr(&Instr{
		Type: InstrTypeExitScope,
	})
}

func (b *IRBuilder) SetInsertionBlock(block *BasicBlock) {
	b.current_block = block
}

func (b *IRBuilder) BuildBinaryOp(left, right constant.Value, op InstrType, span *token.Span) constant.Value {
	result := b.CreateTempVariable()

	b.pushInstr(&Instr{
		Type: op,
		Output: result,
		Input: BinaryOpInstrInput{
			Left: left,
			Right: right,
		},
		Span: span,
	})

	return result
}

func (b *IRBuilder) BuildLoad(addr constant.Value, span *token.Span) constant.Value {
	result := b.CreateTempVariable()

	b.pushInstr(&Instr{
		Type: InstrTypeLoad,
		Output: result,
		Input: LoadInstrInput{
			Addr: addr,
		},
		Span: span,
	})

	return result
}

func (b *IRBuilder) BuildVarDecl(v *VarDecl) constant.Value {
	b.pushInstr(&Instr{
		Type: InstrTypeDeclVar,
		Input: DeclareVarInstrInput{
			Name: v.Name,
			ValueType: v.ValueType,
		},
		Span: v.Span,
	})

	return v
}

func (b *IRBuilder) BuildStore(addr constant.Value, value constant.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeStore,
		Input: StoreInstrInput{
			Addr: addr,
			Value: value,
		},
	})
}

func (b *IRBuilder) BuildFuncDecl(name string, args []VarDecl, body *BasicBlock, return_type constant.ValueType, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeDeclFunc,
		Input: DeclFuncInstrInput{
			Name: name,
			Args: args,
			ReturnType: return_type,
			Body: body,
		},
		Span: span,
	})
}

func (b *IRBuilder) BuildJmp(target *BasicBlock, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeJmp,
		Input: JmpInstrInput{
			Target: target,
		},
		Span: span,
	})
}

func (b *IRBuilder) BuildCondJmp(target *BasicBlock, condition constant.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeCondJmp,
		Input: CondJmpInstrInput{
			Target: target,
			Condition: condition,
		},
	})
}

func (b *IRBuilder) BuildCallFunc(callee constant.Value, args []constant.Value, span *token.Span) constant.Value {
	result := b.CreateTempVariable()

	b.pushInstr(&Instr{
		Type: InstrTypeCallFunc,
		Output: result,
		Input: CallFuncInstrInput{
			Callee: callee,
			Args: args,
		},
		Span: span,
	})

	return result
}

func (b *IRBuilder) BuildReturn(value constant.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeReturn,
		Input: ReturnInstrInput{
			Value: value,
		},
		Span: span,
	})
}

func (b *IRBuilder) BuildUnaryOp(value constant.Value, op InstrType, span *token.Span) constant.Value {
	result := b.CreateTempVariable()

	b.pushInstr(&Instr{
		Type: op,
		Output: result,
		Input: UnaryOpInstrInput{
			Value: value,
		},
		Span: span,
	})

	return result
}

func toString(instrs []*Instr, indent int) string {
	worklist := []*BasicBlock{}
	output := []string{}
	indent_str := strings.Repeat(" ", indent * 2)

	appendBlock := func(block *BasicBlock) {
		output = append(output, indent_str + fmt.Sprintf("%d:", block.Id))
		output = append(output, toString(block.Instrs, indent + 1))
	}

	for _, instr := range instrs {
		output = append(output, indent_str + instr.String())
		switch instr.Type {
		case InstrTypeDeclFunc:
			input := ToDeclFuncInstrInput(instr.Input)
			worklist = append(worklist, input.Body)
		}
	}

	for len(worklist) > 0 {
		block := worklist[0]
		worklist = worklist[1:]
		appendBlock(block)
		worklist = append(worklist, block.Successors...)
	}

	return strings.Join(output, "\n")
}

func (b *IRBuilder) String() string {
	return toString(b.instrs, 0)
}

func (b *IRBuilder) Print() {
	fmt.Println(b.String())
}
