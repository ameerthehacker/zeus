package zeus_value

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
)

type ValueType interface {
	String() string
	GetSpan() *token.Span
}

type NullType struct {
	Span *token.Span
}

func (n NullType) GetSpan() *token.Span {
	return n.Span
}

func (n NullType) String() string {
	return "null"
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
	Size   IntSize
	Span   *token.Span
}

func (i IntType) GetSpan() *token.Span {
	return i.Span
}

func (i IntType) String() string {
	prefix := "i"
	if !i.Signed {
		prefix = "u"
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
	Span *token.Span
}

func (f FloatType) GetSpan() *token.Span {
	return f.Span
}

type UserDefinedType struct {
	Name string
	Span *token.Span
}

func (u UserDefinedType) GetSpan() *token.Span {
	return u.Span
}

func (u UserDefinedType) String() string {
	return u.Name
}

func (f FloatType) String() string {
	return fmt.Sprintf("f%s", f.Size)
}

type BoolType struct {
	Span *token.Span
}

func (b BoolType) GetSpan() *token.Span {
	return b.Span
}

func (b BoolType) String() string {
	return "bool"
}

type VoidType struct {
	Span *token.Span
}

func (v VoidType) GetSpan() *token.Span {
	return v.Span
}

func (v VoidType) String() string {
	return "void"
}

// opaque type is internal to the compiler and not exposed to the user
type OpaqueType struct {
	Span *token.Span
}

func (o OpaqueType) GetSpan() *token.Span {
	return o.Span
}

func (o OpaqueType) String() string {
	return "opaque"
}

func NewOpaqueType(span *token.Span) OpaqueType {
	return OpaqueType{Span: span}
}

type FunctionType struct {
	ReturnType ValueType
	ParamTypes []ValueType
	Span       *token.Span
}

func (f FunctionType) GetSpan() *token.Span {
	return f.Span
}

func NewFunctionType(returnType ValueType, paramTypes []ValueType) FunctionType {
	return FunctionType{
		ReturnType: returnType,
		ParamTypes: paramTypes,
	}
}

type UndefinedType struct {
	Span *token.Span
}

func (u UndefinedType) GetSpan() *token.Span {
	return u.Span
}

func (u UndefinedType) String() string {
	return "undefined"
}

func (f FunctionType) String() string {
	param_types := []string{}
	for _, param := range f.ParamTypes {
		param_types = append(param_types, param.String())
	}
	return fmt.Sprintf("(%s) => %s", strings.Join(param_types, ", "), f.ReturnType)
}

type ObjectType struct {
	Class Class
}

func (o ObjectType) GetSpan() *token.Span {
	return o.Class.Span
}

func (o ObjectType) String() string {
	return o.Class.Name
}

func NewObjectType(class Class) ObjectType {
	return ObjectType{Class: class}
}

func AsObjectType(value ValueType) *ObjectType {
	switch value := value.(type) {
	case ObjectType:
		return &value
	default:
		return nil
	}
}

func IsObjectType(value ValueType) bool {
	return AsObjectType(value) != nil
}

func IsOpaqueType(value ValueType) bool {
	switch value.(type) {
	case OpaqueType:
		return true
	default:
		return false
	}
}

func IsArrayType(value ValueType) bool {
	switch value.(type) {
	case ArrayType:
		return true
	default:
		return false
	}
}

func AsArrayType(value ValueType) *ArrayType {
	switch value := value.(type) {
	case ArrayType:
		return &value
	default:
		return nil
	}
}

type ClassType struct {
	Class Class
}

func (c ClassType) GetSpan() *token.Span {
	return c.Class.Span
}

func (c ClassType) String() string {
	return c.Class.Name
}

func NewClassType(class Class) ClassType {
	return ClassType{Class: class}
}

func AsClassType(value ValueType) *ClassType {
	switch value := value.(type) {
	case ClassType:
		return &value
	default:
		return nil
	}
}

func IsClassType(value ValueType) bool {
	return AsClassType(value) != nil
}

type ArrayType struct {
	ElementType ValueType
	Span        *token.Span
}

func NewArrayType(elementType ValueType, span *token.Span) ArrayType {
	return ArrayType{ElementType: elementType, Span: span}
}

func (a ArrayType) GetSpan() *token.Span {
	return a.Span
}

func (a ArrayType) String() string {
	arraySuffix := "[]"
	if IsObjectType(a.ElementType) || IsUserDefinedType(a.ElementType) {
		return fmt.Sprintf("%s%s", a.ElementType.String(), arraySuffix)
	}
	return fmt.Sprintf("%s%s", a.ElementType.String(), arraySuffix)
}

func ToValueType(t *token.Token) ValueType {
	switch t.Type {
	case token.TokenTypeInt8:
		return IntType{Signed: true, Size: I8, Span: t.Span}
	case token.TokenTypeInt16:
		return IntType{Signed: true, Size: I16, Span: t.Span}
	case token.TokenTypeInt32:
		return IntType{Signed: true, Size: I32, Span: t.Span}
	case token.TokenTypeInt64:
		return IntType{Signed: true, Size: I64, Span: t.Span}
	case token.TokenTypeUInt8:
		return IntType{Signed: false, Size: I8, Span: t.Span}
	case token.TokenTypeUInt16:
		return IntType{Signed: false, Size: I16, Span: t.Span}
	case token.TokenTypeUInt32:
		return IntType{Signed: false, Size: I32, Span: t.Span}
	case token.TokenTypeUInt64:
		return IntType{Signed: false, Size: I64, Span: t.Span}
	case token.TokenTypeFloat32:
		return FloatType{Size: F32, Span: t.Span}
	case token.TokenTypeFloat64:
		return FloatType{Size: F64, Span: t.Span}
	case token.TokenTypeBoolean:
		return BoolType{Span: t.Span}
	case token.TokenTypeVoid:
		return VoidType{Span: t.Span}
	case token.TokenTypeIdentifier:
		return UserDefinedType{Name: t.Value, Span: t.Span}
	case token.TokenTypeNull:
		return NullType{Span: t.Span}
	default:
		panic(fmt.Sprintf("unknown data type token: %s", t.Type))
	}
}

func AsFunctionType(value ValueType) *FunctionType {
	switch value := value.(type) {
	case FunctionType:
		return &value
	default:
		return nil
	}
}

func IsFunctionType(value ValueType) bool {
	return AsFunctionType(value) != nil
}

func ToFunctionType(value Function) FunctionType {
	param_types := []ValueType{}
	for _, param := range value.Params {
		param_types = append(param_types, param.ValueType)
	}

	return FunctionType{
		ReturnType: value.ReturnType,
		ParamTypes: param_types,
	}
}

func IsUserDefinedType(value ValueType) bool {
	switch value.(type) {
	case UserDefinedType:
		return true
	default:
		return false
	}
}

func IsNullType(value ValueType) bool {
	switch value.(type) {
	case NullType:
		return true
	default:
		return false
	}
}

func AsUserDefinedType(value ValueType) *UserDefinedType {
	switch valueType := value.(type) {
	case UserDefinedType:
		return &valueType
	default:
		return nil
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

func IsVoidType(value ValueType) bool {
	switch value.(type) {
	case VoidType:
		return true
	default:
		return false
	}
}

func IsIntType(value ValueType) bool {
	switch value.(type) {
	case IntType:
		return true
	default:
		return false
	}
}

func IsFloatType(value ValueType) bool {
	switch value.(type) {
	case FloatType:
		return true
	default:
		return false
	}
}

func AsFloatType(value ValueType) *FloatType {
	switch value := value.(type) {
	case FloatType:
		return &value
	default:
		return nil
	}
}

func IsUndefinedType(value ValueType) bool {
	switch value.(type) {
	case UndefinedType:
		return true
	default:
		return false
	}
}

func IsPrimitiveType(value ValueType) bool {
	switch value.(type) {
	case IntType, FloatType, BoolType:
		return true
	default:
		return false
	}
}

// currently supports only int and float
func GetBiggerIntType(a, b IntType) ValueType {
	if a.Size >= b.Size {
		return a
	}
	return b
}

func GetBiggerFloatType(a, b FloatType) ValueType {
	if a.Size >= b.Size {
		return a
	}
	return b
}

func GetBiggerType(a, b ValueType) ValueType {
	switch a := a.(type) {
	case IntType:
		switch b := b.(type) {
		case IntType:
			return GetBiggerIntType(a, b)
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
			return GetBiggerFloatType(a, b)
		default:
			return a
		}
	default:
		return b
	}
}

func CmpValueType(a, b ValueType) bool {
	switch a := a.(type) {
	case IntType:
		b, ok := b.(IntType)

		return ok && a.Signed == b.Signed && a.Size == b.Size
	case FloatType:
		bFloat, okFloat := b.(FloatType)
		return okFloat && a.Size == bFloat.Size
	case BoolType:
		_, ok := b.(BoolType)
		if !ok {
			return false
		}
		return true
	case VoidType:
		_, ok := b.(VoidType)
		return ok
	case FunctionType:
		b, ok := b.(FunctionType)
		if !ok || len(a.ParamTypes) != len(b.ParamTypes) {
			return false
		}

		isReturnTypeEqual := CmpValueType(a.ReturnType, b.ReturnType)

		if !isReturnTypeEqual {
			return false
		}

		for i := range a.ParamTypes {
			if !CmpValueType(a.ParamTypes[i], b.ParamTypes[i]) {
				return false
			}
		}

		return true

	case ObjectType:
		bClassType, ok := b.(ObjectType)
		if !ok {
			return false
		}
		if a.Class.Name == bClassType.Class.Name {
			return true
		}
		// Two different functor classes are compatible when their __call__ signatures match.
		aCall := getFunctorCallMethod(&a.Class)
		bCall := getFunctorCallMethod(&bClassType.Class)
		if aCall == nil || bCall == nil {
			return false
		}
		return CmpValueType(GetValueType(aCall), GetValueType(bCall))
	}

	return false
}

// GetFunctorCallMethod returns the __call__ method of a class if it has one, nil otherwise.
func GetFunctorCallMethod(class *Class) *Function {
	return getFunctorCallMethod(class)
}

// getFunctorCallMethod returns the __call__ method of a class if it has one, nil otherwise.
func getFunctorCallMethod(class *Class) *Function {
	for _, m := range class.Methods {
		if m.Method.OriginalName == token.FUNCTOR_CALL_METHOD_NAME {
			return m.Method
		}
	}
	return nil
}
