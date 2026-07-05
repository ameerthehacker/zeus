package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/module"
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
	llvmValues             map[string]llvm.Value
	basicBlocks            map[int]llvm.BasicBlock
	isEntryPoint           bool
	exportedClasses        map[string]ZeusClassModule
	importedClasses        map[string]ZeusClassModule
	zeusClassLLVMStructMap map[string]*ZeusClassLLVMStruct
	targetDataLayout       llvm.TargetData
	globalLLVMFunctions    map[string]GlobalLLVMFunction
	zeusObjectTypeInfoType llvm.Type
	// Debug info
	diBuilder         *llvm.DIBuilder
	diCompileUnit     llvm.Metadata
	diFile            llvm.Metadata
	diCurrentFunction llvm.Metadata
	sourceFilePath    string
	sourceDir         string
}

func NewCodegen() *Codegen {
	ctx := llvm.NewContext()

	return &Codegen{ctx}
}

// NewMergeTarget creates a bare LLVM module used as the destination for
// llvm.LinkModules when building a single merged release module.
func (c *Codegen) NewMergeTarget(dataLayout string, triple string) llvm.Module {
	m := c.cxt.NewModule("zeus_program")
	m.SetDataLayout(dataLayout)
	m.SetTarget(triple)
	return m
}

func (c *Codegen) NewModule(name string, isEntryPoint bool, targetDataLayout llvm.TargetData) *CodegenModule {
	module := c.cxt.NewModule(name)
	module.SetDataLayout(targetDataLayout.String())
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

	// Extract directory and filename from path
	sourceDir := "."
	sourceFile := name
	if lastSlash := strings.LastIndex(name, "/"); lastSlash != -1 {
		sourceDir = name[:lastSlash]
		sourceFile = name[lastSlash+1:]
	}

	// Create debug info builder
	diBuilder := llvm.NewDIBuilder(module)

	// Create compile unit
	diCompileUnit := diBuilder.CreateCompileUnit(llvm.DICompileUnit{
		Language:       llvm.DwarfLang(0x000c), // DW_LANG_C99 (0x000c)
		File:           sourceFile,
		Dir:            sourceDir,
		Producer:       "Zeus Compiler",
		Optimized:      false,
		Flags:          "",
		RuntimeVersion: 0,
	})

	// Create file metadata
	diFile := diBuilder.CreateFile(sourceFile, sourceDir)

	// Add debug info version flag to module
	module.AddNamedMetadataOperand("llvm.module.flags",
		c.cxt.MDNode([]llvm.Metadata{
			llvm.ConstInt(c.cxt.Int32Type(), 2, false).ConstantAsMetadata(), // Warning behavior
			c.cxt.MDString("Debug Info Version"),
			llvm.ConstInt(c.cxt.Int32Type(), 3, false).ConstantAsMetadata(), // DWARF version
		}),
	)

	// Add DWARF version flag
	module.AddNamedMetadataOperand("llvm.module.flags",
		c.cxt.MDNode([]llvm.Metadata{
			llvm.ConstInt(c.cxt.Int32Type(), 2, false).ConstantAsMetadata(),
			c.cxt.MDString("Dwarf Version"),
			llvm.ConstInt(c.cxt.Int32Type(), 4, false).ConstantAsMetadata(),
		}),
	)

	return &CodegenModule{
		module:                 module,
		builder:                builder,
		cxt:                    c.cxt,
		llvmValues:             make(map[string]llvm.Value),
		basicBlocks:            make(map[int]llvm.BasicBlock),
		isEntryPoint:           isEntryPoint,
		exportedClasses:        make(map[string]ZeusClassModule),
		importedClasses:        make(map[string]ZeusClassModule),
		zeusClassLLVMStructMap: make(map[string]*ZeusClassLLVMStruct),
		targetDataLayout:       targetDataLayout,
		globalLLVMFunctions:    globalLLVMFunctions,
		zeusObjectTypeInfoType: zeusObjectInfoStructType,
		diBuilder:              diBuilder,
		diCompileUnit:          diCompileUnit,
		diFile:                 diFile,
		sourceFilePath:         name,
		sourceDir:              sourceDir,
	}
}

