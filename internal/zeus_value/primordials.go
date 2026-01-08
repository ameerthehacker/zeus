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
	STRING_PROPERTY_DATA = "data"
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
	dataProperty := NewClassProperty(NewVar("data", u8ArrayObjectType, true, span), &token.Token{Type: token.TokenTypePrivate, Span: span})
	constructorMethod := NewFunction(token.CONSTRUCTOR_METHOD_NAME, []*Var{
		NewVar("bytes", u8ArrayObjectType, true, span),
	}, VoidType{Span: span}, span)
	return NewClass(ZEUS_PRIMORDIAL_STRING, []*ClassProperty{dataProperty}, []*ClassMethod{NewClassMethod(constructorMethod, &token.Token{Type: token.TokenTypePublic, Span: span})}, ZEUS_PRIMORDIAL_STRING, nil, span)
}

