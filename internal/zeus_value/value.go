package zeus_value

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

const TEMP_VARIABLE_PREFIX = "%"
// first 100 ids are reserved for primordial classes
var classIdCounter = 100

type Value interface {
	String() string
	GetSpan() *token.Span
}

type Constant struct {
	Value     string
	ValueType ValueType
	Span      *token.Span
}

func NewConstant(value string, valueType ValueType, span *token.Span) *Constant {
	return &Constant{
		Value:     value,
		ValueType: valueType,
		Span:      span,
	}
}

func (i Constant) GetSpan() *token.Span {
	return i.Span
}

func (i Constant) String() string {
	return fmt.Sprintf("%s %s", i.ValueType, i.Value)
}

type Object struct {
	ValueType ObjectType
	Name      string
	Span      *token.Span
}

func NewObject(name string, objectType ObjectType, span *token.Span) Object {
	return Object{
		Name:      name,
		ValueType: objectType,
		Span:      span,
	}
}

func (o Object) GetSpan() *token.Span {
	return o.Span
}

func (o Object) String() string {
	return fmt.Sprintf("%s %s", o.ValueType, o.Name)
}

type Var struct {
	Name      string
	ValueType ValueType
	Cxt       *Value
	IsPtr     bool
	Span      *token.Span
	IsUsed    bool
}

func NewVar(name string, valueType ValueType, isPtr bool, span *token.Span) *Var {
	return &Var{
		Name:      name,
		ValueType: valueType,
		IsPtr:     isPtr,
		Span:      span,
		IsUsed:    false,
	}
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
	Name       string
	Params     []*Var
	ReturnType ValueType
	IsUsed     bool
	Span       *token.Span
}

func NewFunction(name string, params []*Var, returnType ValueType, span *token.Span) *Function {
	return &Function{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		IsUsed:     false,
		Span:       span,
	}
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

type ClassProperty struct {
	Property       *Var
	AccessModifier *token.Token
}

func NewClassProperty(property *Var, accessModifier *token.Token) *ClassProperty {
	return &ClassProperty{
		Property:       property,
		AccessModifier: accessModifier,
	}
}

func (p *ClassProperty) String() string {
	return fmt.Sprintf("%s %s", p.AccessModifier, p.Property)
}

type ClassMethod struct {
	Method         *Function
	AccessModifier *token.Token
}

func NewClassMethod(method *Function, accessModifier *token.Token) *ClassMethod {
	return &ClassMethod{
		Method:         method,
		AccessModifier: accessModifier,
	}
}

func (m *ClassMethod) String() string {
	return fmt.Sprintf("%s %s", m.AccessModifier, m.Method)
}

type Class struct {
	Id         int
	Name       string
	Properties []*ClassProperty
	Methods    []*ClassMethod
	IsUsed     bool
	PrimordialName string
	ArrayElementType ValueType
	Span       *token.Span
}

func NewClass(name string, properties []*ClassProperty, methods []*ClassMethod, primordialName string, arrayElementType ValueType, span *token.Span) *Class {
	classIdCounter += 1
	return &Class{
		Id:         classIdCounter,
		Name:       name,
		Properties: properties,
		Methods:    methods,
		IsUsed:     false,
		PrimordialName: primordialName,
		ArrayElementType: arrayElementType,
		Span:       span,
	}
}

func (c Class) GetSpan() *token.Span {
	return c.Span
}

func (c Class) String() string {
	return c.Name
}

func IsClass(value Value) bool {
	switch value.(type) {
	case *Class:
		return true
	default:
		return false
	}
}

func GetValueType(value Value) ValueType {
	switch value := value.(type) {
	case *Var:
		return value.ValueType
	case *Constant:
		return value.ValueType
	case *Object:
		return value.ValueType
	case *Class:
		return NewClassType(*value)
	case *Function:
		param_types := []ValueType{}
		for _, param := range value.Params {
			param_types = append(param_types, param.ValueType)
		}

		return FunctionType{
			ReturnType: value.ReturnType,
			ParamTypes: param_types,
		}
	case *ArrayElementRef:
		// For array element refs, get the type of the array object and extract element type
		arrayType := GetValueType(value.ArrayObject)
		if objType, ok := arrayType.(ObjectType); ok {
			if arrayClassType, ok := objType.Class.ArrayElementType.(ArrayType); ok {
				return arrayClassType.ElementType
			}
			return objType.Class.ArrayElementType
		}
		panic(fmt.Sprintf("ArrayElementRef has non-array object type: %T", arrayType))
	default:
		panic(fmt.Sprintf("unable to identify type for value: %T", value))
	}
}

func AsClass(value Value) *Class {
	switch value := value.(type) {
	case *Class:
		return value
	default:
		return nil
	}
}

func AsObject(value Value) *Object {
	switch value := value.(type) {
	case *Object:
		return value
	default:
		return nil
	}
}

func IsVar(value Value) bool {
	switch value.(type) {
	case *Var:
		return true
	default:
		return false
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

func IsFunction(value Value) bool {
	switch value.(type) {
	case *Function:
		return true
	default:
		return false
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

func GetSignedIntSize(number string) IntSize {
	value, err := strconv.ParseInt(number, 10, 64)

	zeus_error.Assert(err == nil, fmt.Sprintf("failed to parse int: %s", err))

	switch {
	case value >= -128 && value <= 127:
		return I8
	case value >= -32768 && value <= 32767:
		return I16
	case value >= -2147483648 && value <= 2147483647:
		return I32
	default:
		return I64
	}
}

func IsFloat(number string) bool {
	return strings.Contains(number, ".")
}

// ArrayElementRef represents a reference to an array element for assignment
// It stores the array object (after navigating all but the last index) and the last index
// This is used when handling array[0][1] = expr to generate temp1.set(lastIndex, expr)
type ArrayElementRef struct {
	ArrayObject Value      // The array object (could be the result of array.get(0) for multi-dimensional)
	Index       Value      // The last index
	Span        *token.Span
}

func NewArrayElementRef(arrayObject Value, index Value, span *token.Span) *ArrayElementRef {
	return &ArrayElementRef{
		ArrayObject: arrayObject,
		Index:       index,
		Span:        span,
	}
}

func (a ArrayElementRef) GetSpan() *token.Span {
	return a.Span
}

func (a ArrayElementRef) String() string {
	return fmt.Sprintf("ArrayElementRef(%s[%s])", a.ArrayObject, a.Index)
}

func AsArrayElementRef(value Value) *ArrayElementRef {
	switch value := value.(type) {
	case *ArrayElementRef:
		return value
	default:
		return nil
	}
}
