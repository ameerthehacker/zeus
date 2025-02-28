package codegen

import (
	"fmt"
	"strconv"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
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
	basicBlocks map[int]llvm.BasicBlock
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

	return &CodegenModule{ module, builder, c.ctx, symbol_table.NewSymbolTable[llvm.Value](), make(map[int]llvm.BasicBlock) }
}

func (c *CodegenModule) getSymbol(name string) llvm.Value {
	symbol, ok := c.symbolTable.GetSymbol(name)
	if !ok {
		panic(fmt.Sprintf("symbol %s not found in symbol table", name))
	}
	return symbol
}

func (c *CodegenModule) toLLVMValue(value zeus_value.Value) llvm.Value {
	switch value := value.(type) {
	case *zeus_value.Constant:
		return ToLLVMConstant(*value)
	case *zeus_value.Var:
		return c.getSymbol(value.Name)
	case *zeus_value.Function:
		return c.getSymbol(value.Name)
	default:
		panic(fmt.Sprintf("unable to convert zeus value %s to llvm value", value))
	}
}

func (c *CodegenModule) getOrCreateBasicBlock(id int, parent llvm.Value) llvm.BasicBlock {
	if basicBlock, ok := c.basicBlocks[id]; ok {
		return basicBlock
	}
	basicBlock := c.ctx.AddBasicBlock(parent, strconv.Itoa(id))
	c.basicBlocks[id] = basicBlock
	return basicBlock
}

func (c *CodegenModule) genDeclFunc(input ir.DeclFuncInstrInput) llvm.Value {
	llvmFunc := llvm.AddFunction(c.module, input.Function.Name, ToLLVMFunctionType(zeus_value.ToFunctionType(*input.Function)))
	funcParams := input.Function.Params

	for index, param := range llvmFunc.Params() {
		c.symbolTable.DeclareSymbol(funcParams[index].Name, param)
	}

	c.symbolTable.DeclareGlobalSymbol(input.Function.Name, llvmFunc)

	return llvmFunc
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

func (c *CodegenModule) genLoad(input ir.LoadInstrInput, output zeus_value.Var) {
	addr, ok := c.symbolTable.GetSymbol(input.Addr.Name)
	if !ok {
		panic(fmt.Sprintf("symbol %s not found in symbol table", input.Addr.Name))
	}
	llvmValue := c.builder.CreateLoad(ToLLVMType(input.Addr.ValueType), addr, input.Addr.Name)
	c.symbolTable.DeclareSymbol(output.Name, llvmValue)
}

func (c *CodegenModule) genCallFunc(input ir.CallFuncInstrInput, output zeus_value.Var) {
	function := c.toLLVMValue(input.Callee)
	functionType := ToLLVMType(zeus_value.GetValueType(input.Callee))
	args := make([]llvm.Value, len(input.Args))
	for i, arg := range input.Args {
		args[i] = c.toLLVMValue(arg)
	}

	llvmValue := c.builder.CreateCall(functionType, function, args, fmt.Sprintf("%s_call_result", function.Name()))
	c.symbolTable.DeclareSymbol(output.Name, llvmValue)
}

func (c *CodegenModule) genJmp(input ir.JmpInstrInput) {
	basicBlock := c.getOrCreateBasicBlock(input.Target.Id, c.builder.GetInsertBlock().Parent())
	c.builder.CreateBr(basicBlock)
}

func (c *CodegenModule) genCondJmp(input ir.CondJmpInstrInput) {
	trueBlock := c.getOrCreateBasicBlock(input.TrueTarget.Id, c.builder.GetInsertBlock().Parent())
	falseBlock := c.getOrCreateBasicBlock(input.FalseTarget.Id, c.builder.GetInsertBlock().Parent())
	c.builder.CreateCondBr(c.toLLVMValue(input.Condition), trueBlock, falseBlock)
}

type BinaryOpLLVMFunc func (llvm.Value, llvm.Value, string) llvm.Value

func (c *CodegenModule) genLLVMBinaryOp(left zeus_value.Value, right zeus_value.Value, opName string, intIntOp BinaryOpLLVMFunc, uIntuIntOp BinaryOpLLVMFunc, floatFloat BinaryOpLLVMFunc) llvm.Value {
	leftType := zeus_value.GetValueType(left)
	rightType := zeus_value.GetValueType(right)

	switch leftType := leftType.(type) {
		case zeus_value.IntType:
			switch rightType := rightType.(type) {
				case zeus_value.IntType:
					if !leftType.Signed && !rightType.Signed {
						return uIntuIntOp(c.toLLVMValue(left), c.toLLVMValue(right), opName)
					}
					return intIntOp(c.toLLVMValue(left), c.toLLVMValue(right), opName)
			}
		case zeus_value.FloatType:
			switch rightType.(type) {
				case zeus_value.FloatType:
					return floatFloat(c.toLLVMValue(left), c.toLLVMValue(right), opName)
			}
	}

	panic(fmt.Sprintf("invalid types %s and %s for binary operation %s", leftType, rightType, opName))
}

func (c *CodegenModule) genBinaryOp(instr *ir.Instr, input ir.BinaryOpInstrInput, output zeus_value.Var) {
	var result llvm.Value

	switch instr.Type {
		case ir.InstrTypeAdd:
			result = c.genLLVMBinaryOp(input.Left, input.Right, "add", c.builder.CreateAdd, c.builder.CreateAdd, c.builder.CreateFAdd)
		case ir.InstrTypeSub:
			result = c.genLLVMBinaryOp(input.Left, input.Right, "sub", c.builder.CreateSub, c.builder.CreateSub, c.builder.CreateFSub)
		case ir.InstrTypeMul:
			result = c.genLLVMBinaryOp(input.Left, input.Right, "mul", c.builder.CreateMul, c.builder.CreateMul, c.builder.CreateFMul)
		case ir.InstrTypeDiv:
			result = c.genLLVMBinaryOp(input.Left, input.Right, "div", c.builder.CreateSDiv, c.builder.CreateUDiv, c.builder.CreateFDiv)
		case ir.InstrTypeEqEq:
			result = c.genLLVMBinaryOp(input.Left, input.Right, "eq", func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntEQ), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntEQ), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateFCmp(llvm.FloatPredicate(llvm.FloatOEQ), left, right, opName)
			})
		case ir.InstrTypeNotEq:
			result = c.genLLVMBinaryOp(input.Left, input.Right, "notEq", func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntNE), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntNE), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateFCmp(llvm.FloatPredicate(llvm.FloatONE), left, right, opName)
			})
		case ir.InstrTypeLessThan:
			result = c.genLLVMBinaryOp(input.Left, input.Right, "lessThan", func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntSLT), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntULT), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateFCmp(llvm.FloatPredicate(llvm.FloatOLT), left, right, opName)
			})
		case ir.InstrTypeGreaterThan:
			result = c.genLLVMBinaryOp(input.Left, input.Right, "greaterThan", func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntSGT), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntUGT), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateFCmp(llvm.FloatPredicate(llvm.FloatOGT), left, right, opName)
			})
		case ir.InstrTypeLessThanEq:
			result = c.genLLVMBinaryOp(input.Left, input.Right, "lessThanEq", func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntSLE), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntULE), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateFCmp(llvm.FloatPredicate(llvm.FloatOLE), left, right, opName)
			})
		case ir.InstrTypeGreaterThanEq:
			result = c.genLLVMBinaryOp(input.Left, input.Right, "greaterThanEq", func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntSGE), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateICmp(llvm.IntPredicate(llvm.IntUGE), left, right, opName)
			}, func(left llvm.Value, right llvm.Value, opName string) llvm.Value {
				return c.builder.CreateFCmp(llvm.FloatPredicate(llvm.FloatOGE), left, right, opName)
			})
	}

	c.symbolTable.DeclareSymbol(output.Name, result)
}

