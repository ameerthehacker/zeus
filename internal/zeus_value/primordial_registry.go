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

	// Error class - base class for all exceptions (must be registered before any Error subclasses)
	r.classes[ZEUS_PRIMORDIAL_ERROR] = GetErrorPrimordialClassDefinition(r.defaultSpan)
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
}

// getOrCreateArrayClassUnsafe is the internal version without locking (for initialization)
func (r *PrimordialRegistry) getOrCreateArrayClassUnsafe(arrayType ArrayType) *Class {
	className := arrayType.String()

	if class, ok := r.arrayClasses[className]; ok {
		return class
	}

	// Handle nested array types
	if elementArrayType, ok := arrayType.ElementType.(ArrayType); ok {
		r.getOrCreateArrayClassUnsafe(elementArrayType)
	}

	class := GetArrayPrimordialClassDefinition(arrayType)
	r.arrayClasses[className] = class
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
	return class
}

// GetAllClasses returns all registered primordial classes (fixed + array)
// Array classes are returned first since other classes (like string) may depend on them
func (r *PrimordialRegistry) GetAllClasses() []*Class {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Class, 0, len(r.classes)+len(r.arrayClasses))
	// Array classes first - other classes (like string) may depend on u8[]
	for _, class := range r.arrayClasses {
		result = append(result, class)
	}
	for _, class := range r.classes {
		result = append(result, class)
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
