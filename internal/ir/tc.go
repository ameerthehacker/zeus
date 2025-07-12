package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

type TypeChecker struct {
	errors          []*zeus_error.ZeusError
	currentFunction *zeus_value.Function
	currentClass *zeus_value.Class
	builder         *IRBuilder
	currentBlock    *BasicBlock
}

func NewTypeChecker(builder *IRBuilder) *TypeChecker {
	return &TypeChecker{
		builder: builder,
	}
}

func (tc *TypeChecker) pushError(err *zeus_error.ZeusError) {
	tc.errors = append(tc.errors, err)
}

func (tc *TypeChecker) tcFuncDecl(instr *Instr) {
	input := AsDeclFuncInstrInput(instr.Input)
	tc.currentFunction = input.Function
	functionBlock := input.Body
	returnsInAllBlocks := true
	worklist := []*BasicBlock{functionBlock}

	for len(worklist) > 0 {
		block := worklist[0]

		if len(block.Instrs) == 0 || !IsControlFlowInstr(block.Instrs[len(block.Instrs) -1 ].Type) {
			// add implicit return
			if zeus_value.IsVoidType(input.Function.ReturnType) {
				tc.builder.SetInsertionBlock(block)
				tc.builder.BuildReturn(nil, nil)
			} else {
				returnsInAllBlocks = false
			}
		}

		worklist = worklist[1:]
		worklist = append(worklist, block.Successors...)
	}

	if !returnsInAllBlocks {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("function '%s' does not return value of type '%s' in all paths", tc.currentFunction.Name, tc.currentFunction.ReturnType),
			Span:    input.Function.Span,
		})
	}
}

func (tc *TypeChecker) tcDeclVar(instr *Instr) {
	decl_var := AsDeclVarInstrInput(instr.Input)

	if zeus_value.IsVoidType(decl_var.Variable.ValueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot declare variable of type '%s'", decl_var.Variable.ValueType),
			Span:    decl_var.Variable.Span,
		})
	} else if decl_var.Initializer != nil {
		initializer := tc.cmpValueWithImplicitCast(instr, tc.asKnownValueType(decl_var.Variable.ValueType), decl_var.Initializer)
		decl_var.Initializer = initializer
	}
}

func (tc *TypeChecker) getBuiltInValueType(value zeus_value.Value) zeus_value.ValueType {
	switch value := value.(type) {
	case *zeus_value.Var:
		return value.ValueType
	case *zeus_value.Function:
		return zeus_value.ToFunctionType(*value)
	case *zeus_value.Constant:
		return value.ValueType
	case *zeus_value.Object:
		return value.ValueType
	case *zeus_value.Class:
		return zeus_value.NewClassType(*value)
	default:
		panic(fmt.Sprintf("cannot get value type of value: %T", value))
	}
}

func (tc *TypeChecker) asKnownValueType(valueType zeus_value.ValueType) zeus_value.ValueType {
	if zeus_value.IsUserDefinedType(valueType) {
		userDefinedType := zeus_value.AsUserDefinedType(valueType)
		variable, ok := tc.builder.symbolTable.GetSymbol(userDefinedType.Name)

		if !ok {
			panic(fmt.Sprintf("symbol %s not found", userDefinedType.Name))
		}

		return tc.getValueType(variable)
	}

	return valueType
}

func (tc *TypeChecker) getValueType(value zeus_value.Value) zeus_value.ValueType {
	valueType := tc.getBuiltInValueType(value)

	return tc.asKnownValueType(valueType)
}

func (tc *TypeChecker) cmpValueWithImplicitCast(instr *Instr, targetType zeus_value.ValueType, b zeus_value.Value) zeus_value.Value {
	bType := tc.getValueType(b)

	if !tc.cmpValueType(targetType, bType) {
		castedB, ok := tc.tryImplicitCast(instr, b, targetType)

		if ok {
			return castedB
		} else {
			_bType := "undefined"
			_targetType := "undefined"
			if bType != nil {
				_bType = bType.String()
			}

			if targetType != nil {
				_targetType = targetType.String()
			}

			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("type '%s' is not assignable to type '%s'", _bType, _targetType),
				Span:    instr.Span,
			})
		}
	}

	return b
}

