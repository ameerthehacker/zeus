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
	case I128:
		return "128"
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
