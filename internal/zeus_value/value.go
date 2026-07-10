package zeus_value

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

const TEMP_VARIABLE_PREFIX = "%"
const NULL_CONSTANT_VALUE = "null"

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
	Name         string
	OriginalName string
	ValueType    ValueType
	IsPtr        bool
	IsConst      bool
	Span         *token.Span
	IsUsed       bool
	// IsVariadic marks a function's rest parameter (its type is an array; trailing
	// call arguments are collected into it during lowering).
	IsVariadic bool
}

func NewVar(name string, valueType ValueType, isPtr bool, span *token.Span, isVariadic ...bool) *Var {
	variadic := false
	if len(isVariadic) > 0 {
		variadic = isVariadic[0]
	}
	return &Var{
		Name:       name,
		ValueType:  valueType,
		IsPtr:      isPtr,
		Span:       span,
		IsUsed:     false,
		IsVariadic: variadic,
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
	Name         string
	OriginalName string // user-visible name before IR-level uniquification
	Params       []*Var
	ReturnType   ValueType
	IsUsed       bool
	Span         *token.Span
	Class        *Class // non-nil when this function is a class method
	// IsVariadic is true when the final parameter is a rest parameter (`...args: T[]`).
	IsVariadic bool
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

// SourceName returns the user-visible name for the function.
// For user-defined functions/methods, OriginalName is set and carries the source name
// while Name holds the unique IR name. Primordials leave OriginalName empty, so Name is used.
func (f Function) SourceName() string {
	if f.OriginalName != "" {
		return f.OriginalName
	}
	return f.Name
}

func (f Function) String() string {
	params := []string{}
	for _, param := range f.Params {
		params = append(params, param.String())
	}

	return fmt.Sprintf("%s(%s) %s", f.Name, strings.Join(params, ", "), f.ReturnType)
}

type AccessorKind int

const (
	AccessorKindNone   AccessorKind = iota
	AccessorKindGetter
	AccessorKindSetter
)

// ClassAccessor holds the getter and/or setter function for a named accessor property.
type ClassAccessor struct {
	Name           string
	Getter         *Function    // nil if setter-only
	Setter         *Function    // nil if getter-only
	AccessModifier *token.Token
	// IsLowered is true for primordial accessors whose bodies are expanded directly
	// by the lowering pass (no Zig runtime function needed, e.g. arr.length).
	IsLowered bool
	IsStatic  bool
}

func NewClassAccessor(name string, getter *Function, setter *Function, accessModifier *token.Token) *ClassAccessor {
	return &ClassAccessor{
		Name:           name,
		Getter:         getter,
		Setter:         setter,
		AccessModifier: accessModifier,
	}
}

func (a *ClassAccessor) String() string {
	parts := []string{}
	if a.Getter != nil {
		parts = append(parts, fmt.Sprintf("get %s(): %s", a.Name, a.Getter.ReturnType))
	}
	if a.Setter != nil && len(a.Setter.Params) > 0 {
		parts = append(parts, fmt.Sprintf("set %s(%s)", a.Name, a.Setter.Params[0].ValueType))
	}
	return strings.Join(parts, ", ")
}

type ClassProperty struct {
	Property        *Var
	AccessModifier  *token.Token
	IsReadonly      bool
	IsStatic        bool
	StaticGlobalVar *Var // non-nil for static props; the backing LLVM global
}

func NewClassProperty(property *Var, accessModifier *token.Token, isReadonly bool, isStatic bool, staticGlobalVar *Var) *ClassProperty {
	return &ClassProperty{
		Property:        property,
		AccessModifier:  accessModifier,
		IsReadonly:      isReadonly,
		IsStatic:        isStatic,
		StaticGlobalVar: staticGlobalVar,
	}
}

func (p *ClassProperty) String() string {
	readonly := ""
	if p.IsReadonly {
		readonly = "readonly "
	}
	return fmt.Sprintf("%s %s%s", p.AccessModifier, readonly, p.Property)
}

type ClassMethod struct {
	Method         *Function
	AccessModifier *token.Token
	// IsLowered indicates that this method is handled entirely by IR lowering
	// and doesn't need a runtime wrapper function generated
	IsLowered bool
	IsStatic  bool
	// IsAccessor marks a synthesized getter/setter method (mangled name #get_/#set_).
	// It occupies a vtable slot like any instance method, but is not user-callable by
	// name and is excluded from "unused method" diagnostics.
	IsAccessor bool
}

func NewClassMethod(method *Function, accessModifier *token.Token) *ClassMethod {
	return &ClassMethod{
		Method:         method,
		AccessModifier: accessModifier,
		IsLowered:      false,
	}
}

// NewLoweredClassMethod creates a method that is handled by IR lowering
// and doesn't need a runtime wrapper function
func NewLoweredClassMethod(method *Function, accessModifier *token.Token) *ClassMethod {
	return &ClassMethod{
		Method:         method,
		AccessModifier: accessModifier,
		IsLowered:      true,
	}
}

func (m *ClassMethod) String() string {
	return fmt.Sprintf("%s %s", m.AccessModifier, m.Method)
}

type Class struct {
	Id               int
	Name             string
	OriginalName     string // user-visible name before IR-level uniquification
	ParentClass      *Class // Parent class for inheritance (nil if no parent)
	Properties       []*ClassProperty
	Methods          []*ClassMethod
	Accessors        []*ClassAccessor
	IsUsed           bool
	PrimordialName   string
	ArrayElementType ValueType
	Span             *token.Span
}

// SourceName returns the user-visible class name. For user-defined classes,
// OriginalName carries the source name while Name holds the unique IR name.
// Primordials leave OriginalName empty, so Name is used.
func (c Class) SourceName() string {
	if c.OriginalName != "" {
		return c.OriginalName
	}
	return c.Name
}

func NewClass(name string, properties []*ClassProperty, methods []*ClassMethod, accessors []*ClassAccessor, primordialName string, arrayElementType ValueType, span *token.Span) *Class {
	classIdCounter += 1
	return &Class{
		Id:               classIdCounter,
		Name:             name,
		ParentClass:      nil,
		Properties:       properties,
		Methods:          methods,
		Accessors:        accessors,
		IsUsed:           false,
		PrimordialName:   primordialName,
		ArrayElementType: arrayElementType,
		Span:             span,
	}
}

// NewClassWithParent creates a new class that inherits from a parent class
func NewClassWithParent(name string, parentClass *Class, properties []*ClassProperty, methods []*ClassMethod, accessors []*ClassAccessor, primordialName string, arrayElementType ValueType, span *token.Span) *Class {
	classIdCounter += 1
	return &Class{
		Id:               classIdCounter,
		Name:             name,
		ParentClass:      parentClass,
		Properties:       properties,
		Methods:          methods,
		Accessors:        accessors,
		IsUsed:           false,
		PrimordialName:   primordialName,
		ArrayElementType: arrayElementType,
		Span:             span,
	}
}

// NewClassWithId creates a class with a specific ID (for reserved primordial classes like Error)
func NewClassWithId(id int, name string, properties []*ClassProperty, methods []*ClassMethod, primordialName string, span *token.Span) *Class {
	return &Class{
		Id:               id,
		Name:             name,
		ParentClass:      nil,
		Properties:       properties,
		Methods:          methods,
		Accessors:        nil,
		IsUsed:           false,
		PrimordialName:   primordialName,
		ArrayElementType: nil,
		Span:             span,
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
		return NewClassType(value)
	case *Function:
		return ToFunctionType(*value)
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

// LookupInstanceProperty finds a non-static data field by name on the class or any ancestor.
// Returns nil if there is no such field (e.g. the name belongs to a method or accessor).
func LookupInstanceProperty(class *Class, name string) *ClassProperty {
	for c := class; c != nil; c = c.ParentClass {
		for _, prop := range c.Properties {
			if !prop.IsStatic && prop.Property.Name == name {
				return prop
			}
		}
	}
	return nil
}

func AsObject(value Value) *Object {
	switch value := value.(type) {
	case *Object:
		return value
	default:
		return nil
	}
}

func IsObject(value Value) bool {
	switch value.(type) {
	case *Object:
		return true
	default:
		return false
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
	value, err := strconv.ParseInt(number, 0, 64)

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

// RefCellVar represents a variable that has been promoted to a GC-managed heap cell
// for capture-by-reference in closures. The symbol table stores a RefCellVar instead
// of a plain *Var; VisitIdentifier emits OBJECT_PROPERTY_ACCESS cell.value + LOAD
// transparently so the rest of the compiler sees a normal scalar.
type RefCellVar struct {
	OriginalName string
	ValueType    ValueType // type of .value (not of the cell object itself)
	Cell         Value     // *Var holding the GC ref cell object (IsPtr=false)
	Span         *token.Span
}

func (r *RefCellVar) GetSpan() *token.Span { return r.Span }
func (r *RefCellVar) String() string {
	return fmt.Sprintf("RefCellVar(%s: %s)", r.OriginalName, r.ValueType)
}

func AsRefCellVar(value Value) *RefCellVar {
	switch v := value.(type) {
	case *RefCellVar:
		return v
	default:
		return nil
	}
}

// ArrayElementRef represents a reference to an array element for assignment
// It stores the array object (after navigating all but the last index) and the last index
// This is used when handling array[0][1] = expr to generate temp1.set(lastIndex, expr)
type ArrayElementRef struct {
	ArrayObject Value // The array object (could be the result of array.get(0) for multi-dimensional)
	Index       Value // The last index
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

// AccessorLValue is a transient sentinel returned by VisitObjectPropertyAccessExpr
// when isLValueExpr=true and the property is backed by a setter accessor.
// It is never emitted as an IR instruction; VisitBinaryExpr/VisitUnaryExpr consume it.
type AccessorLValue struct {
	Object       Value
	AccessorName string
	Span         *token.Span
}

func NewAccessorLValue(object Value, accessorName string, span *token.Span) *AccessorLValue {
	return &AccessorLValue{
		Object:       object,
		AccessorName: accessorName,
		Span:         span,
	}
}

func (a *AccessorLValue) GetSpan() *token.Span {
	return a.Span
}

func (a *AccessorLValue) String() string {
	return fmt.Sprintf("AccessorLValue(%s.%s)", a.Object, a.AccessorName)
}

func AsAccessorLValue(value Value) *AccessorLValue {
	switch value := value.(type) {
	case *AccessorLValue:
		return value
	default:
		return nil
	}
}
