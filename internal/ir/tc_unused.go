package ir

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// UnusedWarningPass generates warnings for unused identifiers
// This pass should run last after all usage tracking is complete
type UnusedWarningPass struct {
	primordialFunctions []*zeus_value.Function
}

func NewUnusedWarningPass() *UnusedWarningPass {
	return &UnusedWarningPass{
		primordialFunctions: []*zeus_value.Function{},
	}
}

func (p *UnusedWarningPass) GetName() string {
	return "UnusedWarningPass"
}

// markValueAsUsed marks a value (Var or Function) as used if applicable
func markValueAsUsed(value zeus_value.Value) {
	if v := zeus_value.AsVar(value); v != nil {
		v.IsUsed = true
	} else if f := zeus_value.AsFunction(value); f != nil {
		f.IsUsed = true
	}
}

func (p *UnusedWarningPass) HandleInstruction(tc *TypeChecker, instr *Instr) {
	switch instr.Type {
	case InstrTypeDeclVar:
		p.handleVarDecl(instr)
	case InstrTypeStore:
		p.handleStore(instr)
	case InstrTypeLoad:
		p.handleLoad(instr)
	case InstrTypeAdd, InstrTypeSub, InstrTypeMul, InstrTypeDiv, InstrTypeMod, InstrTypePower,
		InstrTypeEqEq, InstrTypeNotEq, InstrTypeLessThan, InstrTypeGreaterThan,
		InstrTypeLessThanEq, InstrTypeGreaterThanEq, InstrTypeAnd, InstrTypeOr,
		InstrTypeBitAnd, InstrTypeBitOr, InstrTypeBitXor, InstrTypeShl, InstrTypeShr:
		p.handleBinaryOp(instr)
	case InstrTypeNot, InstrTypeNeg, InstrTypeBitNot:
		p.handleUnaryOp(instr)
	case InstrTypeCallFunc:
		p.handleCallFunc(instr)
	case InstrTypeIndirectFuncCall:
		p.handleIndirectFuncCall(instr)
	case InstrTypeReturn:
		p.handleReturn(instr)
	case InstrTypeCondJmp:
		p.handleCondJmp(instr)
	case InstrTypeNewObj:
		p.handleNewObj(instr)
	case InstrTypeObjectPropertyAccess:
		p.handleObjectPropertyAccess(tc, instr)
	case InstrTypeMethodCall:
		p.handleMethodCall(tc, instr)
	case InstrTypeDeclPrimordialFunc:
		p.handleDeclPrimordialFunc(instr)
	case InstrTypeExport:
		p.handleExport(instr)
	case InstrTypeGetIndex:
		p.handleGetIndex(instr)
	case InstrTypeCoerce:
		input := AsCoerceInstrInput(instr.Input)
		markValueAsUsed(input.Value)
	case InstrTypeCast:
		input := AsCastInstrInput(instr.Input)
		markValueAsUsed(input.Value)
	}
}

func (p *UnusedWarningPass) handleDeclPrimordialFunc(instr *Instr) {
	input := AsDeclPrimordialFuncInstrInput(instr.Input)
	p.primordialFunctions = append(p.primordialFunctions, input.Function)
}

// handleVarDecl processes variable declarations and marks initializers as used
func (p *UnusedWarningPass) handleVarDecl(instr *Instr) {
	input := AsDeclVarInstrInput(instr.Input)
	if input.Initializer != nil {
		markValueAsUsed(input.Initializer)
	}
}

// handleStore processes variable assignments and marks values as used
func (p *UnusedWarningPass) handleStore(instr *Instr) {
	input := AsStoreInstrInput(instr.Input)
	markValueAsUsed(input.Value)
}

// handleLoad processes variable usage and marks variables as used
func (p *UnusedWarningPass) handleLoad(instr *Instr) {
	input := AsLoadInstrInput(instr.Input)

	// Mark the variable as used
	input.Addr.IsUsed = true
}

