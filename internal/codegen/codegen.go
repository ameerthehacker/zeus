package codegen

import (
	"fmt"
	"strconv"
	"strings"

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
	cxt llvm.Context
}

type ZeusClassLLVMStruct struct {
	ZeusClass                zeus_value.Class
	LLVMStructType           llvm.Type
	LLVMVTableStructType     llvm.Type
	LLVMVTableMethods        []llvm.Value
	LLVMObjHeaderStructType  llvm.Type
	LLVMVTableInstance       *llvm.Value
	LLVMObjHeaderInstance    llvm.Value
	LLVMConstructorMethod    *llvm.Value
	LLVMFactoryFunction      *llvm.Value
	CurrentVTableMethodIndex int
}

type ZeusClassModule struct {
	ModulePath string
	Class      zeus_value.Class
}

type GlobalLLVMFunction struct {
	Function     llvm.Value
	FunctionType llvm.Type
}

func NewZeusClassModule(modulePath string, class zeus_value.Class) ZeusClassModule {
	return ZeusClassModule{modulePath, class}
}

func NewZeusClassLLVMStruct(zeusClass zeus_value.Class, llvmStruct llvm.Type, llvmVTableStruct llvm.Type, llvmVTableMethods []llvm.Value, llvmObjHeaderStruct llvm.Type, llvmVTableInstance *llvm.Value, llvmObjHeaderInstance llvm.Value, llvmConstructorMethod *llvm.Value) *ZeusClassLLVMStruct {
	return &ZeusClassLLVMStruct{zeusClass, llvmStruct, llvmVTableStruct, llvmVTableMethods, llvmObjHeaderStruct, llvmVTableInstance, llvmObjHeaderInstance, llvmConstructorMethod, nil, 0}
}

const MemAllocFunctionName = "zeus_gc_alloc"
const ZeusObjectTypeInfoStructName = "ZeusObjectTypeInfo"
const ZeusObjectClassName = "Object"
const ZeusObjectArrayClassName = ZeusObjectClassName + "[]"

type CodegenModule struct {
	module                 llvm.Module
	builder                llvm.Builder
	cxt                    llvm.Context
	symbolTable            *symbol_table.SymbolTable[llvm.Value]
	basicBlocks            map[int]llvm.BasicBlock
	isEntryPoint           bool
	exportedClasses        map[string]ZeusClassModule
	importedClasses        map[string]ZeusClassModule
	zeusClassLLVMStructMap map[string]*ZeusClassLLVMStruct
	targetDataLayout       llvm.TargetData
	globalLLVMFunctions    map[string]GlobalLLVMFunction
	zeusObjectTypeInfoType llvm.Type
}

func NewCodegen() *Codegen {
	ctx := llvm.NewContext()

	return &Codegen{ctx}
}

func (c *Codegen) NewModule(name string, isEntryPoint bool, targetDataLayout llvm.TargetData) *CodegenModule {
	module := c.cxt.NewModule(name)
	builder := c.cxt.NewBuilder()
	globalLLVMFunctions := c.setupGlobalLLVMFunctions(module)

	zeusObjectInfoStructType := c.cxt.StructCreateNamed(ZeusObjectTypeInfoStructName)
	zeusObjectInfoStructType.StructSetBody([]llvm.Type{
		// type id
		c.cxt.Int8Type(),
		// type
		c.cxt.Int8Type(),
		// array element type
		c.cxt.Int8Type(),
		// parent type info pointer
		llvm.PointerType(zeusObjectInfoStructType, 0),
	}, false)

	return &CodegenModule{
		module:                 module,
		builder:                builder,
		cxt:                    c.cxt,
		symbolTable:            symbol_table.NewSymbolTable[llvm.Value](),
		basicBlocks:            make(map[int]llvm.BasicBlock),
		isEntryPoint:           isEntryPoint,
		exportedClasses:        make(map[string]ZeusClassModule),
		importedClasses:        make(map[string]ZeusClassModule),
		zeusClassLLVMStructMap: make(map[string]*ZeusClassLLVMStruct),
		targetDataLayout:       targetDataLayout,
		globalLLVMFunctions:    globalLLVMFunctions,
		zeusObjectTypeInfoType: zeusObjectInfoStructType,
	}
}

