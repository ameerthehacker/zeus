package codegen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
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
	// allClasses is the whole-program set of user + non-array-primordial classes, deduped by
	// id. It is set once (CollectClasses) before the per-module codegen loop, so an interface
	// dispatch table built in any module can cover conformers defined in *other* modules. Without
	// this, a library function that dispatches through an interface would miss a conforming class
	// defined in a client module (the class wouldn't be in the library module's local struct map).
	allClasses []*zeus_value.Class
	// dispatchedInterfaces records every interface dispatched anywhere in the program (and which
	// tables it needs). Dispatch sites emit an external *declaration* of the table and record it
	// here; a single dedicated itable module then emits the *definitions*. This keeps the
	// whole-program itables in ONE object file so ordinary module .o files stay cacheable.
	dispatchedInterfaces map[int]*interfaceUsage
	// interfaceLayouts memoizes each interface's dispatch layout (keyed by Interface.Id). The
	// layout is identical for every call in a compilation (it is computed over the fixed
	// whole-program class set), so it is built once instead of at each declaration/definition/digest.
	interfaceLayouts map[int]*zeus_value.InterfaceDispatchLayout
}

// interfaceLayout returns iface's dispatch layout, computed once over the whole-program candidate
// classes and memoized.
func (c *Codegen) interfaceLayout(iface *zeus_value.Interface) *zeus_value.InterfaceDispatchLayout {
	if layout, ok := c.interfaceLayouts[iface.Id]; ok {
		return layout
	}
	layout := zeus_value.BuildInterfaceDispatchLayout(iface, c.allClasses)
	c.interfaceLayouts[iface.Id] = layout
	return layout
}

// interfaceUsage tracks which dispatch tables an interface needs (method call vs property access).
type interfaceUsage struct {
	iface       *zeus_value.Interface
	needsMethod bool
	needsProp   bool
}

// recordInterfaceMethodTable notes that iface's method dispatch table is referenced somewhere.
func (c *Codegen) recordInterfaceMethodTable(iface *zeus_value.Interface) {
	u := c.dispatchedInterfaces[iface.Id]
	if u == nil {
		u = &interfaceUsage{iface: iface}
		c.dispatchedInterfaces[iface.Id] = u
	}
	u.needsMethod = true
}

// recordInterfacePropTable notes that iface's property dispatch table is referenced somewhere.
func (c *Codegen) recordInterfacePropTable(iface *zeus_value.Interface) {
	u := c.dispatchedInterfaces[iface.Id]
	if u == nil {
		u = &interfaceUsage{iface: iface}
		c.dispatchedInterfaces[iface.Id] = u
	}
	u.needsProp = true
}

// HasDispatchedInterfaces reports whether any interface dispatch happened, i.e. whether a
// dedicated itable module needs to be emitted.
func (c *Codegen) HasDispatchedInterfaces() bool {
	return len(c.dispatchedInterfaces) > 0
}

// DispatchesInterface reports whether this module emitted an interface dispatch, so its object
// file bakes in itable-content-dependent constants (method slot, itable stride) and must be
// re-keyed when the itable content changes. See the dispatchesInterface field.
func (c *CodegenModule) DispatchesInterface() bool {
	return c.dispatchesInterface
}

// CollectClasses records the whole-program class set (every DECL_CLASS across all modules, deduped
// by id) so interface itables are built over all conformers, not just one module's own classes.
// Call before the per-module codegen loop.
func (c *Codegen) CollectClasses(builders []*ir.IRBuilder) {
	// Reset per-compilation state so a reused Codegen (e.g. the LSP) doesn't carry stale records.
	c.dispatchedInterfaces = make(map[int]*interfaceUsage)
	c.interfaceLayouts = make(map[int]*zeus_value.InterfaceDispatchLayout)
	seen := make(map[int]bool)
	c.allClasses = c.allClasses[:0]
	for _, builder := range builders {
		if builder == nil {
			continue
		}
		for _, instr := range builder.GetInstrs() {
			if instr.Type != ir.InstrTypeDeclClass {
				continue
			}
			cls := ir.AsDeclClassInstrInput(instr.Input).Class
			if cls == nil || seen[cls.Id] {
				continue
			}
			seen[cls.Id] = true
			c.allClasses = append(c.allClasses, cls)
		}
	}
}

// BuildInterfaceTableModule creates the dedicated module that DEFINES every interface dispatch
// table referenced in the program, returning it with a content digest used as its object-file
// cache key. Because the digest hashes the tables' inputs (not source text), the itable object is
// rebuilt only when its contents actually change and is never shared across programs with
// different contents. Returns (nil, "") when no interface is dispatched. Call after the per-module
// codegen loop, once every dispatch site has recorded its interface.
func (c *Codegen) BuildInterfaceTableModule(name string, dataLayout llvm.TargetData) (*CodegenModule, string) {
	if !c.HasDispatchedInterfaces() {
		return nil, ""
	}
	module := c.NewModule(name, false, dataLayout)
	module.DefineDispatchedInterfaceTables()
	return module, c.interfaceTablesDigest()
}