// handleBinaryOp processes binary operations and marks operands as used
func (p *UnusedWarningPass) handleBinaryOp(instr *Instr) {
	input := AsBinaryOpInstrInput(instr.Input)
	markValueAsUsed(input.Left)
	markValueAsUsed(input.Right)
}

// handleUnaryOp processes unary operations and marks the operand as used
func (p *UnusedWarningPass) handleUnaryOp(instr *Instr) {
	input := AsUnaryOpInstrInput(instr.Input)
	markValueAsUsed(input.Value)
}

// handleCallFunc processes function calls and marks arguments as used
func (p *UnusedWarningPass) handleCallFunc(instr *Instr) {
	input := AsCallFuncInstrInput(instr.Input)
	markValueAsUsed(input.Callee)
	markValuesAsUsed(input.Args)
}

// markValuesAsUsed marks all values in a slice as used
func markValuesAsUsed(values []zeus_value.Value) {
	for _, v := range values {
		markValueAsUsed(v)
	}
}

// handleIndirectFuncCall processes indirect function calls and marks the function and arguments as used
func (p *UnusedWarningPass) handleIndirectFuncCall(instr *Instr) {
	input := AsIndirectFuncCallInstrInput(instr.Input)
	markValueAsUsed(input.Function)
	markValuesAsUsed(input.Args)
}

// handleReturn processes return statements and marks the return value as used
func (p *UnusedWarningPass) handleReturn(instr *Instr) {
	input := AsReturnInstrInput(instr.Input)
	markValueAsUsed(input.Value)
}

// handleCondJmp processes conditional jumps and marks the condition as used
func (p *UnusedWarningPass) handleCondJmp(instr *Instr) {
	input := AsCondJmpInstrInput(instr.Input)

	// Mark the condition as used
	if condVar := zeus_value.AsVar(input.Condition); condVar != nil {
		condVar.IsUsed = true
	}
}

// handleNewObj processes new object expressions and marks the class as used
func (p *UnusedWarningPass) handleNewObj(instr *Instr) {
	input := AsNewObjInstrInput(instr.Input)
	if class := zeus_value.AsClass(input.Callee); class != nil {
		class.IsUsed = true
	}
	markValuesAsUsed(input.Args)
}

// handleMethodCall processes method calls and marks the receiver, args, and method as used
func (p *UnusedWarningPass) handleMethodCall(tc *TypeChecker, instr *Instr) {
	input := AsMethodCallInstrInput(instr.Input)

	markValueAsUsed(input.Object)
	markValuesAsUsed(input.Args)

	objectType := tc.getValueType(input.Object)
	if zeus_value.IsObjectType(objectType) {
		class := zeus_value.AsObjectType(objectType).Class
		for _, classMethod := range class.Methods {
			if classMethod.Method.SourceName() == input.MethodName {
				classMethod.Method.IsUsed = true
				return
			}
		}
	}
}

// handleObjectPropertyAccess processes object property accesses and marks the object as used
// Also marks class methods and properties as used when they are accessed
func (p *UnusedWarningPass) handleObjectPropertyAccess(tc *TypeChecker, instr *Instr) {
	input := AsObjectPropertyAccessInstrInput(instr.Input)

	// Mark the object as used
	if objectVar := zeus_value.AsVar(input.Object); objectVar != nil {
		objectVar.IsUsed = true
	}

	// Mark class methods and properties as used when accessed
	objectType := tc.getValueType(input.Object)

	// Check if it's an object type (class instance)
	if zeus_value.IsObjectType(objectType) {
		class := zeus_value.AsObjectType(objectType).Class
		// Look for a method with the matching name
		for _, classMethod := range class.Methods {
			if classMethod.Method.SourceName() == input.Property {
				// Mark the method as used
				classMethod.Method.IsUsed = true
				return
			}
		}

		// Look for a property with the matching name
		for _, classProperty := range class.Properties {
			if classProperty.Property.Name == input.Property {
				// Mark the property as used
				classProperty.Property.IsUsed = true
				return
			}
		}
	}
}

