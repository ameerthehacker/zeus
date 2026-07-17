package zeus_value

import (
	"fmt"
	"sync"

	"github.com/ameerthehacker/zeus/internal/token"
)

// PrimordialRegistry is a centralized registry for all primordial classes and functions.
// It provides a single source of truth for primordial types and ensures consistent
// class definitions throughout the compilation pipeline.
type PrimordialRegistry struct {
	mu sync.RWMutex

	// Fixed primordial classes (string, etc.)
	classes map[string]*Class

	// Parameterized array classes (u8[], i32[], Point[], etc.)
	arrayClasses map[string]*Class

	// classOrder maintains insertion order so GetAllClasses returns classes in
	// dependency-safe order regardless of Go map iteration randomness.
	classOrder []string

	// Primordial functions (log, etc.)
	functions map[string]*Function

	// Primordial interfaces (Number, ...) declared in preludes. Interfaces emit no IR, so
	// ir.loadPreludes harvests them from the compiled prelude's symbol table and registers them
	// here; initializePrimordials injects them into every module. interfaceOrder is insertion order.
	interfaces     map[string]*Interface
	interfaceOrder []string

	// Default span for generated primordials
	defaultSpan *token.Span
}

// Global registry instance
var Registry = newPrimordialRegistry()

func newPrimordialRegistry() *PrimordialRegistry {
	defaultSpan := token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 1))

	r := &PrimordialRegistry{
		classes:        make(map[string]*Class),
		arrayClasses:   make(map[string]*Class),
		classOrder:     make([]string, 0),
		functions:      make(map[string]*Function),
		interfaces:     make(map[string]*Interface),
		interfaceOrder: make([]string, 0),
		defaultSpan:    defaultSpan,
	}

	r.registerBaseClasses()
	// Primordial functions (timers) are compiled from prelude/timers.zs and self-register via
	// RegisterFunction when preludes load (ir.loadPreludes).

	return r
}

func (r *PrimordialRegistry) registerBaseClasses() {
	// u8[] is fundamental — the string prelude references it. Every other primordial class
	// (string, Error, Console) is compiled from prelude/*.zs and injected via RegisterClass when
	// preludes load (ir.loadPreludes); u8[] must exist first so the string prelude resolves it.
	// getOrCreateArrayClassUnsafe registers the array class and marks its runtime-backed methods
	// extern, so no further per-class setup is needed here.
	u8ArrayType := ArrayType{
		ElementType: IntType{Size: I8, Signed: false, Span: r.defaultSpan},
		Span:        r.defaultSpan,
	}
	r.getOrCreateArrayClassUnsafe(u8ArrayType)
}

// MarkExternMethods flags a primordial class's methods (except IsLowered ones, which are
// expanded by IR lowering) as extern — their bodies forward to the Zig runtime function
// zeus_<primordial>_<method>, recorded on the method so codegen needs no primordial context.
func MarkExternMethods(class *Class) {
	for _, method := range class.Methods {
		if method.IsLowered {
			continue
		}
		method.IsExtern = true
		method.Method.ExternRuntimeName = fmt.Sprintf("zeus_%s_%s", class.PrimordialName, method.Method.Name)
	}
}


// resolveArrayMethodTypes replaces raw ArrayType values in a class's ArrayElementType,
// method parameter/return types, and property types with ObjectType{*registeredClass}.
// Must be called after the class itself is registered in r.arrayClasses so that
// self-referential types (e.g. concat returns selfArrayType) resolve correctly.
func (r *PrimordialRegistry) resolveArrayMethodTypes(class *Class) {
	resolveType := func(t ValueType) ValueType {
		if t == nil {
			return t
		}
		at, ok := t.(ArrayType)
		if !ok {
			return t
		}
		if nested, ok := r.arrayClasses[at.String()]; ok {
			return NewObjectType(nested)
		}
		return t
	}
	// Resolve the element type stored on the class (used by codegen to emit runtime type info)
	if class.ArrayElementType != nil {
		class.ArrayElementType = resolveType(class.ArrayElementType)
	}
	for _, prop := range class.Properties {
		prop.Property.ValueType = resolveType(prop.Property.ValueType)
	}
	for _, method := range class.Methods {
		method.Method.ReturnType = resolveType(method.Method.ReturnType)
		for _, param := range method.Method.Params {
			param.ValueType = resolveType(param.ValueType)
		}
	}
}