// interfaceTablesDigest hashes everything that determines the itable contents: each dispatched
// interface (id + which tables it needs) and, per conforming class, its id, the vtable slots /
// field indices, and the field types that fix property byte offsets. It is independent of source
// text, so unrelated edits don't invalidate the itable object file.
func (c *Codegen) interfaceTablesDigest() string {
	ids := make([]int, 0, len(c.dispatchedInterfaces))
	for id := range c.dispatchedInterfaces {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	h := sha256.New()
	for _, id := range ids {
		usage := c.dispatchedInterfaces[id]
		fmt.Fprintf(h, "iface %d m=%t p=%t\n", id, usage.needsMethod, usage.needsProp)
		// The interface's member list (order + count) fixes each method's interface slot and the
		// itable stride — both baked into dispatch sites — so hash it even when there are no
		// conforming classes (which would otherwise leave the rows below empty).
		for _, method := range zeus_value.InterfaceMethods(usage.iface) {
			fmt.Fprintf(h, "  im %s\n", method.SourceName())
		}
		for _, prop := range zeus_value.InterfaceProperties(usage.iface) {
			fmt.Fprintf(h, "  ip %s ro=%t\n", prop.Property.Name, prop.IsReadonly)
		}
		layout := c.interfaceLayout(usage.iface)
		fmt.Fprintf(h, "max %d\n", layout.MaxClassId)
		for _, row := range layout.Rows {
			fmt.Fprintf(h, "row %d slots=%v props=%v\n", row.ClassId, row.MethodSlots, row.PropertyBackings)
			for _, field := range row.Class.Layout().Fields {
				fmt.Fprintf(h, "  f %s\n", field.Property.ValueType.String())
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

type ZeusClassLLVMStruct struct {
	ZeusClass               zeus_value.Class
	LLVMStructType          llvm.Type
	LLVMVTableStructType    llvm.Type
	LLVMVTableMethods       []llvm.Value
	LLVMObjHeaderStructType llvm.Type
	LLVMVTableInstance      *llvm.Value
	LLVMObjHeaderInstance   llvm.Value
	LLVMConstructorMethod   *llvm.Value
	LLVMFactoryFunction     *llvm.Value
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
	return &ZeusClassLLVMStruct{zeusClass, llvmStruct, llvmVTableStruct, llvmVTableMethods, llvmObjHeaderStruct, llvmVTableInstance, llvmObjHeaderInstance, llvmConstructorMethod, nil}
}

const MemAllocFunctionName = "zeus_gc_alloc"
const ZeusObjectTypeInfoStructName = "ZeusObjectTypeInfo"
const ZeusObjectClassName = "Object"
const ZeusObjectArrayClassName = ZeusObjectClassName + "[]"

type CodegenModule struct {
	module     llvm.Module
	builder    llvm.Builder
	cxt        llvm.Context
	llvmValues map[string]llvm.Value
	// llvmFunctions is a separate namespace for functions so their (uniquified) IR names
	// can never collide with parameter/variable names in llvmValues — primordial method
	// params use non-unique literal names (e.g. "count", "value") that would otherwise
	// shadow a user function of the same name and crash codegen.
	llvmFunctions          map[string]llvm.Value
	basicBlocks            map[int]llvm.BasicBlock
	isEntryPoint           bool
	exportedClasses        map[string]ZeusClassModule
	importedClasses        map[string]ZeusClassModule
	zeusClassLLVMStructMap map[string]*ZeusClassLLVMStruct
	// codegen is the parent Codegen (shared across all modules of one compilation). It owns the
	// whole-program class set and interface-usage records; dispatch sites and itable emission read
	// them through this back-reference.
	codegen *Codegen
	// dispatchesInterface is set when this module emits an interface dispatch (it bakes in the
	// interface's method slot and itable stride — data derived from the interface definition, which
	// may live in another file). Such a module's object file must be invalidated when the itable
	// contents change, so the compiler salts its cache key with the itable digest.
	dispatchesInterface bool
	// interfaceDispatch/interfacePropDispatch memoize the per-interface method/property
	// dispatch-table globals (keyed by Interface.Id) so each is built once per module.
	interfaceDispatch      map[int]*interfaceDispatchInfo
	interfacePropDispatch  map[int]*interfacePropDispatchInfo
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

	return &Codegen{
		cxt:                  ctx,
		dispatchedInterfaces: make(map[int]*interfaceUsage),
		interfaceLayouts:     make(map[int]*zeus_value.InterfaceDispatchLayout),
	}
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
		// type id — i32 so the class-id space isn't capped at 255 (interface dispatch keys on
		// it). Must match runtime/abi.zig ZeusObjectTypeInfo.object_type_id (u32).
		c.cxt.Int32Type(),
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
		llvmFunctions:          make(map[string]llvm.Value),
		basicBlocks:            make(map[int]llvm.BasicBlock),
		isEntryPoint:           isEntryPoint,
		exportedClasses:        make(map[string]ZeusClassModule),
		importedClasses:        make(map[string]ZeusClassModule),
		zeusClassLLVMStructMap: make(map[string]*ZeusClassLLVMStruct),
		codegen:                c,
		interfaceDispatch:      make(map[int]*interfaceDispatchInfo),
		interfacePropDispatch:  make(map[int]*interfacePropDispatchInfo),
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

func (c *CodegenModule) getFunctionSymbol(name string) llvm.Value {
	v, ok := c.llvmFunctions[name]
	if !ok {
		panic(fmt.Sprintf("function %s not found", name))
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

// emitExternMethods generates the runtime-forwarding body for every extern method on the
// class (see ClassMethod.IsExtern). Non-extern methods (user-defined and lowered) are
// skipped, so this runs harmlessly for every class — primordials are just the classes that
// happen to have extern methods.
func (c *CodegenModule) emitExternMethods(class zeus_value.Class) {
	currentInsertionBlock := c.builder.GetInsertBlock()

	for _, method := range class.Methods {
		if !method.IsExtern {
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

		// The method body is an extern call into the Zig runtime.
		c.emitExternMethodBody(classFunction, method.Method)
	}
	c.builder.SetInsertPointAtEnd(currentInsertionBlock)
}

// emitExternMethodBody fills an already-declared class-method LLVM function with a body that
// forwards to the Zig runtime: it packs [this, return-buffer, ...arg pointers], calls the
// runtime function for (primordialName, method), and returns the value the runtime writes
// back through the buffer. This is the single "a method whose body is a runtime call"
// primitive that primordial classes are built from — the seed of treating primordials as
// ordinary classes whose methods happen to be extern.
func (c *CodegenModule) emitExternMethodBody(classFunction llvm.Value, method *zeus_value.Function) {
	// Thin wrappers around runtime functions; inline them away entirely.
	alwaysInlineKind := llvm.AttributeKindID("alwaysinline")
	classFunction.AddAttributeAtIndex(-1, c.cxt.CreateEnumAttribute(alwaysInlineKind, 0))

	basicBlock := llvm.AddBasicBlock(classFunction, "entry")
	c.builder.SetInsertPointAtEnd(basicBlock)

	// The runtime symbol is recorded on the method itself (self-describing extern method).
	runtimeFunction, runtimeFuncType := c.genExternalRuntimeFunction(method.ExternRuntimeName, len(method.Params), true)

	// 'this' is the last parameter.
	params := classFunction.Params()
	thisPtr := params[len(params)-1]

	// Runtime ABI: [this_ptr, return_buffer_ptr_ptr, ...param_ptrs].
	var runtimeArgs []llvm.Value
	var returnBufferPtrPtr llvm.Value
	if !zeus_value.IsVoidType(method.ReturnType) {
		voidPtrType := llvm.PointerType(c.cxt.VoidType(), 0)
		returnBufferPtrPtr = c.builder.CreateAlloca(voidPtrType, "return_buffer_ptr_ptr")
		runtimeArgs = []llvm.Value{thisPtr, returnBufferPtrPtr}
	} else {
		nullPtr := llvm.ConstNull(llvm.PointerType(c.cxt.VoidType(), 0))
		runtimeArgs = []llvm.Value{thisPtr, nullPtr}
	}

	for i, param := range method.Params {
		paramType := c.toLLVMType(param.ValueType)
		paramAlloca := c.builder.CreateAlloca(paramType, param.Name+"_alloca")
		c.builder.CreateStore(params[i], paramAlloca)
		runtimeArgs = append(runtimeArgs, paramAlloca)
	}

	c.builder.CreateCall(runtimeFuncType, runtimeFunction, runtimeArgs, "")

	if !zeus_value.IsVoidType(method.ReturnType) {
		voidPtrType := llvm.PointerType(c.cxt.VoidType(), 0)
		returnWrapperPtr := c.builder.CreateLoad(voidPtrType, returnBufferPtrPtr, "return_wrapper_ptr")
		returnType := c.toLLVMType(method.ReturnType)
		zeusObjPtr, zeusObjType := c.deserializeZeusObj(returnWrapperPtr, []llvm.Type{returnType}, "return_wrapper")
		resultFieldPtr := c.builder.CreateStructGEP(zeusObjType, zeusObjPtr, 1, "result_field_ptr")
		returnValue := c.builder.CreateLoad(returnType, resultFieldPtr, "return_value")
		c.builder.CreateRet(returnValue)
	} else {
		c.builder.CreateRetVoid()
	}
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
		cls := c.getZeusClass(valueType.Name)
		return zeus_value.NewObjectType(&cls)
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
		return c.getFunctionSymbol(value.Name)
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
	case zeus_value.InterfaceType:
		// An interface value is represented exactly like an object: a GC object pointer.
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
	if existing, ok := c.llvmFunctions[function.Name]; ok {
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

	c.llvmFunctions[function.Name] = llvmFunc

	return llvmFunc
}

func (c *CodegenModule) genDeclFunc(input ir.DeclFuncInstrInput) llvm.Value {
	llvmFunc := c.genFunc(*input.Function)

	// #_zeus_main is the compiler-generated OS entry point; `#` is not a valid Zeus
	// identifier character so users can never define a function with this name.
	if input.Function.Name == token.ZEUS_ENTRY_FUNCTION_NAME {
		llvmFunc.SetLinkage(llvm.ExternalLinkage)
	} else if strings.HasPrefix(input.Function.Name, util.FactoryFunctionPrefix) {
		// Synthesized class factory (zeus_new_<Class>, from FactoryLoweringPass): external so a
		// `new` in an importing module links to it — mirroring the old declareFactoryFunction
		// linkage. (Harmless for non-exported classes, exactly as before.)
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

func (c *CodegenModule) genDeclGlobalVar(input ir.DeclareVarInstrInput) {
	llvmType := c.toLLVMType(input.Variable.ValueType)
	global := llvm.AddGlobal(c.module, llvmType, input.Variable.Name)
	global.SetLinkage(llvm.InternalLinkage)
	global.SetInitializer(llvm.ConstNull(llvmType))
	c.llvmValues[input.Variable.Name] = global
	if input.Initializer != nil {
		c.builder.CreateStore(c.toLLVMValue(input.Initializer), global)
	} else if zeus_value.IsPrimitiveType(input.Variable.ValueType) {
		c.builder.CreateStore(c.getDefaultLLVMValue(input.Variable.ValueType), global)
	} else if zeus_value.IsObjectType(input.Variable.ValueType) || zeus_value.IsFunctionType(input.Variable.ValueType) {
		c.builder.CreateStore(llvm.ConstPointerNull(llvmType), global)
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
	case zeus_value.ObjectType, zeus_value.InterfaceType:
		// Object/interface comparison (pointer comparison).
		// Handles: value == null, value != null, value == value. An interface value is
		// represented as an object pointer, so it compares identically to an object.
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
		// null compared with object/interface or function pointer (reversed order)
		if zeus_value.IsObjectType(rightType) || zeus_value.IsInterfaceType(rightType) {
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
		c.llvmFunctions[importedValue.Name] = importedFunc
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

	src := c.toLLVMValue(input.Value)
	dstLLVMType := c.toLLVMType(input.CastType)
	name := fmt.Sprintf("%s_cast", input.CastType)

	switch valueType := valueType.(type) {
	case zeus_value.IntType:
		switch castType := input.CastType.(type) {
		case zeus_value.FloatType:
			// Signedness of the *source* selects signed vs unsigned int→float.
			if valueType.Signed {
				result = c.builder.CreateSIToFP(src, dstLLVMType, name)
			} else {
				result = c.builder.CreateUIToFP(src, dstLLVMType, name)
			}
		case zeus_value.IntType:
			// LLVM iN is signedness-agnostic: same width is a no-op reinterpret, widening uses
			// the *source* signedness (sign- vs zero-extend), narrowing truncates (wraps).
			switch {
			case castType.Size == valueType.Size:
				result = src
			case castType.Size > valueType.Size:
				if valueType.Signed {
					result = c.builder.CreateSExt(src, dstLLVMType, name)
				} else {
					result = c.builder.CreateZExt(src, dstLLVMType, name)
				}
			default:
				result = c.builder.CreateTrunc(src, dstLLVMType, name)
			}
		case zeus_value.BoolType:
			// int → bool is `x != 0`.
			zero := llvm.ConstInt(c.toLLVMType(valueType), 0, false)
			result = c.builder.CreateICmp(llvm.IntNE, src, zero, name)
		default:
			panic(castErrorMsg)
		}
	case zeus_value.FloatType:
		switch castType := input.CastType.(type) {
		case zeus_value.IntType:
			// Truncates toward zero; out-of-range is unchecked (LLVM poison) by design.
			if castType.Signed {
				result = c.builder.CreateFPToSI(src, dstLLVMType, name)
			} else {
				result = c.builder.CreateFPToUI(src, dstLLVMType, name)
			}
		case zeus_value.FloatType:
			switch {
			case castType.Size > valueType.Size:
				result = c.builder.CreateFPExt(src, dstLLVMType, name)
			case castType.Size < valueType.Size:
				result = c.builder.CreateFPTrunc(src, dstLLVMType, name)
			default:
				result = src
			}
		case zeus_value.BoolType:
			// float → bool is `x != 0.0` (ordered).
			zero := llvm.ConstFloat(c.toLLVMType(valueType), 0.0)
			result = c.builder.CreateFCmp(llvm.FloatONE, src, zero, name)
		default:
			panic(castErrorMsg)
		}
	case zeus_value.BoolType:
		switch input.CastType.(type) {
		case zeus_value.IntType:
			// bool → int gives 0/1 (i1 zero-extended).
			result = c.builder.CreateZExt(src, dstLLVMType, name)
		case zeus_value.FloatType:
			// bool → float gives 0.0/1.0.
			result = c.builder.CreateUIToFP(src, dstLLVMType, name)
		case zeus_value.BoolType:
			result = src
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
	// Static properties are backed by dedicated globals and must not occupy instance struct slots.
	// Inherited fields come first (layout.Fields is base-first) so a derived object begins with its
	// base's layout and a derived pointer doubles as a base pointer.
	layout := class.Layout()
	classElementTypes := []llvm.Type{llvm.PointerType(objectHeaderStructType, 0)}
	for _, property := range layout.Fields {
		classElementTypes = append(classElementTypes, c.toLLVMType(property.Property.ValueType))
	}
	llvmStructType.StructSetBody(classElementTypes, false)

	// The vtable mirrors the layout's vtable slots (base slots first, overrides in the base's
	// slot). Static methods are standalone functions and the constructor is called directly, so
	// layout.VTable already excludes both.
	vtableElementTypes := []llvm.Type{}
	for _, entry := range layout.VTable {
		vtableElementTypes = append(vtableElementTypes, llvm.PointerType(c.toLLVMClassMethodType(*entry.Method, llvmStructType), 0))
	}
	vtableStructType.StructSetBody(vtableElementTypes, false)

	return llvmStructType, vtableStructType, objectHeaderStructType, class.Name
}

func (c *CodegenModule) genObjArrayClass() *ZeusClassLLVMStruct {
	span := token.NewSpan(*token.NewPosition(0, 0), *token.NewPosition(0, 0))
	objectClass := zeus_value.NewClass(ZeusObjectClassName, []*zeus_value.ClassProperty{}, []*zeus_value.ClassMethod{}, nil, "", nil, span)
	objectArrayClass := zeus_value.GetArrayPrimordialClassDefinition(zeus_value.NewArrayType(zeus_value.NewObjectType(objectClass), span))
	// This array class is built outside the registry, so mark its runtime-backed methods extern
	// (as the registry does for the classes it creates) — see emitExternMethods.
	zeus_value.MarkExternMethods(objectArrayClass)

	if c.zeusClassLLVMStructMap[objectArrayClass.Name] != nil {
		return c.zeusClassLLVMStructMap[objectArrayClass.Name]
	}
	// generate the object class first
	c.genClass(*objectClass)
	// generate the object array class
	return c.genClass(*objectArrayClass)
}

// isObjectArrayHandle reports whether `class` is a specific object-array type (e.g. Point[]) that
// shares the single Object[] struct/vtable/factory but carries its own distinct type handle. Such
// entries reuse Object[]'s emitted code, so per-class emission passes (fillVTables, factory bodies,
// extern methods) must skip them — only the base Object[] entry owns that code.
func isObjectArrayHandle(class zeus_value.Class) bool {
	return class.PrimordialName == zeus_value.ZEUS_PRIMORDIAL_ARRAY &&
		class.Name != ZeusObjectArrayClassName &&
		(zeus_value.IsObjectType(class.ArrayElementType) || zeus_value.IsInterfaceType(class.ArrayElementType))
}

// genObjectArrayTypeHandle gives a specific object-array type (e.g. Point[]) its OWN runtime type
// handle — a distinct type-info (its own class id) and object header — while SHARING the single
// Object[] struct layout, vtable (methods), factory and constructor. Every object array is
// byte-identical in memory (elements are object pointers), so the CODE is shared; but a distinct
// class id is what lets the per-class interface dispatch table key this array type. This mirrors
// C#'s "one shared generic method body, a distinct runtime type handle per instantiation".
func (c *CodegenModule) genObjectArrayTypeHandle(class zeus_value.Class) *ZeusClassLLVMStruct {
	if existing := c.zeusClassLLVMStructMap[class.Name]; existing != nil {
		return existing
	}
	shared := c.genObjArrayClass() // Object[] entry: struct, vtable, methods, factory, ctor

	// Own type info: a distinct class id (object arrays are pointer-element, so the runtime element
	// size is identical to Object[]'s regardless of the element type).
	typeInfo := llvm.AddGlobal(c.module, c.zeusObjectTypeInfoType, GetObjectTypeInfoStructPtrName(class.Name))
	typeInfo.SetLinkage(llvm.InternalLinkage)
	typeInfo.SetInitializer(llvm.ConstStruct([]llvm.Value{
		llvm.ConstInt(c.cxt.Int32Type(), uint64(class.Id), false),
		llvm.ConstInt(c.cxt.Int8Type(), uint64(ZeusRuntimeObjectTypeArray), false),
		llvm.ConstInt(c.cxt.Int8Type(), uint64(toZeusRuntimeType(class.ArrayElementType)), false),
		llvm.ConstNull(llvm.PointerType(c.zeusObjectTypeInfoType, 0)),
	}, false))

	// Own header sharing Object[]'s vtable, so method calls dispatch to the shared code.
	header := llvm.AddGlobal(c.module, shared.LLVMObjHeaderStructType, GetObjectHeaderStructPtrName(class.Name))
	header.SetLinkage(llvm.InternalLinkage)
	header.SetInitializer(llvm.ConstStruct([]llvm.Value{*shared.LLVMVTableInstance, typeInfo}, false))

	entry := NewZeusClassLLVMStruct(class, shared.LLVMStructType, shared.LLVMVTableStructType, shared.LLVMVTableMethods, shared.LLVMObjHeaderStructType, shared.LLVMVTableInstance, header, shared.LLVMConstructorMethod)
	c.zeusClassLLVMStructMap[class.Name] = entry
	return entry
}

// genClass generates LLVM code for a Zeus class including struct types, vtable, and object header
func (c *CodegenModule) genClass(class zeus_value.Class) *ZeusClassLLVMStruct {
	if c.zeusClassLLVMStructMap[class.Name] != nil {
		return c.zeusClassLLVMStructMap[class.Name]
	} else if isObjectArrayHandle(class) {
		// All OBJECT arrays share the single Object[] layout/vtable/factory (they are byte-identical
		// in memory — object pointers). But each element type gets its OWN type handle (distinct
		// class id + header) so it can be keyed in interface dispatch tables. Primitive arrays
		// (u8[], i32[], ...) already take the normal path below with their own type info (element
		// size differs), so they get distinct class ids for free.
		return c.genObjectArrayTypeHandle(class)
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
		llvm.ConstInt(c.cxt.Int32Type(), uint64(class.Id), false),
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
	// initialize the llvm methods array — one slot per vtable entry (inherited base methods +
	// this class's own instance methods, overrides sharing the base's slot). fillVTables sets the
	// real function pointers (by name) once every method has been emitted.
	methodCount := len(class.Layout().VTable)
	llvmVTableMethods := make([]llvm.Value, methodCount)
	zeusClassLLVMStruct := NewZeusClassLLVMStruct(class, llvmStructType, vtableStructType, llvmVTableMethods, objectHeaderStructType, &llvmVTable, llvmObjectHeader, nil)

	// create the vtable global
	// initialize the vtable methods to null
	// this is done here because the vtable methods are not known until we encounter the DECL_CLASS_METHOD instructions
	for llvmVTableMethodIndex := range llvmVTableMethods {
		llvmVTableMethods[llvmVTableMethodIndex] = llvm.ConstNull(llvm.PointerType(llvm.FunctionType(c.cxt.VoidType(), []llvm.Type{}, false), 0))
	}

	c.zeusClassLLVMStructMap[class.Name] = zeusClassLLVMStruct

	// Emit bodies for extern methods (forward to the Zig runtime). No-op for classes with no
	// extern methods, so this needs no "is this a primordial" special-case.
	c.emitExternMethods(class)

	// Declare the factory signature for primordials/arrays only. User-class factories are
	// synthesized as real IR functions by FactoryLoweringPass and declared via their DECL_FUNC,
	// so declaring here too would create a duplicate zeus_new_<Class> symbol.
	if class.PrimordialName != "" {
		c.declareFactoryFunction(class)
	}

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
	if callee.PrimordialName == zeus_value.ZEUS_PRIMORDIAL_ARRAY && callee.Name != ZeusObjectArrayClassName && (zeus_value.IsObjectType(callee.ArrayElementType) || zeus_value.IsInterfaceType(callee.ArrayElementType)) {
		factoryClassName = ZeusObjectArrayClassName
	}
	factoryFunctionName := util.GetFactoryFunctionName(factoryClassName)
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

	// Object arrays are built by the shared Object[] factory, which installs Object[]'s header
	// (and thus Object[]'s class id). Swap in this array type's own header so the object carries a
	// distinct class id — the vtable is shared, so methods still dispatch to the same code, but
	// interface dispatch can now key this element type. (No-op for Object[] itself and primitive
	// arrays, which already carry their own header.)
	if isObjectArrayHandle(*callee) {
		c.genObjectArrayTypeHandle(*callee) // ensure this type's header global exists
		headerField := c.builder.CreateStructGEP(c.getLLVMStructType(ZeusObjectArrayClassName), llvmStruct, OBJ_HEADER_STRUCT_INDEX, "arr_type_header_field")
		c.builder.CreateStore(c.getLLVMObjHeaderPtr(callee.Name), headerField)
	}

	c.llvmValues[output.Name] = llvmStruct
}

// emitAllocAndHeader allocates a zeroed object for the class (the GC allocator, zeus_gc_alloc /
// Boehm GC_malloc, returns zeroed memory — so fields start at their zero-value defaults) and
// installs its header/type-info pointer, returning the object pointer. This is the pure-mechanism
// core shared by ALLOC_OBJ codegen (user-class factories, synthesized in IR) and the primordial
// factory-body synthesis in genFactoryFunctionBody.
func (c *CodegenModule) emitAllocAndHeader(class zeus_value.Class) llvm.Value {
	llvmStructType := c.getLLVMStructType(class.Name)
	llvmStruct := c.callGlobalLLVMFunction(MemAllocFunctionName, llvm.ConstInt(c.cxt.Int32Type(), c.getSizeOfClass(class), false))

	llvmStructObjHeaderField := c.builder.CreateStructGEP(llvmStructType, llvmStruct, OBJ_HEADER_STRUCT_INDEX, fmt.Sprintf("%s_header_field", class.Name))
	llvmObjHeader := c.getLLVMObjHeaderPtr(class.Name)
	c.builder.CreateStore(llvmObjHeader, llvmStructObjHeaderField)
	return llvmStruct
}

// genAllocObj lowers an ALLOC_OBJ instruction: allocate + header, yielding the object pointer.
func (c *CodegenModule) genAllocObj(input ir.AllocObjInstrInput, output zeus_value.Var) {
	c.llvmValues[output.Name] = c.emitAllocAndHeader(*input.Class)
}

// declareFactoryFunction declares the factory function signature for a class
// This is called in Phase 1 before NEW_OBJ instructions are processed
func (c *CodegenModule) declareFactoryFunction(class zeus_value.Class) llvm.Value {
	factoryFunctionName := util.GetFactoryFunctionName(class.Name)

	// Check if already declared
	existingFunc := c.module.NamedFunction(factoryFunctionName)
	if !existingFunc.IsNil() {
		return existingFunc
	}

	// The factory's parameters mirror the effective constructor (own or nearest inherited).
	constructorMethod := class.Layout().Constructor

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
	factoryFunctionName := util.GetFactoryFunctionName(class.Name)
	factoryFunction := c.module.NamedFunction(factoryFunctionName)
	zeus_error.Assert(!factoryFunction.IsNil(), fmt.Sprintf("factory function %s not declared", factoryFunctionName))

	// The effective constructor (own or nearest inherited) drives the factory's params and call.
	layout := class.Layout()
	constructorMethod, constructorClass := layout.Constructor, layout.ConstructorClass

	// Create the function body
	entryBlock := llvm.AddBasicBlock(factoryFunction, "entry")
	currentInsertionBlock := c.builder.GetInsertBlock()
	c.builder.SetInsertPointAtEnd(entryBlock)

	// Allocate the zeroed object and install its header.
	llvmStruct := c.emitAllocAndHeader(class)

	// Initialize instance properties (inherited + own, base-first) to default values.
	// Static properties are backed by globals and have no slot in the instance struct.
	llvmStructType := c.getLLVMStructType(class.Name)
	for instancePropertyIndex, property := range layout.Fields {
		defaultLLVMValue := c.getDefaultLLVMValue(property.Property.ValueType)
		llvmPropertyField := c.builder.CreateStructGEP(llvmStructType, llvmStruct, instancePropertyIndex+1, fmt.Sprintf("%s_property_%s_default_value", class.Name, property.Property.Name))
		c.builder.CreateStore(defaultLLVMValue, llvmPropertyField)
	}

	// Call the effective constructor if the class (or an ancestor) has one. When it belongs to a
	// base class, call that class's constructor (its LLVM function) — with the object as `this`,
	// it initializes the inherited fields; a derived class with its own constructor chains via
	// super(...) instead.
	var llvmConstructorMethod *llvm.Value
	if constructorClass != nil {
		llvmConstructorMethod = c.getLLVMConstructorMethod(constructorClass.Name)
	}
	if llvmConstructorMethod != nil && constructorMethod != nil {
		constructorFunc := *llvmConstructorMethod
		constructorParamTypes := []zeus_value.ValueType{}
		for _, param := range constructorMethod.Params {
			constructorParamTypes = append(constructorParamTypes, param.ValueType)
		}
		constructorParamTypes = append(constructorParamTypes, zeus_value.NewObjectType(constructorClass))
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
			zeus_value.NewObjectType(&class),
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

	// The vtable is not written here: fillVTables resolves every slot by name once all methods
	// (own, inherited, extern) have been emitted. genClassMethod only needs to capture the
	// constructor's LLVM function so the factory can call it.
	if isConstructor {
		structInfo := c.zeusClassLLVMStructMap[class.Name]
		structInfo.LLVMConstructorMethod = &function
	}

	return function
}

func (c *CodegenModule) genDeclClassMethod(input ir.DeclClassMethodInstrInput) llvm.Value {
	return c.genClassMethod(*input.Method, *input.Class)
}

// fillVTables sets every class's vtable global initializer once, after all method bodies (own,
// inherited, and extern) have been emitted. For each slot it resolves the compiled LLVM function by
// name — the method's IR name, or its class-scoped name for extern (primordial) methods — exactly
// as genStaticMethodCall does for super calls. Resolving by name (rather than writing slots per
// method and copying base vtables) collapses the old two-phase fill into one pass and removes its
// base-before-derived ordering dependency. Imported classes are skipped — their vtables are defined
// in the exporting module.
func (c *CodegenModule) fillVTables() {
	for _, structInfo := range c.zeusClassLLVMStructMap {
		class := structInfo.ZeusClass
		if _, isImported := c.importedClasses[class.Name]; isImported {
			continue
		}
		if isObjectArrayHandle(class) {
			continue // shares Object[]'s vtable, filled via the Object[] entry
		}
		// LLVMVTableMethods was sized to len(Layout().VTable) in genClass, so slot is always in range.
		vtable := structInfo.LLVMVTableMethods
		for slot, entry := range class.Layout().VTable {
			fnName := entry.Method.Method.Name
			if entry.Method.IsExtern {
				fnName = util.GetClassMethodName(entry.DefiningClass.Name, entry.Method.Method.Name)
			}
			// Resolve via our controlled IR-name→function map (getFunctionSymbol), not
			// module.NamedFunction which queries LLVM's global symbol table and could return a
			// collision-renamed or non-method symbol. Panics on a genuine miss.
			vtable[slot] = c.getFunctionSymbol(fnName)
		}
		c.getLLVMVTablePtr(class.Name).SetInitializer(llvm.ConstStruct(vtable, true))
	}
}

// loadObjectVTable walks obj → header → vtable (header field 0) using opaque generic-pointer
// GEPs, for dispatch where the concrete class is unknown at compile time (interface calls,
// functor calls). Field offsets MUST match getLLVMObjHeaderStruct: header field 0 = vtable ptr.
func (c *CodegenModule) loadObjectVTable(obj llvm.Value) llvm.Value {
	ptrType := llvm.PointerType(c.cxt.VoidType(), 0)
	genericObjType := c.cxt.StructType([]llvm.Type{ptrType}, false)
	headerPtrAddr := c.builder.CreateStructGEP(genericObjType, obj, OBJ_HEADER_STRUCT_INDEX, "objHeaderPtr")
	header := c.builder.CreateLoad(ptrType, headerPtrAddr, "objHeader")
	genericHeaderType := c.cxt.StructType([]llvm.Type{ptrType, ptrType}, false)
	vtablePtrAddr := c.builder.CreateStructGEP(genericHeaderType, header, VTABLE_STRUCT_INDEX, "vTablePtr")
	return c.builder.CreateLoad(ptrType, vtablePtrAddr, "vTable")
}

// loadObjectClassId walks obj → header → typeInfo → id (typeInfo field 0, i32) using opaque
// generic-pointer GEPs, since the concrete class is unknown at an interface call site.
func (c *CodegenModule) loadObjectClassId(obj llvm.Value) llvm.Value {
	ptrType := llvm.PointerType(c.cxt.VoidType(), 0)
	// obj: { ptr header, ... } — field 0 is the header pointer.
	genericObjType := c.cxt.StructType([]llvm.Type{ptrType}, false)
	headerPtrAddr := c.builder.CreateStructGEP(genericObjType, obj, OBJ_HEADER_STRUCT_INDEX, "objHeaderPtr")
	header := c.builder.CreateLoad(ptrType, headerPtrAddr, "objHeader")
	// header: { ptr vtable, ptr typeInfo } — field 1 is the type-info pointer.
	genericHeaderType := c.cxt.StructType([]llvm.Type{ptrType, ptrType}, false)
	typeInfoPtrAddr := c.builder.CreateStructGEP(genericHeaderType, header, 1, "typeInfoPtr")
	typeInfo := c.builder.CreateLoad(ptrType, typeInfoPtrAddr, "typeInfo")
	// typeInfo: { i32 id, ... } — field 0 is the class id.
	idAddr := c.builder.CreateStructGEP(c.zeusObjectTypeInfoType, typeInfo, 0, "classIdPtr")
	return c.builder.CreateLoad(c.cxt.Int32Type(), idAddr, "classId")
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
	// Generic opaque-pointer dispatch (class unknown at compile time).
	vtable := c.loadObjectVTable(obj)
	slotTypes := make([]llvm.Type, slotIndex+1)
	for i := range slotTypes {
		slotTypes[i] = ptrType
	}
	slotAddr := c.builder.CreateStructGEP(c.cxt.StructType(slotTypes, false), vtable, slotIndex, name)
	return c.builder.CreateLoad(ptrType, slotAddr, name+"_fn_ptr")
}

// interfaceDispatchInfo describes one interface's runtime dispatch-table global. The table has
// type [maxClassId+1 x [numCols x i32]]: indexed by the concrete object's class id, each row
// holds either the class's vtable slot per interface method (method table) or the byte offset
// of the backing field per interface property (property table). Rows for non-conforming class
// ids are zero. Every entry is a compile-time constant, so no finalize pass is needed and
// cross-module dispatch works for free (a method table stores slot indices, read at runtime
// from the object's own vtable — never the imported class's method symbols).
type interfaceDispatchInfo struct {
	global    llvm.Value
	tableType llvm.Type
}

// interfacePropDispatchInfo holds an interface's two property dispatch tables (read/write) and
// their (shared) LLVM type. Each entry is a tagged i32: a field byte-offset, or (accessor)
// a getter/setter vtable slot with interfaceAccessorTag set.
type interfacePropDispatchInfo struct {
	getGlobal llvm.Value
	setGlobal llvm.Value
	tableType llvm.Type
}

// interfaceAccessorTag marks a property-itable entry as a vtable slot (an accessor) rather than a
// field byte offset. Field offsets are small (≥ 8, after the header) and slots small, so the high
// bit is free as a discriminator.
const interfaceAccessorTag = 0x80000000

// interfacePropAccessInfo bundles the operands of an INTERFACE_PROP_GET/SET for the dispatch
// helpers: the receiver object (already lowered to an LLVM value), its interface, and the property.
type interfacePropAccessInfo struct {
	object   llvm.Value
	iface    *zeus_value.Interface
	propName string
}

// Interface dispatch tables (itables) are whole-program data (they list conformers from every
// module), so they are DEFINED once in a dedicated itable module (defineInterface*Table, external
// linkage) and merely REFERENCED (external declaration) at each dispatch site. This keeps ordinary
// module object files free of program-wide data so they stay cacheable. Globals are named by the
// interface's unique id so a declaration and its definition resolve to the same symbol at link.

func interfaceMethodTableName(iface *zeus_value.Interface) string {
	return fmt.Sprintf("__zeus_iface_%d_idispatch", iface.Id)
}

func interfacePropGetTableName(iface *zeus_value.Interface) string {
	return fmt.Sprintf("__zeus_iface_%d_ipropget", iface.Id)
}

func interfacePropSetTableName(iface *zeus_value.Interface) string {
	return fmt.Sprintf("__zeus_iface_%d_ipropset", iface.Id)
}

// interfaceMethodTableType is the LLVM type of iface's method itable: [maxClassId+1][numMethods]
// of i32. Computed from the whole-program class set, so a declaration (dispatch site) and the
// definition (itable module) agree on the type.
func (c *CodegenModule) interfaceMethodTableType(iface *zeus_value.Interface) llvm.Type {
	layout := c.codegen.interfaceLayout(iface)
	numMethods := len(zeus_value.InterfaceMethods(iface))
	return llvm.ArrayType(llvm.ArrayType(c.cxt.Int32Type(), numMethods), layout.MaxClassId+1)
}

func (c *CodegenModule) interfacePropTableType(iface *zeus_value.Interface) llvm.Type {
	layout := c.codegen.interfaceLayout(iface)
	numProps := len(zeus_value.InterfaceProperties(iface))
	return llvm.ArrayType(llvm.ArrayType(c.cxt.Int32Type(), numProps), layout.MaxClassId+1)
}

// refInterfaceDispatchTable returns a reference to iface's method itable, emitting an external
// DECLARATION in this module (the definition lives in the dedicated itable module) and recording
// that the table is needed program-wide.
func (c *CodegenModule) refInterfaceDispatchTable(iface *zeus_value.Interface) *interfaceDispatchInfo {
	if info, ok := c.interfaceDispatch[iface.Id]; ok {
		return info
	}
	tableType := c.interfaceMethodTableType(iface)
	global := llvm.AddGlobal(c.module, tableType, interfaceMethodTableName(iface))
	global.SetLinkage(llvm.ExternalLinkage) // no initializer ⇒ external declaration
	c.codegen.recordInterfaceMethodTable(iface)
	c.dispatchesInterface = true
	info := &interfaceDispatchInfo{global: global, tableType: tableType}
	c.interfaceDispatch[iface.Id] = info
	return info
}

// refInterfacePropDispatchTable is the property-table counterpart of refInterfaceDispatchTable.
// refInterfacePropTables returns references to iface's property get/set itables, emitting external
// DECLARATIONS in this module (definitions live in the dedicated itable module).
func (c *CodegenModule) refInterfacePropTables(iface *zeus_value.Interface) *interfacePropDispatchInfo {
	if info, ok := c.interfacePropDispatch[iface.Id]; ok {
		return info
	}
	tableType := c.interfacePropTableType(iface)
	getGlobal := llvm.AddGlobal(c.module, tableType, interfacePropGetTableName(iface))
	getGlobal.SetLinkage(llvm.ExternalLinkage)
	setGlobal := llvm.AddGlobal(c.module, tableType, interfacePropSetTableName(iface))
	setGlobal.SetLinkage(llvm.ExternalLinkage)
	c.codegen.recordInterfacePropTable(iface)
	c.dispatchesInterface = true
	info := &interfacePropDispatchInfo{getGlobal: getGlobal, setGlobal: setGlobal, tableType: tableType}
	c.interfacePropDispatch[iface.Id] = info
	return info
}

// defineInterfaceDispatchTable emits the DEFINITION of iface's method itable (external, constant)
// — only called in the dedicated itable module. Entries are vtable-slot indices per conforming
// class; non-conforming rows are zero.
func (c *CodegenModule) defineInterfaceDispatchTable(iface *zeus_value.Interface) {
	layout := c.codegen.interfaceLayout(iface)
	numMethods := len(zeus_value.InterfaceMethods(iface))
	i32 := c.cxt.Int32Type()
	innerType := llvm.ArrayType(i32, numMethods)
	tableType := llvm.ArrayType(innerType, layout.MaxClassId+1)

	rows := make([]llvm.Value, layout.MaxClassId+1)
	nullInner := llvm.ConstNull(innerType)
	for i := range rows {
		rows[i] = nullInner
	}
	for _, row := range layout.Rows {
		slots := make([]llvm.Value, numMethods)
		for j, slot := range row.MethodSlots {
			slots[j] = llvm.ConstInt(i32, uint64(slot), false)
		}
		rows[row.ClassId] = llvm.ConstArray(innerType, slots)
	}

	global := llvm.AddGlobal(c.module, tableType, interfaceMethodTableName(iface))
	global.SetInitializer(llvm.ConstArray(innerType, rows))
	global.SetLinkage(llvm.ExternalLinkage)
	global.SetGlobalConstant(true)
}

// defineInterfacePropTables emits the DEFINITIONS of iface's two property itables (get/set). Each
// entry is a tagged i32: a field byte offset, or a getter/setter vtable slot | interfaceAccessorTag.
func (c *CodegenModule) defineInterfacePropTables(iface *zeus_value.Interface) {
	layout := c.codegen.interfaceLayout(iface)
	numProps := len(zeus_value.InterfaceProperties(iface))
	i32 := c.cxt.Int32Type()
	innerType := llvm.ArrayType(i32, numProps)
	tableType := llvm.ArrayType(innerType, layout.MaxClassId+1)

	getRows := make([]llvm.Value, layout.MaxClassId+1)
	setRows := make([]llvm.Value, layout.MaxClassId+1)
	nullInner := llvm.ConstNull(innerType)
	for i := range getRows {
		getRows[i] = nullInner
		setRows[i] = nullInner
	}
	for _, row := range layout.Rows {
		structType := c.instanceStructTypeForOffset(row.Class)
		getEntries := make([]llvm.Value, numProps)
		setEntries := make([]llvm.Value, numProps)
		for k, backing := range row.PropertyBackings {
			if backing.Kind == zeus_value.PropertyBackingAccessor {
				getEntries[k] = llvm.ConstInt(i32, uint64(backing.GetterSlot)|interfaceAccessorTag, false)
				setter := backing.SetterSlot
				if setter < 0 {
					setter = 0 // readonly property: setter never dispatched
				}
				setEntries[k] = llvm.ConstInt(i32, uint64(setter)|interfaceAccessorTag, false)
			} else {
				// Field: both read and write use the field's byte offset (+1 skips the header).
				offset := c.constFieldOffset(structType, backing.FieldIndex+1)
				getEntries[k] = offset
				setEntries[k] = offset
			}
		}
		getRows[row.ClassId] = llvm.ConstArray(innerType, getEntries)
		setRows[row.ClassId] = llvm.ConstArray(innerType, setEntries)
	}

	getGlobal := llvm.AddGlobal(c.module, tableType, interfacePropGetTableName(iface))
	getGlobal.SetInitializer(llvm.ConstArray(innerType, getRows))
	getGlobal.SetLinkage(llvm.ExternalLinkage)
	getGlobal.SetGlobalConstant(true)

	setGlobal := llvm.AddGlobal(c.module, tableType, interfacePropSetTableName(iface))
	setGlobal.SetInitializer(llvm.ConstArray(innerType, setRows))
	setGlobal.SetLinkage(llvm.ExternalLinkage)
	setGlobal.SetGlobalConstant(true)
}

// DefineDispatchedInterfaceTables emits the definitions of every interface table referenced
// anywhere in the program. Call it on the dedicated itable module after all other modules are
// codegen'd (so every dispatched interface has been recorded). Interfaces are emitted in id order
// so the module — and thus its object file — is deterministic.
func (c *CodegenModule) DefineDispatchedInterfaceTables() {
	ids := make([]int, 0, len(c.codegen.dispatchedInterfaces))
	for id := range c.codegen.dispatchedInterfaces {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		usage := c.codegen.dispatchedInterfaces[id]
		if usage.needsMethod {
			c.defineInterfaceDispatchTable(usage.iface)
		}
		if usage.needsProp {
			c.defineInterfacePropTables(usage.iface)
		}
	}
}

// instanceStructTypeForOffset returns a struct type suitable for computing an interface
// property's byte offset on `class`. When the class was emitted in THIS module we use its real
// named struct. When it is a conformer from another module (not in the local struct map) we
// reconstruct a size-equivalent struct from its flattened field layout — the instance layout is
// `[header ptr, field0, field1, …]` (base-first, matching createClassStructTypes), and offsets
// depend only on each element's size/alignment, so a reconstruction yields identical offsets.
func (c *CodegenModule) instanceStructTypeForOffset(class *zeus_value.Class) llvm.Type {
	if info, ok := c.zeusClassLLVMStructMap[class.Name]; ok {
		return info.LLVMStructType
	}
	elems := []llvm.Type{llvm.PointerType(c.cxt.VoidType(), 0)} // object header pointer
	for _, field := range class.Layout().Fields {
		elems = append(elems, c.offsetEquivalentType(field.Property.ValueType))
	}
	return c.cxt.StructType(elems, false)
}

// offsetEquivalentType returns an LLVM type with the same size and alignment as a field of the
// given Zeus type, without needing that type's named LLVM struct (which may live in another
// module). Every reference-shaped field is a GC pointer; primitives use their real scalar type.
func (c *CodegenModule) offsetEquivalentType(t zeus_value.ValueType) llvm.Type {
	switch t.(type) {
	case zeus_value.ObjectType, zeus_value.InterfaceType, zeus_value.FunctionType, zeus_value.ArrayType:
		return llvm.PointerType(c.cxt.VoidType(), 1)
	default:
		return c.toLLVMBuiltInType(t)
	}
}

// constFieldOffset returns the byte offset of a struct field as a compile-time constant using
// the offsetof idiom: ptrtoint(getelementptr(structType, null, 0, fieldIndex)). Independent of
// the target-data-layout API and consistent across modules that build the same struct type.
func (c *CodegenModule) constFieldOffset(structType llvm.Type, fieldIndex int) llvm.Value {
	i32 := c.cxt.Int32Type()
	nullPtr := llvm.ConstNull(llvm.PointerType(structType, 0))
	gep := llvm.ConstInBoundsGEP(structType, nullPtr, []llvm.Value{
		llvm.ConstInt(i32, 0, false),
		llvm.ConstInt(i32, uint64(fieldIndex), false),
	})
	return llvm.ConstPtrToInt(gep, i32)
}

// genInterfaceMethodCall dispatches a method call through an interface value: obj → classId →
// idispatch[classId][methodSlot] → vtable slot → load fn from the object's own vtable → call.
func (c *CodegenModule) genInterfaceMethodCall(input ir.MethodCallInstrInput, iface *zeus_value.Interface, output zeus_value.Var) {
	llvmObject := c.toLLVMValue(input.Object)
	info := c.refInterfaceDispatchTable(iface)

	slot := zeus_value.InterfaceMethodIndex(iface, input.MethodName)
	zeus_error.Assert(slot != -1, fmt.Sprintf("interface %s has no method %s", iface.Name, input.MethodName))

	var ifaceMethod *zeus_value.Function
	for _, m := range zeus_value.InterfaceMethods(iface) {
		if m.SourceName() == input.MethodName {
			ifaceMethod = m
			break
		}
	}
	zeus_error.Assert(ifaceMethod != nil, fmt.Sprintf("interface method %s.%s not found", iface.Name, input.MethodName))

	i32 := c.cxt.Int32Type()
	ptrType := llvm.PointerType(c.cxt.VoidType(), 0)

	// vtableSlot = idispatch[classId][methodSlot]
	classId := c.loadObjectClassId(llvmObject)
	zero := llvm.ConstInt(i32, 0, false)
	methodSlot := llvm.ConstInt(i32, uint64(slot), false)
	slotAddr := c.builder.CreateInBoundsGEP(info.tableType, info.global, []llvm.Value{zero, classId, methodSlot}, "vtableSlotAddr")
	vtableSlot := c.builder.CreateLoad(i32, slotAddr, "vtableSlot")

	// fnPtr = objectVTable[vtableSlot] — read the method from the object's own vtable, so an
	// imported concrete class resolves correctly without referencing its symbols here.
	vtable := c.loadObjectVTable(llvmObject)
	fnPtrAddr := c.builder.CreateInBoundsGEP(ptrType, vtable, []llvm.Value{vtableSlot}, "ifaceMethodPtrAddr")
	fnPtr := c.builder.CreateLoad(ptrType, fnPtrAddr, "ifaceMethodPtr")

	// Build the call: interface method params, then the receiver (a generic object pointer).
	functionArgs := make([]llvm.Value, 0, len(input.Args)+1)
	for _, arg := range input.Args {
		functionArgs = append(functionArgs, c.toLLVMValue(arg))
	}
	functionArgs = append(functionArgs, llvmObject)

	llvmParamTypes := make([]llvm.Type, 0, len(ifaceMethod.Params)+1)
	for _, p := range ifaceMethod.Params {
		llvmParamTypes = append(llvmParamTypes, c.toLLVMType(p.ValueType))
	}
	llvmParamTypes = append(llvmParamTypes, llvm.PointerType(c.cxt.VoidType(), 1))
	callType := llvm.FunctionType(c.toLLVMType(ifaceMethod.ReturnType), llvmParamTypes, false)

	result := c.builder.CreateCall(callType, fnPtr, functionArgs, fmt.Sprintf("%s_result", input.MethodName))
	c.llvmValues[output.Name] = result
}

// interfacePropTagAndSlot loads the tagged property-itable entry for `access` from `table` and
// returns (tag i32, isAccessor i1, slotOrOffset masked-i32). Shared by get and set.
func (c *CodegenModule) interfacePropTagAndSlot(access interfacePropAccessInfo, table llvm.Value, tableType llvm.Type) (tag, isAcc, payload llvm.Value) {
	iface := access.iface
	propSlot := zeus_value.InterfacePropertyIndex(iface, access.propName)
	zeus_error.Assert(propSlot != -1, fmt.Sprintf("interface %s has no property %s", iface.Name, access.propName))
	i32 := c.cxt.Int32Type()
	zero := llvm.ConstInt(i32, 0, false)
	classId := c.loadObjectClassId(access.object)
	pSlot := llvm.ConstInt(i32, uint64(propSlot), false)
	addr := c.builder.CreateInBoundsGEP(tableType, table, []llvm.Value{zero, classId, pSlot}, "ipropEntryAddr")
	tag = c.builder.CreateLoad(i32, addr, "ipropEntry")
	accBit := c.builder.CreateAnd(tag, llvm.ConstInt(i32, interfaceAccessorTag, false), "accBit")
	isAcc = c.builder.CreateICmp(llvm.IntNE, accBit, zero, "isAccessor")
	payload = c.builder.CreateAnd(tag, llvm.ConstInt(i32, interfaceAccessorTag-1, false), "ipropPayload")
	return tag, isAcc, payload
}

// interfacePropertyType is the LLVM type of interface property `name`.
func (c *CodegenModule) interfacePropertyType(iface *zeus_value.Interface, name string) llvm.Type {
	for _, p := range zeus_value.InterfaceProperties(iface) {
		if p.Property.Name == name {
			return c.toLLVMType(p.Property.ValueType)
		}
	}
	panic(fmt.Sprintf("interface %s has no property %s", iface.Name, name))
}

// genInterfacePropGet lowers an INTERFACE_PROP_GET instruction: read the property value through
// the interface receiver and bind it to the instruction's output.
func (c *CodegenModule) genInterfacePropGet(input ir.InterfacePropGetInstrInput, output zeus_value.Var) {
	access := interfacePropAccessInfo{
		object:   c.toLLVMValue(input.Object),
		iface:    input.Iface,
		propName: input.PropName,
	}
	c.llvmValues[output.Name] = c.genInterfacePropertyGet(access)
}

// genInterfacePropSet lowers an INTERFACE_PROP_SET instruction: write a value through the
// interface receiver.
func (c *CodegenModule) genInterfacePropSet(input ir.InterfacePropSetInstrInput) {
	access := interfacePropAccessInfo{
		object:   c.toLLVMValue(input.Object),
		iface:    input.Iface,
		propName: input.PropName,
	}
	c.genInterfacePropertySet(access, c.toLLVMValue(input.Value))
}

// genInterfacePropertyGet reads an interface property through the tagged get-itable: a field load
// at the byte offset, or a getter call through the object's own vtable — chosen at runtime.
func (c *CodegenModule) genInterfacePropertyGet(access interfacePropAccessInfo) llvm.Value {
	obj := access.object
	info := c.refInterfacePropTables(access.iface)
	propType := c.interfacePropertyType(access.iface, access.propName)
	tag, isAcc, slot := c.interfacePropTagAndSlot(access, info.getGlobal, info.tableType)
	ptrType := llvm.PointerType(c.cxt.VoidType(), 0)

	fn := c.builder.GetInsertBlock().Parent()
	fieldBlock := c.cxt.AddBasicBlock(fn, "iprop.get.field")
	accBlock := c.cxt.AddBasicBlock(fn, "iprop.get.acc")
	mergeBlock := c.cxt.AddBasicBlock(fn, "iprop.get.merge")
	c.builder.CreateCondBr(isAcc, accBlock, fieldBlock)

	// field: load(obj + offset)  (tag is the offset when the accessor bit is clear)
	c.builder.SetInsertPointAtEnd(fieldBlock)
	fieldPtr := c.builder.CreateInBoundsGEP(c.cxt.Int8Type(), obj, []llvm.Value{tag}, "ipropFieldPtr")
	vField := c.builder.CreateLoad(propType, fieldPtr, "ipropField")
	c.builder.CreateBr(mergeBlock)
	fieldEnd := c.builder.GetInsertBlock()

	// accessor: getter = objVtable[slot]; call getter(obj)
	c.builder.SetInsertPointAtEnd(accBlock)
	vtable := c.loadObjectVTable(obj)
	fnPtrAddr := c.builder.CreateInBoundsGEP(ptrType, vtable, []llvm.Value{slot}, "getterPtrAddr")
	fnPtr := c.builder.CreateLoad(ptrType, fnPtrAddr, "getterPtr")
	getterType := llvm.FunctionType(propType, []llvm.Type{llvm.PointerType(c.cxt.VoidType(), 1)}, false)
	vAcc := c.builder.CreateCall(getterType, fnPtr, []llvm.Value{obj}, "ipropGetter")
	c.builder.CreateBr(mergeBlock)
	accEnd := c.builder.GetInsertBlock()

	c.builder.SetInsertPointAtEnd(mergeBlock)
	phi := c.builder.CreatePHI(propType, "ipropValue")
	phi.AddIncoming([]llvm.Value{vField, vAcc}, []llvm.BasicBlock{fieldEnd, accEnd})
	return phi
}

// genInterfacePropertySet writes an interface property through the tagged set-itable: a field store
// at the byte offset, or a setter call through the object's own vtable — chosen at runtime.
func (c *CodegenModule) genInterfacePropertySet(access interfacePropAccessInfo, value llvm.Value) {
	obj := access.object
	info := c.refInterfacePropTables(access.iface)
	propType := c.interfacePropertyType(access.iface, access.propName)
	tag, isAcc, slot := c.interfacePropTagAndSlot(access, info.setGlobal, info.tableType)
	ptrType := llvm.PointerType(c.cxt.VoidType(), 0)

	fn := c.builder.GetInsertBlock().Parent()
	fieldBlock := c.cxt.AddBasicBlock(fn, "iprop.set.field")
	accBlock := c.cxt.AddBasicBlock(fn, "iprop.set.acc")
	mergeBlock := c.cxt.AddBasicBlock(fn, "iprop.set.merge")
	c.builder.CreateCondBr(isAcc, accBlock, fieldBlock)

	// field: store value, (obj + offset)
	c.builder.SetInsertPointAtEnd(fieldBlock)
	fieldPtr := c.builder.CreateInBoundsGEP(c.cxt.Int8Type(), obj, []llvm.Value{tag}, "ipropFieldPtr")
	c.builder.CreateStore(value, fieldPtr)
	c.builder.CreateBr(mergeBlock)

	// accessor: setter = objVtable[slot]; call setter(value, obj)  (method ABI: args then receiver)
	c.builder.SetInsertPointAtEnd(accBlock)
	vtable := c.loadObjectVTable(obj)
	fnPtrAddr := c.builder.CreateInBoundsGEP(ptrType, vtable, []llvm.Value{slot}, "setterPtrAddr")
	fnPtr := c.builder.CreateLoad(ptrType, fnPtrAddr, "setterPtr")
	setterType := llvm.FunctionType(c.cxt.VoidType(), []llvm.Type{propType, llvm.PointerType(c.cxt.VoidType(), 1)}, false)
	c.builder.CreateCall(setterType, fnPtr, []llvm.Value{value, obj}, "")
	c.builder.CreateBr(mergeBlock)

	c.builder.SetInsertPointAtEnd(mergeBlock)
}

func (c *CodegenModule) genObjectPropertyAccess(input ir.ObjectPropertyAccessInstrInput, output zeus_value.Var) {
	objectType := c.getValueType(input.Object)
	llvmValue := c.toLLVMValue(input.Object)

	// Property access through an interface value never reaches codegen as an OBJECT_PROPERTY_ACCESS:
	// InterfacePropertyLoweringPass has already folded it into INTERFACE_PROP_GET/SET (a field is
	// not guaranteed — the concrete member may be a get/set accessor, resolved at runtime).

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

// genElemLoad lowers a primitive array element read to a direct GEP + load on the
// array's raw element buffer: value = data[index]. No vtable dispatch, no runtime call.
func (c *CodegenModule) genElemLoad(input ir.ElemLoadInstrInput, output zeus_value.Var) {
	dataPtr := c.toLLVMValue(input.Data)
	index := c.toLLVMValue(input.Index)
	elemType := c.toLLVMType(input.ElemType)
	elemPtr := c.builder.CreateInBoundsGEP(elemType, dataPtr, []llvm.Value{index}, "elem_ptr")
	c.llvmValues[output.Name] = c.builder.CreateLoad(elemType, elemPtr, "elem_val")
}

// genElemStore lowers a primitive array element write to a direct GEP + store on the
// array's raw element buffer: data[index] = value. No vtable dispatch, no runtime call.
func (c *CodegenModule) genElemStore(input ir.ElemStoreInstrInput) {
	dataPtr := c.toLLVMValue(input.Data)
	index := c.toLLVMValue(input.Index)
	value := c.toLLVMValue(input.Value)
	elemType := c.toLLVMType(input.ElemType)
	elemPtr := c.builder.CreateInBoundsGEP(elemType, dataPtr, []llvm.Value{index}, "elem_ptr")
	c.builder.CreateStore(value, elemPtr)
}

func (c *CodegenModule) genIndirectFuncCall(input ir.IndirectFuncCallInstrInput, output zeus_value.Var) {
	functionType := zeus_value.AsFunctionType(zeus_value.GetValueType(input.Function))
	zeus_error.Assert(functionType != nil, fmt.Sprintf("INDIRECT_FUNC_CALL: %s is not a FunctionType", input.Function))

	// All FunctionType values are functor objects (ptr addrspace(1)); dispatch via vtable slot 0.
	functorObj := c.toLLVMValue(input.Function)
	methodPtr := c.loadVTableMethodPtr(functorObj, nil, 0, token.FUNCTOR_CALL_METHOD_NAME)

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
	// super.method() — a non-virtual call resolved directly on the base class.
	if input.StaticClass != nil {
		c.genStaticMethodCall(input, output)
		return
	}

	objectType := c.getValueType(input.Object)

	// Method call through an interface value dispatches dynamically via the interface's
	// itable, keyed by the concrete object's class id.
	if ifaceType := zeus_value.AsInterfaceType(objectType); ifaceType != nil {
		c.genInterfaceMethodCall(input, ifaceType.Interface, output)
		return
	}

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

	// Resolve the method through the inheritance chain (it may be defined on a base class).
	// The signature drives the call type; dispatch itself goes through the vtable slot above,
	// which for an overridden method holds the derived implementation.
	var foundMethod *zeus_value.Function
	if m := zeus_value.LookupMethod(objectClass.Class, input.MethodName); m != nil {
		foundMethod = m.Method
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

// genStaticMethodCall emits super.method(): a non-virtual call. The method is resolved on the
// base class chain (StaticClass) and called by its symbol directly — never through the vtable —
// so an override on the receiver is bypassed. `this` is passed unchanged as the receiver.
func (c *CodegenModule) genStaticMethodCall(input ir.MethodCallInstrInput, output zeus_value.Var) {
	llvmObject := c.toLLVMValue(input.Object)

	// Resolve the method (and the class that defines it) by walking the base chain.
	var foundMethod *zeus_value.ClassMethod
	definingClass := input.StaticClass
	for cur := input.StaticClass; cur != nil; cur = cur.ParentClass {
		for _, m := range cur.Methods {
			if m.Method.SourceName() == input.MethodName {
				foundMethod, definingClass = m, cur
				break
			}
		}
		if foundMethod != nil {
			break
		}
	}
	zeus_error.Assert(foundMethod != nil, fmt.Sprintf("super method %s not found in class %s", input.MethodName, input.StaticClass.Name))

	// User methods are named by their IR name; extern (primordial) methods are class-scoped.
	fnName := foundMethod.Method.Name
	if foundMethod.IsExtern {
		fnName = util.GetClassMethodName(definingClass.Name, foundMethod.Method.Name)
	}
	// Resolve via our own IR-name→function map, not module.NamedFunction: the latter queries LLVM's
	// global symbol table (which also holds runtime, factory, and global symbols) and would silently
	// return a collision-renamed or non-method function. getFunctionSymbol panics on a genuine miss.
	llvmFn := c.getFunctionSymbol(fnName)

	paramTypes := make([]zeus_value.ValueType, len(foundMethod.Method.Params))
	for i, p := range foundMethod.Method.Params {
		paramTypes[i] = p.ValueType
	}
	paramTypes = append(paramTypes, zeus_value.NewObjectType(definingClass))
	fnType := c.toLLVMFunctionType(zeus_value.FunctionType{ReturnType: foundMethod.Method.ReturnType, ParamTypes: paramTypes})

	args := []llvm.Value{}
	for _, arg := range input.Args {
		args = append(args, c.toLLVMValue(arg))
	}
	args = append(args, llvmObject)

	llvmValue := c.builder.CreateCall(fnType, llvmFn, args, fmt.Sprintf("%s_super_result", input.MethodName))
	c.llvmValues[output.Name] = llvmValue
}

// genSuperConstructorCall emits `super(...)`: a direct (non-virtual) call to the base class's
// constructor with the current object as `this`. ParentClass is the nearest ancestor that
// declares a constructor, so its LLVMConstructorMethod is available.
func (c *CodegenModule) genSuperConstructorCall(input ir.SuperConstructorCallInstrInput) {
	parentClass := input.ParentClass
	llvmConstructor := c.getLLVMConstructorMethod(parentClass.Name)
	zeus_error.Assert(llvmConstructor != nil, fmt.Sprintf("constructor for base class %s not found", parentClass.Name))

	// Locate the base constructor descriptor to build the call's function type.
	var constructor *zeus_value.Function
	for _, method := range parentClass.Methods {
		if method.Method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
			constructor = method.Method
			break
		}
	}
	zeus_error.Assert(constructor != nil, fmt.Sprintf("constructor descriptor for base class %s not found", parentClass.Name))

	paramTypes := []zeus_value.ValueType{}
	for _, param := range constructor.Params {
		paramTypes = append(paramTypes, param.ValueType)
	}
	paramTypes = append(paramTypes, zeus_value.NewObjectType(parentClass))
	constructorType := c.toLLVMFunctionType(zeus_value.NewFunctionType(zeus_value.VoidType{}, paramTypes))

	// Arguments are the super(...) args followed by `this`.
	args := []llvm.Value{}
	for _, arg := range input.Args {
		args = append(args, c.toLLVMValue(arg))
	}
	args = append(args, c.toLLVMValue(input.ThisObject))

	c.builder.CreateCall(constructorType, *llvmConstructor, args, "")
}

func (c *CodegenModule) genDeclPrimordialFunc(input ir.DeclPrimordialFuncInstrInput) {
	function := input.Function
	functionType := zeus_value.ToFunctionType(*function)

	// The runtime symbol is the function's ExternRuntimeName when set (extern prelude functions),
	// else the derived zeus_<name> (the historical convention for Go-registered primordial fns).
	externalFuncName := function.ExternRuntimeName
	if externalFuncName == "" {
		externalFuncName = fmt.Sprintf("zeus_%s", function.Name)
	}
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
	case zeus_value.InterfaceType:
		// An interface value is an object pointer; its zero value is null.
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
			input := ir.AsDeclClassInstrInput(instr.Input)
			c.genDeclClass(*input, *instr.Output)
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
		case ir.InstrTypeDeclGlobalVar:
			c.setDebugLocation(instr.Span)
			c.genDeclGlobalVar(*ir.AsDeclGlobalVarInstrInput(instr.Input))
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
		case ir.InstrTypeAllocObj:
			c.setDebugLocation(instr.Span)
			c.genAllocObj(*ir.AsAllocObjInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeObjectPropertyAccess:
			c.setDebugLocation(instr.Span)
			c.genObjectPropertyAccess(*ir.AsObjectPropertyAccessInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeInterfacePropGet:
			c.setDebugLocation(instr.Span)
			c.genInterfacePropGet(*ir.AsInterfacePropGetInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeInterfacePropSet:
			c.setDebugLocation(instr.Span)
			c.genInterfacePropSet(*ir.AsInterfacePropSetInstrInput(instr.Input))
		case ir.InstrTypeIndirectFuncCall:
			c.setDebugLocation(instr.Span)
			c.genIndirectFuncCall(*ir.AsIndirectFuncCallInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeMethodCall:
			c.setDebugLocation(instr.Span)
			c.genMethodCall(*ir.AsMethodCallInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeElemLoad:
			c.setDebugLocation(instr.Span)
			c.genElemLoad(*ir.AsElemLoadInstrInput(instr.Input), *instr.Output)
		case ir.InstrTypeElemStore:
			c.setDebugLocation(instr.Span)
			c.genElemStore(*ir.AsElemStoreInstrInput(instr.Input))
		case ir.InstrTypeSuperConstructorCall:
			c.setDebugLocation(instr.Span)
			c.genSuperConstructorCall(*ir.AsSuperConstructorCallInstrInput(instr.Input))
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

	// Fill every (non-imported) class's vtable in one pass, resolving each slot's function by name
	// now that all method bodies (own, inherited, extern) have been emitted.
	c.fillVTables()

	// Phase 3: Generate factory bodies for locally-defined primordials/arrays only. Imported
	// classes already have their factory bodies in the exporting module, and user-class factory
	// bodies are now synthesized as IR (FactoryLoweringPass) and compiled like any other function.
	for _, structInfo := range c.zeusClassLLVMStructMap {
		if _, isImported := c.importedClasses[structInfo.ZeusClass.Name]; isImported {
			continue
		}
		if structInfo.ZeusClass.PrimordialName == "" {
			continue
		}
		if isObjectArrayHandle(structInfo.ZeusClass) {
			continue // uses the shared Object[] factory (no per-type factory declared)
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
	// Allocate jmp_buf as an array of i64 words so the alloca has natural 8-byte
	// alignment on all platforms. sizeof(jmp_buf) on macOS arm64 is 192 bytes;
	// 256 bytes (32 × 8) is a safe upper bound that also covers x86-64/Linux.
	jmpBufType := llvm.ArrayType(c.cxt.Int64Type(), 32)
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
