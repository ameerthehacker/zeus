package zeus_value

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
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
	// IsAmbient marks the defining declaration of a `global` — an ambient,
	// program-wide-unique symbol (external linkage, stable un-mangled name).
	IsAmbient bool
	// IsExtern marks a module-local *reference* to an ambient global defined in
	// another module (external declaration only; no storage/initializer here).
	IsExtern bool
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
	// ExternRuntimeName, when non-empty, is the Zig runtime symbol this method's body
	// forwards to (see ClassMethod.IsExtern). It makes an extern method self-describing —
	// codegen no longer needs the primordial name to derive the runtime symbol.
	ExternRuntimeName string
	// ExportModulePath, when non-empty, is the path of the module that `export`s this function.
	// Codegen gives it the module-scoped external symbol name so importers link to it. Recorded
	// on the Function (not just the EXPORT instruction) so genDeclFunc can apply the linkage
	// independently of the order in which EXPORT vs DECL_FUNC are walked.
	ExportModulePath string
	// IsOSEntry marks the entry module's `main` as the program's OS entry point (the linker's
	// `-e` target). Codegen gives it external linkage. The module-init dispatch is emitted as its
	// first statements, so there is no separate synthetic entry function.
	IsOSEntry bool
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
	AccessorKindNone AccessorKind = iota
	AccessorKindGetter
	AccessorKindSetter
)