// getOrCreateArrayClassUnsafe is the internal version without locking (for initialization)
func (r *PrimordialRegistry) getOrCreateArrayClassUnsafe(arrayType ArrayType) *Class {
	className := arrayType.String()

	if class, ok := r.arrayClasses[className]; ok {
		return class
	}

	// Handle nested array types: ensure the element array class exists first
	if elementArrayType, ok := arrayType.ElementType.(ArrayType); ok {
		r.getOrCreateArrayClassUnsafe(elementArrayType)
	}

	class := GetArrayPrimordialClassDefinition(arrayType)
	r.arrayClasses[className] = class
	r.classOrder = append(r.classOrder, className)
	// Resolve raw ArrayType signatures now that all nested classes are registered.
	r.resolveArrayMethodTypes(class)
	MarkExternMethods(class)
	return class
}

// GetOrCreateArrayClass returns an array class for the given element type
func (r *PrimordialRegistry) GetOrCreateArrayClass(arrayType ArrayType) *Class {
	className := arrayType.String()

	r.mu.Lock()
	defer r.mu.Unlock()

	if class, ok := r.arrayClasses[className]; ok {
		return class
	}

	// Handle nested array types
	if elementArrayType, ok := arrayType.ElementType.(ArrayType); ok {
		r.getOrCreateArrayClassUnsafe(elementArrayType)
	}

	class := GetArrayPrimordialClassDefinition(arrayType)
	r.arrayClasses[className] = class
	r.classOrder = append(r.classOrder, className)
	// Resolve raw ArrayType signatures now that all nested classes are registered.
	r.resolveArrayMethodTypes(class)
	MarkExternMethods(class)
	return class
}

// RegisterClass registers (or replaces) a fixed primordial class — e.g. one compiled from a
// prelude source file at compiler startup. It appends to classOrder on first registration so
// GetAllClasses emits it in a dependency-safe position (after the base classes it may reference).
func (r *PrimordialRegistry) RegisterClass(class *Class) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.classes[class.Name]; !exists {
		r.classOrder = append(r.classOrder, class.Name)
	}
	r.classes[class.Name] = class
}

// RegisterInterface registers a primordial interface declared in a prelude (Number, …), idempotent
// by name. Interfaces emit no IR, so ir.loadPreludes harvests them from the compiled prelude's
// symbol table; initializePrimordials injects them into every module via GetAllInterfaces.
func (r *PrimordialRegistry) RegisterInterface(iface *Interface) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.interfaces[iface.Name]; !exists {
		r.interfaceOrder = append(r.interfaceOrder, iface.Name)
	}
	r.interfaces[iface.Name] = iface
}

// GetInterface returns a registered primordial interface by name, or nil.
func (r *PrimordialRegistry) GetInterface(name string) *Interface {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.interfaces[name]
}

// GetAllInterfaces returns the registered primordial interfaces in insertion order.
func (r *PrimordialRegistry) GetAllInterfaces() []*Interface {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Interface, 0, len(r.interfaceOrder))
	for _, name := range r.interfaceOrder {
		out = append(out, r.interfaces[name])
	}
	return out
}

// RegisterFunction registers a primordial free function — e.g. an extern function compiled from a
// prelude (setTimeout, …). It is then emitted into every module via initializePrimordialFunctions.
func (r *PrimordialRegistry) RegisterFunction(fn *Function) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.functions[fn.Name] = fn
}

// GetClass returns a fixed primordial class by name (string, error, Console, ...).
// Returns nil if not found. Does not search parameterized array classes.
func (r *PrimordialRegistry) GetClass(name string) *Class {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.classes[name]
}

// GetAllClasses returns all registered primordial classes in dependency-safe insertion order.
// Array classes come before fixed classes (e.g. u8[] before string before Error) because
// the registry appends them to classOrder in that exact sequence.
func (r *PrimordialRegistry) GetAllClasses() []*Class {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Class, 0, len(r.classOrder))
	for _, name := range r.classOrder {
		if class, ok := r.arrayClasses[name]; ok {
			result = append(result, class)
		} else if class, ok := r.classes[name]; ok {
			result = append(result, class)
		}
	}
	return result
}

// GetAllFunctions returns all registered primordial functions
func (r *PrimordialRegistry) GetAllFunctions() []*Function {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Function, 0, len(r.functions))
	for _, fn := range r.functions {
		result = append(result, fn)
	}
	return result
}