func (tc *TypeChecker) cmpValueType(a, b zeus_value.ValueType) bool {
	switch a := a.(type) {
	case zeus_value.IntType:
		b, ok := b.(zeus_value.IntType)

		return ok && a.Signed == b.Signed && a.Size == b.Size
	case zeus_value.FloatType:
		bFloat, okFloat := b.(zeus_value.FloatType)
		return okFloat && a.Size == bFloat.Size
	case zeus_value.BoolType:
		_, ok := b.(zeus_value.BoolType)
		if !ok {
			return false
		}
		return true
	case zeus_value.FunctionType:
		b, ok := b.(zeus_value.FunctionType)
		if !ok || len(a.ParamTypes) != len(b.ParamTypes) {
			return false
		}

		isReturnTypeEqual := tc.cmpValueType(a.ReturnType, b.ReturnType)

		if !isReturnTypeEqual {
			return false
		}

		for i := range a.ParamTypes {
			if !tc.cmpValueType(a.ParamTypes[i], b.ParamTypes[i]) {
				return false
			}
		}

		return true
	
	case zeus_value.ObjectType:
	  bClassType, ok := b.(zeus_value.ClassType)
		return ok && a.Class.Name == bClassType.Class.Name
	}

	return false
}

// Performs the following implicit casts:
// - int to float
// - int to int of bigger size
// - float to float of bigger size
func (tc *TypeChecker) tryImplicitCast(instr *Instr, value zeus_value.Value, targetType zeus_value.ValueType) (zeus_value.Value, bool) {
	valueType := tc.getValueType(value)

	// if they both are same type, no need to cast
	if tc.cmpValueType(valueType, targetType) {
		return value, true
	}

	zeus_error.Assert(tc.currentBlock != nil, "current block is nil")
	tc.builder.SetBlockInsertionBefore(tc.currentBlock, instr)

	castIntToFloat := func(intType zeus_value.IntType, value zeus_value.Value) zeus_value.Value {
		size := zeus_value.F32
		if intType.Size == zeus_value.I64 {
			size = zeus_value.F64
		}

		return tc.builder.BuildCast(value, zeus_value.FloatType{Size: size}, value.GetSpan())
	}

	switch valueType := valueType.(type) {
	case zeus_value.IntType:
		switch targetType := targetType.(type) {
		case zeus_value.IntType:
			// value and target are signed and the target is larger
			canFitValue := targetType.Size > valueType.Size && valueType.Signed == targetType.Signed
			// value is unsigned and target is signed and the target is larger
			canFitUnsigned := targetType.Signed && !valueType.Signed && targetType.Size > valueType.Size
			// value is a constant and the target is larger
			constant := zeus_value.AsConstant(value)
			canFitUnsignedConstant := constant != nil && targetType.Size >= zeus_value.GetSignedIntSize(constant.Value)

			if canFitValue || canFitUnsigned || canFitUnsignedConstant {
				return tc.builder.BuildCast(value, targetType, value.GetSpan()), true
			}
		case zeus_value.FloatType:
			return castIntToFloat(valueType, value), true
		}
	case zeus_value.FloatType:
		switch targetType := targetType.(type) {
		case zeus_value.FloatType:
			if targetType.Size > valueType.Size {
				return tc.builder.BuildCast(value, targetType, value.GetSpan()), true
			}
		}
	}

	return value, false
}

// converts left and right to the same type
func (tc *TypeChecker) doImplicitCastToSameType(instr *Instr, left, right zeus_value.Value) (zeus_value.Value, zeus_value.Value) {
	leftValueType := tc.getValueType(left)
	rightValueType := tc.getValueType(right)
	castErrMsg := fmt.Sprintf("cannot do implicit cast to same type: %s and %s", leftValueType, rightValueType)
	ok := false

	switch leftValueType := leftValueType.(type) {
	case zeus_value.IntType:
		switch rightValueType := rightValueType.(type) {
		case zeus_value.IntType:
			if leftValueType.Size > rightValueType.Size {
				right, ok = tc.tryImplicitCast(instr, right, leftValueType)
				if !ok {
					tc.pushError(&zeus_error.ZeusError{
						Message: fmt.Sprintf("cannot cast %s to %s without an explicit cast", rightValueType, leftValueType),
						Span:    instr.Span,
					})
				}
			} else if rightValueType.Size > leftValueType.Size {
				left, ok = tc.tryImplicitCast(instr, left, rightValueType)
				if !ok {
					tc.pushError(&zeus_error.ZeusError{
						Message: fmt.Sprintf("cannot cast %s to %s without an explicit cast", leftValueType, rightValueType),
						Span:    instr.Span,
					})
				}
			}
		case zeus_value.FloatType:
			left, ok = tc.tryImplicitCast(instr, left, rightValueType)
			zeus_error.Assert(ok, "failed to cast int to float")
		default:
			tc.pushError(&zeus_error.ZeusError{
				Message: castErrMsg,
				Span:    instr.Span,
			})
		}
	case zeus_value.FloatType:
		switch rightValueType := rightValueType.(type) {
		case zeus_value.FloatType:
			if leftValueType.Size > rightValueType.Size {
				right, ok = tc.tryImplicitCast(instr, right, leftValueType)
				zeus_error.Assert(ok, "failed to cast smaller float to larger float")
			} else if rightValueType.Size > leftValueType.Size {
				left, ok = tc.tryImplicitCast(instr, left, rightValueType)
				zeus_error.Assert(ok, "failed to cast smaller float to larger float")
			}
		case zeus_value.IntType:
			right, ok = tc.tryImplicitCast(instr, right, leftValueType)
			zeus_error.Assert(ok, "failed to cast int to float")
		default:
			panic(castErrMsg)
		}
	}

	return left, right
}

