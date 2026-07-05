package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// ToKnownTypesPass converts all UserDefinedType references to their actual known types
type ToKnownTypesPass struct{}

func NewToKnownTypesPass() *ToKnownTypesPass {
	return &ToKnownTypesPass{}
}

func (p *ToKnownTypesPass) GetName() string {
	return "ToKnownTypesPass"
}

func (p *ToKnownTypesPass) Finalize(tc *TypeChecker) {
	// No finalization needed for this pass
}

func (p *ToKnownTypesPass) HandleInstruction(tc *TypeChecker, instr *Instr) {
	// Resolve output type if it exists and is a UserDefinedType
	if instr.Output != nil && instr.Output.ValueType != nil {
		instr.Output.ValueType = p.resolveValueType(tc, instr.Output.ValueType, true)
	}

	switch instr.Type {
	case InstrTypeDeclVar:
		p.resolveVarDecl(tc, instr)
	case InstrTypeDeclFunc:
		p.resolveFuncDecl(tc, instr)
	case InstrTypeDeclPrimordialFunc:
		p.resolvePrimordialFuncDecl(tc, instr)
	case InstrTypeDeclClass:
		p.resolveClassDecl(tc, instr)
	case InstrTypeDeclClassMethod:
		p.resolveClassMethodDecl(tc, instr)
	}
}

// resolveValueType converts a UserDefinedType to its actual known type
func (p *ToKnownTypesPass) resolveValueType(tc *TypeChecker, valueType zeus_value.ValueType, isReturnType bool) zeus_value.ValueType {
	undefinedType := zeus_value.UndefinedType{Span: valueType.GetSpan()}
	if zeus_value.IsUserDefinedType(valueType) {
		userDefinedType := zeus_value.AsUserDefinedType(valueType)
		variable, ok := tc.builder.symbolTable.GetSymbol(userDefinedType.Name)

		if !ok {
			// Return undefined type if we can't resolve it
			// The TypeCheckingPass will handle the error
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("unknown type '%s'", userDefinedType.Name),
				Span:    valueType.GetSpan(),
			})
			return undefinedType
		}

		return tc.getBuiltInValueType(variable)
	} else if zeus_value.IsArrayType(valueType) {
		// we convert all the array types to objects
		arrayType := zeus_value.AsArrayType(valueType)
		resolvedElementType := p.resolveValueType(tc, arrayType.ElementType, false)

		// If element type resolution failed, return undefined type
		if zeus_value.IsUndefinedType(resolvedElementType) {
			return undefinedType
		}

		arrayType.ElementType = resolvedElementType
		class := tc.getClassFromArrayType(*arrayType)
		return zeus_value.NewObjectType(*class)
	} else if zeus_value.IsNullType(valueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: "cannot use null as a standalone type",
			Span:    valueType.GetSpan(),
		})
		return undefinedType
	} else if !isReturnType && zeus_value.IsVoidType(valueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: "void type can be used only as a return type",
			Span:    valueType.GetSpan(),
		})
		return undefinedType
	}

	return valueType
}

func (p *ToKnownTypesPass) resolveVarDecl(tc *TypeChecker, instr *Instr) {
	input := AsDeclVarInstrInput(instr.Input)
	if input.Variable.ValueType == nil || zeus_value.IsUndefinedType(input.Variable.ValueType) {
		// Type will be inferred later (from initializer or stores).
		return
	}
	input.Variable.ValueType = p.resolveValueType(tc, input.Variable.ValueType, false)
}

func (p *ToKnownTypesPass) resolveFuncDecl(tc *TypeChecker, instr *Instr) {
	input := AsDeclFuncInstrInput(instr.Input)

	// Resolve return type
	input.Function.ReturnType = p.resolveValueType(tc, input.Function.ReturnType, true)

	// Resolve parameter types
	for i := range input.Function.Params {
		input.Function.Params[i].ValueType = p.resolveValueType(tc, input.Function.Params[i].ValueType, false)
	}
}

func (p *ToKnownTypesPass) resolvePrimordialFuncDecl(tc *TypeChecker, instr *Instr) {
	input := AsDeclPrimordialFuncInstrInput(instr.Input)
	input.Function.ReturnType = p.resolveValueType(tc, input.Function.ReturnType, true)
	for i := range input.Function.Params {
		input.Function.Params[i].ValueType = p.resolveValueType(tc, input.Function.Params[i].ValueType, false)
	}
}

func (p *ToKnownTypesPass) resolveClass(tc *TypeChecker, class *zeus_value.Class) *zeus_value.Class {
	// Resolve property types
	for i := range class.Properties {
		class.Properties[i].Property.ValueType = p.resolveValueType(tc, class.Properties[i].Property.ValueType, false)
	}

	if class.ArrayElementType != nil {
		class.ArrayElementType = p.resolveValueType(tc, class.ArrayElementType, false)
	}

	// Resolve method types
	for i := range class.Methods {
		method := class.Methods[i].Method
		method.ReturnType = p.resolveValueType(tc, method.ReturnType, true)

		for j := range method.Params {
			method.Params[j].ValueType = p.resolveValueType(tc, method.Params[j].ValueType, false)
		}
	}

	return class
}

func (p *ToKnownTypesPass) resolveClassDecl(tc *TypeChecker, instr *Instr) {
	input := AsDeclClassInstrInput(instr.Input)

	p.resolveClass(tc, input.Class)
}

func (p *ToKnownTypesPass) resolveClassMethodDecl(tc *TypeChecker, instr *Instr) {
	input := AsDeclClassMethodInstrInput(instr.Input)

	// Resolve return type
	input.Method.ReturnType = p.resolveValueType(tc, input.Method.ReturnType, true)

	// Resolve parameter types
	for i := range input.Method.Params {
		input.Method.Params[i].ValueType = p.resolveValueType(tc, input.Method.Params[i].ValueType, false)
	}
}
