package value

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
)

type ValueType interface {
	String() string
}

type IntSize int

const (
	I8 IntSize = iota
	I16
	I32
	I64
)

func (i IntSize) String() string {
	switch i {
	case I8:
		return "8"
	case I16:
		return "16"
	case I32:
		return "32"
	case I64:
		return "64"
	default:
		panic(fmt.Sprintf("unknown int size: %d", i))
	}
}

type IntType struct {
	Signed bool
	Size IntSize
}

func (i IntType) String() string {
	prefix := "i"
	if !i.Signed {
		prefix = "u" + prefix
	}
	return fmt.Sprintf("%s%s", prefix, i.Size)
}

type FloatSize int

const (
	F32 FloatSize = iota
	F64
)

func (f FloatSize) String() string {
	switch f {
	case F32:
		return "32"
	case F64:
		return "64"
	default:
		panic(fmt.Sprintf("unknown float size: %d", f))
	}
}

type FloatType struct {
	Size FloatSize
}

func (f FloatType) String() string {
	return fmt.Sprintf("f%s", f.Size)
}

type BoolType struct {}

func (b BoolType) String() string {
	return "bool"
}

type FunctionType struct {
	ReturnType ValueType
	ParamTypes []ValueType
}

func (f FunctionType) String() string {
	param_types := []string{}
	for _, param := range f.ParamTypes {
		param_types = append(param_types, param.String())
	}
	return fmt.Sprintf("(%s) => %s", strings.Join(param_types, ", "), f.ReturnType)
}

func ToValueType(t *token.Token) ValueType {
	switch t.Type {
	case token.TokenTypeInt8:
		return IntType{Signed: true, Size: I8}
	case token.TokenTypeInt16:
		return IntType{Signed: true, Size: I16}
	case token.TokenTypeInt32:
		return IntType{Signed: true, Size: I32}
	case token.TokenTypeInt64:
		return IntType{Signed: true, Size: I64}
	case token.TokenTypeUInt8:
		return IntType{Signed: false, Size: I8}
	case token.TokenTypeUInt16:
		return IntType{Signed: false, Size: I16}
	case token.TokenTypeUInt32:
		return IntType{Signed: false, Size: I32}
	case token.TokenTypeUInt64:
		return IntType{Signed: false, Size: I64}
	case token.TokenTypeFloat32:
		return FloatType{Size: F32}
	case token.TokenTypeFloat64:
		return FloatType{Size: F64}
	case token.TokenTypeBoolean:
		return BoolType{}
	default:
		panic(fmt.Sprintf("unknown data type token: %s", t.Type))
	}
}

func AsFunctionType(value ValueType) *FunctionType {
	switch value := value.(type) {
	case *FunctionType:
		return value
	default:
		return nil
	}
}

func ToFunctionType(value Function) *FunctionType {
	param_types := []ValueType{}
	for _, param := range value.Params {
		param_types = append(param_types, param.ValueType)
	}

	return &FunctionType{
		ReturnType: value.ReturnType,
		ParamTypes: param_types,
	}
}

func IsNumberType(value ValueType) bool {
	switch value.(type) {
	case IntType, FloatType:
		return true
	default:
		return false
	}
}

func IsBoolType(value ValueType) bool {
	switch value.(type) {
	case BoolType:
		return true
	default:
		return false
	}
}

// currently supports only int and float
func GetBiggerType(a, b ValueType) ValueType {
	switch a := a.(type) {
	case IntType:
		switch b := b.(type) {
		case IntType:
			if a.Size >= b.Size {
				return a
			}
			return b
		case FloatType:
			return b
		default:
			return a
		}
	case FloatType:
		switch b := b.(type) {
		case IntType:
			return a
		case FloatType:
			if a.Size >= b.Size {
				return a
			}
			return b
		default:
			return a
		}
	default:
		return b
	}
}
