package zeus_value

import (
	"github.com/ameerthehacker/zeus/internal/token"
)

const ZEUS_PRIMORDIAL_ARRAY = "array"

// Array method names
const (
	ARRAY_METHOD_PUSH = "push"
	ARRAY_METHOD_POP  = "pop"
	ARRAY_METHOD_GET  = "get"
	ARRAY_METHOD_SET  = "set"
)

func GetArrayPrimordialClassDefinition(arrayType ArrayType) *Class {
	// capacity of the array
	capacityProperty := NewClassProperty(NewVar("capacity", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan()), &token.Token{Type: token.TokenTypePrivate, Span: arrayType.GetSpan()})
	// length of the array
	lengthProperty := NewClassProperty(NewVar("length", IntType{Size: I32, Span: arrayType.GetSpan()}, false, arrayType.GetSpan()), &token.Token{Type: token.TokenTypePrivate, Span: arrayType.GetSpan()})
	// opaque pointer to the data of the array
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
	methods := []*ClassMethod{
		NewClassMethod(constructorMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(pushMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(popMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(getMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
		NewClassMethod(setMethod, &token.Token{Type: token.TokenTypePublic, Span: arrayType.GetSpan()}),
	}
	return NewClass(arrayType.String(), properties, methods, ZEUS_PRIMORDIAL_ARRAY, arrayType.ElementType, arrayType.GetSpan())
}

func GetPrimordialFunctionDefinitions() []*Function {
	span := token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 1))
		logFunction := NewFunction(
		"log",
		[]*Var{
			NewVar("message", NewArrayType(IntType{
				Signed: false,
				Size: I8,
				Span: span,
			}, span), false, span),
		},
		VoidType{Span: span},
		span,
	)

	return []*Function{logFunction}
}