func (c *Codegen) setupGlobalLLVMFunctions(module llvm.Module) map[string]GlobalLLVMFunction {
	globalFunctions := make(map[string]GlobalLLVMFunction)

	// Memory allocation function (GC-tracked objects)
	memAllocFunctionType := llvm.FunctionType(llvm.PointerType(c.cxt.VoidType(), 1), []llvm.Type{c.cxt.Int32Type()}, false)
	memAllocFunction := llvm.AddFunction(module, MemAllocFunctionName, memAllocFunctionType)
	globalFunctions[MemAllocFunctionName] = GlobalLLVMFunction{memAllocFunction, memAllocFunctionType}

	// Exception handling runtime functions

	// zeus_throw(class_id: i32, object_ptr: ptr, source_file: ptr, source_line: i32) - throws an exception (noreturn)
	zeusThrowType := llvm.FunctionType(c.cxt.VoidType(), []llvm.Type{
		c.cxt.Int32Type(),                     // class_id
		llvm.PointerType(c.cxt.VoidType(), 0), // object_ptr
		llvm.PointerType(c.cxt.Int8Type(), 0), // source_file (char*)
		c.cxt.Int32Type(),                     // source_line
	}, false)
	zeusThrowFunc := llvm.AddFunction(module, "zeus_throw", zeusThrowType)
	zeusThrowFunc.AddFunctionAttr(c.cxt.CreateEnumAttribute(llvm.AttributeKindID("noreturn"), 0))
	globalFunctions["zeus_throw"] = GlobalLLVMFunction{zeusThrowFunc, zeusThrowType}

	// zeus_try_begin(jmp_buf: ptr, class_ids: ptr, num_classes: i32) -> i32 - begin try block with setjmp
	// Returns 0 for normal execution, 1 when exception is caught
	zeusTryBeginType := llvm.FunctionType(c.cxt.Int32Type(), []llvm.Type{
		llvm.PointerType(c.cxt.VoidType(), 0),  // jmp_buf pointer
		llvm.PointerType(c.cxt.Int32Type(), 0), // class_ids array
		c.cxt.Int32Type(),                      // num_classes
	}, false)
	zeusTryBeginFunc := llvm.AddFunction(module, "zeus_try_begin", zeusTryBeginType)
	// returns_twice tells LLVM this function behaves like setjmp — it can return
	// a second time (with a non-zero value) when longjmp is called with its jmp_buf.
	// Without this attribute, LLVM may optimize under the assumption that the function
	// returns exactly once, breaking longjmp semantics for non-trivial try bodies.
	zeusTryBeginFunc.AddFunctionAttr(c.cxt.CreateEnumAttribute(llvm.AttributeKindID("returns_twice"), 0))
	globalFunctions["zeus_try_begin"] = GlobalLLVMFunction{zeusTryBeginFunc, zeusTryBeginType}

	// zeus_pop_handler() - unregister exception handler
	zeusPopHandlerType := llvm.FunctionType(c.cxt.VoidType(), []llvm.Type{}, false)
	zeusPopHandlerFunc := llvm.AddFunction(module, "zeus_pop_handler", zeusPopHandlerType)
	globalFunctions["zeus_pop_handler"] = GlobalLLVMFunction{zeusPopHandlerFunc, zeusPopHandlerType}

	// zeus_get_current_exception() -> ptr - get current exception (or null)
	zeusGetCurrentExceptionType := llvm.FunctionType(llvm.PointerType(c.cxt.VoidType(), 0), []llvm.Type{}, false)
	zeusGetCurrentExceptionFunc := llvm.AddFunction(module, "zeus_get_current_exception", zeusGetCurrentExceptionType)
	globalFunctions["zeus_get_current_exception"] = GlobalLLVMFunction{zeusGetCurrentExceptionFunc, zeusGetCurrentExceptionType}

	// zeus_get_exception_object(exc: ptr) -> ptr - get error object from exception
	zeusGetExceptionObjectType := llvm.FunctionType(llvm.PointerType(c.cxt.VoidType(), 0), []llvm.Type{llvm.PointerType(c.cxt.VoidType(), 0)}, false)
	zeusGetExceptionObjectFunc := llvm.AddFunction(module, "zeus_get_exception_object", zeusGetExceptionObjectType)
	globalFunctions["zeus_get_exception_object"] = GlobalLLVMFunction{zeusGetExceptionObjectFunc, zeusGetExceptionObjectType}

	// zeus_get_exception_class_id(exc: ptr) -> i32 - get class ID from exception
	zeusGetExceptionClassIdType := llvm.FunctionType(c.cxt.Int32Type(), []llvm.Type{llvm.PointerType(c.cxt.VoidType(), 0)}, false)
	zeusGetExceptionClassIdFunc := llvm.AddFunction(module, "zeus_get_exception_class_id", zeusGetExceptionClassIdType)
	globalFunctions["zeus_get_exception_class_id"] = GlobalLLVMFunction{zeusGetExceptionClassIdFunc, zeusGetExceptionClassIdType}

	// zeus_exception_instanceof(exc: ptr, target_class_id: i32) -> i1 - check if exception is instance of class
	zeusExceptionInstanceofType := llvm.FunctionType(c.cxt.Int1Type(), []llvm.Type{llvm.PointerType(c.cxt.VoidType(), 0), c.cxt.Int32Type()}, false)
	zeusExceptionInstanceofFunc := llvm.AddFunction(module, "zeus_exception_instanceof", zeusExceptionInstanceofType)
	globalFunctions["zeus_exception_instanceof"] = GlobalLLVMFunction{zeusExceptionInstanceofFunc, zeusExceptionInstanceofType}

	// zeus_clear_exception() - clear current exception
	zeusClearExceptionType := llvm.FunctionType(c.cxt.VoidType(), []llvm.Type{}, false)
	zeusClearExceptionFunc := llvm.AddFunction(module, "zeus_clear_exception", zeusClearExceptionType)
	globalFunctions["zeus_clear_exception"] = GlobalLLVMFunction{zeusClearExceptionFunc, zeusClearExceptionType}

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
	v, ok := c.llvmValues[name]
	if !ok {
		panic(fmt.Sprintf("symbol %s not found", name))
	}
	return v
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
		// Skip lowered methods - they are handled entirely by IR lowering
		// and don't need runtime wrapper functions
		if method.IsLowered {
			continue
		}

		// Build the LLVM function using class-prefixed name; set OriginalName for constructor detection.
		scopedFn := zeus_value.NewFunction(
			util.GetClassMethodName(class.Name, method.Method.Name),
			method.Method.Params,
			method.Method.ReturnType,
			method.Method.Span,
		)
		scopedFn.OriginalName = method.Method.Name
		scopedClassMethod := zeus_value.NewClassMethod(scopedFn, method.AccessModifier)
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
		// All FunctionType values are functor objects at runtime (ptr addrspace(1))
		return llvm.PointerType(c.cxt.VoidType(), 1)
	case zeus_value.ObjectType:
		// In LLVM opaque-pointer mode the element type is irrelevant; ptr addrspace(1) is the GC pointer.
		return llvm.PointerType(c.cxt.VoidType(), 1)
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

func (c *CodegenModule) addFramePointerAttr(fn llvm.Value) {
	attr := c.cxt.CreateStringAttribute("frame-pointer", "all")
	fn.AddFunctionAttr(attr)
}

func (c *CodegenModule) genFunc(function zeus_value.Function) llvm.Value {
	// If the function was already pre-declared (by the forward-declaration phase),
	// return the existing LLVM value — calling AddFunction again would cause LLVM
	// to rename the second call to "name.1", producing a body-less stub.
	if existing, ok := c.llvmValues[function.Name]; ok {
		for index, param := range existing.Params() {
			c.llvmValues[function.Params[index].Name] = param
		}
		return existing
	}

	llvmFunc := llvm.AddFunction(c.module, function.Name, c.toLLVMFunctionType(zeus_value.ToFunctionType(function)))
	funcParams := function.Params

	for index, param := range llvmFunc.Params() {
		c.llvmValues[funcParams[index].Name] = param
	}

	c.addFramePointerAttr(llvmFunc)

	c.llvmValues[function.Name] = llvmFunc

	return llvmFunc
}

func (c *CodegenModule) genDeclFunc(input ir.DeclFuncInstrInput) llvm.Value {
	llvmFunc := c.genFunc(*input.Function)

	if c.isEntryPoint && input.Function.Name == token.MAIN_FUNCTION_NAME {
		llvmFunc.SetLinkage(llvm.ExternalLinkage)
	} else {
		// Use InternalLinkage instead of PrivateLinkage to preserve symbol names
		// for stack traces and debugging
		llvmFunc.SetLinkage(llvm.InternalLinkage)
	}

	// Create debug info for function
	if c.diBuilder != nil && input.Function.Span != nil {
		// Create subroutine type (void function type for simplicity)
		diSubroutineType := c.diBuilder.CreateSubroutineType(llvm.DISubroutineType{
			File: c.diFile,
		})

		// Create function debug info
		diFunc := c.diBuilder.CreateFunction(c.diFile, llvm.DIFunction{
			Name:         input.Function.Name,
			LinkageName:  input.Function.Name,
			File:         c.diFile,
			Line:         input.Function.Span.Start.Line,
			Type:         diSubroutineType,
			LocalToUnit:  true,
			IsDefinition: true,
			ScopeLine:    input.Function.Span.Start.Line,
			Flags:        0,
			Optimized:    false,
		})

		// Attach debug info to the function
		llvmFunc.SetSubprogram(diFunc)
		c.diCurrentFunction = diFunc
	}

	return llvmFunc
}

// setDebugLocation sets the debug location on the builder for the given span
func (c *CodegenModule) setDebugLocation(span *token.Span) {
	if c.diBuilder != nil && !c.diCurrentFunction.IsNil() && span != nil {
		c.builder.SetCurrentDebugLocation(
			uint(span.Start.Line),
			uint(span.Start.Column),
			c.diCurrentFunction,
			llvm.Metadata{},
		)
	}
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
	} else if zeus_value.IsPrimitiveType(input.Variable.ValueType) {
		c.builder.CreateStore(c.getDefaultLLVMValue(input.Variable.ValueType), variable)
	} else if zeus_value.IsObjectType(input.Variable.ValueType) || zeus_value.IsFunctionType(input.Variable.ValueType) {
		// uninitialized object/function-pointer variables must be explicitly null so that
		// null-reference checks produce deterministic results (alloca is otherwise UB)
		c.builder.CreateStore(llvm.ConstPointerNull(variableType), variable)
	}

	c.llvmValues[input.Variable.Name] = variable
}

