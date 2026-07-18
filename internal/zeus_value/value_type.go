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

// CTypeKind enumerates the C-ABI primitive types Zeus can name at the FFI boundary.
type CTypeKind int

const (
	CInt    CTypeKind = iota // C `int`      -> i32 (signed)
	CLong                    // C `long`/`ssize_t`/`off_t` -> i64 (signed); LP64 on macOS/Linux
	CSize                    // C `size_t`   -> i64 (unsigned)
	CPtr                     // C `void*`    -> ptr addrspace(0) (raw, non-GC)
	CStr                     // C `char*`    -> ptr addrspace(0) (raw, non-GC)
	CDouble                  // C `double`   -> f64
)

// CType is an inert C-ABI type used only to cross the FFI boundary or convert explicitly to/from a
// Zeus type via `as`. Unlike OpaqueType (a GC pointer, addrspace 1), CPtr/CStr are raw addrspace(0)
// pointers the collector never treats as roots. C types support no arithmetic/indexing/member access.
type CType struct {
	Kind CTypeKind
	Span *token.Span
}

func (c CType) GetSpan() *token.Span {
	return c.Span
}

func (c CType) String() string {
	switch c.Kind {
	case CInt:
		return "cint"
	case CLong:
		return "clong"
	case CSize:
		return "csize"
	case CPtr:
		return "cptr"
	case CStr:
		return "cstr"
	case CDouble:
		return "cdouble"
	default:
		return "cunknown"
	}
}

func NewCType(kind CTypeKind, span *token.Span) CType {
	return CType{Kind: kind, Span: span}
}

func IsCType(value ValueType) bool {
	_, ok := value.(CType)
	return ok
}

func AsCType(value ValueType) *CType {
	if c, ok := value.(CType); ok {
		return &c
	}
	return nil
}

// IsCNumericType reports whether a C type participates in unchecked numeric `as` casts (the
// cint/clong/csize/cdouble bridge). Pointer kinds (cptr/cstr) are strict and never bridge.
func IsCNumericType(value ValueType) bool {
	if c, ok := value.(CType); ok {
		return c.Kind == CInt || c.Kind == CLong || c.Kind == CSize || c.Kind == CDouble
	}
	return false
}

type FunctionType struct {
	ReturnType ValueType
	ParamTypes []ValueType
	Span       *token.Span
	// IsVariadic is true when the final entry of ParamTypes is a rest parameter
	// (an array type whose element type accepts the trailing call arguments).
	IsVariadic bool
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
	Class *Class
}

func (o ObjectType) GetSpan() *token.Span {
	return o.Class.Span
}

func (o ObjectType) String() string {
	return o.Class.Name
}

func NewObjectType(class *Class) ObjectType {
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
	Class *Class
}

func (c ClassType) GetSpan() *token.Span {
	return c.Class.Span
}

func (c ClassType) String() string {
	return c.Class.Name
}

