package codegen

import (
	"fmt"
	"strconv"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/module"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/util"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"tinygo.org/x/go-llvm"
)

type Codegen struct {
	ctx llvm.Context
}

type ZeusClassLLVMStruct struct {
	ZeusClass zeus_value.Class
	LLVMStruct llvm.Type
	LLVMVTableStruct llvm.Type
	LLVMVTableMethods []llvm.Value
  LLVMObjHeaderStruct llvm.Type
	CurrentVTableMethodIndex int
}

type ZeusClassModule struct {
	ModulePath string
	Class zeus_value.Class
}

type GlobalLLVMFunction struct {
	Function llvm.Value
	FunctionType llvm.Type
}

func NewZeusClassModule(modulePath string, class zeus_value.Class) ZeusClassModule {
	return ZeusClassModule{modulePath, class}
}

func NewZeusClassLLVMStruct(zeusClass zeus_value.Class, llvmStruct llvm.Type, llvmVTableStruct llvm.Type, llvmVTableMethods []llvm.Value, llvmObjHeaderStruct llvm.Type) *ZeusClassLLVMStruct {
	return &ZeusClassLLVMStruct{zeusClass, llvmStruct, llvmVTableStruct, llvmVTableMethods, llvmObjHeaderStruct, 0}
}

const MemAllocFunctionName = "zeus_gc_alloc"

type CodegenModule struct {
	module          llvm.Module
	builder         llvm.Builder
	ctx             llvm.Context
	symbolTable     *symbol_table.SymbolTable[llvm.Value]
	basicBlocks     map[int]llvm.BasicBlock
	isEntryPoint    bool
	exportedClasses map[string]ZeusClassModule
	importedClasses map[string]ZeusClassModule
	zeusClassLLVMStructMap map[string]*ZeusClassLLVMStruct
	targetDataLayout llvm.TargetData
	globalLLVMFunctions map[string]GlobalLLVMFunction
}

func NewCodegen() *Codegen {
	ctx := llvm.NewContext()

	return &Codegen{ctx}
}

func (c *Codegen) NewModule(name string,isEntryPoint bool, targetDataLayout llvm.TargetData) *CodegenModule {
	module := c.ctx.NewModule(name)
	builder := c.ctx.NewBuilder()
	globalLLVMFunctions := c.setupGlobalLLVMFunctions(module)

	return &CodegenModule{module, builder, c.ctx, symbol_table.NewSymbolTable[llvm.Value](), make(map[int]llvm.BasicBlock), isEntryPoint, make(map[string]ZeusClassModule), make(map[string]ZeusClassModule), make(map[string]*ZeusClassLLVMStruct), targetDataLayout, globalLLVMFunctions}
}

func (c *Codegen) setupGlobalLLVMFunctions(module llvm.Module) map[string]GlobalLLVMFunction {
	globalFunctions := make(map[string]GlobalLLVMFunction)
	
	// Memory allocation function
	memAllocFunctionType := llvm.FunctionType(llvm.PointerType(c.ctx.VoidType(), 1), []llvm.Type{c.ctx.Int32Type()}, false)
	memAllocFunction := llvm.AddFunction(module, MemAllocFunctionName, memAllocFunctionType)
	globalFunctions[MemAllocFunctionName] = GlobalLLVMFunction{memAllocFunction, memAllocFunctionType}

	// GC safepoint slow path function (external declaration)
	gcSafepointSlowPathType := llvm.FunctionType(c.ctx.VoidType(), []llvm.Type{}, false)
	gcSafepointSlowPathFunction := llvm.AddFunction(module, "zeus_gc_poll", gcSafepointSlowPathType)
	gcSafepointSlowPathFunction.SetLinkage(llvm.InternalLinkage)
	globalFunctions["zeus_gc_poll"] = GlobalLLVMFunction{gcSafepointSlowPathFunction, gcSafepointSlowPathType}

	// GC safepoint poll function (defined function)
	gcSafepointPollType := llvm.FunctionType(c.ctx.VoidType(), []llvm.Type{}, false)
	gcSafepointPollFunction := llvm.AddFunction(module, "gc.safepoint_poll", gcSafepointPollType)
	gcSafepointPollFunction.SetLinkage(llvm.InternalLinkage)
	globalFunctions["gc.safepoint_poll"] = GlobalLLVMFunction{gcSafepointPollFunction, gcSafepointPollType}

	// Create the body for gc.safepoint_poll
	builder := c.ctx.NewBuilder()
	entryBlock := c.ctx.AddBasicBlock(gcSafepointPollFunction, "entry")
	builder.SetInsertPointAtEnd(entryBlock)
	builder.CreateCall(gcSafepointSlowPathType, gcSafepointSlowPathFunction, []llvm.Value{}, "")
	builder.CreateRetVoid()
	builder.Dispose()

	return globalFunctions
}