// ClassAccessor holds the getter and/or setter function for a named accessor property.
type ClassAccessor struct {
	Name           string
	Getter         *Function // nil if setter-only
	Setter         *Function // nil if getter-only
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
	// IsExtern marks a method whose body is not user IR but a forwarding call into the Zig
	// runtime (codegen emits it via emitExternMethodBody). This is how primordial classes
	// are built — they are ordinary classes whose methods happen to be extern.
	IsExtern bool
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
	// layout memoizes the physical class layout (see Layout). Built lazily on first access,
	// always after the class is finalized. Because Class is often copied by value, this cache
	// is effectively per-copy; that is harmless since the builder is idempotent and cheap.
	layout *ClassLayout
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
	if value == nil {
		return UndefinedType{}
	}
	switch value := value.(type) {
	case *Var:
		return value.ValueType
	case *Constant:
		return value.ValueType
	case *Object:
		return value.ValueType
	case *Class:
		return NewClassType(value)
	case *Interface:
		return NewInterfaceType(value)
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
		// The indexed object is not an array (e.g. indexing an untyped variable on
		// malformed input). Return an undefined type so callers surface a clean type
		// error rather than crashing.
		return UndefinedType{Span: value.GetSpan()}
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

// interfaceIdCounter gives every interface a stable unique id, used to key per-interface
// dispatch tables in codegen. Interfaces and classes have separate id spaces.
var interfaceIdCounter = 0

// Interface is a TypeScript-style structural interface. It is a purely type-level
// value: it lives in the symbol table and drives conformance checking, but emits no
// IR/LLVM of its own. Methods carry only signatures (no body); Properties reuse
// ClassProperty for name + type + readonly (AccessModifier is always nil).
type Interface struct {
	Id         int
	Name       string
	Parents    []*Interface // interfaces this one extends (structural union of members)
	Properties []*ClassProperty
	Methods    []*Function
	Span       *token.Span
}

func NewInterface(name string, parents []*Interface, properties []*ClassProperty, methods []*Function, span *token.Span) *Interface {
	interfaceIdCounter += 1
	return &Interface{
		Id:         interfaceIdCounter,
		Name:       name,
		Parents:    parents,
		Properties: properties,
		Methods:    methods,
		Span:       span,
	}
}

func (i Interface) GetSpan() *token.Span {
	return i.Span
}

func (i Interface) String() string {
	return i.Name
}

func IsInterface(value Value) bool {
	_, ok := value.(*Interface)
	return ok
}

func AsInterface(value Value) *Interface {
	switch value := value.(type) {
	case *Interface:
		return value
	default:
		return nil
	}
}

// PropertyBackingKind distinguishes how a conforming class provides an interface property.
type PropertyBackingKind int

const (
	PropertyBackingField    PropertyBackingKind = iota // a real data field at FieldIndex
	PropertyBackingAccessor                            // a get/set accessor at GetterSlot/SetterSlot
)

// PropertyBacking says how one class provides one interface property: either a real field (its
// 0-based index in Layout().Fields, which codegen turns into a byte offset) or a get/set accessor
// (vtable slots; SetterSlot is -1 when the property is readonly / has no setter). The type checker
// and the itable builder both derive this via ResolveInterfacePropertyBacking so they agree.
type PropertyBacking struct {
	Kind       PropertyBackingKind
	FieldIndex int
	GetterSlot int
	SetterSlot int
}

// ResolveInterfacePropertyBacking determines how `class` backs interface property `prop` and
// whether it conforms. Fields win over accessors (a class can't declare both of the same name).
// `writable` (the property is not readonly) additionally requires a setter for an accessor backing.
func ResolveInterfacePropertyBacking(class *Class, prop *ClassProperty, writable bool) (PropertyBacking, bool) {
	name := prop.Property.Name
	wantType := prop.Property.ValueType
	layout := class.Layout()

	// Field first (fast path): a real data field, base-first index (what codegen offsets from).
	for i, field := range layout.Fields {
		if field.Property.Name == name {
			if !CmpValueType(wantType, field.Property.ValueType) {
				return PropertyBacking{}, false
			}
			return PropertyBacking{Kind: PropertyBackingField, FieldIndex: i}, true
		}
	}

	// Else a get/set accessor (already a first-class vtable method).
	acc := LookupAccessor(class, name)
	if acc == nil || acc.Getter == nil || !CmpValueType(wantType, acc.Getter.ReturnType) {
		return PropertyBacking{}, false
	}
	setterSlot := -1
	if acc.Setter != nil {
		setterSlot = vtableSlotOf(layout, acc.Setter.SourceName())
	}
	if writable {
		// A writable interface property needs a setter accepting the property type.
		if acc.Setter == nil || len(acc.Setter.Params) == 0 || !CmpValueType(wantType, acc.Setter.Params[0].ValueType) {
			return PropertyBacking{}, false
		}
	}
	return PropertyBacking{
		Kind:       PropertyBackingAccessor,
		GetterSlot: vtableSlotOf(layout, acc.Getter.SourceName()),
		SetterSlot: setterSlot,
	}, true
}

// InterfaceDispatchRow describes one conforming class's dispatch data for an interface,
// as pure integers (no LLVM). MethodSlots[j] is the class's vtable slot for interface
// method j (in InterfaceMethods order); PropertyBackings[k] describes how the class provides
// interface property k (field offset vs accessor vtable slot), in InterfaceProperties order.
type InterfaceDispatchRow struct {
	ClassId          int
	Class            *Class
	MethodSlots      []int
	PropertyBackings []PropertyBacking
}

// InterfaceDispatchLayout is the LLVM-independent description of an interface's runtime
// dispatch tables. Codegen consumes it mechanically to emit the [maxClassId+1 x N] globals.
type InterfaceDispatchLayout struct {
	MaxClassId int
	Rows       []InterfaceDispatchRow // conforming classes only
}

// BuildInterfaceDispatchLayout computes, as pure data, the dispatch layout of an interface
// over a set of candidate classes: which classes structurally conform, and for each, the
// vtable slot of every interface method and the field index of every interface property.
// It performs no LLVM work — codegen emits the tables and derives byte offsets from the
// field indices. Must be called post IR-gen (it reads Class.Layout()).
func BuildInterfaceDispatchLayout(iface *Interface, classes []*Class) *InterfaceDispatchLayout {
	methods := InterfaceMethods(iface)
	props := InterfaceProperties(iface)

	layout := &InterfaceDispatchLayout{}
	for _, class := range classes {
		if !ClassConformsToInterface(class, iface) {
			continue
		}
		classLayout := class.Layout()

		methodSlots := make([]int, len(methods))
		for j, m := range methods {
			methodSlots[j] = vtableSlotOf(classLayout, m.SourceName())
		}
		propBackings := make([]PropertyBacking, len(props))
		for k, p := range props {
			// class conforms (checked above), so backing resolution succeeds.
			propBackings[k], _ = ResolveInterfacePropertyBacking(class, p, !p.IsReadonly)
		}

		layout.Rows = append(layout.Rows, InterfaceDispatchRow{
			ClassId:          class.Id,
			Class:            class,
			MethodSlots:      methodSlots,
			PropertyBackings: propBackings,
		})
		if class.Id > layout.MaxClassId {
			layout.MaxClassId = class.Id
		}
	}
	return layout
}

// vtableSlotOf returns the vtable slot of the method named `name` in a class layout, or -1.
// Mirrors util.GetMethodIndex without the import cycle (util imports zeus_value).
func vtableSlotOf(layout *ClassLayout, name string) int {
	for slot, entry := range layout.VTable {
		if entry.Method.Method.SourceName() == name {
			return slot
		}
	}
	return -1
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

// LookupAccessor finds an accessor (get/set) by name on the class or any ancestor.
func LookupAccessor(class *Class, name string) *ClassAccessor {
	for c := class; c != nil; c = c.ParentClass {
		for _, acc := range c.Accessors {
			if acc.Name == name {
				return acc
			}
		}
	}
	return nil
}

// LookupStaticAccessor finds a static accessor by name on the class or any ancestor.
func LookupStaticAccessor(class *Class, name string) *ClassAccessor {
	for c := class; c != nil; c = c.ParentClass {
		for _, acc := range c.Accessors {
			if acc.IsStatic && acc.Name == name {
				return acc
			}
		}
	}
	return nil
}

// LookupMethod finds a method by (source) name on the class or any ancestor. A method on a
// derived class shadows a same-named method on a base class (the walk starts at `class`).
func LookupMethod(class *Class, name string) *ClassMethod {
	for c := class; c != nil; c = c.ParentClass {
		for _, m := range c.Methods {
			if m.Method.SourceName() == name {
				return m
			}
		}
	}
	return nil
}

// LookupConstructorClass returns the nearest class in the chain (starting at `class`) that
// declares its own constructor, or nil if no class in the chain has one. This is the class
// whose constructor a `super(...)`/factory call should target and forward arguments to.
func LookupConstructorClass(class *Class) *Class {
	for c := class; c != nil; c = c.ParentClass {
		for _, m := range c.Methods {
			if m.Method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
				return c
			}
		}
	}
	return nil
}

// IsSubclassOf reports whether `class` is `ancestor` or transitively derives from it.
func IsSubclassOf(class *Class, ancestor *Class) bool {
	for c := class; c != nil; c = c.ParentClass {
		if c == ancestor || c.Name == ancestor.Name {
			return true
		}
	}
	return false
}

// FlattenedInstanceProperties returns a class's instance (non-static) data fields ordered
// base-first, then own — the physical order of fields in the object struct. A derived object
// therefore begins with its base's fields, so a derived pointer is a valid base pointer.
func FlattenedInstanceProperties(class *Class) []*ClassProperty {
	var result []*ClassProperty
	if class.ParentClass != nil {
		result = FlattenedInstanceProperties(class.ParentClass)
	}
	for _, prop := range class.Properties {
		if prop.IsStatic {
			continue
		}
		result = append(result, prop)
	}
	return result
}

// isVTableMethod reports whether a method occupies a vtable slot (instance, non-constructor,
// non-lowered). Static methods, the constructor, and lowered methods are dispatched directly.
func isVTableMethod(m *ClassMethod) bool {
	return m.Method.SourceName() != token.CONSTRUCTOR_METHOD_NAME && !m.IsLowered && !m.IsStatic
}

// FlattenedVTableMethods returns a class's vtable methods in physical slot order: the base's
// vtable methods first (an override replaces its base method in-place, keeping the slot index),
// then the derived class's newly-introduced methods. Because a slot index is stable across the
// hierarchy, dispatching an inherited slot through a derived object's vtable reaches the
// override — the essence of dynamic dispatch.
func FlattenedVTableMethods(class *Class) []*ClassMethod {
	var result []*ClassMethod
	if class.ParentClass != nil {
		result = FlattenedVTableMethods(class.ParentClass)
	}
	for _, m := range class.Methods {
		if !isVTableMethod(m) {
			continue
		}
		overridden := false
		for i, existing := range result {
			if existing.Method.SourceName() == m.Method.SourceName() {
				result[i] = m
				overridden = true
				break
			}
		}
		if !overridden {
			result = append(result, m)
		}
	}
	return result
}

// FlattenedMethods returns all of a class's methods (any kind) base-first, then own — so a
// member lookup that scans the result sees a derived method after (and thus shadowing) a
// same-named base method. Used for name-based member resolution, not vtable layout.
func FlattenedMethods(class *Class) []*ClassMethod {
	var result []*ClassMethod
	if class.ParentClass != nil {
		result = FlattenedMethods(class.ParentClass)
	}
	return append(result, class.Methods...)
}

// VTableEntry is one slot of a class's vtable: the method that fills the slot (an override or an
// inherited method) together with the class whose method list contributed it. DefiningClass is
// needed to name extern (primordial) methods, which are compiled under a class-scoped symbol.
type VTableEntry struct {
	Method        *ClassMethod
	DefiningClass *Class
}

// ClassLayout is a class's physical layout computed once as explicit data: base-first instance
// fields, base-first vtable slots (an override sharing the base slot), and the effective
// constructor. Codegen, util, and the type checker read this instead of re-deriving layout from
// the Flattened* helpers, so layout policy lives here (Zeus semantics) rather than in the LLVM
// backend. It is unit-testable without LLVM.
type ClassLayout struct {
	Fields           []*ClassProperty // base-first instance fields; instance struct slot = index+1
	VTable           []VTableEntry    // base-first vtable slots; slot = index
	Constructor      *Function        // effective constructor (own or nearest inherited), or nil
	ConstructorClass *Class           // class that declares Constructor, or nil
}

// Layout returns the class's physical layout, building and memoizing it on first access. It must
// only be called after the class is finalized (post IR-gen); IR-gen itself uses the name-resolution
// Lookup* helpers, never Layout, so the memoized result is always built against a complete class.
func (c *Class) Layout() *ClassLayout {
	if c.layout != nil {
		return c.layout
	}
	layout := &ClassLayout{
		Fields: FlattenedInstanceProperties(c),
		VTable: flattenedVTableEntries(c),
	}
	// Effective constructor: the nearest class in the chain declaring its own constructor.
	if ctorClass := LookupConstructorClass(c); ctorClass != nil {
		for _, m := range ctorClass.Methods {
			if m.Method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
				layout.Constructor = m.Method
				layout.ConstructorClass = ctorClass
				break
			}
		}
	}
	c.layout = layout
	return c.layout
}

// flattenedVTableEntries mirrors FlattenedVTableMethods but also records, per slot, the class whose
// method list contributed the method (DefiningClass): an override records the derived class, an
// inherited-not-overridden slot keeps the base. Each recursive frame builds a fresh slice (via
// append), so overwriting result[i] for an override is safe.
func flattenedVTableEntries(class *Class) []VTableEntry {
	var result []VTableEntry
	if class.ParentClass != nil {
		result = flattenedVTableEntries(class.ParentClass)
	}
	for _, m := range class.Methods {
		if !isVTableMethod(m) {
			continue
		}
		overridden := false
		for i, existing := range result {
			if existing.Method.Method.SourceName() == m.Method.SourceName() {
				result[i] = VTableEntry{Method: m, DefiningClass: class}
				overridden = true
				break
			}
		}
		if !overridden {
			result = append(result, VTableEntry{Method: m, DefiningClass: class})
		}
	}
	return result
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
	if err != nil {
		// The number does not fit in a signed 64-bit integer (e.g. a large u64 literal), or
		// it is a malformed literal (e.g. "0x" with no digits, from partial/incorrect source).
		// Fall back to the widest size so IR generation never panics; malformed literals are
		// reported as errors by the lexer, not here.
		return I64
	}

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

// DefaultLiteralIntType returns the type for a context-less integer literal: signed, and at least
// i32 (i64 when the value doesn't fit i32). A literal narrows to a smaller/unsigned type only when
// assigned into one that can represent it (see ConstantFitsInIntType) — so `let x = 5` is i32,
// while `let b: u8 = 200` retypes the constant to u8.
func DefaultLiteralIntType(valueStr string, span *token.Span) IntType {
	size := GetSignedIntSize(valueStr)
	if size < I32 {
		size = I32
	}
	return IntType{Signed: true, Size: size, Span: span}
}

// ConstantFitsInIntType reports whether the integer literal `valueStr` fits within the range of the
// target int type (signed or unsigned, i8..i64). Base-0 parsing handles 0x/0b/0o literals.
func ConstantFitsInIntType(valueStr string, t IntType) bool {
	if v, err := strconv.ParseInt(valueStr, 0, 64); err == nil {
		if t.Signed {
			var min, max int64
			switch t.Size {
			case I8:
				min, max = -128, 127
			case I16:
				min, max = -32768, 32767
			case I32:
				min, max = -2147483648, 2147483647
			default: // I64: every int64 fits
				return true
			}
			return v >= min && v <= max
		}
		// Unsigned target: value must be non-negative and within the unsigned max.
		if v < 0 {
			return false
		}
		var max uint64
		switch t.Size {
		case I8:
			max = 255
		case I16:
			max = 65535
		case I32:
			max = 4294967295
		default: // u64: every non-negative int64 fits
			return true
		}
		return uint64(v) <= max
	}
	// Value overflows int64 — only a large u64 literal can still fit, and only an unsigned i64 target.
	if _, err := strconv.ParseUint(valueStr, 0, 64); err == nil {
		return !t.Signed && t.Size == I64
	}
	return false
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
