package ir

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/module"
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
	case InstrTypeDeclVar, InstrTypeDeclGlobalVar:
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
	case InstrTypeSuperConstructorCall:
		input := AsSuperConstructorCallInstrInput(instr.Input)
		markValueAsUsed(input.ThisObject)
		markValuesAsUsed(input.Args)
	case InstrTypeDeclPrimordialFunc:
		p.handleDeclPrimordialFunc(instr)
	case InstrTypeExport:
		p.handleExport(instr)
	case InstrTypeGetAccessor:
		input := AsGetAccessorInstrInput(instr.Input)
		markValueAsUsed(input.Object)
	case InstrTypeSetAccessor:
		input := AsSetAccessorInstrInput(instr.Input)
		markValueAsUsed(input.Object)
		markValueAsUsed(input.Value)
	case InstrTypeGetIndex:
		p.handleGetIndex(instr)
	case InstrTypeSetIndex:
		p.handleSetIndex(instr)
	case InstrTypeCoerce:
		input := AsCoerceInstrInput(instr.Input)
		markValueAsUsed(input.Value)
	case InstrTypeCast:
		input := AsCastInstrInput(instr.Input)
		markValueAsUsed(input.Value)
	case InstrTypeBox:
		// The autoboxed scalar is used (it becomes the box's field value).
		markValueAsUsed(AsBoxInstrInput(instr.Input).Value)
	case InstrTypeUnbox:
		// The unboxed box object is used (its `value` field is read out).
		markValueAsUsed(AsUnboxInstrInput(instr.Input).Value)
	case InstrTypeStringTemplate:
		// Each interpolated part value is used (concatenated into the result string).
		for _, part := range AsStringTemplateInstrInput(instr.Input).Parts {
			if part.IsExpr {
				markValueAsUsed(part.Value)
			}
		}
	case InstrTypeInstanceOf:
		input := AsInstanceOfInstrInput(instr.Input)
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

	// Resolve on the base class for super.method(), otherwise the receiver's class; walk the chain
	// (LookupMethod) so an inherited or super-invoked base method is correctly marked used.
	class := input.StaticClass
	if class == nil {
		if objectType := tc.getValueType(input.Object); zeus_value.IsObjectType(objectType) {
			class = zeus_value.AsObjectType(objectType).Class
		}
	}
	if class != nil {
		if m := zeus_value.LookupMethod(class, input.MethodName); m != nil {
			m.Method.IsUsed = true
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

	// Mark exported classes and all their members as used (importers can call any public member).
	if class := zeus_value.AsClass(input.Value); class != nil {
		class.IsUsed = true
		for _, m := range class.Methods {
			m.Method.IsUsed = true
		}
		for _, p := range class.Properties {
			p.Property.IsUsed = true
		}
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

func (p *UnusedWarningPass) handleSetIndex(instr *Instr) {
	input := AsSetIndexInstrInput(instr.Input)

	if arrayVar := zeus_value.AsVar(input.Array); arrayVar != nil {
		arrayVar.IsUsed = true
	}
	if indexVar := zeus_value.AsVar(input.Index); indexVar != nil {
		indexVar.IsUsed = true
	}
	if valueVar := zeus_value.AsVar(input.Value); valueVar != nil {
		valueVar.IsUsed = true
	}
}

func (p *UnusedWarningPass) Finalize(tc *TypeChecker) {
	// Collect in-scope interfaces first. A method/property of a class that structurally conforms to
	// an interface can be invoked through dynamic interface dispatch (possibly from another module,
	// which this per-module pass never sees), so such members must not be reported as unused.
	var interfaces []*zeus_value.Interface
	tc.builder.symbolTable.Walk(func(_ string, value zeus_value.Value) {
		if iface := zeus_value.AsInterface(value); iface != nil {
			interfaces = append(interfaces, iface)
		}
	})

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
			isEntryFunction := (function.Name == token.MAIN_FUNCTION_NAME && tc.IsEntryPoint) || function.IsOSEntry
			// The synthetic per-module init function is invoked via CALL_MODULE_INIT (from the entry
			// point's `main`), which the usage tracker doesn't follow, so it always looks unused.
			isModuleInitFn := strings.HasPrefix(function.Name, module.ModuleInitFuncPrefix)
			// Skip static methods and static accessors — they are emitted as InstrTypeDeclFunc but
			// tracked via class.Methods/Accessors in the Finalize loop below; checking them here
			// would produce spurious "declared but not used" warnings for used statics.
			isStaticClassFn := function.Class != nil
			// Check for unused functions, ignore instance class methods and static class functions
			if !function.IsUsed && !strings.Contains(function.Name, ".") && !isEntryFunction && !isModuleInitFn && !isStaticClassFn {
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

					// Check if the method is unused (interface-dispatchable methods count as used).
					if !method.IsUsed && class.PrimordialName == "" &&
						!reachableViaInterface(class, method.SourceName(), true, interfaces) {
						tc.pushError(&zeus_error.ZeusError{
							Severity: zeus_error.ErrorSeverityWarning,
							Message:  fmt.Sprintf("method '%s' in class '%s' is declared but not used", method.SourceName(), class.SourceName()),
							Span:     method.Span,
						})
					}
				}

				// Check all properties in this class
				for _, classProperty := range class.Properties {
					if classProperty.IsStatic {
						// Static property usage is tracked via the backing global var
						if classProperty.StaticGlobalVar != nil && !classProperty.StaticGlobalVar.IsUsed && class.PrimordialName == "" {
							tc.pushError(&zeus_error.ZeusError{
								Severity: zeus_error.ErrorSeverityWarning,
								Message:  fmt.Sprintf("static property '%s' in class '%s' is declared but not used", classProperty.Property.Name, class.SourceName()),
								Span:     classProperty.Property.Span,
							})
						}
						continue
					}
					property := classProperty.Property
					if !property.IsUsed && class.PrimordialName == "" &&
						!reachableViaInterface(class, property.Name, false, interfaces) {
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

// reachableViaInterface reports whether a member named `memberName` on `class` satisfies a member of
// some in-scope interface the class structurally conforms to. Such a member can be invoked through
// dynamic interface dispatch, so it must not be flagged as unused. `isMethod` selects the interface
// member kind (method vs property). The cheap name lookup runs before the structural conformance
// check so the (more expensive) conformance walk only happens for a matching member name.
func reachableViaInterface(class *zeus_value.Class, memberName string, isMethod bool, interfaces []*zeus_value.Interface) bool {
	for _, iface := range interfaces {
		var declares bool
		if isMethod {
			declares = zeus_value.InterfaceMethodIndex(iface, memberName) != -1
		} else {
			declares = zeus_value.InterfacePropertyIndex(iface, memberName) != -1
		}
		if declares && zeus_value.ClassConformsToInterface(class, iface) {
			return true
		}
	}
	return false
}
