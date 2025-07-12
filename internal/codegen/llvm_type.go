package codegen

import (
	"fmt"
	"strconv"

	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"tinygo.org/x/go-llvm"
)

func ToLLVMFunctionType(functionType zeus_value.FunctionType) llvm.Type {
	param_llvm_types := []llvm.Type{}

	for _, param := range functionType.ParamTypes {
		param_llvm_types = append(param_llvm_types, ToLLVMType(param))
	}

	return llvm.FunctionType(ToLLVMType(functionType.ReturnType), param_llvm_types, false)
}

func ToLLVMStructType(classType zeus_value.ClassType) llvm.Type {
	properties := []llvm.Type{}
	for _, field := range classType.Class.Properties {
		properties = append(properties, ToLLVMType(field.Property.ValueType))
	}
	return llvm.StructType(properties, false)
}

func ToLLVMIntType(intType zeus_value.IntType) llvm.Type {
	switch intType.Size {
	case zeus_value.I8:
		return llvm.GlobalContext().Int8Type()
	case zeus_value.I16:
		return llvm.GlobalContext().Int16Type()
	case zeus_value.I32:
		return llvm.GlobalContext().Int32Type()
	case zeus_value.I64:
		return llvm.GlobalContext().Int64Type()
	default:
		panic(fmt.Sprintf("cannot convert int type to llvm type: %s", intType))
	}
}

func ToLLVMFloatType(floatType zeus_value.FloatType) llvm.Type {
	switch floatType.Size {
	case zeus_value.F32:
		return llvm.GlobalContext().FloatType()
	case zeus_value.F64:
		return llvm.GlobalContext().DoubleType()
	default:
		panic(fmt.Sprintf("cannot convert float type to llvm type: %s", floatType))
	}
}

func ToLLVMType(_type zeus_value.ValueType) llvm.Type {
	switch _type := _type.(type) {
	case zeus_value.IntType:
		return ToLLVMIntType(_type)
	case zeus_value.FloatType:
		return ToLLVMFloatType(_type)
	case zeus_value.BoolType:
		return llvm.GlobalContext().Int1Type()
	case zeus_value.FunctionType:
		return ToLLVMFunctionType(_type)
	case zeus_value.VoidType:
		return llvm.GlobalContext().VoidType()
	case zeus_value.ClassType:
		return ToLLVMStructType(_type)
	default:
		panic(fmt.Sprintf("cannot convert zeus type to llvm type: %T", _type))
	}
}

func ToLLVMConstant(value zeus_value.Constant) llvm.Value {
	switch value.ValueType.(type) {
	case zeus_value.IntType:
		intType := value.ValueType.(zeus_value.IntType)
		intValue, err := strconv.ParseInt(value.Value, 10, 64)
		zeus_error.Assert(err == nil, fmt.Sprintf("cannot convert int constant string to int: %d", intValue))

		return llvm.ConstInt(ToLLVMIntType(intType), uint64(intValue), intType.Signed)
	case zeus_value.FloatType:
		floatType := value.ValueType.(zeus_value.FloatType)
		floatValue, err := strconv.ParseFloat(value.Value, 64)
		zeus_error.Assert(err == nil, fmt.Sprintf("cannot convert float constant string to float: %f", floatValue))

		return llvm.ConstFloat(ToLLVMFloatType(floatType), floatValue)
	case zeus_value.BoolType:
		if value.Value == "true" {
			return llvm.ConstInt(ToLLVMType(value.ValueType), 1, false)
		} else {
			return llvm.ConstInt(ToLLVMType(value.ValueType), 0, false)
		}
	default:
		panic(fmt.Sprintf("cannot convert constant to llvm constant: %s", value))
	}
}

