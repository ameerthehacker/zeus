package codegen

import (
	"fmt"
	"strconv"

	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"tinygo.org/x/go-llvm"
)

func (c *CodegenModule) toLLVMIntType(intType zeus_value.IntType) llvm.Type {
	switch intType.Size {
	case zeus_value.I8:
		return c.ctx.Int8Type()
	case zeus_value.I16:
		return c.ctx.Int16Type()
	case zeus_value.I32:
		return c.ctx.Int32Type()
	case zeus_value.I64:
		return c.ctx.Int64Type()
	default:
		panic(fmt.Sprintf("cannot convert int type to llvm type: %T", intType))
	}
}

func (c *CodegenModule) toLLVMFloatType(floatType zeus_value.FloatType) llvm.Type {
	switch floatType.Size {
	case zeus_value.F32:
		return c.ctx.FloatType()
	case zeus_value.F64:
		return c.ctx.DoubleType()
	default:
		panic(fmt.Sprintf("cannot convert float type to llvm type: %T", floatType))
	}
}

func (c *CodegenModule) toLLVMBuiltInType(_type zeus_value.ValueType) llvm.Type {
	switch _type := _type.(type) {
	case zeus_value.IntType:
		return c.toLLVMIntType(_type)
	case zeus_value.FloatType:
		return c.toLLVMFloatType(_type)
	case zeus_value.BoolType:
		return c.ctx.Int1Type()
	case zeus_value.VoidType:
		return c.ctx.VoidType()
	case zeus_value.OpaqueType:
		return llvm.PointerType(c.ctx.VoidType(), 1) 
	default:
		panic(fmt.Sprintf("cannot convert zeus type to llvm type: %T", _type))
	}
}

func (c *CodegenModule) toLLVMConstant(value zeus_value.Constant) llvm.Value {
	switch value.ValueType.(type) {
	case zeus_value.IntType:
		intType := value.ValueType.(zeus_value.IntType)
		intValue, err := strconv.ParseInt(value.Value, 10, 64)
		zeus_error.Assert(err == nil, fmt.Sprintf("cannot convert int constant string to int: %d", intValue))

		return llvm.ConstInt(c.toLLVMIntType(intType), uint64(intValue), intType.Signed)
	case zeus_value.FloatType:
		floatType := value.ValueType.(zeus_value.FloatType)
		floatValue, err := strconv.ParseFloat(value.Value, 64)
		zeus_error.Assert(err == nil, fmt.Sprintf("cannot convert float constant string to float: %f", floatValue))

		return llvm.ConstFloat(c.toLLVMFloatType(floatType), floatValue)
	case zeus_value.BoolType:
		if value.Value == "true" {
			return llvm.ConstInt(c.toLLVMBuiltInType(value.ValueType), 1, false)
		} else {
			return llvm.ConstInt(c.toLLVMBuiltInType(value.ValueType), 0, false)
		}
	case zeus_value.NullType:
		return llvm.ConstNull(llvm.PointerType(c.ctx.VoidType(), 0))
	default:
		panic(fmt.Sprintf("cannot convert constant to llvm constant: %T", value))
	}
}
