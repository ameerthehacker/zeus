package ir

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

type IRBuilder struct {
	instrs      []*Instr
	blocks      []*BasicBlock
	current_block *BasicBlock
}

func NewIRBuilder() *IRBuilder {
	return &IRBuilder{
		blocks:      []*BasicBlock{},
		current_block: nil,
	}
}

func (b* IRBuilder) assertBlockExists() {
	zeus_error.Assert(b.current_block != nil, "current block is nil")
}

func (b *IRBuilder) pushInstr(instr *Instr) {
	if b.current_block == nil {
		b.instrs = append(b.instrs, instr)
	} else {
		b.current_block.Instrs = append(b.current_block.Instrs, instr)
	}
}

func (b *IRBuilder) BuildBasicBlock() *BasicBlock {
	new_block := NewBasicBlock(len(b.blocks), b.current_block)
	b.blocks = append(b.blocks, new_block)

	return new_block
}

func (b *IRBuilder) SetInsertionBlock(block *BasicBlock) {
	b.current_block = block
}

func (b *IRBuilder) EndBasicBlock() {
	zeus_error.Assert(b.current_block.Parent != nil, "current block is nil")
	b.current_block = b.current_block.Parent
}

func (b *IRBuilder) BuildBinaryOp(left, right Value, op InstrType, span *token.Span) Value {
	b.assertBlockExists()
	result := b.current_block.CreateTempVariable()

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

func (b *IRBuilder) BuildLoad(addr string, span *token.Span) Value {
	b.assertBlockExists()
	result := b.current_block.CreateTempVariable()

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

func (b *IRBuilder) BuildVarDecl(v *Variable) Value {
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

func (b *IRBuilder) BuildStore(addr string, value Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeStore,
		Input: StoreInstrInput{
			Addr: addr,
			Value: value,
		},
	})
}

func (b *IRBuilder) BuildFuncDecl(name string, args []Variable, body *BasicBlock, return_type ValueType, span *token.Span) {
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

func (b *IRBuilder) BuildCondJmp(target *BasicBlock, condition Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeCondJmp,
		Input: CondJmpInstrInput{
			Target: target,
			Condition: condition,
		},
	})
}

func (b *IRBuilder) BuildCallFunc(func_name string, args []Value, span *token.Span) Value {
	b.assertBlockExists()
	result := b.current_block.CreateTempVariable()

	b.pushInstr(&Instr{
		Type: InstrTypeCallFunc,
		Output: result,
		Input: CallFuncInstrInput{
			FuncName: func_name,
			Args: args,
		},
		Span: span,
	})

	return result
}

func (b *IRBuilder) BuildReturn(value Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeReturn,
		Input: ReturnInstrInput{
			Value: value,
		},
		Span: span,
	})
}

func (b *IRBuilder) BuildUnaryOp(value Value, op InstrType, span *token.Span) Value {
	b.assertBlockExists()
	result := b.current_block.CreateTempVariable()

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
	output := []string{}
	indent_str := strings.Repeat(" ", indent * 2)

	for _, instr := range instrs {
		output = append(output, indent_str + instr.String())
		switch instr.Type {
		case InstrTypeDeclFunc:
			input := toDeclFuncInstrInput(instr.Input)
			output = append(output, indent_str + fmt.Sprintf("%d:", input.Body.Id))
			output = append(output, toString(input.Body.Instrs, indent + 1))
		case InstrTypeJmp:
			input := toJmpInstrInput(instr.Input)
			output = append(output, indent_str + fmt.Sprintf("%d:", input.Target.Id))
			output = append(output, toString(input.Target.Instrs, indent + 1))
		case InstrTypeCondJmp:
			input := toCondJmpInstrInput(instr.Input)
			output = append(output, indent_str + fmt.Sprintf("%d:", input.Target.Id))
			output = append(output, toString(input.Target.Instrs, indent + 1))
		}
	}

	return strings.Join(output, "\n")
}

func (b *IRBuilder) String() string {
	return toString(b.instrs, 0)
}