func (tc *TypeChecker) tcBinaryOp(instr *Instr, resultTypeFn func(a, b zeus_value.ValueType) zeus_value.ValueType, cmpTypeFn func(a, b zeus_value.ValueType) bool) {
	input := AsBinaryOpInstrInput(instr.Input)

	if !cmpTypeFn(tc.getValueType(input.Left), tc.getValueType(input.Right)) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("invalid operands of type '%s' and '%s' for binary operation", tc.getValueType(input.Left), tc.getValueType(input.Right)),
			Span:    instr.Span,
		})
	} else {
		left, right := tc.doImplicitCastToSameType(instr, input.Left, input.Right)
		input.Left = left
		input.Right = right
	}

	instr.Output.ValueType = resultTypeFn(tc.getValueType(input.Left), tc.getValueType(input.Right))
}

func (tc *TypeChecker) tcUnaryOp(instr *Instr, resultTypeFn func(a zeus_value.ValueType) zeus_value.ValueType, cmpTypeFn func(a zeus_value.ValueType) bool) {
	input := AsUnaryOpInstrInput(instr.Input)

	if !cmpTypeFn(tc.getValueType(input.Value)) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("invalid operand of type '%s' for unary operation", tc.getValueType(input.Value)),
			Span:    instr.Span,
		})
	}

	instr.Output.ValueType = resultTypeFn(tc.getValueType(input.Value))
}

func (tc *TypeChecker) tcCondJmp(instr *Instr) {
	input := AsCondJmpInstrInput(instr.Input)

	if !zeus_value.IsBoolType(tc.getValueType(input.Condition)) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("condition must be of type bool, but found %s", tc.getValueType(input.Condition)),
			Span:    instr.Span,
		})
	}
}

func (tc *TypeChecker) tcLoad(instr *Instr) {
	input := AsLoadInstrInput(instr.Input)
	instr.Output.ValueType = input.Addr.ValueType
}

func (tc *TypeChecker) tcStore(instr *Instr) {
	input := AsStoreInstrInput(instr.Input)

	input.Value = tc.cmpValueWithImplicitCast(instr, input.Addr.ValueType, input.Value)
}

func (tc *TypeChecker) tcReturn(instr *Instr) {
	input := AsReturnInstrInput(instr.Input)

	if tc.currentFunction == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "return statement outside of function",
			Span:    instr.Span,
		})

		return
	}

	if zeus_value.IsVoidType(tc.currentFunction.ReturnType) && input.Value != nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "cannot return a value from void function",
			Span:    instr.Span,
		})
	} else if !zeus_value.IsVoidType(tc.currentFunction.ReturnType) && input.Value == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("return value of type '%s' is expected", tc.getValueType(input.Value)),
			Span:    instr.Span,
		})
	} else if zeus_value.IsVoidType(tc.currentFunction.ReturnType) && input.Value == nil {
		return
	} else {
		input.Value = tc.cmpValueWithImplicitCast(instr, tc.currentFunction.ReturnType, input.Value)
	}
}

func (tc *TypeChecker) tcExport(instr *Instr) {
	input := AsExportInstrInput(instr.Input)
	valueType := tc.getValueType(input.Value)

	if !zeus_value.IsFunctionType(valueType) && !zeus_value.IsClassType(valueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot export value of type '%s'", valueType),
			Span:    instr.Span,
		})
	}
}

