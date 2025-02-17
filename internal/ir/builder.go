package ir

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/value"
)

type IRBuilder struct {
	instrs      []*Instr
	current_block *BasicBlock
	temp_var_count int
	blocks_count int
	symbol_table *symbol_table.SymbolTable[value.Value]
}

func NewIRBuilder() *IRBuilder {
	symbol_table := symbol_table.NewSymbolTable[value.Value]()
	symbol_table.EnterScope()

	return &IRBuilder{
		current_block: nil,
		temp_var_count: 0,
		blocks_count: 0,
		symbol_table: symbol_table,
	}
}

func (b *IRBuilder) generateUniqueSymbolName(name string) string {
	unique_name := name
	count := 1

	for {
		if _, ok := b.symbol_table.GetSymbol(unique_name); !ok {
			break
		}
		unique_name = name + strconv.Itoa(count)
		count++
	}

	return unique_name
}

func (b *IRBuilder) createTempVariable() *value.Var {
	temp_variable_name := value.TEMP_VARIABLE_PREFIX + strconv.Itoa(b.temp_var_count)
	b.temp_var_count++

	return &value.Var{
		Name: temp_variable_name,
		ValueType: nil,
	}
}

func (b *IRBuilder) GetInsertionBlock() *BasicBlock {
	return b.current_block
}

func (b *IRBuilder) pushInstr(instr *Instr) {
	if b.current_block == nil {
		b.instrs = append(b.instrs, instr)
	} else {
		b.current_block.Instrs = append(b.current_block.Instrs, instr)
	}
}

func (b *IRBuilder) BuildSuccessorBlock() *BasicBlock {
	new_block := b.BuildBasicBlock()

	if b.current_block != nil {
		b.current_block.Successors = append(b.current_block.Successors, new_block)
	}

	return new_block
}

func (b *IRBuilder) BuildBasicBlock() *BasicBlock {
	new_block := NewBasicBlock(b.blocks_count)
	b.blocks_count++

	return new_block
}

func (b *IRBuilder) SetInsertionBlock(block *BasicBlock) {
	b.current_block = block
}

func (b *IRBuilder) BuildBinaryOp(left, right value.Value, op InstrType, span *token.Span) value.Value {
	result := b.createTempVariable()

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

func (b *IRBuilder) BuildLoad(addr *value.Var, span *token.Span) value.Value {
	result := b.createTempVariable()

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

func (b *IRBuilder) BuildVarDecl(v *VarDecl) *value.Var {
	unique_name := b.generateUniqueSymbolName(v.Name)

	variable := &value.Var{
		Name: unique_name,
		ValueType: v.ValueType,
		Span: v.Span,
	}

	b.symbol_table.DeclareSymbol(unique_name, variable)

	b.pushInstr(&Instr{
		Type: InstrTypeDeclVar,
		Input: DeclareVarInstrInput{
			Variable: variable,
			Initializer: v.Initializer,
		},
		Span: v.Span,
	})

	return variable
}

func (b *IRBuilder) BuildStore(addr *value.Var, value value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeStore,
		Input: StoreInstrInput{
			Addr: addr,
			Value: value,
		},
		Span: span,
	})
}

func (b *IRBuilder) BuildFuncDecl(name string, args []VarDecl, body *BasicBlock, return_type value.ValueType, span *token.Span) *value.Function {
	b.symbol_table.EnterScope()
	params := []*value.Var{}
	for _, arg := range args {
		variable := &value.Var{
			Name: b.generateUniqueSymbolName(arg.Name),
			ValueType: arg.ValueType,
			Span: arg.Span,
		}
		b.symbol_table.DeclareSymbol(variable.Name, variable)

		params = append(params, variable)
	}

	fn := &value.Function{
		Name:  b.generateUniqueSymbolName(name),
		ReturnType: return_type,
		Params: params,
		Span: span,
	}
	// functions are global
	b.symbol_table.DeclareGlobalSymbol(fn.Name, fn)

	b.pushInstr(&Instr{
		Type: InstrTypeDeclFunc,
		Input: DeclFuncInstrInput{
			Name: fn.Name,
			Args: args,
			ReturnType: return_type,
			Body: body,
		},
		Span: span,
	})

	b.symbol_table.ExitScope()

	return fn
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

func (b *IRBuilder) BuildCondJmp(true_target *BasicBlock, false_target *BasicBlock, condition value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeCondJmp,
		Input: CondJmpInstrInput{
			TrueTarget: true_target,
			FalseTarget: false_target,
			Condition: condition,
		},
		Span: span,
	})
}

func (b *IRBuilder) BuildCallFunc(callee *value.Function, args []value.Value, span *token.Span) value.Value {
	result := b.createTempVariable()

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

func (b *IRBuilder) BuildReturn(value value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeReturn,
		Input: ReturnInstrInput{
			Value: value,
		},
		Span: span,
	})
}

func (b *IRBuilder) BuildUnaryOp(value value.Value, op InstrType, span *token.Span) value.Value {
	result := b.createTempVariable()

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

func (b *IRBuilder) Walk(fnInstr func(instr *Instr), fnBlock func(block *BasicBlock)) {
	worklist := []*BasicBlock{}

	for _, instr := range b.instrs {
		fnInstr(instr)

		switch instr.Type {
		case InstrTypeDeclFunc:
			worklist = append(worklist, AsDeclFuncInstrInput(instr.Input).Body)
		}

		for len(worklist) > 0 {
			block := worklist[0]
			worklist = worklist[1:]
			fnBlock(block)
			// walk the instructions in the block
			for _, instr := range block.Instrs {fnInstr(instr)}
			worklist = append(worklist, block.Successors...)
		}
	}
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
