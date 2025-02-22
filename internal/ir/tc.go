package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/value"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

type TypeChecker struct {
	errors []*zeus_error.ZeusError
	currentFunction *value.Function
	builder *IRBuilder
	currentBlock *BasicBlock
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
	func_decl := AsDeclFuncInstrInput(instr.Input)
	tc.currentFunction = &func_decl.Function
}

func (tc *TypeChecker) tcDeclVar(instr *Instr) {
	decl_var := AsDeclVarInstrInput(instr.Input)

	if value.IsVoidType(decl_var.Variable.ValueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot declare variable of type '%s'", decl_var.Variable.ValueType),
			Span: decl_var.Variable.Span,
		})
	} else if decl_var.Initializer != nil {
		if !tc.cmpValueType(decl_var.Variable.ValueType, tc.getValueType(decl_var.Initializer)) {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("type '%s' is not assignable to type '%s'", decl_var.Variable.ValueType, tc.getValueType(decl_var.Initializer)),
				Span: decl_var.Variable.Span,
			})
		}
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

func (tc *TypeChecker) cmpValueType(a, b value.ValueType) bool {
	switch a := a.(type) {
	case value.IntType:
		b, ok := b.(value.IntType)
		
		if !ok {
			return false
		}

		if a.Signed && !b.Signed {
			return a.Size > b.Size
		}

		return a.Size >= b.Size && a.Signed == b.Signed
	case value.FloatType:
		bFloat, okFloat := b.(value.FloatType)
		_, okInt := b.(value.IntType)

		if okFloat {
			return a.Size >= bFloat.Size
		} else if okInt {
			return true
		}

		return false
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

func (tc *TypeChecker) tcBinaryOp(instr *Instr, resultTypeFn func(a, b value.ValueType) value.ValueType ,cmpTypeFn func(a, b value.ValueType) bool) {
	input := AsBinaryOpInstrInput(instr.Input)

	if !cmpTypeFn(tc.getValueType(input.Left), tc.getValueType(input.Right)) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("invalid operands of type '%s' and '%s' for binary operation", tc.getValueType(input.Left), tc.getValueType(input.Right)),
			Span: instr.Span,
		})
	} else {
		// if any of the operand is float then we cast both to float
		if (value.IsFloatType(tc.getValueType(input.Left)) || value.IsFloatType(tc.getValueType(input.Right))) {
			zeus_error.Assert(tc.currentBlock != nil, "current block is nil")
			tc.builder.SetBlockInsertionBefore(tc.currentBlock, instr)
			instr.Input = BinaryOpInstrInput{
				Left: tc.castToFloat(input.Left),
				Right: tc.castToFloat(input.Right),
			}
		}
	}
	
	instr.Output.ValueType = resultTypeFn(tc.getValueType(input.Left), tc.getValueType(input.Right))
}

func (tc *TypeChecker) tcUnaryOp(instr *Instr, resultTypeFn func(a value.ValueType) value.ValueType, cmpTypeFn func(a value.ValueType) bool) {
	input := AsUnaryOpInstrInput(instr.Input)

	if !cmpTypeFn(tc.getValueType(input.Value)) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("invalid operand of type '%s' for unary operation", tc.getValueType(input.Value)),
			Span: instr.Span,
		})
	} 
	
	instr.Output.ValueType = resultTypeFn(tc.getValueType(input.Value))
}

func (tc *TypeChecker) tcCondJmp(instr *Instr) {
	input := AsCondJmpInstrInput(instr.Input)

	if !value.IsBoolType(tc.getValueType(input.Condition)) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("condition must be of type bool, but found %s", tc.getValueType(input.Condition)),
			Span: instr.Span,
		})
	}
}

func (tc *TypeChecker) tcLoad(instr *Instr) {
	input := AsLoadInstrInput(instr.Input)
	instr.Output.ValueType = input.Addr.ValueType
}

func (tc *TypeChecker) tcStore(instr *Instr) {
	input := AsStoreInstrInput(instr.Input)
	valueType := tc.getValueType(input.Value)

	if input.Addr.IsTempVariable() {
		tc.pushError(&zeus_error.ZeusError{
			Message: "invalid assignment",
			Span: input.Addr.Span,
		})
	} else if !tc.cmpValueType(input.Addr.ValueType, valueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("type '%s' is not assignable to type '%s'", valueType, input.Addr.ValueType),
			Span: instr.Span,
		})
	}
}

func (tc *TypeChecker) tcReturn(instr *Instr) {
	input := AsReturnInstrInput(instr.Input)

	if tc.currentFunction == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "return statement outside of function",
			Span: instr.Span,
		})

		return
	}

	if (value.IsVoidType(tc.currentFunction.ReturnType) && input.Value != nil) {
		tc.pushError(&zeus_error.ZeusError{
			Message: "cannot return a value from void function",
			Span: instr.Span,
		})
	} else if (!value.IsVoidType(tc.currentFunction.ReturnType) && input.Value == nil) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("return value of type '%s' is expected", tc.getValueType(input.Value)),
			Span: instr.Span,
		})
	} else if (value.IsVoidType(tc.currentFunction.ReturnType) && input.Value == nil) {
		return;
	} else if (!tc.cmpValueType(tc.currentFunction.ReturnType, tc.getValueType(input.Value))) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("return type '%s' does not match function return type '%s'", tc.getValueType(input.Value), tc.currentFunction.ReturnType),
			Span: instr.Span,
		})
	}
}

func (tc *TypeChecker) tcCallFunc(instr *Instr) {
	input := AsCallFuncInstrInput(instr.Input)
	function := value.AsFunction(input.Callee)

	if function == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "expression is not callable",
			Span: input.Callee.GetSpan(),
		})

		return;
	}

	functionType := value.ToFunctionType(*function)

	if len(input.Args) != len(functionType.ParamTypes) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("expected %d arguments for function '%s', but found %d", len(functionType.ParamTypes), function.Name, len(input.Args)),
			Span: input.Callee.GetSpan(),
		})
	} else {
		for i := range input.Args {
			if !tc.cmpValueType(functionType.ParamTypes[i], tc.getValueType(input.Args[i])) {
				tc.pushError(&zeus_error.ZeusError{
					Message: fmt.Sprintf("argument %d of type '%s' does not match expected type '%s'", i + 1, tc.getValueType(input.Args[i]), functionType.ParamTypes[i]),
					Span: input.Args[i].GetSpan(),
				})
			}
		}
	}

	instr.Output.ValueType = functionType.ReturnType
}

func (tc *TypeChecker) castToFloat(_value value.Value) value.Value {
	valueType := value.GetValueType(_value)

	switch valueType := valueType.(type) {
	case value.IntType:
		size := value.F32
		if valueType.Size == value.I64 {
			size = value.F64
		}
		zeus_error.Assert(tc.currentBlock != nil, "current block is nil")
		return tc.builder.BuildCast(_value, value.FloatType{Size: size}, _value.GetSpan())
	case value.FloatType:
		return _value
	default:
		panic(fmt.Sprintf("cannot cast value of type %s to float", valueType))
	}
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
						Size: operandType.Size,
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
