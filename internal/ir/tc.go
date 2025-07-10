package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

type TypeChecker struct {
	errors          []*zeus_error.ZeusError
	currentFunction *zeus_value.Function
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
		initializer := tc.cmpValueWithImplicitCast(instr, decl_var.Variable.ValueType, decl_var.Initializer)
		decl_var.Initializer = initializer
	}
}

func (tc *TypeChecker) getValueType(value zeus_value.Value) zeus_value.ValueType {
	switch value := value.(type) {
	case *zeus_value.Var:
		return value.ValueType
	case *zeus_value.Function:
		return zeus_value.ToFunctionType(*value)
	case *zeus_value.Constant:
		return value.ValueType
	default:
		panic(fmt.Sprintf("cannot get value type of value: %T", value))
	}
}

func (tc *TypeChecker) cmpValueWithImplicitCast(instr *Instr, targetType zeus_value.ValueType, b zeus_value.Value) zeus_value.Value {
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

	if !zeus_value.IsFunctionType(valueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot export value of type '%s'", valueType),
			Span:    instr.Span,
		})
	}
}

func (tc *TypeChecker) tcImport(instr *Instr) {
	input := AsImportInstrInput(instr.Input)
	valueType := tc.getValueType(input.Value)

	if !zeus_value.IsFunctionType(valueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot import value of type '%s'", valueType),
			Span:    instr.Span,
		})
	}
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
		instr.Input = NewCallFuncInstrInput(input.Callee, input.Args)
	}

	instr.Output.ValueType = functionType.ReturnType
}

func (tc *TypeChecker) tcDeclClass(instr *Instr) {
	
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
		case InstrTypeExport:
			tc.tcExport(instr)
		case InstrTypeImport:
			tc.tcImport(instr)
		case InstrTypeDeclClass:
			tc.tcDeclClass(instr)
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
