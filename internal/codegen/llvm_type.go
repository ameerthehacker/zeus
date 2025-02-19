package codegen

import (
	"fmt"
	"strconv"

	"github.com/ameerthehacker/zeus/internal/value"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"tinygo.org/x/go-llvm"
)

func ToLLVMFunctionType(functionType value.FunctionType) llvm.Type {
	param_llvm_types := []llvm.Type{}

	for _, param := range functionType.ParamTypes {
		param_llvm_types = append(param_llvm_types, ToLLVMType(param))
	}

	return llvm.FunctionType(ToLLVMType(functionType.ReturnType), param_llvm_types, false)
}

func ToLLVMIntType(intType value.IntType) llvm.Type {
	switch intType.Size {
	case value.I8:
		return llvm.GlobalContext().Int8Type()
	case value.I16:
		return llvm.GlobalContext().Int16Type()
	case value.I32:
		return llvm.GlobalContext().Int32Type()
	case value.I64:
		return llvm.GlobalContext().Int64Type()
	default:
		panic(fmt.Sprintf("cannot convert int type to llvm type: %s", intType))
	}
}

func ToLLVMFloatType(floatType value.FloatType) llvm.Type {
	switch floatType.Size {
	case value.F32:
		return llvm.GlobalContext().FloatType()
	case value.F64:
		return llvm.GlobalContext().DoubleType()
	default:
		panic(fmt.Sprintf("cannot convert float type to llvm type: %s", floatType))
	}
}

func ToLLVMType(_type value.ValueType) llvm.Type {
	switch _type.(type) {
	case value.IntType:
		return ToLLVMIntType(_type.(value.IntType))
	case value.FloatType:
		return ToLLVMFloatType(_type.(value.FloatType))
	case value.BoolType:
		return llvm.GlobalContext().Int1Type()
	case value.FunctionType:
		return ToLLVMFunctionType(_type.(value.FunctionType))
	default:
		panic(fmt.Sprintf("cannot convert zeus type to llvm type: %s", _type))
	}
}

func ToLLVMConstant(_value value.Constant) llvm.Value {
	switch _value.ValueType.(type) {
	case value.IntType:
		intType := _value.ValueType.(value.IntType)
		intValue, err := strconv.ParseInt(_value.Value, 10, 64)
		zeus_error.Assert(err == nil, fmt.Sprintf("cannot convert int constant string to int: %d", intValue))

		return llvm.ConstInt(ToLLVMIntType(intType), uint64(intValue), intType.Signed)
	case value.FloatType:
		floatType := _value.ValueType.(value.FloatType)
		floatValue, err := strconv.ParseFloat(_value.Value, 64)
		zeus_error.Assert(err == nil, fmt.Sprintf("cannot convert float constant string to float: %f", floatValue))

		return llvm.ConstFloat(ToLLVMFloatType(floatType), floatValue)
	case value.BoolType:
		if _value.Value == "true" {
			return llvm.ConstInt(ToLLVMType(_value.ValueType), 1, false)
		} else {
			return llvm.ConstInt(ToLLVMType(_value.ValueType), 0, false)
		}
	default:
		panic(fmt.Sprintf("cannot convert constant to llvm constant: %s", _value))
	}
}