func (c *CodegenModule) genExport(input ir.ExportInstrInput) {
	llvmValue := c.toLLVMValue(input.Value)
	llvmValue.SetLinkage(llvm.ExternalLinkage)
}

func (c *CodegenModule) genCast(input ir.CastInstrInput, output zeus_value.Var) {
	var result llvm.Value
	valueType := zeus_value.GetValueType(input.Value)
	castErrorMsg := fmt.Sprintf("cannot cast %s to %s", input.Value, input.CastType)

	switch valueType := valueType.(type) {
		case zeus_value.IntType:
			switch castType := input.CastType.(type) {
				case zeus_value.FloatType:
					if valueType.Signed {
						result = c.builder.CreateSIToFP(c.toLLVMValue(input.Value), ToLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
					} else {
						result = c.builder.CreateUIToFP(c.toLLVMValue(input.Value), ToLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
					}
				case zeus_value.IntType:
					if castType.Signed {
						result = c.builder.CreateSExt(c.toLLVMValue(input.Value), ToLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
					} else {
						result = c.builder.CreateZExt(c.toLLVMValue(input.Value), ToLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
					}
				default:
					panic(castErrorMsg)
			}
		case zeus_value.FloatType:
			result = c.builder.CreateFPExt(c.toLLVMValue(input.Value), ToLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
		default:
			panic(castErrorMsg)
	}

	c.symbolTable.DeclareSymbol(output.Name, result)
}

func (c *CodegenModule) Generate(irBuilder ir.IRBuilder) {
	var currentFunction llvm.Value

	c.symbolTable.EnterScope()

	irBuilder.Walk(func(instr *ir.Instr) {
		switch instr.Type {
		case ir.InstrTypeDeclFunc:
			// maintain function level scoping
			if !c.symbolTable.IsGlobalScope() {
				c.symbolTable.ExitScope()
			} else {
				c.symbolTable.EnterScope()
			}
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
		case ir.InstrTypeJmp:
			c.genJmp(*ir.AsJmpInstrInput(instr.Input))
		case ir.InstrTypeCondJmp:
			c.genCondJmp(*ir.AsCondJmpInstrInput(instr.Input))
		case ir.InstrTypeAdd:
			fallthrough
		case ir.InstrTypeSub:
			fallthrough
		case ir.InstrTypeMul:
			fallthrough
		case ir.InstrTypeDiv:
			fallthrough
		case ir.InstrTypeEqEq:
			fallthrough
		case ir.InstrTypeNotEq:
			fallthrough
		case ir.InstrTypeLessThan:
			fallthrough
		case ir.InstrTypeGreaterThan:
			fallthrough
		case ir.InstrTypeLessThanEq:
			fallthrough
		case ir.InstrTypeGreaterThanEq:
			c.genBinaryOp(instr, *ir.AsBinaryOpInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeCast:
			c.genCast(*ir.AsCastInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeExport:
			c.genExport(*ir.AsExportInstrInput(instr.Input))
		default:
			panic(fmt.Sprintf("codegen for instruction %s is not implemented", instr.Type))
		}
	}, func(block *ir.BasicBlock) {
		basicBlock := c.getOrCreateBasicBlock(block.Id, currentFunction)
		c.builder.SetInsertPointAtEnd(basicBlock)
	})

	c.symbolTable.ExitScope()
}

func (c *CodegenModule) GetModule() llvm.Module {
	return c.module
}

func (c *CodegenModule) Dump() {
	c.module.Dump()
}

func (c *CodegenModule) String() string {
	return c.module.String()
}