func (c *CodegenModule) genStore(input ir.StoreInstrInput) {
	c.builder.CreateStore(c.toLLVMValue(input.Value), c.getSymbol(input.Addr.Name))
}

func (c *CodegenModule) genLoad(input ir.LoadInstrInput, output zeus_value.Var) {
	llvmValue := c.builder.CreateLoad(c.toLLVMType(input.Addr.ValueType), c.getSymbol(input.Addr.Name), input.Addr.Name)
	c.llvmValues[output.Name] = llvmValue
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
	c.llvmValues[output.Name] = llvmValue
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

// genComparisonOp generates LLVM code for comparison operations
// This reduces repetition for ==, !=, <, >, <=, >= operators
func (c *CodegenModule) genComparisonOp(left, right zeus_value.Value, name string, signedPred, unsignedPred llvm.IntPredicate, floatPred llvm.FloatPredicate) llvm.Value {
	return c.genLLVMBinaryOp(left, right, name,
		func(l, r llvm.Value, n string) llvm.Value { return c.builder.CreateICmp(signedPred, l, r, n) },
		func(l, r llvm.Value, n string) llvm.Value { return c.builder.CreateICmp(unsignedPred, l, r, n) },
		func(l, r llvm.Value, n string) llvm.Value { return c.builder.CreateFCmp(floatPred, l, r, n) },
	)
}

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
			leftVal := c.toLLVMValue(left)
			rightVal := c.toLLVMValue(right)
			// Promote to the wider type if widths differ
			if leftVal.Type() != rightVal.Type() {
				f64Type := c.cxt.DoubleType()
				if leftVal.Type() != f64Type {
					leftVal = c.builder.CreateFPExt(leftVal, f64Type, "fpext")
				}
				if rightVal.Type() != f64Type {
					rightVal = c.builder.CreateFPExt(rightVal, f64Type, "fpext")
				}
			}
			return floatFloat(leftVal, rightVal, opName)
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
	case zeus_value.FunctionType:
		// Function pointer compared with null (used by null-check lowering pass)
		leftValue := c.toLLVMValue(left)
		if zeus_value.IsNullType(rightType) {
			rightValue := llvm.ConstPointerNull(leftValue.Type())
			return intIntOp(leftValue, rightValue, opName)
		}
	case zeus_value.NullType:
		// null compared with object or function pointer (reversed order)
		if zeus_value.IsObjectType(rightType) {
			rightValue := c.toLLVMValue(right)
			leftValue := llvm.ConstPointerNull(rightValue.Type())
			return intIntOp(leftValue, rightValue, opName)
		}
		if zeus_value.IsFunctionType(rightType) {
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
		result = c.genComparisonOp(input.Left, input.Right, "eq", llvm.IntEQ, llvm.IntEQ, llvm.FloatOEQ)
	case ir.InstrTypeNotEq:
		result = c.genComparisonOp(input.Left, input.Right, "notEq", llvm.IntNE, llvm.IntNE, llvm.FloatONE)
	case ir.InstrTypeLessThan:
		result = c.genComparisonOp(input.Left, input.Right, "lessThan", llvm.IntSLT, llvm.IntULT, llvm.FloatOLT)
	case ir.InstrTypeGreaterThan:
		result = c.genComparisonOp(input.Left, input.Right, "greaterThan", llvm.IntSGT, llvm.IntUGT, llvm.FloatOGT)
	case ir.InstrTypeLessThanEq:
		result = c.genComparisonOp(input.Left, input.Right, "lessThanEq", llvm.IntSLE, llvm.IntULE, llvm.FloatOLE)
	case ir.InstrTypeGreaterThanEq:
		result = c.genComparisonOp(input.Left, input.Right, "greaterThanEq", llvm.IntSGE, llvm.IntUGE, llvm.FloatOGE)
	case ir.InstrTypeAnd:
		// Logical AND (bool && bool)
		result = c.builder.CreateAnd(c.toLLVMValue(input.Left), c.toLLVMValue(input.Right), "and")
	case ir.InstrTypeOr:
		// Logical OR (bool || bool)
		result = c.builder.CreateOr(c.toLLVMValue(input.Left), c.toLLVMValue(input.Right), "or")
	case ir.InstrTypeBitAnd:
		result = c.genLLVMBinaryOp(input.Left, input.Right, "band", c.builder.CreateAnd, c.builder.CreateAnd, nil)
	case ir.InstrTypeBitOr:
		result = c.genLLVMBinaryOp(input.Left, input.Right, "bor", c.builder.CreateOr, c.builder.CreateOr, nil)
	case ir.InstrTypeBitXor:
		result = c.genLLVMBinaryOp(input.Left, input.Right, "bxor", c.builder.CreateXor, c.builder.CreateXor, nil)
	case ir.InstrTypeShl:
		result = c.genLLVMBinaryOp(input.Left, input.Right, "shl", c.builder.CreateShl, c.builder.CreateShl, nil)
	case ir.InstrTypeShr:
		// arithmetic right shift for signed integers, logical for unsigned
		result = c.genLLVMBinaryOp(input.Left, input.Right, "shr", c.builder.CreateAShr, c.builder.CreateLShr, nil)
	}

	c.llvmValues[output.Name] = result
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
	case ir.InstrTypeBitNot:
		result = c.builder.CreateNot(llvmValue, "bitnot")
	}

	c.llvmValues[output.Name] = result
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
		if method.Method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
			scopedConstructorName := module.GetModuleScopedName(modulePath, method.Method.Name)
			constructorFunc := llvm.AddFunction(c.module, scopedConstructorName, c.toLLVMFunctionType(zeus_value.ToFunctionType(*method.Method)))
			c.addFramePointerAttr(constructorFunc)
			llvmConstructorMethod = &constructorFunc
			break
		}
	}
	zeusClassLLVMStruct := NewZeusClassLLVMStruct(class, llvmStructType, vtableStructType, make([]llvm.Value, 0), objectHeaderStructType, nil, objectHeaderInstance, llvmConstructorMethod)
	c.zeusClassLLVMStructMap[class.Name] = zeusClassLLVMStruct
	// track the struct info for the imported class
	c.importedClasses[class.Name] = NewZeusClassModule(modulePath, class)
	// declare the factory function as an external so new ImportedClass() can be used
	c.declareFactoryFunction(class)
}