func (c *CodegenModule) callGlobalLLVMFunction(name string, args ...llvm.Value) llvm.Value {
	function, ok := c.globalLLVMFunctions[name]
	if !ok {
		panic(fmt.Sprintf("global function %s not found", name))
	}
	return c.builder.CreateCall(function.FunctionType, function.Function, args, name)
}

func (c *CodegenModule) getSymbol(name string) llvm.Value {
	symbol, ok := c.symbolTable.GetSymbol(name)
	if !ok {
		panic(fmt.Sprintf("symbol %s not found in symbol table", name))
	}
	return symbol
}

func (c *CodegenModule) getSizeOfClass(class zeus_value.Class) uint64 {
	llvmStructType := c.getLLVMStructType(class.Name)
	size := uint64(0)
	for _, elementType := range llvmStructType.StructElementTypes() {
		size += c.targetDataLayout.TypeAllocSize(elementType)
	}
	return size
}

func (c *CodegenModule) isImportedClass(name string) bool {
	_, ok := c.importedClasses[name]
	return ok
}

func (c *CodegenModule) getLLVMStructType(name string) llvm.Type {
	zeusClassLLVMStruct, ok := c.zeusClassLLVMStructMap[name]
	if !ok {
		panic(fmt.Sprintf("llvm struct type %s not found", name))
	}
	return zeusClassLLVMStruct.LLVMStruct
}


func (c *CodegenModule) getLLVMVTablePtr(name string) llvm.Value {
  llvmVTable := c.module.NamedGlobal(GetVTableStructPtrName(name))
  if llvmVTable.IsNil() {
    panic(fmt.Sprintf("llvm vtable ptr %s not found", name))
  }
  return llvmVTable
}

func (c *CodegenModule) getLLVMObjHeaderPtr(name string) llvm.Value {
  llvmObjHeader := c.module.NamedGlobal(GetObjectHeaderStructPtrName(name))
  if llvmObjHeader.IsNil() {
    panic(fmt.Sprintf("llvm obj header ptr %s not found", name))
  }
  return llvmObjHeader
}

func (c *CodegenModule) getLLVMVTableStruct(name string) llvm.Type {
	zeusClassLLVMStruct, ok := c.zeusClassLLVMStructMap[name]
	if !ok {
		panic(fmt.Sprintf("llvm vtable struct %s not found", name))
	}
	return zeusClassLLVMStruct.LLVMVTableStruct
}

func (c *CodegenModule) getLLVMObjHeaderStruct(name string) llvm.Type {
	zeusClassLLVMStruct, ok := c.zeusClassLLVMStructMap[name]
	if !ok {
		panic(fmt.Sprintf("llvm obj header struct %s not found", name))
	}
	return zeusClassLLVMStruct.LLVMObjHeaderStruct
}

func (c *CodegenModule) getZeusClass(name string) zeus_value.Class {
	return c.zeusClassLLVMStructMap[name].ZeusClass
}

func (c *CodegenModule) toLLVMClassMethodType(method zeus_value.ClassMethod, llvmStructType llvm.Type) llvm.Type {
	paramLLVMTypes := []llvm.Type{}
	for _, param := range method.Method.Params {
		paramLLVMTypes = append(paramLLVMTypes, c.toLLVMType(c.getValueType(param)))
	}

	paramLLVMTypes = append(paramLLVMTypes, llvmStructType)

	return llvm.FunctionType(c.toLLVMType(method.Method.ReturnType), paramLLVMTypes, false)
}

func (c *CodegenModule) getValueType(value zeus_value.Value) zeus_value.ValueType {
	valueType := zeus_value.GetValueType(value)
	switch valueType := valueType.(type) {
	case zeus_value.UserDefinedType:
		return zeus_value.NewObjectType(c.getZeusClass(valueType.Name))
	default:
		return valueType
	}
}