func (c *Codegen) setupGlobalLLVMFunctions(module llvm.Module) map[string]GlobalLLVMFunction {
	globalFunctions := make(map[string]GlobalLLVMFunction)

	// Memory allocation function (GC-tracked objects)
	memAllocFunctionType := llvm.FunctionType(llvm.PointerType(c.cxt.VoidType(), 1), []llvm.Type{c.cxt.Int32Type()}, false)
	memAllocFunction := llvm.AddFunction(module, MemAllocFunctionName, memAllocFunctionType)
	globalFunctions[MemAllocFunctionName] = GlobalLLVMFunction{memAllocFunction, memAllocFunctionType}

	// GC safepoint slow path function (external declaration)
	gcSafepointSlowPathType := llvm.FunctionType(c.cxt.VoidType(), []llvm.Type{}, false)
	gcSafepointSlowPathFunction := llvm.AddFunction(module, "zeus_gc_poll", gcSafepointSlowPathType)
	gcSafepointSlowPathFunction.SetLinkage(llvm.InternalLinkage)
	globalFunctions["zeus_gc_poll"] = GlobalLLVMFunction{gcSafepointSlowPathFunction, gcSafepointSlowPathType}

	// GC safepoint poll function (defined function)
	gcSafepointPollType := llvm.FunctionType(c.cxt.VoidType(), []llvm.Type{}, false)
	gcSafepointPollFunction := llvm.AddFunction(module, "gc.safepoint_poll", gcSafepointPollType)
	gcSafepointPollFunction.SetLinkage(llvm.InternalLinkage)
	globalFunctions["gc.safepoint_poll"] = GlobalLLVMFunction{gcSafepointPollFunction, gcSafepointPollType}

	// Create the body for gc.safepoint_poll
	builder := c.cxt.NewBuilder()
	entryBlock := c.cxt.AddBasicBlock(gcSafepointPollFunction, "entry")
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

func (c *CodegenModule) getPrimordialRuntimeFunctionName(methodName string, primordialName string) string {
	return fmt.Sprintf("%s_%s_%s", "zeus", primordialName, methodName)
}

func (c *CodegenModule) genExternalRuntimeFunction(functionName string, numParams int, hasThisPtr bool) (llvm.Value, llvm.Type) {
	// Check if function already exists in global functions
	if globalFunc, exists := c.globalLLVMFunctions[functionName]; exists {
		return globalFunc.Function, globalFunc.FunctionType
	}

	// Build parameter types: optional this_ptr, return_buffer_ptr, ...param_ptrs
	paramTypes := []llvm.Type{}
	if hasThisPtr {
		paramTypes = append(paramTypes, llvm.PointerType(c.cxt.VoidType(), 0)) // this pointer
	}
	paramTypes = append(paramTypes, llvm.PointerType(c.cxt.VoidType(), 0)) // return buffer pointer

	// Add pointer type for each parameter
	for i := 0; i < numParams; i++ {
		paramTypes = append(paramTypes, llvm.PointerType(c.cxt.VoidType(), 0))
	}

	// Function returns void
	functionType := llvm.FunctionType(c.cxt.VoidType(), paramTypes, false)
	function := llvm.AddFunction(c.module, functionName, functionType)
	function.SetLinkage(llvm.ExternalLinkage)

	// Store in global functions map
	c.globalLLVMFunctions[functionName] = GlobalLLVMFunction{function, functionType}

	return function, functionType
}

func (c *CodegenModule) genPrimordialRuntimeFunction(method zeus_value.Function, primordialName string) (llvm.Value, llvm.Type) {
	functionName := c.getPrimordialRuntimeFunctionName(method.Name, primordialName)
	return c.genExternalRuntimeFunction(functionName, len(method.Params), true)
}

func (c *CodegenModule) genPrimordialClassMethods(class zeus_value.Class) {
	currentInsertionBlock := c.builder.GetInsertBlock()

	for _, method := range class.Methods {
		// update the method name to include the class name prefix
		scopedClassMethod := zeus_value.NewClassMethod(
			zeus_value.NewFunction(
				util.GetClassMethodName(class.Name, method.Method.Name),
				method.Method.Params,
				method.Method.ReturnType,
				method.Method.Span,
			),
			method.AccessModifier,
		)
		classFunction := c.genClassMethod(*scopedClassMethod.Method, class)

		// Mark primordial wrappers as alwaysinline to eliminate them from binary
		// These are just thin wrappers around runtime functions, so inlining is beneficial
		alwaysInlineKind := llvm.AttributeKindID("alwaysinline")
		classFunction.AddAttributeAtIndex(-1, c.cxt.CreateEnumAttribute(alwaysInlineKind, 0))

		basicBlock := llvm.AddBasicBlock(classFunction, "entry")
		c.builder.SetInsertPointAtEnd(basicBlock)

		// Get the runtime function
		runtimeFunction, runtimeFuncType := c.genPrimordialRuntimeFunction(*method.Method, class.PrimordialName)

		// Get the 'this' parameter (last parameter of the function)
		params := classFunction.Params()
		thisPtr := params[len(params)-1]

		// Prepare arguments for runtime call: [this_ptr, return_buffer_ptr_ptr, ...param_ptrs]
		var runtimeArgs []llvm.Value
		var returnBufferPtrPtr llvm.Value

		// Only allocate return buffer pointer if return type is not void
		if !zeus_value.IsVoidType(method.Method.ReturnType) {
			// Allocate a pointer on the stack to hold the address of the result
			// The runtime will allocate memory and store the pointer to it here
			voidPtrType := llvm.PointerType(c.cxt.VoidType(), 0)
			returnBufferPtrPtr = c.builder.CreateAlloca(voidPtrType, "return_buffer_ptr_ptr")
			runtimeArgs = []llvm.Value{thisPtr, returnBufferPtrPtr}
		} else {
			// For void returns, pass null pointer as return buffer
			nullPtr := llvm.ConstNull(llvm.PointerType(c.cxt.VoidType(), 0))
			runtimeArgs = []llvm.Value{thisPtr, nullPtr}
		}

		// Allocate stack memory for each method parameter and store values
		for i, param := range method.Method.Params {
			paramType := c.toLLVMType(param.ValueType)
			paramAlloca := c.builder.CreateAlloca(paramType, param.Name+"_alloca")
			c.builder.CreateStore(params[i], paramAlloca)
			runtimeArgs = append(runtimeArgs, paramAlloca)
		}

		// Call the runtime function
		c.builder.CreateCall(runtimeFuncType, runtimeFunction, runtimeArgs, "")

		// Handle return value
		if !zeus_value.IsVoidType(method.Method.ReturnType) {
			// Load the return wrapper pointer from the alloca'd location
			voidPtrType := llvm.PointerType(c.cxt.VoidType(), 0)
			returnWrapperPtr := c.builder.CreateLoad(voidPtrType, returnBufferPtrPtr, "return_wrapper_ptr")

			// Define the result field type
			returnType := c.toLLVMType(method.Method.ReturnType)

			// Deserialize memory into a Zeus object with the result field
			zeusObjPtr, zeusObjType := c.deserializeZeusObj(returnWrapperPtr, []llvm.Type{returnType}, "return_wrapper")

			// Extract the result field (index 1, since header is at index 0)
			resultFieldPtr := c.builder.CreateStructGEP(zeusObjType, zeusObjPtr, 1, "result_field_ptr")
			returnValue := c.builder.CreateLoad(returnType, resultFieldPtr, "return_value")
			c.builder.CreateRet(returnValue)
		} else {
			// Return void
			c.builder.CreateRetVoid()
		}
	}
	c.builder.SetInsertPointAtEnd(currentInsertionBlock)
}

func (c *CodegenModule) getLLVMStructType(name string) llvm.Type {
	zeusClassLLVMStruct, ok := c.zeusClassLLVMStructMap[name]
	if !ok {
		panic(fmt.Sprintf("llvm struct type %s not found", name))
	}
	return zeusClassLLVMStruct.LLVMStructType
}

// deserializeZeusObj interprets raw memory as a Zeus object following the Zeus ABI.
// This function automatically prepends the Zeus object header pointer as the first field,
// then appends the provided data fields. The consumer can access any field from the
// resulting struct.
//
// Zeus ABI struct layout: [*ZeusObjectHeader, ...dataFields]
//
// Parameters:
//   - memPtr: A pointer to the memory location to deserialize
//   - dataFields: LLVM types for the data fields (header is automatically prepended)
//   - name: A name prefix for the generated LLVM values
//
// Returns:
//   - A typed pointer to the Zeus object and the object type itself
func (c *CodegenModule) deserializeZeusObj(memPtr llvm.Value, dataFields []llvm.Type, name string) (llvm.Value, llvm.Type) {
	// Build the Zeus object struct: [*ZeusObjectHeader, ...dataFields]
	headerPtrType := llvm.PointerType(c.cxt.VoidType(), 0)
	structFields := make([]llvm.Type, 0, len(dataFields)+1)
	structFields = append(structFields, headerPtrType) // Field 0: header pointer
	structFields = append(structFields, dataFields...) // Fields 1+: data fields

	zeusObjType := c.cxt.StructType(structFields, false)

	// Cast the opaque pointer to the Zeus object type pointer
	typedPtr := c.builder.CreateBitCast(memPtr, llvm.PointerType(zeusObjType, 0), name+"_ptr")

	return typedPtr, zeusObjType
}

func (c *CodegenModule) getLLVMVTablePtr(name string) llvm.Value {
	zeusClassLLVMStruct, ok := c.zeusClassLLVMStructMap[name]
	zeus_error.Assert(ok, fmt.Sprintf("zeus class llm struct %s not found", name))
	zeus_error.Assert(zeusClassLLVMStruct.LLVMVTableInstance != nil, fmt.Sprintf("llvm vtable instance %s not found", name))
	return *zeusClassLLVMStruct.LLVMVTableInstance
}

func (c *CodegenModule) getLLVMObjHeaderPtr(name string) llvm.Value {
	zeusClassLLVMStruct, ok := c.zeusClassLLVMStructMap[name]
	zeus_error.Assert(ok, fmt.Sprintf("zeus class llm struct %s not found", name))

	return zeusClassLLVMStruct.LLVMObjHeaderInstance
}

func (c *CodegenModule) getLLVMConstructorMethod(name string) *llvm.Value {
	zeusClassLLVMStruct, ok := c.zeusClassLLVMStructMap[name]
	zeus_error.Assert(ok, fmt.Sprintf("zeus class llm struct %s not found", name))

	return zeusClassLLVMStruct.LLVMConstructorMethod
}

func (c *CodegenModule) getLLVMVTableStruct(name string) llvm.Type {
	zeusClassLLVMStruct, ok := c.zeusClassLLVMStructMap[name]
	if !ok {
		panic(fmt.Sprintf("llvm vtable struct %s not found", name))
	}
	return zeusClassLLVMStruct.LLVMVTableStructType
}

func (c *CodegenModule) getLLVMObjHeaderStruct(name string) llvm.Type {
	zeusClassLLVMStruct, ok := c.zeusClassLLVMStructMap[name]
	if !ok {
		panic(fmt.Sprintf("llvm obj header struct %s not found", name))
	}
	return zeusClassLLVMStruct.LLVMObjHeaderStructType
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
	case zeus_value.FunctionType:
		return llvm.PointerType(c.toLLVMFunctionType(_type), 0)
	case zeus_value.ObjectType:
		return llvm.PointerType(c.getLLVMStructType(_type.Class.Name), 1)
	case zeus_value.ArrayType:
		// Arrays are objects - use the array class name to get the struct type
		return llvm.PointerType(c.getLLVMStructType(_type.String()), 1)
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
	basicBlock := c.cxt.AddBasicBlock(parent, strconv.Itoa(id))
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
		// for primitive types we need to store the default value
	} else if zeus_value.IsPrimitiveType(input.Variable.ValueType) {
		c.builder.CreateStore(c.getDefaultLLVMValue(input.Variable.ValueType), variable)
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
	case zeus_value.ObjectType:
		// Object comparison (pointer comparison)
		// Handles: object == null, object != null, object == object
		leftValue := c.toLLVMValue(left)
		var rightValue llvm.Value
		if zeus_value.IsNullType(rightType) {
			// Compare with null pointer
			rightValue = llvm.ConstPointerNull(leftValue.Type())
		} else {
			rightValue = c.toLLVMValue(right)
		}
		// Use intIntOp for pointer comparison (IntEQ/IntNE work on pointers)
		return intIntOp(leftValue, rightValue, opName)
	case zeus_value.NullType:
		// null compared with object (reversed order)
		if zeus_value.IsObjectType(rightType) {
			rightValue := c.toLLVMValue(right)
			leftValue := llvm.ConstPointerNull(rightValue.Type())
			return intIntOp(leftValue, rightValue, opName)
		}
	case zeus_value.BoolType:
		// Boolean comparison (i1 values use integer comparison)
		switch rightType.(type) {
		case zeus_value.BoolType:
			return intIntOp(c.toLLVMValue(left), c.toLLVMValue(right), opName)
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
	case ir.InstrTypeMod:
		// Modulo (integer only)
		result = c.genLLVMBinaryOp(input.Left, input.Right, "mod", c.builder.CreateSRem, c.builder.CreateURem, nil)
	case ir.InstrTypePower:
		// Power operation - use llvm.pow intrinsic for floats, implement for ints
		result = c.genPowerOp(input.Left, input.Right)
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
	case ir.InstrTypeAnd:
		// Logical AND (bool && bool)
		result = c.builder.CreateAnd(c.toLLVMValue(input.Left), c.toLLVMValue(input.Right), "and")
	case ir.InstrTypeOr:
		// Logical OR (bool || bool)
		result = c.builder.CreateOr(c.toLLVMValue(input.Left), c.toLLVMValue(input.Right), "or")
	}

	c.symbolTable.DeclareSymbol(output.Name, result)
}

// genPowerOp generates LLVM code for the power operation (a ** b)
// Expects f64 operands - type casting should be done via CAST IR instructions
func (c *CodegenModule) genPowerOp(left zeus_value.Value, right zeus_value.Value) llvm.Value {
	leftValue := c.toLLVMValue(left)
	rightValue := c.toLLVMValue(right)

	// Get or create the pow intrinsic
	powFnName := "llvm.pow.f64"
	f64Type := c.cxt.DoubleType()

	powFn := c.module.NamedFunction(powFnName)
	if powFn.IsNil() {
		powFnType := llvm.FunctionType(f64Type, []llvm.Type{f64Type, f64Type}, false)
		powFn = llvm.AddFunction(c.module, powFnName, powFnType)
	}

	// Call pow intrinsic - operands should already be f64 (via CAST instructions)
	powFnType := llvm.FunctionType(f64Type, []llvm.Type{f64Type, f64Type}, false)
	return c.builder.CreateCall(powFnType, powFn, []llvm.Value{leftValue, rightValue}, "pow_result")
}

func (c *CodegenModule) genUnaryOp(instr *ir.Instr, input ir.UnaryOpInstrInput, output zeus_value.Var) {
	var result llvm.Value
	valueType := zeus_value.GetValueType(input.Value)
	llvmValue := c.toLLVMValue(input.Value)

	switch instr.Type {
	case ir.InstrTypeNeg:
		switch valueType.(type) {
		case zeus_value.IntType:
			result = c.builder.CreateNeg(llvmValue, "neg")
		case zeus_value.FloatType:
			result = c.builder.CreateFNeg(llvmValue, "fneg")
		default:
			panic(fmt.Sprintf("unsupported type for negation: %s", valueType))
		}
	case ir.InstrTypeNot:
		result = c.builder.CreateNot(llvmValue, "not")
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
		llvmConstructorMethod := c.getLLVMConstructorMethod(exportedValue.Name)
		if llvmConstructorMethod != nil {
			constructorMethod := *llvmConstructorMethod
			constructorMethod.SetName(module.GetModuleScopedName(input.ModulePath, constructorMethod.Name()))
		}
	}
}

func (c *CodegenModule) genImportedClass(class zeus_value.Class, modulePath string) {
	// if the class is already imported, don't generate it again
	if _, ok := c.importedClasses[class.Name]; ok {
		return
	}
	llvmStructType, vtableStructType, objectHeaderStructType, structName := c.createClassStructTypes(class)
	moduleScopedName := module.GetModuleScopedName(modulePath, structName)
	// declare the external obj header global
	objectHeaderInstance := llvm.AddGlobal(c.module, objectHeaderStructType, GetObjectHeaderStructPtrName(moduleScopedName))
	var llvmConstructorMethod *llvm.Value = nil
	// declare the external constructor method
	for _, method := range class.Methods {
		constructorMethodName := util.GetClassMethodName(class.Name, token.CONSTRUCTOR_METHOD_NAME)
		scopedConstructorName := module.GetModuleScopedName(modulePath, constructorMethodName)
		if method.Method.Name == token.CONSTRUCTOR_METHOD_NAME {
			constructorMethod := method.Method
			constructorFunc := llvm.AddFunction(c.module, scopedConstructorName, c.toLLVMFunctionType(zeus_value.ToFunctionType(*constructorMethod)))
			// Set GC strategy for imported constructor methods
			constructorFunc.SetGC("statepoint-example")
			llvmConstructorMethod = &constructorFunc
			break
		}
	}
	zeusClassLLVMStruct := NewZeusClassLLVMStruct(class, llvmStructType, vtableStructType, make([]llvm.Value, 0), objectHeaderStructType, nil, objectHeaderInstance, llvmConstructorMethod)
	c.zeusClassLLVMStructMap[class.Name] = zeusClassLLVMStruct
	// track the struct info for the imported class
	c.importedClasses[class.Name] = NewZeusClassModule(modulePath, class)
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
		c.genImportedClass(*importedValue, input.ModulePath)
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
	vtableStructType := c.cxt.StructCreateNamed(vtableStructName)

	// create object header struct
	objectHeaderStructName := GetObjectHeaderStructName(class.Name)

	gcOffsetsCount := 0

	for _, property := range class.Properties {
		if zeus_value.IsObjectType(property.Property.ValueType) {
			gcOffsetsCount += 1
		}
	}

	objectHeaderElementTypes := []llvm.Type{
		llvm.PointerType(vtableStructType, 0),         // vtable
		llvm.PointerType(c.zeusObjectTypeInfoType, 0), // object type info
		c.cxt.Int8Type(), // gc offsets count
		llvm.ArrayType(c.cxt.Int8Type(), gcOffsetsCount), // gc offsets
	}

	objectHeaderStructType := c.cxt.StructCreateNamed(objectHeaderStructName)
	objectHeaderStructType.StructSetBody(objectHeaderElementTypes, false)

	// Create the class struct type first as an opaque type (without body)
	// This allows self-referential types where a class has a property of its own type
	llvmStructType := c.cxt.StructCreateNamed(class.Name)

	// Register the struct type in the map BEFORE setting its body
	// This is necessary for self-referential types (e.g., Node with next: Node)
	// When toLLVMType is called for a property of the same type, it can find this type in the map
	c.zeusClassLLVMStructMap[class.Name] = &ZeusClassLLVMStruct{
		ZeusClass:      class,
		LLVMStructType: llvmStructType,
	}

	// Now build the class element types - self-references will work because the type is already in the map
	classElementTypes := []llvm.Type{llvm.PointerType(objectHeaderStructType, 0)}
	for _, property := range class.Properties {
		classElementTypes = append(classElementTypes, c.toLLVMType(property.Property.ValueType))
	}
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

func (c *CodegenModule) genObjArrayClass() *ZeusClassLLVMStruct {
	span := token.NewSpan(*token.NewPosition(0, 0), *token.NewPosition(0, 0))
	objectClass := zeus_value.NewClass(ZeusObjectClassName, []*zeus_value.ClassProperty{}, []*zeus_value.ClassMethod{}, "", nil, span)
	objectArrayClass := zeus_value.GetArrayPrimordialClassDefinition(zeus_value.NewArrayType(zeus_value.NewObjectType(*objectClass), span))

	if c.zeusClassLLVMStructMap[objectArrayClass.Name] != nil {
		return c.zeusClassLLVMStructMap[objectArrayClass.Name]
	}
	// generate the object class first
	c.genClass(*objectClass)
	// generate the object array class
	return c.genClass(*objectArrayClass)
}

// genClass generates LLVM code for a Zeus class including struct types, vtable, and object header
func (c *CodegenModule) genClass(class zeus_value.Class) *ZeusClassLLVMStruct {
	if c.zeusClassLLVMStructMap[class.Name] != nil {
		return c.zeusClassLLVMStructMap[class.Name]
	} else if class.PrimordialName == zeus_value.ZEUS_PRIMORDIAL_ARRAY && class.Name != ZeusObjectArrayClassName && zeus_value.IsObjectType(class.ArrayElementType) {
		// we generate single object array class for all types of OBJECT arrays
		// because they are represented exactly the same way in memory
		// NOTE: primitive type arrays (u8[], i32[], etc.) need their own type info
		// so the runtime knows the correct element size
		c.zeusClassLLVMStructMap[class.Name] = c.genObjArrayClass()
		return c.zeusClassLLVMStructMap[class.Name]
	}

	llvmStructType, vtableStructType, objectHeaderStructType, structName := c.createClassStructTypes(class)
	// create the vtable global
	llvmVTable := llvm.AddGlobal(c.module, vtableStructType, GetVTableStructPtrName(structName))
	llvmVTable.SetInitializer(llvm.ConstNull(vtableStructType))
	// create the object type info global
	llvmObjectTypeInfo := llvm.AddGlobal(c.module, c.zeusObjectTypeInfoType, GetObjectTypeInfoStructPtrName(structName))
	zeusRuntimeObjectType := ZeusRuntimeObjectTypeObject
	zeusRuntimeArrayElementType := ZeusRuntimeTypeNull

	// if it is an array, set the array element type
	if class.ArrayElementType != nil {
		zeusRuntimeObjectType = ZeusRuntimeObjectTypeArray
		zeusRuntimeArrayElementType = toZeusRuntimeType(class.ArrayElementType)
	}

	llvmObjectTypeInfo.SetInitializer(llvm.ConstStruct([]llvm.Value{
		llvm.ConstInt(c.cxt.Int8Type(), uint64(class.Id), false),
		llvm.ConstInt(c.cxt.Int8Type(), uint64(zeusRuntimeObjectType), false),
		llvm.ConstInt(c.cxt.Int8Type(), uint64(zeusRuntimeArrayElementType), false),
		llvm.ConstNull(llvm.PointerType(c.zeusObjectTypeInfoType, 0)),
	}, false))
	// create the obj header global
	llvmObjectHeader := llvm.AddGlobal(c.module, objectHeaderStructType, GetObjectHeaderStructPtrName(structName))
	gcOffsetsArray := []llvm.Value{}

	// Calculate GC offsets using LLVM's actual struct layout
	// We use ElementOffset to get the exact byte offset LLVM calculated for each field
	// This accounts for all padding and alignment that LLVM adds to the struct
	for propertyIndex, property := range class.Properties {
		if zeus_value.IsObjectType(property.Property.ValueType) {
			// propertyIndex + 1 because index 0 in the struct is the obj_header pointer
			actualOffset := c.targetDataLayout.ElementOffset(llvmStructType, propertyIndex+1)
			gcOffsetsArray = append(gcOffsetsArray, llvm.ConstInt(c.cxt.Int8Type(), actualOffset, false))
		}
	}
	llvmObjectHeader.SetInitializer(llvm.ConstStruct(
		[]llvm.Value{
			llvmVTable,
			llvmObjectTypeInfo,
			llvm.ConstInt(c.cxt.Int8Type(), uint64(len(gcOffsetsArray)), false),
			llvm.ConstArray(c.cxt.Int8Type(), gcOffsetsArray)},
		false))
	// initialize the llvm methods array
	methodCount := 0
	for _, method := range class.Methods {
		if method.Method.Name == token.CONSTRUCTOR_METHOD_NAME {
			continue
		}
		methodCount += 1
	}
	llvmVTableMethods := make([]llvm.Value, methodCount)
	zeusClassLLVMStruct := NewZeusClassLLVMStruct(class, llvmStructType, vtableStructType, llvmVTableMethods, objectHeaderStructType, &llvmVTable, llvmObjectHeader, nil)

	// create the vtable global
	// initialize the vtable methods to null
	// this is done here because the vtable methods are not known until we encounter the DECL_CLASS_METHOD instructions
	for llvmVTableMethodIndex := range llvmVTableMethods {
		llvmVTableMethods[llvmVTableMethodIndex] = llvm.ConstNull(llvm.PointerType(llvm.FunctionType(c.cxt.VoidType(), []llvm.Type{}, false), 0))
	}

	c.zeusClassLLVMStructMap[class.Name] = zeusClassLLVMStruct

	// if it is a primordial class, generate the primordial class
	if class.PrimordialName != "" {
		c.genPrimordialClassMethods(class)
	}

	// Declare factory function signature (body will be generated in Phase 3)
	c.declareFactoryFunction(class)

	return zeusClassLLVMStruct
}

func (c *CodegenModule) genDeclClass(input ir.DeclClassInstrInput, output zeus_value.Var) {
	zeusClassLLVMStruct := c.genClass(*input.Class)
	// track the struct info
	c.zeusClassLLVMStructMap[output.Name] = zeusClassLLVMStruct
}

func (c *CodegenModule) genNewObj(input ir.NewObjInstrInput, output zeus_value.Var) {
	callee, ok := input.Callee.(*zeus_value.Class)
	zeus_error.Assert(ok, fmt.Sprintf("callee %s is not a class", input.Callee))

	// Determine the factory function name
	// For object array types (e.g., Point[]), use the shared Object[] factory
	factoryClassName := callee.Name
	if callee.PrimordialName == zeus_value.ZEUS_PRIMORDIAL_ARRAY && callee.Name != ZeusObjectArrayClassName && zeus_value.IsObjectType(callee.ArrayElementType) {
		factoryClassName = ZeusObjectArrayClassName
	}
	factoryFunctionName := getFactoryFunctionName(factoryClassName)
	factoryFunc := c.module.NamedFunction(factoryFunctionName)
	zeus_error.Assert(!factoryFunc.IsNil(), fmt.Sprintf("factory function %s not found", factoryFunctionName))

	// Build factory function parameter types
	paramTypes := []llvm.Type{}
	for _, arg := range input.Args {
		paramTypes = append(paramTypes, c.toLLVMType(zeus_value.GetValueType(arg)))
	}
	returnType := llvm.PointerType(c.cxt.VoidType(), 0)
	factoryFunctionType := llvm.FunctionType(returnType, paramTypes, false)

	// Convert args to LLVM values
	factoryArgs := []llvm.Value{}
	for _, arg := range input.Args {
		factoryArgs = append(factoryArgs, c.toLLVMValue(arg))
	}

	// Call the factory function
	llvmStruct := c.builder.CreateCall(factoryFunctionType, factoryFunc, factoryArgs, factoryFunctionName)
	c.symbolTable.DeclareSymbol(output.Name, llvmStruct)
}

// getFactoryFunctionName returns the factory function name for a class
// e.g., "u8[]" -> "zeus_new_u8_array", "string" -> "zeus_new_string", "MyClass" -> "zeus_new_MyClass"
func getFactoryFunctionName(className string) string {
	// Replace [] with _array for array types
	mangledName := strings.ReplaceAll(className, "[]", "_array")
	return fmt.Sprintf("zeus_new_%s", mangledName)
}

// declareFactoryFunction declares the factory function signature for a class
// This is called in Phase 1 before NEW_OBJ instructions are processed
func (c *CodegenModule) declareFactoryFunction(class zeus_value.Class) llvm.Value {
	factoryFunctionName := getFactoryFunctionName(class.Name)

	// Check if already declared
	existingFunc := c.module.NamedFunction(factoryFunctionName)
	if !existingFunc.IsNil() {
		return existingFunc
	}

	// Find constructor method to get parameter types
	var constructorMethod *zeus_value.Function
	for _, method := range class.Methods {
		if method.Method.Name == token.CONSTRUCTOR_METHOD_NAME {
			constructorMethod = method.Method
			break
		}
	}

	// Build parameter types for factory function (same as constructor params)
	paramTypes := []llvm.Type{}
	if constructorMethod != nil {
		for _, param := range constructorMethod.Params {
			paramTypes = append(paramTypes, c.toLLVMType(param.ValueType))
		}
	}

	// Factory function returns a pointer to the object
	returnType := llvm.PointerType(c.cxt.VoidType(), 0)
	factoryFunctionType := llvm.FunctionType(returnType, paramTypes, false)
	factoryFunction := llvm.AddFunction(c.module, factoryFunctionName, factoryFunctionType)
	factoryFunction.SetLinkage(llvm.ExternalLinkage)

	// Store factory function reference
	structInfo := c.zeusClassLLVMStructMap[class.Name]
	if structInfo != nil {
		structInfo.LLVMFactoryFunction = &factoryFunction
	}

	return factoryFunction
}

// genFactoryFunctionBody generates the body for a factory function
// This is called in Phase 3 after all constructors are available
func (c *CodegenModule) genFactoryFunctionBody(class zeus_value.Class) {
	factoryFunctionName := getFactoryFunctionName(class.Name)
	factoryFunction := c.module.NamedFunction(factoryFunctionName)
	zeus_error.Assert(!factoryFunction.IsNil(), fmt.Sprintf("factory function %s not declared", factoryFunctionName))

	// Find constructor method to get parameter types
	var constructorMethod *zeus_value.Function
	for _, method := range class.Methods {
		if method.Method.Name == token.CONSTRUCTOR_METHOD_NAME {
			constructorMethod = method.Method
			break
		}
	}

	// Create the function body
	entryBlock := llvm.AddBasicBlock(factoryFunction, "entry")
	currentInsertionBlock := c.builder.GetInsertBlock()
	c.builder.SetInsertPointAtEnd(entryBlock)

	// Allocate memory for the object
	llvmStructType := c.getLLVMStructType(class.Name)
	llvmStruct := c.callGlobalLLVMFunction(MemAllocFunctionName, llvm.ConstInt(c.cxt.Int32Type(), c.getSizeOfClass(class), false))

	// Set up object header
	llvmStructObjHeaderField := c.builder.CreateStructGEP(llvmStructType, llvmStruct, OBJ_HEADER_STRUCT_INDEX, fmt.Sprintf("%s_header_field", class.Name))
	llvmObjHeader := c.getLLVMObjHeaderPtr(class.Name)
	c.builder.CreateStore(llvmObjHeader, llvmStructObjHeaderField)

	// Initialize properties to default values
	for propertyIndex, property := range class.Properties {
		defaultLLVMValue := c.getDefaultLLVMValue(property.Property.ValueType)
		llvmPropertyField := c.builder.CreateStructGEP(llvmStructType, llvmStruct, propertyIndex+1, fmt.Sprintf("%s_property_%s_default_value", class.Name, property.Property.Name))
		c.builder.CreateStore(defaultLLVMValue, llvmPropertyField)
	}

	// Call constructor if it exists
	llvmConstructorMethod := c.getLLVMConstructorMethod(class.Name)
	if llvmConstructorMethod != nil && constructorMethod != nil {
		constructorFunc := *llvmConstructorMethod
		constructorParamTypes := []zeus_value.ValueType{}
		for _, param := range constructorMethod.Params {
			constructorParamTypes = append(constructorParamTypes, param.ValueType)
		}
		constructorParamTypes = append(constructorParamTypes, zeus_value.NewObjectType(class))
		constructorMethodType := c.toLLVMFunctionType(zeus_value.NewFunctionType(zeus_value.VoidType{}, constructorParamTypes))

		// Get constructor params from factory function params
		constructorParams := []llvm.Value{}
		for i := 0; i < len(constructorMethod.Params); i++ {
			constructorParams = append(constructorParams, factoryFunction.Param(i))
		}
		constructorParams = append(constructorParams, llvmStruct)

		c.builder.CreateCall(constructorMethodType, constructorFunc, constructorParams, "")
	}

	// Return the created object
	c.builder.CreateRet(llvmStruct)

	// Restore insertion point
	if !currentInsertionBlock.IsNil() {
		c.builder.SetInsertPointAtEnd(currentInsertionBlock)
	}
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

// genClassMethod generates LLVM code for a class method given the method body and class
func (c *CodegenModule) genClassMethod(method zeus_value.Function, class zeus_value.Class) llvm.Value {
	methodWithThisParam := c.appendThisParamToFunction(method, class)
	isConstructor := methodWithThisParam.Name == util.GetClassMethodName(class.Name, token.CONSTRUCTOR_METHOD_NAME)
	function := c.genFunc(methodWithThisParam)

	if !isConstructor {
		// update the vtable global initializer
		structInfo := c.zeusClassLLVMStructMap[class.Name]
		structInfo.LLVMVTableMethods[structInfo.CurrentVTableMethodIndex] = function
		structInfo.CurrentVTableMethodIndex += 1
		c.getLLVMVTablePtr(class.Name).SetInitializer(llvm.ConstStruct(structInfo.LLVMVTableMethods, true))
	} else {
		// store the constructor method reference
		structInfo := c.zeusClassLLVMStructMap[class.Name]
		structInfo.LLVMConstructorMethod = &function
	}

	return function
}

func (c *CodegenModule) genDeclClassMethod(input ir.DeclClassMethodInstrInput) llvm.Value {
	return c.genClassMethod(*input.Method, *input.Class)
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

func (c *CodegenModule) genDeclPrimordialFunc(input ir.DeclPrimordialFuncInstrInput) {
	function := input.Function
	functionType := zeus_value.ToFunctionType(*function)

	// Create the external function zeus_{function_name} using the shared helper
	externalFuncName := fmt.Sprintf("zeus_%s", function.Name)
	externalFunc, externalFuncType := c.genExternalRuntimeFunction(externalFuncName, len(function.Params), false)

	// Reuse genFunc to create the wrapper function
	llvmFunc := c.genFunc(*function)
	llvmFunc.SetLinkage(llvm.InternalLinkage)

	// Create entry basic block for the function body
	entryBlock := llvm.AddBasicBlock(llvmFunc, "entry")
	c.builder.SetInsertPointAtEnd(entryBlock)

	// Alloca return buffer if function has a return type
	var returnBufferPtr llvm.Value
	hasReturnValue := functionType.ReturnType != nil && !zeus_value.IsVoidType(functionType.ReturnType)
	if hasReturnValue {
		returnBufferPtr = c.builder.CreateAlloca(c.toLLVMType(functionType.ReturnType), "return_buffer")
	} else {
		// Create a dummy pointer for void return
		returnBufferPtr = llvm.ConstNull(llvm.PointerType(c.cxt.VoidType(), 0))
	}

	// Build args for the external call: return buffer ptr + alloca'd arg ptrs
	externalCallArgs := []llvm.Value{returnBufferPtr}
	for index, param := range llvmFunc.Params() {
		// Alloca for each argument
		argAlloca := c.builder.CreateAlloca(param.Type(), fmt.Sprintf("arg_%d_ptr", index))
		c.builder.CreateStore(param, argAlloca)
		externalCallArgs = append(externalCallArgs, argAlloca)
	}

	// Call the external function
	c.builder.CreateCall(externalFuncType, externalFunc, externalCallArgs, "")

	// Return the value from return buffer if needed
	if hasReturnValue {
		returnValue := c.builder.CreateLoad(c.toLLVMType(functionType.ReturnType), returnBufferPtr, "return_value")
		c.builder.CreateRet(returnValue)
	} else {
		c.builder.CreateRetVoid()
	}
}

func (c *CodegenModule) getDefaultLLVMValue(value zeus_value.ValueType) llvm.Value {
	switch value := value.(type) {
	case zeus_value.IntType:
		return llvm.ConstInt(c.toLLVMIntType(value), 0, false)
	case zeus_value.FloatType:
		return llvm.ConstFloat(c.toLLVMFloatType(value), 0.0)
	case zeus_value.BoolType:
		return llvm.ConstInt(c.cxt.Int1Type(), 0, false)
	case zeus_value.ObjectType:
		return llvm.ConstNull(llvm.PointerType(c.cxt.VoidType(), 0))
	case zeus_value.OpaqueType:
		return llvm.ConstNull(llvm.PointerType(c.cxt.VoidType(), 0))
	default:
		panic(fmt.Sprintf("cannot get default llvm value for type: %T", value))
	}
}

func (c *CodegenModule) Generate(irBuilder ir.IRBuilder) {
	var currentFunction llvm.Value

	c.symbolTable.EnterScope()

	// Phase 1: Process all DECL_CLASS instructions first
	// This ensures all LLVM struct types are created before they're used
	processedClassIds := make(map[int]bool)
	for _, instr := range irBuilder.GetInstrs() {
		if instr.Type == ir.InstrTypeDeclClass {
			c.genDeclClass(*ir.AsDeclClassInstrInput(instr.Input), *instr.Output)
			processedClassIds[instr.Id] = true
		}
	}

	// Phase 2: Process all other instructions
	irBuilder.Walk(func(instr *ir.Instr) {
		// Skip already processed DECL_CLASS instructions
		if processedClassIds[instr.Id] {
			return
		}
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
		case ir.InstrTypeMod:
			fallthrough
		case ir.InstrTypePower:
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
			fallthrough
		case ir.InstrTypeAnd:
			fallthrough
		case ir.InstrTypeOr:
			c.genBinaryOp(instr, *ir.AsBinaryOpInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeNeg:
			fallthrough
		case ir.InstrTypeNot:
			c.genUnaryOp(instr, *ir.AsUnaryOpInstrInput(instr.Input), *instr.Output)
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
		case ir.InstrTypeDeclPrimordialFunc:
			c.genDeclPrimordialFunc(*ir.AsDeclPrimordialFuncInstrInput(instr.Input))
		default:
			panic(fmt.Sprintf("codegen for instruction %s is not implemented", instr.Type))
		}
	}, func(block *ir.BasicBlock) {
		basicBlock := c.getOrCreateBasicBlock(block.Id, currentFunction)
		c.builder.SetInsertPointAtEnd(basicBlock)
	})

	// Phase 3: Generate factory function bodies for all classes
	// This must happen after all class methods (including constructors) are processed
	for _, structInfo := range c.zeusClassLLVMStructMap {
		c.genFactoryFunctionBody(structInfo.ZeusClass)
	}

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
