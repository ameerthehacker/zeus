package codegen

import (
	"fmt"
	"strconv"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/value"
	"tinygo.org/x/go-llvm"
)

type Codegen struct {
	ctx llvm.Context
}

type CodegenModule struct {
	module llvm.Module
	builder llvm.Builder
	ctx llvm.Context
	symbolTable *symbol_table.SymbolTable[llvm.Value]
}

func NewCodegen() *Codegen {
	llvm.InitializeAllTargets()
	llvm.InitializeAllTargetMCs()
	llvm.InitializeAllTargetInfos()
	llvm.InitializeAllAsmParsers()
	llvm.InitializeAllAsmPrinters()
	ctx := llvm.NewContext()

	return &Codegen{ ctx }
}

func (c *Codegen) NewModule(name string) *CodegenModule {
	module := c.ctx.NewModule(name)
	builder := c.ctx.NewBuilder()

	return &CodegenModule{ module, builder, c.ctx, symbol_table.NewSymbolTable[llvm.Value]() }
}

func (c *CodegenModule) toLLVMValue(_value value.Value) llvm.Value {
	switch _value := _value.(type) {
	case *value.Constant:
		return ToLLVMConstant(*_value)
	case *value.Var:
		llvmValue, ok := c.symbolTable.GetSymbol(_value.Name)
		if !ok {
			panic(fmt.Sprintf("symbol %s not found in symbol table", _value.Name))
		}
		return llvmValue
	default:
		panic(fmt.Sprintf("unable to convert zeus value %s to llvm value", _value))
	}
}

func (c *CodegenModule) genDeclFunc(input ir.DeclFuncInstrInput) llvm.Value {
	return llvm.AddFunction(c.module, input.Function.Name, ToLLVMFunctionType(value.ToFunctionType(input.Function)))
}

func (c *CodegenModule) genReturn(input ir.ReturnInstrInput) {
	if input.Value != nil {
		c.builder.CreateRet(c.toLLVMValue(input.Value))
	} else {
		c.builder.CreateRetVoid()
	}
}

func (c* CodegenModule) genDeclVar(input ir.DeclareVarInstrInput) {
	variableType := ToLLVMType(input.Variable.ValueType)
	variable := c.builder.CreateAlloca(variableType, input.Variable.Name)

	if input.Initializer != nil {
		c.builder.CreateStore(c.toLLVMValue(input.Initializer), variable)
	}

	c.symbolTable.DeclareSymbol(input.Variable.Name, variable)
}

func (c *CodegenModule) genStore(input ir.StoreInstrInput) {
	addr, ok := c.symbolTable.GetSymbol(input.Addr.Name)
	if !ok {
		panic(fmt.Sprintf("symbol %s not found in symbol table", input.Addr.Name))
	}
	c.builder.CreateStore(c.toLLVMValue(input.Value), addr)
}

func (c *CodegenModule) genLoad(input ir.LoadInstrInput, output value.Var) {
	addr, ok := c.symbolTable.GetSymbol(input.Addr.Name)
	if !ok {
		panic(fmt.Sprintf("symbol %s not found in symbol table", input.Addr.Name))
	}
	llvmValue := c.builder.CreateLoad(ToLLVMType(input.Addr.ValueType), addr, input.Addr.Name)
	c.symbolTable.DeclareSymbol(output.Name, llvmValue)
}

func (c *CodegenModule) genCallFunc(input ir.CallFuncInstrInput, output value.Var) {
	function := c.toLLVMValue(input.Callee)
	args := make([]llvm.Value, len(input.Args))
	for i, arg := range input.Args {
		args[i] = c.toLLVMValue(arg)
	}

	llvmValue := c.builder.CreateCall(function.Type(), function, args, function.Name())
	c.symbolTable.DeclareSymbol(output.Name, llvmValue)
}

func (c *CodegenModule) Generate(irBuilder ir.IRBuilder) {
	var currentFunction llvm.Value

	c.symbolTable.EnterScope()

	irBuilder.Walk(func(instr *ir.Instr) {
		switch instr.Type {
		case ir.InstrTypeDeclFunc:
			currentFunction = c.genDeclFunc(*ir.AsDeclFuncInstrInput(instr.Input))
		case ir.InstrTypeDeclVar:
			c.genDeclVar(*ir.AsDeclVarInstrInput(instr.Input))
		case ir.InstrTypeStore:
			c.genStore(*ir.AsStoreInstrInput(instr.Input))
		case ir.InstrTypeReturn:
			c.genReturn(*ir.AsReturnInstrInput(instr.Input))
		case ir.InstrTypeLoad:
			c.genLoad(*ir.AsLoadInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeCallFunc:
			c.genCallFunc(*ir.AsCallFuncInstrInput(instr.Input), *instr.Output)
		}
	}, func(block *ir.BasicBlock) {
		basicBlock := c.ctx.AddBasicBlock(currentFunction, strconv.Itoa(block.Id))
		c.builder.SetInsertPointAtEnd(basicBlock)
	})

	c.symbolTable.ExitScope()
}

func (c *CodegenModule) Dump() {
	c.module.Dump()
}

func (c *CodegenModule) String() string {
	return c.module.String()
}
