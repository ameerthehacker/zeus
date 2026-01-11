package zeus_value

import (
	"github.com/ameerthehacker/zeus/internal/token"
)

const ZEUS_PRIMORDIAL_ARRAY = "array"
const ZEUS_PRIMORDIAL_STRING = "string"

// Array method names
const (
	ARRAY_METHOD_PUSH = "push"
	ARRAY_METHOD_POP  = "pop"
	ARRAY_METHOD_GET  = "get"
	ARRAY_METHOD_SET  = "set"
	ARRAY_METHOD_COPY = "copy"
)

// Array property names
const (
	ARRAY_PROPERTY_CAPACITY = "capacity"
	ARRAY_PROPERTY_LENGTH   = "length"
	ARRAY_PROPERTY_DATA     = "data"
)

// String property names
const (
	STRING_PROPERTY_DATA   = "data"
	STRING_PROPERTY_LENGTH = "length"
)

// String method names
const (
	STRING_METHOD_COMPARE = "compare"
	STRING_METHOD_EQUALS  = "equals"
	STRING_METHOD_CONCAT  = "concat"
)

func GetArrayPrimordialClassDefinition(arrayType ArrayType) *Class {
	// capacity of the array (private - internal implementation detail)
	capacityProperty := NewClassProperty(NewVar("capacity", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan()), &token.Token{Type: token.TokenTypePrivate, Span: arrayType.GetSpan()})
	// length of the array (public - commonly accessed property)
	lengthProperty := NewClassProperty(NewVar("length", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan()), &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()})
	// opaque pointer to the data of the array (private - internal implementation)
	dataProperty := NewClassProperty(NewVar("data", OpaqueType{Span: arrayType.GetSpan()}, true, arrayType.GetSpan()), &token.Token{Type: token.TokenTypePrivate, Span: arrayType.GetSpan()})
	properties := []*ClassProperty{capacityProperty, lengthProperty, dataProperty}
	// declare all the public methods of an array
	constructorMethod := NewFunction(token.CONSTRUCTOR_METHOD_NAME, []*Var{
		NewVar("capacity", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan()),
	}, VoidType{Span: arrayType.GetSpan()}, arrayType.GetSpan())
	pushMethod := NewFunction(ARRAY_METHOD_PUSH, []*Var{NewVar("value", arrayType.ElementType, false, arrayType.GetSpan())}, VoidType{Span: arrayType.GetSpan()}, arrayType.GetSpan())
	popMethod := NewFunction(ARRAY_METHOD_POP, []*Var{}, arrayType.ElementType, arrayType.GetSpan())
	getMethod := NewFunction(ARRAY_METHOD_GET, []*Var{NewVar("index", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan())}, arrayType.ElementType, arrayType.GetSpan())
	setMethod := NewFunction(ARRAY_METHOD_SET, []*Var{
		NewVar("index", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan()),
		NewVar("value", arrayType.ElementType, false, arrayType.GetSpan()),
	}, VoidType{Span: arrayType.GetSpan()}, arrayType.GetSpan())
	// copy method: copies data from another array of the same type
	copyMethod := NewFunction(ARRAY_METHOD_COPY, []*Var{
		NewVar("source", NewArrayType(arrayType.ElementType, arrayType.GetSpan()), false, arrayType.GetSpan()),
	}, VoidType{Span: arrayType.GetSpan()}, arrayType.GetSpan())
	methods := []*ClassMethod{
		NewClassMethod(constructorMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(pushMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(popMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(getMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(setMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(copyMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
	}
	return NewClass(arrayType.String(), properties, methods, ZEUS_PRIMORDIAL_ARRAY, arrayType.ElementType, arrayType.GetSpan())
}

// string is nothing but an array of u8
// in the runtime constructor we intern the string and return the same pointer
func GetStringPrimordialClassDefinition(span *token.Span) *Class {
	u8ArrayObjectType := ObjectType{Class: *GetArrayPrimordialClassDefinition(ArrayType{ElementType: IntType{Size: I8, Signed: false, Span: span}, Span: span})}

	// Properties
	dataProperty := NewClassProperty(NewVar(STRING_PROPERTY_DATA, u8ArrayObjectType, true, span), &token.Token{Type: token.TokenTypePrivate, Span: span})
	lengthProperty := NewClassProperty(NewVar(STRING_PROPERTY_LENGTH, IntType{Size: I32, Signed: true, Span: span}, false, span), &token.Token{Type: token.TokenTypePublic, Span: span})
	properties := []*ClassProperty{dataProperty, lengthProperty}

	// Create the string class first (without methods that reference itself)
	stringClass := NewClass(ZEUS_PRIMORDIAL_STRING, properties, nil, ZEUS_PRIMORDIAL_STRING, nil, span)

	// Methods - use UserDefinedType for self-reference to avoid copy issues
	// The type checker will resolve "string" to the actual string ObjectType
	selfType := UserDefinedType{Name: ZEUS_PRIMORDIAL_STRING, Span: span}

	constructorMethod := NewFunction(token.CONSTRUCTOR_METHOD_NAME, []*Var{
		NewVar("bytes", u8ArrayObjectType, true, span),
	}, VoidType{Span: span}, span)
	// compare: returns -1 if this < other, 0 if equal, 1 if this > other
	compareMethod := NewFunction(STRING_METHOD_COMPARE, []*Var{
		NewVar("other", selfType, true, span),
	}, IntType{Size: I8, Signed: true, Span: span}, span)
	// equals: returns true if strings are equal, false otherwise
	equalsMethod := NewFunction(STRING_METHOD_EQUALS, []*Var{
		NewVar("other", selfType, true, span),
	}, BoolType{Span: span}, span)
	// concat: returns a new string with the other string appended
	concatMethod := NewFunction(STRING_METHOD_CONCAT, []*Var{
		NewVar("other", selfType, true, span),
	}, selfType, span)

	// Update the class with methods
	stringClass.Methods = []*ClassMethod{
		NewClassMethod(constructorMethod, &token.Token{Type: token.TokenTypePublic, Span: span}),
		NewClassMethod(compareMethod, &token.Token{Type: token.TokenTypePublic, Span: span}),
		NewClassMethod(equalsMethod, &token.Token{Type: token.TokenTypePublic, Span: span}),
		NewClassMethod(concatMethod, &token.Token{Type: token.TokenTypePublic, Span: span}),
	}

	return stringClass
}

