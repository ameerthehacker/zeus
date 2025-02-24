package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/value"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

type TypeChecker struct {
	errors          []*zeus_error.ZeusError
	currentFunction *value.Function
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
			if value.IsVoidType(input.Function.ReturnType) {
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

	if value.IsVoidType(decl_var.Variable.ValueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot declare variable of type '%s'", decl_var.Variable.ValueType),
			Span:    decl_var.Variable.Span,
		})
	} else if decl_var.Initializer != nil {
		initializer := tc.cmpValueWithImplicitCast(instr, decl_var.Variable.ValueType, decl_var.Initializer)
		decl_var.Initializer = initializer
	}
}

func (tc *TypeChecker) getValueType(_value value.Value) value.ValueType {
	switch _value := _value.(type) {
	case *value.Var:
		return _value.ValueType
	case *value.Function:
		return value.ToFunctionType(*_value)
	case *value.Constant:
		return _value.ValueType
	default:
		panic(fmt.Sprintf("cannot get value type of value: %T", _value))
	}
}

func (tc *TypeChecker) cmpValueWithImplicitCast(instr *Instr, targetType value.ValueType, b value.Value) value.Value {
	bType := tc.getValueType(b)

	if !tc.cmpValueType(targetType, bType) {
		castedB, ok := tc.tryImplicitCast(instr, b, targetType)

		if ok {
			return castedB
		} else {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("type '%s' is not assignable to type '%s'", bType, targetType),
				Span:    instr.Span,
			})
		}
	}

	return b
}

func (tc *TypeChecker) cmpValueType(a, b value.ValueType) bool {
	switch a := a.(type) {
	case value.IntType:
		b, ok := b.(value.IntType)

		return ok && a.Signed == b.Signed && a.Size == b.Size
	case value.FloatType:
		bFloat, okFloat := b.(value.FloatType)
		return okFloat && a.Size == bFloat.Size
	case value.BoolType:
		_, ok := b.(value.BoolType)
		if !ok {
			return false
		}
		return true
	case value.FunctionType:
		b, ok := b.(value.FunctionType)
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
	}

	return false
}

// Performs the following implicit casts:
// - int to float
// - int to int of bigger size
// - float to float of bigger size
func (tc *TypeChecker) tryImplicitCast(instr *Instr, _value value.Value, targetType value.ValueType) (value.Value, bool) {
	valueType := tc.getValueType(_value)

	zeus_error.Assert(tc.currentBlock != nil, "current block is nil")
	tc.builder.SetBlockInsertionBefore(tc.currentBlock, instr)

	castIntToFloat := func(intType value.IntType, _value value.Value) value.Value {
		size := value.F32
		if intType.Size == value.I64 {
			size = value.F64
		}

		return tc.builder.BuildCast(_value, value.FloatType{Size: size}, _value.GetSpan())
	}

	switch valueType := valueType.(type) {
	case value.IntType:
		switch targetType := targetType.(type) {
		case value.IntType:
			// value and target are signed and the target is larger
			canFitValue := targetType.Size > valueType.Size && valueType.Signed == targetType.Signed
			// value is unsigned and target is signed and the target is larger
			canFitUnsigned := targetType.Signed && !valueType.Signed && targetType.Size > valueType.Size
			// value is a constant and the target is larger
			constant := value.AsConstant(_value)
			canFitUnsignedConstant := constant != nil && targetType.Size >= value.GetSignedIntSize(constant.Value)

			if canFitValue || canFitUnsigned || canFitUnsignedConstant {
				return tc.builder.BuildCast(_value, targetType, _value.GetSpan()), true
			}
		case value.FloatType:
			return castIntToFloat(valueType, _value), true
		}
	case value.FloatType:
		switch targetType := targetType.(type) {
		case value.FloatType:
			if targetType.Size > valueType.Size {
				return tc.builder.BuildCast(_value, targetType, _value.GetSpan()), true
			}
		}
	}

	return _value, false
}