func NewClassType(class *Class) ClassType {
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

// InterfaceType is the ValueType of an interface annotation (e.g. `let s: Shape`).
// At runtime an interface value is represented exactly like an object (a one-word
// pointer); the InterfaceType exists only for structural conformance checking.
type InterfaceType struct {
	Interface *Interface
}

func (i InterfaceType) GetSpan() *token.Span {
	return i.Interface.Span
}

func (i InterfaceType) String() string {
	return i.Interface.Name
}

func NewInterfaceType(iface *Interface) InterfaceType {
	return InterfaceType{Interface: iface}
}

func AsInterfaceType(value ValueType) *InterfaceType {
	switch value := value.(type) {
	case InterfaceType:
		return &value
	default:
		return nil
	}
}

func IsInterfaceType(value ValueType) bool {
	return AsInterfaceType(value) != nil
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
	case token.TokenTypeCInt:
		return CType{Kind: CInt, Span: t.Span}
	case token.TokenTypeCLong:
		return CType{Kind: CLong, Span: t.Span}
	case token.TokenTypeCSize:
		return CType{Kind: CSize, Span: t.Span}
	case token.TokenTypeCPtr:
		return CType{Kind: CPtr, Span: t.Span}
	case token.TokenTypeCStr:
		return CType{Kind: CStr, Span: t.Span}
	case token.TokenTypeCDouble:
		return CType{Kind: CDouble, Span: t.Span}
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
		IsVariadic: value.IsVariadic,
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
	case CType:
		bc, ok := b.(CType)
		return ok && a.Kind == bc.Kind
	case FunctionType:
		b, ok := b.(FunctionType)
		if !ok || len(a.ParamTypes) != len(b.ParamTypes) || a.IsVariadic != b.IsVariadic {
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
		aCall := getFunctorCallMethod(a.Class)
		bCall := getFunctorCallMethod(bClassType.Class)
		if aCall == nil || bCall == nil {
			return false
		}
		return CmpValueType(GetValueType(aCall), GetValueType(bCall))

	case InterfaceType:
		// Callers pass CmpValueType(target, source): does `b` structurally satisfy
		// interface `a`? A conforming class (ObjectType) or interface (InterfaceType)
		// on the source side is assignable to the interface.
		return isAssignableToInterface(a.Interface, b)
	}

	return false
}

// interfaceCmpInProgress breaks cycles during structural interface comparison so that
// self-referential (`interface Node { next: Node }`) and mutually-referential interfaces
// don't recurse forever. A cycle is treated coinductively as conformant. Safe as a package
// global because the compiler is single-threaded, and each entry is removed on return.
var interfaceCmpInProgress = map[[2]*Interface]bool{}

// isAssignableToInterface reports whether a value of type `source` structurally
// satisfies every member of `iface`, including members inherited via `extends`.
func isAssignableToInterface(iface *Interface, source ValueType) bool {
	// Only objects and other interfaces can satisfy an interface.
	if !IsObjectType(source) && !IsInterfaceType(source) {
		return false
	}

	// Array classes (i32[], Point[], ...) participate: each element type has its own runtime type
	// handle (a distinct class id), so codegen can key it in the interface itable — see
	// genObjectArrayTypeHandle. This predicate must stay in lockstep with codegen's
	// candidateClasses (both now include arrays) or a match would type-check but misdispatch.

	// Cycle handling for self-/mutually-referential interfaces.
	if si := AsInterfaceType(source); si != nil {
		if si.Interface == iface {
			return true // an interface is trivially assignable to itself
		}
		key := [2]*Interface{iface, si.Interface}
		if interfaceCmpInProgress[key] {
			return true
		}
		interfaceCmpInProgress[key] = true
		defer delete(interfaceCmpInProgress, key)
	}

	props, methods := flattenInterfaceMembers(iface)

	// An interface property is backed by a real data **field** OR a get/set **accessor**
	// (readonly needs only a getter; a writable property needs a setter too); an interface
	// method must be a real method (a function-typed field is a functor object, not a vtable
	// entry, so it can't satisfy a method). Class conformers resolve the backing via
	// ResolveInterfacePropertyBacking — kept in lockstep with the itable so dispatch matches;
	// an interface *source* satisfies a property only with a matching interface property.
	for _, prop := range props {
		if src := AsObjectType(source); src != nil {
			if _, ok := ResolveInterfacePropertyBacking(src.Class, prop, !prop.IsReadonly); !ok {
				return false
			}
		} else {
			srcType, ok := lookupFieldType(source, prop.Property.Name)
			if !ok || !CmpValueType(prop.Property.ValueType, srcType) {
				return false
			}
		}
	}
	for _, method := range methods {
		srcType, ok := lookupMethodType(source, method.SourceName())
		if !ok || !CmpValueType(ToFunctionType(*method), srcType) {
			return false
		}
	}
	return true
}

// flattenInterfaceMembers collects an interface's own members plus every member
// inherited through `extends`, with own members shadowing inherited ones by name.
func flattenInterfaceMembers(iface *Interface) ([]*ClassProperty, []*Function) {
	seenProp := map[string]bool{}
	seenMethod := map[string]bool{}
	visited := map[*Interface]bool{}
	var props []*ClassProperty
	var methods []*Function

	var visit func(i *Interface)
	visit = func(i *Interface) {
		if i == nil || visited[i] {
			return
		}
		visited[i] = true
		for _, p := range i.Properties {
			if !seenProp[p.Property.Name] {
				seenProp[p.Property.Name] = true
				props = append(props, p)
			}
		}
		for _, m := range i.Methods {
			if !seenMethod[m.SourceName()] {
				seenMethod[m.SourceName()] = true
				methods = append(methods, m)
			}
		}
		for _, parent := range i.Parents {
			visit(parent)
		}
	}
	visit(iface)
	return props, methods
}

// InterfaceMethods returns an interface's method signatures in dispatch order — own
// members first, then those inherited via `extends`, deduped by name. This order defines
// the itable slot layout used for dynamic dispatch.
func InterfaceMethods(iface *Interface) []*Function {
	_, methods := flattenInterfaceMembers(iface)
	return methods
}

// InterfaceMethodIndex returns the itable slot of method `name` in `iface`, or -1.
func InterfaceMethodIndex(iface *Interface, name string) int {
	for i, m := range InterfaceMethods(iface) {
		if m.SourceName() == name {
			return i
		}
	}
	return -1
}

// InterfaceProperties returns an interface's property signatures in dispatch order — own
// members first, then those inherited via `extends`, deduped by name.
func InterfaceProperties(iface *Interface) []*ClassProperty {
	props, _ := flattenInterfaceMembers(iface)
	return props
}

// InterfacePropertyIndex returns the dispatch slot of property `name` in `iface`, or -1.
func InterfacePropertyIndex(iface *Interface, name string) int {
	for i, p := range InterfaceProperties(iface) {
		if p.Property.Name == name {
			return i
		}
	}
	return -1
}

// ClassConformsToInterface reports whether `class` structurally satisfies `iface`.
func ClassConformsToInterface(class *Class, iface *Interface) bool {
	return isAssignableToInterface(iface, NewObjectType(class))
}

// lookupFieldType returns the type of a data field named `name` on `source` (a class
// property or interface property), searching parent classes / extended interfaces. Methods
// and accessors do NOT count — interface property dispatch resolves to a field offset, which
// only a real field provides.
func lookupFieldType(source ValueType, name string) (ValueType, bool) {
	switch src := source.(type) {
	case ObjectType:
		for cls := src.Class; cls != nil; cls = cls.ParentClass {
			for _, prop := range cls.Properties {
				if prop.Property.Name == name {
					return prop.Property.ValueType, true
				}
			}
		}
	case InterfaceType:
		for _, prop := range InterfaceProperties(src.Interface) {
			if prop.Property.Name == name {
				return prop.Property.ValueType, true
			}
		}
	}
	return nil, false
}

// lookupMethodType returns the FunctionType of a method named `name` on `source` (a class
// method or interface method), searching parent classes / extended interfaces (params
// exclude the implicit receiver). Fields — including function-typed ones — and accessors do
// NOT count, since interface method dispatch resolves to a vtable slot.
func lookupMethodType(source ValueType, name string) (ValueType, bool) {
	switch src := source.(type) {
	case ObjectType:
		for cls := src.Class; cls != nil; cls = cls.ParentClass {
			for _, method := range cls.Methods {
				if method.Method.SourceName() == name {
					return ToFunctionType(*method.Method), true
				}
			}
		}
	case InterfaceType:
		for _, method := range InterfaceMethods(src.Interface) {
			if method.SourceName() == name {
				return ToFunctionType(*method), true
			}
		}
	}
	return nil, false
}

// GetFunctorCallMethod returns the __call__ method of a class if it has one, nil otherwise.
func GetFunctorCallMethod(class *Class) *Function {
	for _, m := range class.Methods {
		if m.Method.OriginalName == token.FUNCTOR_CALL_METHOD_NAME {
			return m.Method
		}
	}
	return nil
}

// getFunctorCallMethod is the package-private alias used within this package.
func getFunctorCallMethod(class *Class) *Function {
	return GetFunctorCallMethod(class)
}

// TypeVar is a unification variable used during type inference for generics/overloads.
// It is never emitted to LLVM — the unifier resolves all TypeVars before codegen.
type TypeVar struct{ ID int }

func (t TypeVar) String() string       { return fmt.Sprintf("T%d", t.ID) }
func (t TypeVar) GetSpan() *token.Span { return nil }

func IsTypeVar(t ValueType) bool { _, ok := t.(TypeVar); return ok }

func AsTypeVar(t ValueType) *TypeVar {
	if tv, ok := t.(TypeVar); ok {
		return &tv
	}
	return nil
}
