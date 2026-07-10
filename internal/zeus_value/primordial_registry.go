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

	// Default span for generated primordials
	defaultSpan *token.Span
}

// Global registry instance
var Registry = newPrimordialRegistry()

func newPrimordialRegistry() *PrimordialRegistry {
	defaultSpan := token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 1))

	r := &PrimordialRegistry{
		classes:      make(map[string]*Class),
		arrayClasses: make(map[string]*Class),
		classOrder:   make([]string, 0),
		functions:    make(map[string]*Function),
		defaultSpan:  defaultSpan,
	}

	r.registerBaseClasses()
	r.registerFunctions()

	return r
}

func (r *PrimordialRegistry) registerBaseClasses() {
	// u8[] is fundamental - needed by string
	u8ArrayType := ArrayType{
		ElementType: IntType{Size: I8, Signed: false, Span: r.defaultSpan},
		Span:        r.defaultSpan,
	}
	r.getOrCreateArrayClassUnsafe(u8ArrayType)

	// string class - reuse the existing definition
	r.classes[ZEUS_PRIMORDIAL_STRING] = GetStringPrimordialClassDefinition(r.defaultSpan)
	r.classOrder = append(r.classOrder, ZEUS_PRIMORDIAL_STRING)

	// Error class - base class for all exceptions (must be registered before any Error subclasses)
	r.classes[ZEUS_PRIMORDIAL_ERROR] = GetErrorPrimordialClassDefinition(r.defaultSpan)
	r.classOrder = append(r.classOrder, ZEUS_PRIMORDIAL_ERROR)

	// Console class - global console object (log/error/info)
	r.classes[ZEUS_PRIMORDIAL_CONSOLE] = GetConsolePrimordialClassDefinition(r.defaultSpan)
	r.classOrder = append(r.classOrder, ZEUS_PRIMORDIAL_CONSOLE)

	// Resolve UserDefinedType{"string"} to ObjectType{*stringClass} in all registered class
	// method signatures. Primordial definitions use UserDefinedType as a placeholder to avoid
	// forward-reference issues; now that string is registered we can resolve them.
	r.resolveStringRefs()

	// Every non-lowered primordial method is backed by a Zig runtime function; mark them extern
	// so codegen emits their bodies through the shared extern-method path (not a special case).
	for _, class := range r.classes {
		MarkExternMethods(class)
	}
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

// resolveStringRefs rewrites every UserDefinedType{"string"} in all registered primordial
// class property/method signatures to ObjectType{*stringClass}. Called once after all base
// classes are registered so the type checker sees concrete ObjectType values, not raw
// UserDefinedType placeholders.
func (r *PrimordialRegistry) resolveStringRefs() {
	stringClass := r.classes[ZEUS_PRIMORDIAL_STRING]
	if stringClass == nil {
		return
	}

	resolveType := func(t ValueType) ValueType {
		if t == nil {
			return t
		}
		if udt, ok := t.(UserDefinedType); ok && udt.Name == ZEUS_PRIMORDIAL_STRING {
			return NewObjectType(stringClass)
		}
		return t
	}

	resolveClass := func(class *Class) {
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

	for _, class := range r.classes {
		resolveClass(class)
	}
	for _, class := range r.arrayClasses {
		resolveClass(class)
	}
}

func (r *PrimordialRegistry) registerFunctions() {
	span := r.defaultSpan
	r.functions["setTimeout"] = NewFunction(
		"setTimeout",
		[]*Var{NewVar("callback", FunctionType{ParamTypes: []ValueType{}, ReturnType: VoidType{Span: span}, Span: span}, false, span), NewVar("delay", IntType{Size: I32, Signed: true, Span: span}, false, span)},
		IntType{Size: I32, Signed: true, Span: span},
		span,
	)
	r.functions["clearTimeout"] = NewFunction(
		"clearTimeout",
		[]*Var{NewVar("id", IntType{Size: I32, Signed: true, Span: span}, false, span)},
		VoidType{Span: span},
		span,
	)
	r.functions["setInterval"] = NewFunction(
		"setInterval",
		[]*Var{NewVar("callback", FunctionType{ParamTypes: []ValueType{}, ReturnType: VoidType{Span: span}, Span: span}, false, span), NewVar("delay", IntType{Size: I32, Signed: true, Span: span}, false, span)},
		IntType{Size: I32, Signed: true, Span: span},
		span,
	)
	r.functions["clearInterval"] = NewFunction(
		"clearInterval",
		[]*Var{NewVar("id", IntType{Size: I32, Signed: true, Span: span}, false, span)},
		VoidType{Span: span},
		span,
	)
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