// converts left and right to the same type
func (tc *TypeChecker) doImplicitCastToSameType(instr *Instr, left, right value.Value) (value.Value, value.Value) {
	leftValueType := tc.getValueType(left)
	rightValueType := tc.getValueType(right)
	castErrMsg := fmt.Sprintf("cannot do implicit cast to same type: %s and %s", leftValueType, rightValueType)
	ok := false

	switch leftValueType := leftValueType.(type) {
	case value.IntType:
		switch rightValueType := rightValueType.(type) {
		case value.IntType:
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
		case value.FloatType:
			left, ok = tc.tryImplicitCast(instr, left, rightValueType)
			zeus_error.Assert(ok, "failed to cast int to float")
		default:
			tc.pushError(&zeus_error.ZeusError{
				Message: castErrMsg,
				Span:    instr.Span,
			})
		}
	case value.FloatType:
		switch rightValueType := rightValueType.(type) {
		case value.FloatType:
			if leftValueType.Size > rightValueType.Size {
				right, ok = tc.tryImplicitCast(instr, right, leftValueType)
				zeus_error.Assert(ok, "failed to cast smaller float to larger float")
			} else if rightValueType.Size > leftValueType.Size {
				left, ok = tc.tryImplicitCast(instr, left, rightValueType)
				zeus_error.Assert(ok, "failed to cast smaller float to larger float")
			}
		case value.IntType:
			right, ok = tc.tryImplicitCast(instr, right, leftValueType)
			zeus_error.Assert(ok, "failed to cast int to float")
		default:
			panic(castErrMsg)
		}
	}

	return left, right
}

func (tc *TypeChecker) tcBinaryOp(instr *Instr, resultTypeFn func(a, b value.ValueType) value.ValueType, cmpTypeFn func(a, b value.ValueType) bool) {
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

func (tc *TypeChecker) tcUnaryOp(instr *Instr, resultTypeFn func(a value.ValueType) value.ValueType, cmpTypeFn func(a value.ValueType) bool) {
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

	if !value.IsBoolType(tc.getValueType(input.Condition)) {
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

	if input.Addr.IsTempVariable() {
		tc.pushError(&zeus_error.ZeusError{
			Message: "invalid assignment",
			Span:    input.Addr.Span,
		})
	} else {
		input.Value = tc.cmpValueWithImplicitCast(instr, input.Addr.ValueType, input.Value)
	}
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

	if value.IsVoidType(tc.currentFunction.ReturnType) && input.Value != nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "cannot return a value from void function",
			Span:    instr.Span,
		})
	} else if !value.IsVoidType(tc.currentFunction.ReturnType) && input.Value == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("return value of type '%s' is expected", tc.getValueType(input.Value)),
			Span:    instr.Span,
		})
	} else if value.IsVoidType(tc.currentFunction.ReturnType) && input.Value == nil {
		return
	} else {
		input.Value = tc.cmpValueWithImplicitCast(instr, tc.currentFunction.ReturnType, input.Value)
	}
}

func (tc *TypeChecker) tcCallFunc(instr *Instr) {
	input := AsCallFuncInstrInput(instr.Input)
	function := value.AsFunction(input.Callee)

	if function == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "expression is not callable",
			Span:    input.Callee.GetSpan(),
		})

		return
	}

	functionType := value.ToFunctionType(*function)

	if len(input.Args) != len(functionType.ParamTypes) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("expected %d arguments for function '%s', but found %d", len(functionType.ParamTypes), function.Name, len(input.Args)),
			Span:    input.Callee.GetSpan(),
		})
	} else {
		for i := range input.Args {
			castedArg, ok := tc.tryImplicitCast(instr, input.Args[i], functionType.ParamTypes[i])
			input.Args[i] = castedArg
			if !ok {
				tc.pushError(&zeus_error.ZeusError{
					Message: fmt.Sprintf("argument %d of type '%s' does not match expected type '%s'", i+1, tc.getValueType(input.Args[i]), functionType.ParamTypes[i]),
					Span:    input.Args[i].GetSpan(),
				})
			}
		}
		instr.Input = CallFuncInstrInput{
			Callee: input.Callee,
			Args:   input.Args,
		}
	}

	instr.Output.ValueType = functionType.ReturnType
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
			tc.tcBinaryOp(instr, value.GetBiggerType, func(a, b value.ValueType) bool {
				return value.IsNumberType(a) && value.IsNumberType(b)
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
			tc.tcBinaryOp(instr, func(_, _ value.ValueType) value.ValueType {
				return value.BoolType{}
			}, func(a, b value.ValueType) bool {
				return value.IsNumberType(a) && value.IsNumberType(b)
			})
		case InstrTypeNot:
			tc.tcUnaryOp(instr, func(_ value.ValueType) value.ValueType {
				return value.BoolType{}
			}, value.IsBoolType)
		case InstrTypeNeg:
			tc.tcUnaryOp(instr, func(operandType value.ValueType) value.ValueType {
				switch operandType := operandType.(type) {
				// negation on int makes it signed
				case value.IntType:
					return value.IntType{
						Signed: true,
						Size:   operandType.Size,
					}
				}
				return operandType
			}, value.IsNumberType)
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
		default:
			panic(fmt.Sprintf("type checking not handled for instruction: %s", instr.Type))
		}
	}, func(block *BasicBlock) {
		tc.currentBlock = block
	})

	return tc.errors
}
