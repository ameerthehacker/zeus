package value

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
)

const TEMP_VARIABLE_PREFIX = "%"

type Value interface {
	String() string
	GetSpan() *token.Span
}

type Constant struct {
	Value string
	ValueType ValueType
	Span *token.Span
}

func (i Constant) GetSpan() *token.Span {
	return i.Span
}	

func (i Constant) String() string {
	return fmt.Sprintf("%s %s", i.ValueType, i.Value)
}

type Var struct {
	Name string
	ValueType ValueType
	Span *token.Span
}

func (v Var) GetSpan() *token.Span {
	return v.Span
}

func (v Var) String() string {
	if v.ValueType != nil {
		return fmt.Sprintf("%s %s", v.ValueType, v.Name)
	}
	return v.Name
}

func (v Var) IsTempVariable() bool {
	return strings.HasPrefix(v.Name, TEMP_VARIABLE_PREFIX)
}

type Function struct {
	Name string
	Params []*Var
	ReturnType ValueType
	Span *token.Span
}

func (f Function) GetSpan() *token.Span {
	return f.Span
}

func (f Function) String() string {
	params := []string{}
	for _, param := range f.Params {
		params = append(params, param.String())
	}

	return fmt.Sprintf("%s(%s) %s", f.Name, strings.Join(params, ", "), f.ReturnType)
}

func GetValueType(value Value) ValueType {
	switch value := value.(type) {
	case *Var:
		return value.ValueType
	case *Constant:
		return value.ValueType
	case *Function:
		param_types := []ValueType{}
		for _, param := range value.Params {
			param_types = append(param_types, param.ValueType)
		}

		return FunctionType{
			ReturnType: value.ReturnType,
			ParamTypes: param_types,
		}
	default:
		panic(fmt.Sprintf("unable to identify type for value: %T", value))
	}
}

func AsVar(value Value) *Var {
	switch value := value.(type) {
	case *Var:
		return value
	default:
		return nil
	}
}

func AsFunction(value Value) *Function {
	switch value := value.(type) {
	case *Function:
		return value
	default:
		return nil
	}
}

func AsConstant(value Value) *Constant {
	switch value := value.(type) {
	case *Constant:
		return value
	default:
		return nil
	}
}

func GetIntSize(number string) IntSize {
	value, err := strconv.ParseInt(number, 10, 64)

	if err != nil {
		panic(fmt.Sprintf("failed to parse int: %s", err))
	}

	switch {
	case value >= -128 && value <= 127:
		return I8
	case value >= -32768 && value <= 32767:
		return I16
	case value >= -2147483648 && value <= 2147483647:
		return I32
	case value >= -9223372036854775808 && value <= 9223372036854775807:
		return I64
	default:
		return I128
	}
}

func IsFloat(number string) bool {
	return strings.Contains(number, ".")
}