// handleExport processes export statements and marks exported functions as used
func (p *UnusedWarningPass) handleExport(instr *Instr) {
	input := AsExportInstrInput(instr.Input)

	// Mark exported functions as used
	if function := zeus_value.AsFunction(input.Value); function != nil {
		function.IsUsed = true
	}

	// Mark exported classes as used
	if class := zeus_value.AsClass(input.Value); class != nil {
		class.IsUsed = true
	}
}

func (p *UnusedWarningPass) handleGetIndex(instr *Instr) {
	input := AsGetIndexInstrInput(instr.Input)

	// Mark the array as used
	if arrayVar := zeus_value.AsVar(input.Array); arrayVar != nil {
		arrayVar.IsUsed = true
	}

	// Mark all index variables as used
	for _, index := range input.Indices {
		if indexVar := zeus_value.AsVar(index); indexVar != nil {
			indexVar.IsUsed = true
		}
	}
}

func (p *UnusedWarningPass) Finalize(tc *TypeChecker) {
	// Check for unused variables, functions, classes, and class members - push warnings
	tc.builder.symbolTable.Walk(func(name string, value zeus_value.Value) {
		if variable := zeus_value.AsVar(value); variable != nil {
			// Skip temporary variables as they shouldn't generate warnings
			if !variable.IsTempVariable() && !variable.IsUsed {
				displayName := variable.OriginalName
				if displayName == "" {
					displayName = variable.Name
				}
				tc.pushError(&zeus_error.ZeusError{
					Severity: zeus_error.ErrorSeverityWarning,
					Message:  fmt.Sprintf("identifier '%s' is declared but not used", displayName),
					Span:     variable.Span,
				})
			}
		} else if function := zeus_value.AsFunction(value); function != nil {
			// ignore unused primordial functions
			for _, primordialFunction := range p.primordialFunctions {
				if function.Name == primordialFunction.Name {
					return
				}
			}
			isMainFunction := function.Name == token.MAIN_FUNCTION_NAME && tc.IsEntryPoint
			// Check for unused functions, ignore class methods
			if !function.IsUsed && !strings.Contains(function.Name, ".") && !isMainFunction {
				tc.pushError(&zeus_error.ZeusError{
					Severity: zeus_error.ErrorSeverityWarning,
					Message:  fmt.Sprintf("function '%s' is declared but not used", function.Name),
					Span:     function.Span,
				})
			}
		} else if class := zeus_value.AsClass(value); class != nil {
			// Check if the class is unused (no objects created from it)
			if !class.IsUsed && class.PrimordialName == "" {
				tc.pushError(&zeus_error.ZeusError{
					Severity: zeus_error.ErrorSeverityWarning,
					Message:  fmt.Sprintf("class '%s' is declared but not used", class.SourceName()),
					Span:     class.Span,
				})
			} else {
				// Only check class members if the class itself is used
				// Check all methods in this class
				for _, classMethod := range class.Methods {
					method := classMethod.Method

					// Skip constructor methods as they're implicitly used
					if method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
						continue
					}

					// Check if the method is unused
					if !method.IsUsed && class.PrimordialName == "" {
						tc.pushError(&zeus_error.ZeusError{
							Severity: zeus_error.ErrorSeverityWarning,
							Message:  fmt.Sprintf("method '%s' in class '%s' is declared but not used", method.SourceName(), class.SourceName()),
							Span:     method.Span,
						})
					}
				}

				// Check all properties in this class
				for _, classProperty := range class.Properties {
					property := classProperty.Property

					// Check if the property is unused
					if !property.IsUsed && class.PrimordialName == "" {
						tc.pushError(&zeus_error.ZeusError{
							Severity: zeus_error.ErrorSeverityWarning,
							Message:  fmt.Sprintf("property '%s' in class '%s' is declared but not used", property.Name, class.SourceName()),
							Span:     property.Span,
						})
					}
				}
			}
		}
	})
}
