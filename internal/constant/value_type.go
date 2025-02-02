package constant

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/token"
)

type ValueType interface {}

type IntSize int

const (
	I8 IntSize = iota
	I16
	I32
	I64
	I128
)

type IntType struct {
	Signed bool
	Size IntSize
}

func (i IntType) String() string {
	prefix := "i"
	if !i.Signed {
		prefix = "u" + prefix
	}
	return fmt.Sprintf("%s%d", prefix, i.Size)
}

type FloatSize int

const (
	F32 FloatSize = iota
	F64
)

type FloatType struct {
	Size FloatSize
}

func (f FloatType) String() string {
	return fmt.Sprintf("f%d", f.Size)
}

type BoolType struct {}

func (b BoolType) String() string {
	return "bool"
}

type FunctionType struct {
	ReturnType ValueType
	ParamTypes []ValueType
}

func ToValueType(t *token.Token) ValueType {
	switch t.Type {
	case token.TokenTypeInt8:
		return IntType{Signed: true, Size: 8}
	case token.TokenTypeInt16:
		return IntType{Signed: true, Size: 16}
	case token.TokenTypeInt32:
		return IntType{Signed: true, Size: 32}
	case token.TokenTypeInt64:
		return IntType{Signed: true, Size: 64}
	case token.TokenTypeUInt8:
		return IntType{Signed: false, Size: 8}
	case token.TokenTypeUInt16:
		return IntType{Signed: false, Size: 16}
	case token.TokenTypeUInt32:
		return IntType{Signed: false, Size: 32}
	case token.TokenTypeUInt64:
		return IntType{Signed: false, Size: 64}
	case token.TokenTypeFloat32:
		return FloatType{Size: 32}
	case token.TokenTypeFloat64:
		return FloatType{Size: 64}
	case token.TokenTypeBoolean:
		return BoolType{}
	default:
		panic(fmt.Sprintf("unknown data type token: %s", t.Type))
	}
}
