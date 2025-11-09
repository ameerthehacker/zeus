package zeus_value

import (
	"github.com/ameerthehacker/zeus/internal/token"
)

const ZEUS_PRIMORDIAL_ARRAY = "array"

func GetArrayPrimordialClassDefinition(arrayType ArrayType) *Class {
	// capacity of the array
	capacityProperty := NewClassProperty(NewVar("capacity", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan()), &token.Token{Type: token.TokenTypePrivate, Span: arrayType.GetSpan()})
	// length of the array
	lengthProperty := NewClassProperty(NewVar("length", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan()), &token.Token{Type: token.TokenTypePrivate, Span: arrayType.GetSpan()})
	elementSizeProperty := NewClassProperty(NewVar("element_size", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan()), &token.Token{Type: token.TokenTypePrivate, Span: arrayType.GetSpan()})
	// opaque pointer to the data of the array
	dataProperty := NewClassProperty(NewVar("data", OpaqueType{Span: arrayType.GetSpan()}, true, arrayType.GetSpan()), &token.Token{Type: token.TokenTypePrivate, Span: arrayType.GetSpan()})
	properties := []*ClassProperty{capacityProperty, lengthProperty, elementSizeProperty, dataProperty}
	// declare all the public methods of an array
	constructorMethod := NewFunction(token.CONSTRUCTOR_METHOD_NAME, []*Var{
		NewVar("capacity", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan()),
		NewVar("element_size", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan()),
	}, VoidType{Span: arrayType.GetSpan()}, arrayType.GetSpan())
	pushMethod := NewFunction("push", []*Var{NewVar("value", arrayType.ElementType, false, arrayType.GetSpan())}, VoidType{Span: arrayType.GetSpan()}, arrayType.GetSpan())
	popMethod := NewFunction("pop", []*Var{}, arrayType.ElementType, arrayType.GetSpan())
	getMethod := NewFunction("get", []*Var{NewVar("index", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan())}, arrayType.ElementType, arrayType.GetSpan())
	methods := []*ClassMethod{
		NewClassMethod(constructorMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(pushMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(popMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(getMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
	}
	return NewClass(arrayType.String(), properties, methods, ZEUS_PRIMORDIAL_ARRAY, arrayType.GetSpan())
}
