package zeus_value

import (
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
}

func (r *PrimordialRegistry) registerFunctions() {
	span := r.defaultSpan
	stringClass := r.classes[ZEUS_PRIMORDIAL_STRING]
	r.functions["log"] = NewFunction(
		"log",
		[]*Var{NewVar("message", ObjectType{Class: *stringClass}, false, span)},
		VoidType{Span: span},
		span,
	)
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
	return class
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