func (tc *TypeChecker) tcImport(instr *Instr) {
	input := AsImportInstrInput(instr.Input)
	valueType := tc.getValueType(input.Value)

	if !zeus_value.IsFunctionType(valueType) && !zeus_value.IsClassType(valueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot import value of type '%s'", valueType),
			Span:    instr.Span,
		})
	}
}

// tcFunctionCall performs common type checking logic for function calls
// It validates arguments and performs implicit casting based on function signature
func (tc *TypeChecker) tcFunctionCall(instr *Instr, functionType zeus_value.FunctionType, args []zeus_value.Value, calleeSpan *token.Span) []zeus_value.Value {
	if len(args) != len(functionType.ParamTypes) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("expected %d arguments for function, but found %d", len(functionType.ParamTypes), len(args)),
			Span:    calleeSpan,
		})
		return args
	}

	// Perform implicit casting on arguments
	for i := range args {
		castedArg, ok := tc.tryImplicitCast(instr, args[i], functionType.ParamTypes[i])
		args[i] = castedArg
		if !ok {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("argument %d of type '%s' does not match expected type '%s'", i+1, tc.getValueType(args[i]), functionType.ParamTypes[i]),
				Span:    args[i].GetSpan(),
			})
		}
	}

	return args
}

func (tc *TypeChecker) tcCallFunc(instr *Instr) {
	input := AsCallFuncInstrInput(instr.Input)
	function := zeus_value.AsFunction(input.Callee)

	if function == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "expression is not callable",
			Span:    input.Callee.GetSpan(),
		})

		return
	}

	functionType := zeus_value.ToFunctionType(*function)

	// Use the abstracted function call type checking
	input.Args = tc.tcFunctionCall(instr, functionType, input.Args, input.Callee.GetSpan())
	instr.Input = NewCallFuncInstrInput(input.Callee, input.Args)

	instr.Output.ValueType = functionType.ReturnType
}

func (tc *TypeChecker) tcDeclClass(instr *Instr) {}

func (tc *TypeChecker) tcNewObj(instr *Instr) {
	input := AsNewObjInstrInput(instr.Input)
	calleeType := tc.getValueType(input.Callee)

	if !zeus_value.IsClassType(calleeType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot create object of type '%s'", calleeType),
			Span:    input.Callee.GetSpan(),
		})
	}

	classType := zeus_value.AsClassType(calleeType)
	class := classType.Class

	var constructorMethod *zeus_value.Function = nil

	for _, method := range class.Methods {
		if method.Method.Name == token.CONSTRUCTOR_METHOD_NAME {
			constructorMethod = method.Method
			break
		}
	}
	

	if constructorMethod == nil && len(input.Args) > 0 {
		tc.pushError(&zeus_error.ZeusError{
			Message: "no constructor method found in class",
			Span:    instr.Output.Span,
		})
	}

	if constructorMethod != nil {
		if len(input.Args) != len(constructorMethod.Params) {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("expected %d arguments for constructor, but found %d", len(constructorMethod.Params), len(input.Args)),
				Span:    instr.Output.Span,
			})
		} else {
			for i := range input.Args {
				input.Args[i] = tc.cmpValueWithImplicitCast(instr, constructorMethod.Params[i].ValueType, input.Args[i])
			}
		}
	}

	instr.Output.ValueType = zeus_value.NewObjectType(class)
}

func (tc *TypeChecker) tcObjectPropertyAccess(instr *Instr) {
	input := AsObjectPropertyAccessInstrInput(instr.Input)
	output := instr.Output
	objectType := tc.getValueType(input.Object)

	if !zeus_value.IsClassType(objectType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot access property %s of type '%s'", input.Property, objectType),
			Span:   output.Span,
		})
	} else {
		class := zeus_value.AsClassType(objectType).Class
		properties := class.Properties
		methods := class.Methods
		isFound := false
		isAccessible := false

		for _, property := range properties {
			if property.Property.Name == input.Property {
				isFound = true
				if property.AccessModifier != nil {
					isAccessible = property.AccessModifier.Type == token.TokenTypePublic
				}
				instr.Output.ValueType = property.Property.ValueType
			}
		}

		for _, method := range methods {
			if method.Method.Name == input.Property {
				isFound = true
				if method.AccessModifier != nil {
					isAccessible = method.AccessModifier.Type == token.TokenTypePublic
				}
				instr.Output.ValueType = zeus_value.ToFunctionType(*method.Method)
			}
		}

		if !isFound {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("property %s not found in class %s", input.Property, class.Name),
				Span:    output.Span,
			})
		}

		propertyOfSameClass := tc.currentClass != nil && tc.currentClass.Name == class.Name
		if !isAccessible && !propertyOfSameClass {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("property %s is not accessible in class %s", input.Property, class.Name),
				Span:    output.Span,
			})
		}
	}
}