func (c *CodegenModule) toLLVMValue(value zeus_value.Value) llvm.Value {
	switch value := value.(type) {
	case *zeus_value.Constant:
		return c.toLLVMConstant(*value)
	case *zeus_value.Var:
		return c.getSymbol(value.Name)
	case *zeus_value.Function:
		return c.getSymbol(value.Name)
	case *zeus_value.Object:
		return c.getSymbol(value.Name)
	default:
		panic(fmt.Sprintf("unable to convert zeus value %T to llvm value", value))
	}
}

func (c *CodegenModule) toLLVMType(_type zeus_value.ValueType) llvm.Type {
	switch _type := _type.(type) {
	case zeus_value.UserDefinedType:
		return llvm.PointerType(c.getLLVMStructType(_type.Name), 1)
	case zeus_value.FunctionType:
		return llvm.PointerType(c.toLLVMFunctionType(_type), 1)
	case zeus_value.ObjectType:
		return llvm.PointerType(c.getLLVMStructType(_type.Class.Name), 1)
	default:
		return c.toLLVMBuiltInType(_type)
	}
}

func (c *CodegenModule) toLLVMStructType(_type zeus_value.ValueType) llvm.Type {
	switch _type := _type.(type) {
	case zeus_value.UserDefinedType:
		return c.getLLVMStructType(_type.Name)
	case zeus_value.ObjectType:
		return c.getLLVMStructType(_type.Class.Name)
	default:
		panic(fmt.Sprintf("unable to convert zeus value %T to llvm struct type", _type))
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


func (c *CodegenModule) toLLVMFunctionType(functionType zeus_value.FunctionType) llvm.Type {
	param_llvm_types := []llvm.Type{}

	for _, param := range functionType.ParamTypes {
		param_llvm_types = append(param_llvm_types, c.toLLVMType(param))
	}

	return llvm.FunctionType(c.toLLVMType(functionType.ReturnType), param_llvm_types, false)
}

func (c *CodegenModule) genFunc(function zeus_value.Function) llvm.Value {
	llvmFunc := llvm.AddFunction(c.module, function.Name, c.toLLVMFunctionType(zeus_value.ToFunctionType(function)))
	funcParams := function.Params

	for index, param := range llvmFunc.Params() {
		c.symbolTable.DeclareSymbol(funcParams[index].Name, param)
	}

	// Set GC strategy for functions that might allocate memory
	llvmFunc.SetGC("statepoint-example")

	c.symbolTable.DeclareGlobalSymbol(function.Name, llvmFunc)

	return llvmFunc
}

func (c *CodegenModule) genDeclFunc(input ir.DeclFuncInstrInput) llvm.Value {
	llvmFunc := c.genFunc(*input.Function)

	if c.isEntryPoint && input.Function.Name == token.MAIN_FUNCTION_NAME {
		llvmFunc.SetLinkage(llvm.ExternalLinkage)
	} else {
		llvmFunc.SetLinkage(llvm.PrivateLinkage)
	}

	return llvmFunc
}

func (c *CodegenModule) genReturn(input ir.ReturnInstrInput) {
	if input.Value != nil {
		c.builder.CreateRet(c.toLLVMValue(input.Value))
	} else {
		c.builder.CreateRetVoid()
	}
}

func (c *CodegenModule) genDeclVar(input ir.DeclareVarInstrInput) {
	variableType := c.toLLVMType(input.Variable.ValueType)
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
	llvmValue := c.builder.CreateLoad(c.toLLVMType(input.Addr.ValueType), addr, input.Addr.Name)
	c.symbolTable.DeclareSymbol(output.Name, llvmValue)
}

func (c *CodegenModule) genCallFunc(input ir.CallFuncInstrInput, output zeus_value.Var) {
	function := c.toLLVMValue(input.Callee)
	functionType := zeus_value.AsFunctionType(zeus_value.GetValueType(input.Callee))
	zeus_error.Assert(functionType != nil, fmt.Sprintf("%s is not a function", input.Callee))
	llvmFunctionType := c.toLLVMFunctionType(*functionType)
	args := make([]llvm.Value, len(input.Args))
	for i, arg := range input.Args {
		args[i] = c.toLLVMValue(arg)
	}

	llvmValue := c.builder.CreateCall(llvmFunctionType, function, args, fmt.Sprintf("%s_call_result", function.Name()))
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

type BinaryOpLLVMFunc func(llvm.Value, llvm.Value, string) llvm.Value

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
	exportedValue := input.Value

	switch exportedValue := exportedValue.(type) {
	case *zeus_value.Function:
		llvmValue := c.toLLVMValue(input.Value)
		llvmValue.SetName(module.GetModuleScopedName(input.ModulePath, exportedValue.Name))
		llvmValue.SetLinkage(llvm.ExternalLinkage)
	case *zeus_value.Class:
		c.exportedClasses[exportedValue.Name] = NewZeusClassModule(input.ModulePath, *exportedValue)
		// update the vtable and constructor method names to include the module resolution
    llvmObjHeader := c.getLLVMObjHeaderPtr(exportedValue.Name)
		scopedVTableName := module.GetModuleScopedName(input.ModulePath, llvmObjHeader.Name())
		llvmObjHeader.SetName(scopedVTableName)
		constructorMethod := c.module.NamedFunction(util.GetClassMethodName(exportedValue.Name, token.CONSTRUCTOR_METHOD_NAME))
		constructorMethod.SetName(module.GetModuleScopedName(input.ModulePath, constructorMethod.Name()))
	}
}

func (c *CodegenModule) genImport(input ir.ImportInstrInput) {
	importedValue := input.Value

	switch importedValue := importedValue.(type) {
	case *zeus_value.Function:
		importedFunc := llvm.AddFunction(c.module, module.GetModuleScopedName(input.ModulePath, importedValue.Name), c.toLLVMFunctionType(zeus_value.ToFunctionType(*importedValue)))
		// Set GC strategy for imported functions
		importedFunc.SetGC("statepoint-example")
		c.symbolTable.DeclareGlobalSymbol(importedValue.Name, importedFunc)
	case *zeus_value.Class:
		llvmStructType, vtableStructType, objectHeaderStructType, structName  := c.createClassStructTypes(*importedValue)
		moduleScopedName := module.GetModuleScopedName(input.ModulePath, structName)
		// declare the external obj header global
		llvm.AddGlobal(c.module, objectHeaderStructType, GetObjectHeaderStructPtrName(moduleScopedName))
		zeusClassLLVMStruct := NewZeusClassLLVMStruct(*importedValue, llvmStructType, vtableStructType,  make([]llvm.Value, 0), objectHeaderStructType)
		c.zeusClassLLVMStructMap[importedValue.Name] = zeusClassLLVMStruct
		// track the struct info for the imported class
		c.importedClasses[importedValue.Name] = NewZeusClassModule(input.ModulePath, *importedValue)
		// declare the external constructor method
		for _, method := range importedValue.Methods {
			constructorMethodName := util.GetClassMethodName(importedValue.Name, token.CONSTRUCTOR_METHOD_NAME)
			scopedConstructorName := module.GetModuleScopedName(input.ModulePath, constructorMethodName)
			if method.Method.Name == token.CONSTRUCTOR_METHOD_NAME {
				constructorMethod := method.Method
				constructorFunc := llvm.AddFunction(c.module, scopedConstructorName, c.toLLVMFunctionType(zeus_value.ToFunctionType(*constructorMethod)))
				// Set GC strategy for imported constructor methods
				constructorFunc.SetGC("statepoint-example")
				break
			}
		}
	default:
		panic(fmt.Sprintf("cannot codegen for imported value %s", importedValue))
	}
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
				result = c.builder.CreateSIToFP(c.toLLVMValue(input.Value), c.toLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
			} else {
				result = c.builder.CreateUIToFP(c.toLLVMValue(input.Value), c.toLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
			}
		case zeus_value.IntType:
			if castType.Signed {
				result = c.builder.CreateSExt(c.toLLVMValue(input.Value), c.toLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
			} else {
				result = c.builder.CreateZExt(c.toLLVMValue(input.Value), c.toLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
			}
		default:
			panic(castErrorMsg)
		}
	case zeus_value.FloatType:
		result = c.builder.CreateFPExt(c.toLLVMValue(input.Value), c.toLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
	default:
		panic(castErrorMsg)
	}

	c.symbolTable.DeclareSymbol(output.Name, result)
}

func (c *CodegenModule) createClassStructTypes(class zeus_value.Class) (llvm.Type, llvm.Type, llvm.Type, string) {
	// create the vtable struct
	vtableStructName := GetVTableStructName(class.Name)
	vtableStructType := c.ctx.StructCreateNamed(vtableStructName)

  // create object header struct
	objectHeaderStructName := GetObjectHeaderStructName(class.Name)

  gcOffsetsCount := 0

  for _, property := range class.Properties {
    if zeus_value.IsUserDefinedType(property.Property.ValueType) {
      gcOffsetsCount += 1
    }
  }

  objectHeaderElementTypes := []llvm.Type{
    llvm.PointerType(vtableStructType, 0), // vtable 
		c.ctx.Int8Type(), // gc offsets count
    llvm.ArrayType(c.ctx.Int8Type(), gcOffsetsCount), // gc offsets
  }

  objectHeaderStructType := c.ctx.StructCreateNamed(objectHeaderStructName)
  objectHeaderStructType.StructSetBody(objectHeaderElementTypes, false)

	// create the class struct with the vtable struct as the first element
	classElementTypes := []llvm.Type{llvm.PointerType(objectHeaderStructType, 0)}
	for _, property := range class.Properties {
		classElementTypes = append(classElementTypes, c.toLLVMType(property.Property.ValueType))
	}
	llvmStructType := c.ctx.StructCreateNamed(class.Name)
	llvmStructType.StructSetBody(classElementTypes, false)

	vtableElementTypes := []llvm.Type{}
	for _, method := range class.Methods {
		// constructor method is not part of the vtable
		if method.Method.Name == token.CONSTRUCTOR_METHOD_NAME {
			continue
		}
		vtableElementTypes = append(vtableElementTypes, llvm.PointerType(c.toLLVMClassMethodType(*method, llvmStructType), 0))
	}
	vtableStructType.StructSetBody(vtableElementTypes, false)
	
	return llvmStructType, vtableStructType, objectHeaderStructType, class.Name 
}

func (c *CodegenModule) genDeclClass(input ir.DeclClassInstrInput, output zeus_value.Var) {
	llvmStructType, vtableStructType, objectHeaderStructType, structName := c.createClassStructTypes(*input.Class)
	// create the vtable global
	llvmVTable := llvm.AddGlobal(c.module, vtableStructType, GetVTableStructPtrName(structName))
	llvmVTable.SetInitializer(llvm.ConstNull(vtableStructType))
  // create the obj header global 
  llvmObjectHeader := llvm.AddGlobal(c.module, objectHeaderStructType, GetObjectHeaderStructPtrName(structName))
  gcOffsetsArray := []llvm.Value{}
	
	// Calculate proper field offsets manually with alignment
	currentOffset := c.targetDataLayout.TypeAllocSize(llvm.PointerType(objectHeaderStructType, 0)) // Start after header pointer
  for _, property := range input.Class.Properties {
		propertyType := c.toLLVMType(property.Property.ValueType)
		// Get required alignment for this type
		typeAlign := uint64(c.targetDataLayout.ABITypeAlignment(propertyType))
		// Round up current offset to proper alignment
		if currentOffset%typeAlign != 0 {
			currentOffset = ((currentOffset + typeAlign - 1) / typeAlign) * typeAlign
		}
		
    if zeus_value.IsUserDefinedType(property.Property.ValueType) {
      gcOffsetsArray = append(gcOffsetsArray, llvm.ConstInt(c.ctx.Int8Type(), currentOffset, false))
		}
		
		// Move to next field
		currentOffset += c.targetDataLayout.TypeAllocSize(propertyType)
  }
  llvmObjectHeader.SetInitializer(llvm.ConstStruct([]llvm.Value{llvmVTable, llvm.ConstInt(c.ctx.Int8Type(), uint64(len(gcOffsetsArray)), false), llvm.ConstArray(c.ctx.Int8Type(), gcOffsetsArray)}, false))
  // initialize the llvm methods array
	methodCount := 0
	for _, method := range input.Class.Methods {
		if method.Method.Name == token.CONSTRUCTOR_METHOD_NAME {
			continue
		}
		methodCount += 1
	}
	llvmVTableMethods := make([]llvm.Value, methodCount)
	zeusClassLLVMStruct := NewZeusClassLLVMStruct(*input.Class, llvmStructType, vtableStructType, llvmVTableMethods, objectHeaderStructType)
	c.zeusClassLLVMStructMap[input.Class.Name] = zeusClassLLVMStruct

	// create the vtable global
	// initialize the vtable methods to null
	// this is done here because the vtable methods are not known until we encounter the DECL_CLASS_METHOD instructions
	for llvmVTableMethodIndex := range llvmVTableMethods {
		llvmVTableMethods[llvmVTableMethodIndex] = llvm.ConstNull(llvm.PointerType(llvm.FunctionType(c.ctx.VoidType(), []llvm.Type{}, false), 0))
	}
	// track the struct info
	c.zeusClassLLVMStructMap[output.Name] = zeusClassLLVMStruct
}

func (c *CodegenModule) genNewObj(input ir.NewObjInstrInput, output zeus_value.Var) {
	callee, ok := input.Callee.(*zeus_value.Class)
	if !ok {
		panic(fmt.Sprintf("trying to create new object of non class type %s", input.Callee))
	}
	llvmStructType := c.getLLVMStructType(callee.Name)
	llvmStruct := c.callGlobalLLVMFunction(MemAllocFunctionName, llvm.ConstInt(c.ctx.Int32Type(), c.getSizeOfClass(*callee), false))
	llvmStructObjHeaderField := c.builder.CreateStructGEP(llvmStructType, llvmStruct, OBJ_HEADER_STRUCT_INDEX, fmt.Sprintf("%s_header_field", callee.Name))
	llvmObjHeader := c.getLLVMObjHeaderPtr(callee.Name)
	c.builder.CreateStore(llvmObjHeader, llvmStructObjHeaderField)
	// Track the allocated object with the GC
	if len(input.Args) > 0 {
		constructorMethodName := fmt.Sprintf("%s.%s", callee.Name, token.CONSTRUCTOR_METHOD_NAME)
		// imported constructor name has module resolution in name
		if c.isImportedClass(callee.Name) {
			classModule := c.importedClasses[callee.Name]
			constructorMethodName = module.GetModuleScopedName(classModule.ModulePath, constructorMethodName)
		}
		constructorMethod := c.module.NamedFunction(constructorMethodName)
		zeus_error.Assert(!constructorMethod.IsNil(), fmt.Sprintf("constructor method %s not found", constructorMethodName))
		// create the param types
		constructorParamTypes := []zeus_value.ValueType{}
		for _, arg := range input.Args {
			constructorParamTypes = append(constructorParamTypes, zeus_value.GetValueType(arg))
		}
		constructorParamTypes = append(constructorParamTypes, zeus_value.NewObjectType(*callee))
		constructorMethodType := c.toLLVMFunctionType(zeus_value.NewFunctionType(zeus_value.VoidType{}, constructorParamTypes))
		// create the param values
		constructorMethodParams := []llvm.Value{}
		for _, arg := range input.Args {
			constructorMethodParams = append(constructorMethodParams, c.toLLVMValue(arg))
		}
		constructorMethodParams = append(constructorMethodParams, llvmStruct)
		// call the constructor method
		c.builder.CreateCall(constructorMethodType, constructorMethod, constructorMethodParams, constructorMethodName)
	}
	c.symbolTable.DeclareSymbol(output.Name, llvmStruct)
}

func (c *CodegenModule) appendThisParamToFunction(method zeus_value.Function, class zeus_value.Class) zeus_value.Function {
	method.Params = append(
		method.Params,
		zeus_value.NewVar(
			token.THIS_KEYWORD,
			zeus_value.NewObjectType(class),
			false,
			method.Span,
		),
	)

	return method
}

func (c *CodegenModule) genDeclClassMethod(input ir.DeclClassMethodInstrInput) llvm.Value {
	methodWithThisParam := c.appendThisParamToFunction(*input.Method, *input.Class)
	isConstructor := methodWithThisParam.Name == util.GetClassMethodName(input.Class.Name, token.CONSTRUCTOR_METHOD_NAME)
	function := c.genFunc(methodWithThisParam)

	if !isConstructor {
		// update the vtable global initializer
		structInfo := c.zeusClassLLVMStructMap[input.Class.Name]
		structInfo.LLVMVTableMethods[structInfo.CurrentVTableMethodIndex] = function
		structInfo.CurrentVTableMethodIndex += 1
		c.getLLVMVTablePtr(input.Class.Name).SetInitializer(llvm.ConstStruct(structInfo.LLVMVTableMethods, true))
	}

	return function
}

func (c *CodegenModule) genObjectPropertyAccess(input ir.ObjectPropertyAccessInstrInput, output zeus_value.Var) {
	objectType := c.getValueType(input.Object)
	llvmValue := c.toLLVMValue(input.Object)
	objectClass := zeus_value.AsObjectType(objectType)
	zeus_error.Assert(objectClass != nil, fmt.Sprintf("object %s is not a class", input.Object))
	propertyIndex := util.GetPropertyIndex(objectClass.Class, input.Property)
	if propertyIndex == -1 {
		methodIndex := util.GetMethodIndex(objectClass.Class, input.Property)
		zeus_error.Assert(methodIndex != -1, fmt.Sprintf("property %s not found in class %s", input.Property, objectClass.Class.Name))
		llvmObjHeaderPtr := c.builder.CreateStructGEP(c.toLLVMStructType(objectType), llvmValue, OBJ_HEADER_STRUCT_INDEX, "objHeaderPtr")
    llvmObjHeader := c.builder.CreateLoad(llvm.PointerType(c.getLLVMObjHeaderStruct(objectClass.Class.Name), 0), llvmObjHeaderPtr, "objHeader") 
    llvmVTablePtr := c.builder.CreateStructGEP(c.getLLVMObjHeaderStruct(objectClass.Class.Name), llvmObjHeader, VTABLE_STRUCT_INDEX, "vTablePtr")
		llvmVTable := c.builder.CreateLoad(llvm.PointerType(c.getLLVMVTableStruct(objectClass.Class.Name), 0), llvmVTablePtr, "vTable")
		classMethodPtr := c.builder.CreateStructGEP(c.getLLVMVTableStruct(objectClass.Class.Name), llvmVTable, methodIndex, input.Property)
		c.symbolTable.DeclareSymbol(output.Name, classMethodPtr)
		return
	}
	llvmValue = c.builder.CreateStructGEP(c.toLLVMStructType(objectType), llvmValue, propertyIndex, input.Property)
	c.symbolTable.DeclareSymbol(output.Name, llvmValue)
}

func (c *CodegenModule) genIndirectFuncCall(input ir.IndirectFuncCallInstrInput, output zeus_value.Var) {
	functionVar := zeus_value.AsVar(input.Function)
	zeus_error.Assert(functionVar != nil, fmt.Sprintf("function %s is not a variable", input.Function))
	cxt := functionVar.Cxt
	function := c.toLLVMValue(input.Function)
	functionType := zeus_value.AsFunctionType(zeus_value.GetValueType(input.Function))
	functionArgs := []llvm.Value{}

	for _, arg := range input.Args {
		functionArgs = append(functionArgs, c.toLLVMValue(arg))
	}

	if cxt != nil {
		llvmObject := c.toLLVMValue(*cxt)
		functionArgs = append(functionArgs, llvmObject)
		objectType := zeus_value.AsObjectType(c.getValueType(*cxt))
		zeus_error.Assert(objectType != nil, fmt.Sprintf("cxt is not an object %s", *cxt))
		functionType.ParamTypes = append(functionType.ParamTypes, zeus_value.NewObjectType(objectType.Class))
	}

	llvmValue := c.builder.CreateCall(c.toLLVMFunctionType(*functionType), function, functionArgs, fmt.Sprintf("%s_call_result", function.Name()))
	c.symbolTable.DeclareSymbol(output.Name, llvmValue)
}

func (c *CodegenModule) Generate(irBuilder ir.IRBuilder) {
	var currentFunction llvm.Value

	c.symbolTable.EnterScope()

	irBuilder.Walk(func(instr *ir.Instr) {
		switch instr.Type {
		case ir.InstrTypeDeclFunc:
			fallthrough
		case ir.InstrTypeDeclClassMethod:
			// maintain function level scoping
			if !c.symbolTable.IsGlobalScope() {
				c.symbolTable.ExitScope()
			} else {
				c.symbolTable.EnterScope()
			}
			if instr.Type == ir.InstrTypeDeclClassMethod {
				currentFunction = c.genDeclClassMethod(*ir.AsDeclClassMethodInstrInput(instr.Input))
			} else {
				currentFunction = c.genDeclFunc(*ir.AsDeclFuncInstrInput(instr.Input))
			}
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
		case ir.InstrTypeImport:
			c.genImport(*ir.AsImportInstrInput(instr.Input))
		case ir.InstrTypeDeclClass:
			c.genDeclClass(*ir.AsDeclClassInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeNewObj:
			c.genNewObj(*ir.AsNewObjInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeObjectPropertyAccess:
			c.genObjectPropertyAccess(*ir.AsObjectPropertyAccessInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeIndirectFuncCall:
			c.genIndirectFuncCall(*ir.AsIndirectFuncCallInstrInput(instr.Input), *instr.Output)
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