func (c *CodegenModule) genImport(input ir.ImportInstrInput) {
	importedValue := input.Value

	switch importedValue := importedValue.(type) {
	case *zeus_value.Function:
		importedFunc := llvm.AddFunction(c.module, module.GetModuleScopedName(input.ModulePath, importedValue.Name), c.toLLVMFunctionType(zeus_value.ToFunctionType(*importedValue)))
		c.addFramePointerAttr(importedFunc)
		c.llvmValues[importedValue.Name] = importedFunc
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

	// ObjectType → FunctionType: both are ptr addrspace(1) at runtime — identity cast
	if zeus_value.IsObjectType(valueType) {
		if _, ok := input.CastType.(zeus_value.FunctionType); ok {
			c.llvmValues[output.Name] = c.toLLVMValue(input.Value)
			return
		}
	}

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
		switch castType := input.CastType.(type) {
		case zeus_value.IntType:
			if castType.Signed {
				result = c.builder.CreateFPToSI(c.toLLVMValue(input.Value), c.toLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
			} else {
				result = c.builder.CreateFPToUI(c.toLLVMValue(input.Value), c.toLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
			}
		case zeus_value.FloatType:
			result = c.builder.CreateFPExt(c.toLLVMValue(input.Value), c.toLLVMType(input.CastType), fmt.Sprintf("%s_cast", input.CastType))
		default:
			panic(castErrorMsg)
		}
	default:
		panic(castErrorMsg)
	}

	c.llvmValues[output.Name] = result
}

func (c *CodegenModule) createClassStructTypes(class zeus_value.Class) (llvm.Type, llvm.Type, llvm.Type, string) {
	// create the vtable struct
	vtableStructName := GetVTableStructName(class.Name)
	vtableStructType := c.cxt.StructCreateNamed(vtableStructName)

	// create object header struct
	objectHeaderStructName := GetObjectHeaderStructName(class.Name)

	objectHeaderElementTypes := []llvm.Type{
		llvm.PointerType(vtableStructType, 0),
		llvm.PointerType(c.zeusObjectTypeInfoType, 0),
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
		if method.Method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
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

	// Primordial classes (string, u8[], Error, …) are emitted identically into every compilation
	// unit. Use InternalLinkage so the linker sees only one definition per TU instead of
	// reporting duplicate-symbol errors. User-defined classes keep ExternalLinkage so they
	// can be referenced after export (genExport renames them to a module-scoped symbol).
	isPrimordial := class.PrimordialName != ""
	globalLinkage := llvm.ExternalLinkage
	if isPrimordial {
		globalLinkage = llvm.InternalLinkage
	}

	// create the vtable global
	llvmVTable := llvm.AddGlobal(c.module, vtableStructType, GetVTableStructPtrName(structName))
	llvmVTable.SetInitializer(llvm.ConstNull(vtableStructType))
	llvmVTable.SetLinkage(globalLinkage)
	// create the object type info global
	llvmObjectTypeInfo := llvm.AddGlobal(c.module, c.zeusObjectTypeInfoType, GetObjectTypeInfoStructPtrName(structName))
	llvmObjectTypeInfo.SetLinkage(globalLinkage)
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
	llvmObjectHeader.SetLinkage(globalLinkage)
	llvmObjectHeader.SetInitializer(llvm.ConstStruct(
		[]llvm.Value{llvmVTable, llvmObjectTypeInfo},
		false))
	// initialize the llvm methods array
	// Count only non-constructor, non-lowered methods for vtable
	methodCount := 0
	for _, method := range class.Methods {
		if method.Method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
			continue
		}
		if method.IsLowered {
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
	c.llvmValues[output.Name] = llvmStruct
}

// FactoryFunctionPrefix is the naming prefix for all primordial factory functions.
// The Zig runtime calls these by name (e.g. zeus_new_string), so this prefix must
// stay in sync with the runtime's extern declarations.
const FactoryFunctionPrefix = "zeus_new_"

// getFactoryFunctionName returns the factory function name for a class
// e.g., "u8[]" -> "zeus_new_u8_array", "string" -> "zeus_new_string", "MyClass" -> "zeus_new_MyClass"
func getFactoryFunctionName(className string) string {
	// Replace [] with _array for array types
	mangledName := strings.ReplaceAll(className, "[]", "_array")
	return FactoryFunctionPrefix + mangledName
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
		if method.Method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
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
	// Primordial factory functions are emitted identically into every compilation unit.
	// LinkOnceODRLinkage tells the linker to keep exactly one copy (all are identical per
	// the One Definition Rule) while keeping the symbol externally visible so the Zig
	// runtime can call zeus_new_string / zeus_new_u8_array etc.
	// User-defined class factories use ExternalLinkage so importing modules can link to them.
	if class.PrimordialName != "" {
		factoryFunction.SetLinkage(llvm.LinkOnceODRLinkage)
	} else {
		factoryFunction.SetLinkage(llvm.ExternalLinkage)
	}

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
		if method.Method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
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
	isConstructor := methodWithThisParam.SourceName() == token.CONSTRUCTOR_METHOD_NAME
	function := c.genFunc(methodWithThisParam)

	// Create debug info for method
	if c.diBuilder != nil && method.Span != nil {
		// Create subroutine type
		diSubroutineType := c.diBuilder.CreateSubroutineType(llvm.DISubroutineType{
			File: c.diFile,
		})

		// Create function debug info
		diFunc := c.diBuilder.CreateFunction(c.diFile, llvm.DIFunction{
			Name:         methodWithThisParam.Name,
			LinkageName:  methodWithThisParam.Name,
			File:         c.diFile,
			Line:         method.Span.Start.Line,
			Type:         diSubroutineType,
			LocalToUnit:  true,
			IsDefinition: true,
			ScopeLine:    method.Span.Start.Line,
			Flags:        0,
			Optimized:    false,
		})

		// Attach debug info to the function
		function.SetSubprogram(diFunc)
		c.diCurrentFunction = diFunc
	}

	// Class methods are always dispatched through vtable pointers, never called by symbol name
	// across module boundaries — InternalLinkage prevents duplicate-symbol errors at link time.
	function.SetLinkage(llvm.InternalLinkage)

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

// loadVTableMethodPtr navigates obj → header → vtable → slot[slotIndex] and returns the fn ptr.
// When objType is non-nil the precise typed structs for that class are used.
// When objType is nil it falls back to opaque-pointer GEPs for generic/unknown-class dispatch.
func (c *CodegenModule) loadVTableMethodPtr(obj llvm.Value, objType *zeus_value.ObjectType, slotIndex int, name string) llvm.Value {
	ptrType := llvm.PointerType(c.cxt.VoidType(), 0)
	if objType != nil {
		className := objType.Class.Name
		objStructType := c.getLLVMStructType(className)
		headerPtrAddr := c.builder.CreateStructGEP(objStructType, obj, OBJ_HEADER_STRUCT_INDEX, "objHeaderPtr")
		header := c.builder.CreateLoad(llvm.PointerType(c.getLLVMObjHeaderStruct(className), 0), headerPtrAddr, "objHeader")
		vtablePtrAddr := c.builder.CreateStructGEP(c.getLLVMObjHeaderStruct(className), header, VTABLE_STRUCT_INDEX, "vTablePtr")
		vtable := c.builder.CreateLoad(llvm.PointerType(c.getLLVMVTableStruct(className), 0), vtablePtrAddr, "vTable")
		slotAddr := c.builder.CreateStructGEP(c.getLLVMVTableStruct(className), vtable, slotIndex, name)
		return c.builder.CreateLoad(ptrType, slotAddr, name+"_fn_ptr")
	}
	// Generic opaque-pointer dispatch (class unknown at compile time)
	genericObjType := c.cxt.StructType([]llvm.Type{ptrType}, false)
	headerPtrAddr := c.builder.CreateStructGEP(genericObjType, obj, 0, "objHeaderPtr")
	header := c.builder.CreateLoad(ptrType, headerPtrAddr, "objHeader")
	genericHeaderType := c.cxt.StructType([]llvm.Type{ptrType, ptrType}, false)
	vtablePtrAddr := c.builder.CreateStructGEP(genericHeaderType, header, 0, "vTablePtr")
	vtable := c.builder.CreateLoad(ptrType, vtablePtrAddr, "vTable")
	slotTypes := make([]llvm.Type, slotIndex+1)
	for i := range slotTypes {
		slotTypes[i] = ptrType
	}
	slotAddr := c.builder.CreateStructGEP(c.cxt.StructType(slotTypes, false), vtable, slotIndex, name)
	return c.builder.CreateLoad(ptrType, slotAddr, name+"_fn_ptr")
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
		c.llvmValues[output.Name] = c.loadVTableMethodPtr(llvmValue, objectClass, methodIndex, input.Property)
		return
	}
	llvmValue = c.builder.CreateStructGEP(c.toLLVMStructType(objectType), llvmValue, propertyIndex, input.Property)
	c.llvmValues[output.Name] = llvmValue
}

func (c *CodegenModule) genIndirectFuncCall(input ir.IndirectFuncCallInstrInput, output zeus_value.Var) {
	functionType := zeus_value.AsFunctionType(zeus_value.GetValueType(input.Function))
	zeus_error.Assert(functionType != nil, fmt.Sprintf("INDIRECT_FUNC_CALL: %s is not a FunctionType", input.Function))

	// All FunctionType values are functor objects (ptr addrspace(1)); dispatch via vtable slot 0.
	functorObj := c.toLLVMValue(input.Function)
	methodPtr := c.loadVTableMethodPtr(functorObj, nil, 0, "__call__")

	functionArgs := make([]llvm.Value, 0, len(input.Args)+1)
	for _, arg := range input.Args {
		functionArgs = append(functionArgs, c.toLLVMValue(arg))
	}
	functionArgs = append(functionArgs, functorObj)

	ptrAs1Type := llvm.PointerType(c.cxt.VoidType(), 1)
	llvmParamTypes := make([]llvm.Type, len(functionType.ParamTypes)+1)
	for i, pt := range functionType.ParamTypes {
		llvmParamTypes[i] = c.toLLVMType(pt)
	}
	llvmParamTypes[len(functionType.ParamTypes)] = ptrAs1Type
	callType := llvm.FunctionType(c.toLLVMType(functionType.ReturnType), llvmParamTypes, false)

	llvmValue := c.builder.CreateCall(callType, methodPtr, functionArgs, "indirect_call")
	c.llvmValues[output.Name] = llvmValue
}

func (c *CodegenModule) genMethodCall(input ir.MethodCallInstrInput, output zeus_value.Var) {
	objectType := c.getValueType(input.Object)
	llvmObject := c.toLLVMValue(input.Object)
	objectClass := zeus_value.AsObjectType(objectType)
	zeus_error.Assert(objectClass != nil, fmt.Sprintf("CALL_METHOD receiver is not an object: %s", input.Object))

	methodIndex := util.GetMethodIndex(objectClass.Class, input.MethodName)
	zeus_error.Assert(methodIndex != -1, fmt.Sprintf("method %s not found in class %s", input.MethodName, objectClass.Class.Name))

	methodPtr := c.loadVTableMethodPtr(llvmObject, objectClass, methodIndex, input.MethodName)

	functionArgs := []llvm.Value{}
	for _, arg := range input.Args {
		functionArgs = append(functionArgs, c.toLLVMValue(arg))
	}
	functionArgs = append(functionArgs, llvmObject)

	var foundMethod *zeus_value.Function
	for _, m := range objectClass.Class.Methods {
		if m.Method.SourceName() == input.MethodName {
			foundMethod = m.Method
			break
		}
	}
	zeus_error.Assert(foundMethod != nil, fmt.Sprintf("method %s not found in class %s for codegen", input.MethodName, objectClass.Class.Name))

	paramTypes := make([]zeus_value.ValueType, len(foundMethod.Params))
	for i, p := range foundMethod.Params {
		paramTypes[i] = p.ValueType
	}
	paramTypes = append(paramTypes, zeus_value.NewObjectType(objectClass.Class))
	fullFunctionType := zeus_value.FunctionType{
		ReturnType: foundMethod.ReturnType,
		ParamTypes: paramTypes,
	}

	llvmValue := c.builder.CreateCall(c.toLLVMFunctionType(fullFunctionType), methodPtr, functionArgs, fmt.Sprintf("%s_result", input.MethodName))
	c.llvmValues[output.Name] = llvmValue
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
	case zeus_value.FunctionType:
		return llvm.ConstNull(llvm.PointerType(c.cxt.VoidType(), 0))
	default:
		panic(fmt.Sprintf("cannot get default llvm value for type: %T", value))
	}
}

func (c *CodegenModule) Generate(irBuilder ir.IRBuilder) {
	var currentFunction llvm.Value

	// Phase 1: Process all DECL_CLASS instructions first
	// This ensures all LLVM struct types are created before they're used
	processedClassIds := make(map[int]bool)
	for _, instr := range irBuilder.GetInstrs() {
		if instr.Type == ir.InstrTypeDeclClass {
			c.genDeclClass(*ir.AsDeclClassInstrInput(instr.Input), *instr.Output)
			processedClassIds[instr.Id] = true
		}
	}

	// Phase 2: Pre-declare all user functions and class methods so forward calls resolve.
	// Walk processes each function's body immediately after its DECL_FUNC, so without this
	// a call to a later-declared function would not find an LLVM value.
	for _, instr := range irBuilder.GetInstrs() {
		switch instr.Type {
		case ir.InstrTypeDeclFunc:
			input := ir.AsDeclFuncInstrInput(instr.Input)
			c.genFunc(*input.Function)
		case ir.InstrTypeDeclClassMethod:
			input := ir.AsDeclClassMethodInstrInput(instr.Input)
			c.genFunc(c.appendThisParamToFunction(*input.Method, *input.Class))
		}
	}

	// Phase 3: Process all other instructions
	irBuilder.Walk(func(instr *ir.Instr) {
		// Skip already processed DECL_CLASS instructions
		if processedClassIds[instr.Id] {
			return
		}
		switch instr.Type {
		case ir.InstrTypeDeclFunc:
			currentFunction = c.genDeclFunc(*ir.AsDeclFuncInstrInput(instr.Input))
		case ir.InstrTypeDeclClassMethod:
			currentFunction = c.genDeclClassMethod(*ir.AsDeclClassMethodInstrInput(instr.Input))
		case ir.InstrTypeDeclVar:
			c.setDebugLocation(instr.Span)
			c.genDeclVar(*ir.AsDeclVarInstrInput(instr.Input))
		case ir.InstrTypeStore:
			c.setDebugLocation(instr.Span)
			c.genStore(*ir.AsStoreInstrInput(instr.Input))
		case ir.InstrTypeReturn:
			c.setDebugLocation(instr.Span)
			c.genReturn(*ir.AsReturnInstrInput(instr.Input))
		case ir.InstrTypeLoad:
			c.setDebugLocation(instr.Span)
			c.genLoad(*ir.AsLoadInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeCallFunc:
			c.setDebugLocation(instr.Span)
			c.genCallFunc(*ir.AsCallFuncInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeJmp:
			c.setDebugLocation(instr.Span)
			c.genJmp(*ir.AsJmpInstrInput(instr.Input))
		case ir.InstrTypeCondJmp:
			c.setDebugLocation(instr.Span)
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
			fallthrough
		case ir.InstrTypeBitAnd:
			fallthrough
		case ir.InstrTypeBitOr:
			fallthrough
		case ir.InstrTypeBitXor:
			fallthrough
		case ir.InstrTypeShl:
			fallthrough
		case ir.InstrTypeShr:
			c.setDebugLocation(instr.Span)
			c.genBinaryOp(instr, *ir.AsBinaryOpInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeNeg:
			fallthrough
		case ir.InstrTypeNot:
			fallthrough
		case ir.InstrTypeBitNot:
			c.setDebugLocation(instr.Span)
			c.genUnaryOp(instr, *ir.AsUnaryOpInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeCast:
			c.setDebugLocation(instr.Span)
			c.genCast(*ir.AsCastInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeExport:
			c.genExport(*ir.AsExportInstrInput(instr.Input))
		case ir.InstrTypeImport:
			c.genImport(*ir.AsImportInstrInput(instr.Input))
		case ir.InstrTypeDeclClass:
			c.genDeclClass(*ir.AsDeclClassInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeNewObj:
			c.setDebugLocation(instr.Span)
			c.genNewObj(*ir.AsNewObjInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeObjectPropertyAccess:
			c.setDebugLocation(instr.Span)
			c.genObjectPropertyAccess(*ir.AsObjectPropertyAccessInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeIndirectFuncCall:
			c.setDebugLocation(instr.Span)
			c.genIndirectFuncCall(*ir.AsIndirectFuncCallInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeMethodCall:
			c.setDebugLocation(instr.Span)
			c.genMethodCall(*ir.AsMethodCallInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeDeclPrimordialFunc:
			c.genDeclPrimordialFunc(*ir.AsDeclPrimordialFuncInstrInput(instr.Input))
		// Exception handling instructions
		case ir.InstrTypeThrow:
			c.setDebugLocation(instr.Span)
			c.genThrow(*ir.AsThrowInstrInput(instr.Input))
		case ir.InstrTypePushHandler:
			c.setDebugLocation(instr.Span)
			input := ir.AsPushHandlerInstrInput(instr.Input)
			c.genPushHandler(*input, currentFunction)
		case ir.InstrTypePopHandler:
			c.genPopHandler()
		case ir.InstrTypeCheckException:
			c.genCheckException(*ir.AsCheckExceptionInstrInput(instr.Input))
		case ir.InstrTypeGetException:
			c.genGetException(*instr.Output)
		case ir.InstrTypeClearException:
			c.genClearException()
		case ir.InstrTypeCoerce:
			// Zero-cost type annotation: output maps to the same LLVM value as input.
			input := ir.AsCoerceInstrInput(instr.Input)
			c.llvmValues[instr.Output.Name] = c.toLLVMValue(input.Value)
		default:
			panic(fmt.Sprintf("codegen for instruction %s is not implemented", instr.Type))
		}
	}, func(block *ir.BasicBlock) {
		basicBlock := c.getOrCreateBasicBlock(block.Id, currentFunction)
		c.builder.SetInsertPointAtEnd(basicBlock)
	})

	// Phase 3: Generate factory function bodies for locally-defined classes only.
	// Imported classes already have their factory bodies in the exporting module.
	for _, structInfo := range c.zeusClassLLVMStructMap {
		if _, isImported := c.importedClasses[structInfo.ZeusClass.Name]; isImported {
			continue
		}
		c.genFactoryFunctionBody(structInfo.ZeusClass)
	}

	// Finalize debug info
	if c.diBuilder != nil {
		c.diBuilder.Finalize()
	}
}

// DumpIR returns the LLVM IR as a string for debugging
func (c *CodegenModule) DumpIR() string {
	return c.module.String()
}

// genThrow generates LLVM code for throwing an exception
func (c *CodegenModule) genThrow(input ir.ThrowInstrInput) {
	// Get the class ID as an i32 constant
	classIdValue := llvm.ConstInt(c.cxt.Int32Type(), uint64(input.ClassId), false)

	// Get the object pointer
	objectPtr := c.toLLVMValue(input.ObjectPtr)

	// Create source file string global
	sourceFileStr := llvm.ConstString(input.SourceFile, true) // null-terminated
	sourceFileGlobal := llvm.AddGlobal(c.module, sourceFileStr.Type(), "throw_source_file")
	sourceFileGlobal.SetInitializer(sourceFileStr)
	sourceFileGlobal.SetLinkage(llvm.PrivateLinkage)
	sourceFileGlobal.SetGlobalConstant(true)

	// Get pointer to source file string
	sourceFilePtr := c.builder.CreateInBoundsGEP(
		sourceFileStr.Type(),
		sourceFileGlobal,
		[]llvm.Value{
			llvm.ConstInt(c.cxt.Int32Type(), 0, false),
			llvm.ConstInt(c.cxt.Int32Type(), 0, false),
		},
		"source_file_ptr",
	)

	// Get source line as i32 constant
	sourceLineValue := llvm.ConstInt(c.cxt.Int32Type(), uint64(input.SourceLine), false)

	// Call zeus_throw(class_id, object_ptr, source_file, source_line)
	c.callGlobalLLVMFunction("zeus_throw", classIdValue, objectPtr, sourceFilePtr, sourceLineValue)

	// Mark as unreachable (zeus_throw is noreturn)
	c.builder.CreateUnreachable()
}

// genPushHandler generates LLVM code for registering an exception handler
// This uses setjmp/longjmp to implement try-catch semantics
func (c *CodegenModule) genPushHandler(input ir.PushHandlerInstrInput, currentFunction llvm.Value) {
	// Create a global array of class IDs
	classIdValues := make([]llvm.Value, len(input.ClassIds))
	for i, classId := range input.ClassIds {
		classIdValues[i] = llvm.ConstInt(c.cxt.Int32Type(), uint64(classId), false)
	}

	// Create a constant array of class IDs
	classIdsArrayType := llvm.ArrayType(c.cxt.Int32Type(), len(input.ClassIds))
	classIdsArray := llvm.ConstArray(c.cxt.Int32Type(), classIdValues)

	// Create a global variable for the class IDs array
	globalClassIds := llvm.AddGlobal(c.module, classIdsArrayType, "exception_class_ids")
	globalClassIds.SetInitializer(classIdsArray)
	globalClassIds.SetLinkage(llvm.PrivateLinkage)
	globalClassIds.SetGlobalConstant(true)

	// Get pointer to first element
	classIdsPtr := c.builder.CreateInBoundsGEP(classIdsArrayType, globalClassIds, []llvm.Value{
		llvm.ConstInt(c.cxt.Int32Type(), 0, false),
		llvm.ConstInt(c.cxt.Int32Type(), 0, false),
	}, "class_ids_ptr")

	// Get number of classes
	numClasses := llvm.ConstInt(c.cxt.Int32Type(), uint64(len(input.ClassIds)), false)

	// Allocate jmp_buf on the stack
	// jmp_buf is typically an array of platform-specific size
	// On most platforms, we can use a large enough array (256 bytes should be safe)
	jmpBufType := llvm.ArrayType(c.cxt.Int8Type(), 256)
	jmpBuf := c.builder.CreateAlloca(jmpBufType, "jmp_buf")

	// Cast to void pointer for the runtime function
	jmpBufPtr := c.builder.CreateBitCast(jmpBuf, llvm.PointerType(c.cxt.VoidType(), 0), "jmp_buf_ptr")

	// Call zeus_try_begin(jmp_buf, class_ids_ptr, num_classes) -> i32
	// Returns 0 for normal execution, 1 when exception is caught
	tryBeginFunc := c.globalLLVMFunctions["zeus_try_begin"]
	setjmpResult := c.builder.CreateCall(tryBeginFunc.FunctionType, tryBeginFunc.Function,
		[]llvm.Value{jmpBufPtr, classIdsPtr, numClasses}, "setjmp_result")

	// Get the try body block
	tryBodyBlock := c.getOrCreateBasicBlock(input.TryBodyBlock.Id, currentFunction)

	// Get the handler block (must use getOrCreateBasicBlock to ensure it exists)
	handlerBlock := c.getOrCreateBasicBlock(input.HandlerBlock.Id, currentFunction)

	// Branch based on setjmp result: if 0, go to try body; else go to catch handler
	zero := llvm.ConstInt(c.cxt.Int32Type(), 0, false)
	cmp := c.builder.CreateICmp(llvm.IntEQ, setjmpResult, zero, "is_normal")
	c.builder.CreateCondBr(cmp, tryBodyBlock, handlerBlock)
}

// genPopHandler generates LLVM code for unregistering an exception handler
func (c *CodegenModule) genPopHandler() {
	c.callGlobalLLVMFunction("zeus_pop_handler")
}

// genCheckException generates LLVM code for checking if an exception is pending
func (c *CodegenModule) genCheckException(input ir.CheckExceptionInstrInput) {
	// Call zeus_get_current_exception()
	exc := c.callGlobalLLVMFunction("zeus_get_current_exception")

	// Check if exception is not null
	nullPtr := llvm.ConstPointerNull(llvm.PointerType(c.cxt.VoidType(), 0))
	hasException := c.builder.CreateICmp(llvm.IntNE, exc, nullPtr, "has_exception")

	// Get or create the basic blocks
	handlerBlock := c.basicBlocks[input.HandlerBlock.Id]
	continueBlock := c.basicBlocks[input.ContinueBlock.Id]

	// Branch to handler if exception, otherwise continue
	c.builder.CreateCondBr(hasException, handlerBlock, continueBlock)
}

// genGetException generates LLVM code for getting the current exception object
func (c *CodegenModule) genGetException(output zeus_value.Var) {
	// Call zeus_get_current_exception()
	exc := c.callGlobalLLVMFunction("zeus_get_current_exception")

	// Call zeus_get_exception_object(exc) to get the actual Error object
	errorObj := c.callGlobalLLVMFunction("zeus_get_exception_object", exc)

	// Store in output variable
	c.llvmValues[output.Name] = errorObj
}

// genClearException generates LLVM code for clearing the current exception
func (c *CodegenModule) genClearException() {
	c.callGlobalLLVMFunction("zeus_clear_exception")
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