func (tc *TypeChecker) tcIndirectFuncCall(instr *Instr) {
	input := AsIndirectFuncCallInstrInput(instr.Input)

	methodType := tc.getValueType(input.Method)
	functionType := zeus_value.AsFunctionType(methodType)

	if functionType == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "expression is not callable",
			Span:    instr.Output.Span,
		})
		return
	}

	input.Args = tc.tcFunctionCall(instr, *functionType, input.Args, input.Method.GetSpan())
	instr.Input = NewIndirectFuncCallInstrInput(input.Method, input.Args)

	instr.Output.ValueType = functionType.ReturnType
}

func (tc *TypeChecker) tcDeclClassMethod(instr *Instr) {
	input := AsDeclClassMethodInstrInput(instr.Input)
	tc.currentClass = input.Class
	tc.currentFunction = input.Method
}

func (tc *TypeChecker) TypeCheck() []*zeus_error.ZeusError {
	tc.builder.Walk(func(instr *Instr) {
		switch instr.Type {
		// jmp requires no type checking
		case InstrTypeJmp:
		case InstrTypeDeclFunc:
			tc.tcFuncDecl(instr)
		case InstrTypeDeclVar:
			tc.tcDeclVar(instr)
		case InstrTypeAdd:
			fallthrough
		case InstrTypeSub:
			fallthrough
		case InstrTypeMul:
			fallthrough
		case InstrTypeDiv:
			tc.tcBinaryOp(instr, zeus_value.GetBiggerType, func(a, b zeus_value.ValueType) bool {
				return zeus_value.IsNumberType(a) && zeus_value.IsNumberType(b)
			})
		case InstrTypeEqEq:
			fallthrough
		case InstrTypeNotEq:
			fallthrough
		case InstrTypeLessThan:
			fallthrough
		case InstrTypeGreaterThan:
			fallthrough
		case InstrTypeLessThanEq:
			fallthrough
		case InstrTypeGreaterThanEq:
			tc.tcBinaryOp(instr, func(_, _ zeus_value.ValueType) zeus_value.ValueType {
				return zeus_value.BoolType{}
			}, func(a, b zeus_value.ValueType) bool {
				return zeus_value.IsNumberType(a) && zeus_value.IsNumberType(b)
			})
		case InstrTypeNot:
			tc.tcUnaryOp(instr, func(_ zeus_value.ValueType) zeus_value.ValueType {
				return zeus_value.BoolType{}
			}, zeus_value.IsBoolType)
		case InstrTypeNeg:
			tc.tcUnaryOp(instr, func(operandType zeus_value.ValueType) zeus_value.ValueType {
				switch operandType := operandType.(type) {
				// negation on int makes it signed
				case zeus_value.IntType:
					return zeus_value.IntType{
						Signed: true,
						Size:   operandType.Size,
					}
				}
				return operandType
			}, zeus_value.IsNumberType)
		case InstrTypeCondJmp:
			tc.tcCondJmp(instr)
		case InstrTypeLoad:
			tc.tcLoad(instr)
		case InstrTypeStore:
			tc.tcStore(instr)
		case InstrTypeReturn:
			tc.tcReturn(instr)
		case InstrTypeCallFunc:
			tc.tcCallFunc(instr)
		case InstrTypeIndirectFuncCall:
			tc.tcIndirectFuncCall(instr)
		case InstrTypeExport:
			tc.tcExport(instr)
		case InstrTypeImport:
			tc.tcImport(instr)
		case InstrTypeDeclClass:
			tc.tcDeclClass(instr)
		case InstrTypeNewObj:
			tc.tcNewObj(instr)
		case InstrTypeObjectPropertyAccess:
			tc.tcObjectPropertyAccess(instr)
		case InstrTypeDeclClassMethod:
			tc.tcDeclClassMethod(instr)
		case InstrTypeCast:
			// TODO: add type checking for cast
		default:
			panic(fmt.Sprintf("type checking not handled for instruction: %s", instr.Type))
		}
	}, func(block *BasicBlock) {
		tc.currentBlock = block
	})

	return tc.errors
}
