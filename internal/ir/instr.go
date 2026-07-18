package ir

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

type InstrType int

type InstrInput interface {
	String() string
}

type VarDecl struct {
	Name        string
	ValueType   zeus_value.ValueType
	IsConst     bool
	Initializer zeus_value.Value
	Span        *token.Span
	// IsVariadic marks a function's rest parameter; propagated onto the built Var.
	IsVariadic bool
	// IsAmbient marks a `global` definition — the built Var gets the stable, un-mangled ambient
	// symbol name and external linkage instead of a per-module unique name (see BuildGlobalVarDecl).
	IsAmbient bool
}

func NewVarDecl(name string, valueType zeus_value.ValueType, isConst bool, initializer zeus_value.Value, span *token.Span) *VarDecl {
	return &VarDecl{
		Name:        name,
		ValueType:   valueType,
		IsConst:     isConst,
		Initializer: initializer,
		Span:        span,
	}
}

func (v VarDecl) String() string {
	return fmt.Sprintf("%s %s", v.ValueType, v.Name)
}

func panicInvalidInputType(expected string, actual InstrInput) {
	panic(fmt.Sprintf("invalid input type: %s, but found: %+v", expected, actual))
}

type BinaryOpInstrInput struct {
	Left  zeus_value.Value
	Right zeus_value.Value
}

func NewBinaryOpInstrInput(left, right zeus_value.Value) *BinaryOpInstrInput {
	return &BinaryOpInstrInput{
		Left:  left,
		Right: right,
	}
}

func (i BinaryOpInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.Left, i.Right)
}

func AsBinaryOpInstrInput(input InstrInput) *BinaryOpInstrInput {
	switch input := input.(type) {
	case *BinaryOpInstrInput:
		return input
	default:
		panicInvalidInputType("BinaryOpInstrInput", input)
	}

	return nil
}

type UnaryOpInstrInput struct {
	Value zeus_value.Value
}

func NewUnaryOpInstrInput(value zeus_value.Value) *UnaryOpInstrInput {
	return &UnaryOpInstrInput{
		Value: value,
	}
}

func (i UnaryOpInstrInput) String() string {
	return i.Value.String()
}

func AsUnaryOpInstrInput(input InstrInput) *UnaryOpInstrInput {
	switch input := input.(type) {
	case *UnaryOpInstrInput:
		return input
	default:
		panicInvalidInputType("UnaryOpInstrInput", input)
	}

	return nil
}

type DeclareVarInstrInput struct {
	Variable    *zeus_value.Var
	Initializer zeus_value.Value
	IsConst     bool
}

func NewDeclareVarInstrInput(variable *zeus_value.Var, initializer zeus_value.Value, isConst bool) *DeclareVarInstrInput {
	return &DeclareVarInstrInput{
		Variable:    variable,
		Initializer: initializer,
		IsConst:     isConst,
	}
}

func (i DeclareVarInstrInput) String() string {
	if i.Initializer != nil {
		return fmt.Sprintf("%s %s = %s", i.Variable.ValueType, i.Variable.Name, i.Initializer)
	}
	return fmt.Sprintf("%s %s", i.Variable.ValueType, i.Variable.Name)
}

func AsDeclVarInstrInput(input InstrInput) *DeclareVarInstrInput {
	switch input := input.(type) {
	case *DeclareVarInstrInput:
		return input
	default:
		panicInvalidInputType("DeclareVarInstrInput", input)
	}

	return nil
}

func AsDeclGlobalVarInstrInput(input InstrInput) *DeclareVarInstrInput {
	return AsDeclVarInstrInput(input)
}

// CallModuleInitInstrInput names the module-init symbol to invoke (module.ModuleInitFuncName).
type CallModuleInitInstrInput struct {
	SymbolName string
}

func NewCallModuleInitInstrInput(symbolName string) *CallModuleInitInstrInput {
	return &CallModuleInitInstrInput{SymbolName: symbolName}
}

func (i CallModuleInitInstrInput) String() string {
	return i.SymbolName + "()"
}

func AsCallModuleInitInstrInput(input InstrInput) *CallModuleInitInstrInput {
	switch input := input.(type) {
	case *CallModuleInitInstrInput:
		return input
	default:
		panicInvalidInputType("CallModuleInitInstrInput", input)
	}

	return nil
}

type LoadInstrInput struct {
	Addr *zeus_value.Var
}

func NewLoadInstrInput(addr *zeus_value.Var) *LoadInstrInput {
	return &LoadInstrInput{
		Addr: addr,
	}
}

func (i LoadInstrInput) String() string {
	return i.Addr.String()
}

func AsLoadInstrInput(input InstrInput) *LoadInstrInput {
	switch input := input.(type) {
	case *LoadInstrInput:
		return input
	default:
		panicInvalidInputType("LoadInstrInput", input)
	}

	return nil
}

type StoreInstrInput struct {
	Addr  *zeus_value.Var
	Value zeus_value.Value
}

func NewStoreInstrInput(addr *zeus_value.Var, value zeus_value.Value) *StoreInstrInput {
	return &StoreInstrInput{
		Addr:  addr,
		Value: value,
	}
}

func (i StoreInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.Addr, i.Value)
}

func AsStoreInstrInput(input InstrInput) *StoreInstrInput {
	switch input := input.(type) {
	case *StoreInstrInput:
		return input
	default:
		panicInvalidInputType("StoreInstrInput", input)
	}

	return nil
}

type CallFuncInstrInput struct {
	Callee zeus_value.Value
	Args   []zeus_value.Value
}

func NewCallFuncInstrInput(callee zeus_value.Value, args []zeus_value.Value) *CallFuncInstrInput {
	return &CallFuncInstrInput{
		Callee: callee,
		Args:   args,
	}
}

func (i CallFuncInstrInput) String() string {
	args := []string{}
	for _, arg := range i.Args {
		args = append(args, arg.String())
	}
	return fmt.Sprintf("%s (%s)", i.Callee, strings.Join(args, ", "))
}

func AsCallFuncInstrInput(input InstrInput) *CallFuncInstrInput {
	switch input := input.(type) {
	case *CallFuncInstrInput:
		return input
	default:
		panicInvalidInputType("CallFuncInstrInput", input)
	}

	return nil
}

type ReturnInstrInput struct {
	Value zeus_value.Value
}

func NewReturnInstrInput(value zeus_value.Value) *ReturnInstrInput {
	return &ReturnInstrInput{
		Value: value,
	}
}

func (i ReturnInstrInput) String() string {
	if i.Value != nil {
		return i.Value.String()
	}
	return "void"
}

func AsReturnInstrInput(input InstrInput) *ReturnInstrInput {
	switch input := input.(type) {
	case *ReturnInstrInput:
		return input
	default:
		panicInvalidInputType("ReturnInstrInput", input)
	}

	return nil
}

type DeclFuncInstrInput struct {
	Function *zeus_value.Function
	Body     *BasicBlock
}

func AsDeclFuncInstrInput(input InstrInput) *DeclFuncInstrInput {
	switch input := input.(type) {
	case *DeclFuncInstrInput:
		return input
	default:
		panicInvalidInputType("DeclFuncInstrInput", input)
	}

	return nil
}

func NewDeclFuncInstrInput(function *zeus_value.Function, body *BasicBlock) *DeclFuncInstrInput {
	return &DeclFuncInstrInput{
		Function: function,
		Body:     body,
	}
}

func (i DeclFuncInstrInput) String() string {
	return i.Function.String()
}

type JmpInstrInput struct {
	Target *BasicBlock
}

func NewJmpInstrInput(target *BasicBlock) *JmpInstrInput {
	return &JmpInstrInput{
		Target: target,
	}
}

func (i JmpInstrInput) String() string {
	return fmt.Sprintf("%d", i.Target.Id)
}

func AsJmpInstrInput(input InstrInput) *JmpInstrInput {
	switch input := input.(type) {
	case *JmpInstrInput:
		return input
	default:
		panicInvalidInputType("JmpInstrInput", input)
	}

	return nil
}

type ExportInstrInput struct {
	ModulePath string
	Value      zeus_value.Value
}

func NewExportInstrInput(modulePath string, value zeus_value.Value) *ExportInstrInput {
	return &ExportInstrInput{
		ModulePath: modulePath,
		Value:      value,
	}
}

func (i ExportInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.ModulePath, i.Value)
}

func AsExportInstrInput(input InstrInput) *ExportInstrInput {
	switch input := input.(type) {
	case *ExportInstrInput:
		return input
	default:
		panicInvalidInputType("ExportInstrInput", input)
	}

	return nil
}

type ImportInstrInput struct {
	ModulePath string
	Value      zeus_value.Value
}

func NewImportInstrInput(modulePath string, value zeus_value.Value) *ImportInstrInput {
	return &ImportInstrInput{
		ModulePath: modulePath,
		Value:      value,
	}
}

func (i ImportInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.ModulePath, i.Value)
}

func AsImportInstrInput(input InstrInput) *ImportInstrInput {
	switch input := input.(type) {
	case *ImportInstrInput:
		return input
	default:
		panicInvalidInputType("ImportInstrInput", input)
	}

	return nil
}

type CondJmpInstrInput struct {
	TrueTarget  *BasicBlock
	FalseTarget *BasicBlock
	Condition   zeus_value.Value
}

func NewCondJmpInstrInput(trueTarget, falseTarget *BasicBlock, condition zeus_value.Value) *CondJmpInstrInput {
	return &CondJmpInstrInput{
		TrueTarget:  trueTarget,
		FalseTarget: falseTarget,
		Condition:   condition,
	}
}

func (i CondJmpInstrInput) String() string {
	return fmt.Sprintf("%s, %d, %d", i.Condition, i.TrueTarget.Id, i.FalseTarget.Id)
}

func AsCondJmpInstrInput(input InstrInput) *CondJmpInstrInput {
	switch input := input.(type) {
	case *CondJmpInstrInput:
		return input
	default:
		panicInvalidInputType("CondJmpInstrInput", input)
	}

	return nil
}

type CastInstrInput struct {
	Value    zeus_value.Value
	CastType zeus_value.ValueType
}

func NewCastInstrInput(value zeus_value.Value, castType zeus_value.ValueType) *CastInstrInput {
	return &CastInstrInput{
		Value:    value,
		CastType: castType,
	}
}

func (i CastInstrInput) String() string {
	return fmt.Sprintf("%s %s", i.Value, i.CastType)
}

func AsCastInstrInput(input InstrInput) *CastInstrInput {
	switch input := input.(type) {
	case *CastInstrInput:
		return input
	default:
		panicInvalidInputType("CastInstrInput", input)
	}

	return nil
}

// InstanceOfInstrInput is the input to an INSTANCEOF instruction: a runtime type test asking
// whether Value's dynamic class is (a subclass of) the class with id ClassId. Output is a bool.
type InstanceOfInstrInput struct {
	Value   zeus_value.Value
	ClassId int
}

func NewInstanceOfInstrInput(value zeus_value.Value, classId int) *InstanceOfInstrInput {
	return &InstanceOfInstrInput{Value: value, ClassId: classId}
}

func (i InstanceOfInstrInput) String() string {
	return fmt.Sprintf("%s isa #%d", i.Value, i.ClassId)
}

func AsInstanceOfInstrInput(input InstrInput) *InstanceOfInstrInput {
	switch input := input.(type) {
	case *InstanceOfInstrInput:
		return input
	default:
		panicInvalidInputType("InstanceOfInstrInput", input)
	}

	return nil
}

type CoerceInstrInput struct {
	Value      zeus_value.Value
	TargetType zeus_value.ValueType // only used for IR dumps; codegen uses Value's ObjectType directly
}

func NewCoerceInstrInput(value zeus_value.Value, targetType zeus_value.ValueType) *CoerceInstrInput {
	return &CoerceInstrInput{Value: value, TargetType: targetType}
}

func (i CoerceInstrInput) String() string {
	return fmt.Sprintf("COERCE(%s as %s)", i.Value, i.TargetType)
}

func AsCoerceInstrInput(input InstrInput) *CoerceInstrInput {
	switch input := input.(type) {
	case *CoerceInstrInput:
		return input
	default:
		panicInvalidInputType("CoerceInstrInput", input)
	}
	return nil
}

type DeclClassInstrInput struct {
	Class *zeus_value.Class
}

func NewDeclClassInstrInput(class *zeus_value.Class) *DeclClassInstrInput {
	return &DeclClassInstrInput{
		Class: class,
	}
}

func (i DeclClassInstrInput) String() string {
	properties := []string{}
	for _, property := range i.Class.Properties {
		prefix := ""
		if property.IsReadonly {
			prefix = "readonly "
		}
		properties = append(properties, fmt.Sprintf("%s%s: %s", prefix, property.Property.Name, property.Property.ValueType))
	}
	return fmt.Sprintf("%s { %s }", i.Class.Name, strings.Join(properties, ", "))
}

func AsDeclClassInstrInput(input InstrInput) *DeclClassInstrInput {
	switch input := input.(type) {
	case *DeclClassInstrInput:
		return input
	default:
		panicInvalidInputType("DeclClassInstrInput", input)
	}

	return nil
}

type NewObjInstrInput struct {
	Callee zeus_value.Value
	Args   []zeus_value.Value
}

func NewNewObjInstrInput(callee zeus_value.Value, args []zeus_value.Value) *NewObjInstrInput {
	return &NewObjInstrInput{
		Callee: callee,
		Args:   args,
	}
}

func (i NewObjInstrInput) String() string {
	args := []string{}
	for _, arg := range i.Args {
		args = append(args, arg.String())
	}
	return fmt.Sprintf("%s(%s)", i.Callee, strings.Join(args, ", "))
}

func AsNewObjInstrInput(input InstrInput) *NewObjInstrInput {
	switch input := input.(type) {
	case *NewObjInstrInput:
		return input
	default:
		panicInvalidInputType("NewObjInstrInput", input)
	}
	return nil
}

// AllocObjInstrInput allocates a zeroed instance of Class and installs its header/type-info,
// yielding the object pointer. It is the mechanism half of object creation (no field-init — the
// GC allocator zeroes memory — and no constructor call). FactoryLoweringPass emits it as the first
// instruction of a synthesized zeus_new_<Class> factory function.
type AllocObjInstrInput struct {
	Class *zeus_value.Class
}

func NewAllocObjInstrInput(class *zeus_value.Class) *AllocObjInstrInput {
	return &AllocObjInstrInput{Class: class}
}

func (i AllocObjInstrInput) String() string {
	return fmt.Sprintf("alloc %s", i.Class.Name)
}

func AsAllocObjInstrInput(input InstrInput) *AllocObjInstrInput {
	switch input := input.(type) {
	case *AllocObjInstrInput:
		return input
	default:
		panicInvalidInputType("AllocObjInstrInput", input)
	}

	return nil
}

// BoxInstrInput autoboxes Value (a scalar of the box's field type) into the boxed primordial
// TargetClass (Number or Bool). BoxLoweringPass expands it to ALLOC_OBJ + a store of Value into the
// box's `value` field.
type BoxInstrInput struct {
	Value       zeus_value.Value
	TargetClass *zeus_value.Class
}

func NewBoxInstrInput(value zeus_value.Value, targetClass *zeus_value.Class) *BoxInstrInput {
	return &BoxInstrInput{Value: value, TargetClass: targetClass}
}

func (i BoxInstrInput) String() string {
	return fmt.Sprintf("box %s as %s", i.Value, i.TargetClass.Name)
}

func AsBoxInstrInput(input InstrInput) *BoxInstrInput {
	switch input := input.(type) {
	case *BoxInstrInput:
		return input
	default:
		panicInvalidInputType("BoxInstrInput", input)
	}

	return nil
}

// UnboxInstrInput reads the scalar `value` out of Value, a boxed primordial (Number/Bool).
// BoxLoweringPass expands it to OBJECT_PROPERTY_ACCESS + LOAD.
type UnboxInstrInput struct {
	Value zeus_value.Value
}

func NewUnboxInstrInput(value zeus_value.Value) *UnboxInstrInput {
	return &UnboxInstrInput{Value: value}
}

func (i UnboxInstrInput) String() string {
	return fmt.Sprintf("unbox %s", i.Value)
}

func AsUnboxInstrInput(input InstrInput) *UnboxInstrInput {
	switch input := input.(type) {
	case *UnboxInstrInput:
		return input
	default:
		panicInvalidInputType("UnboxInstrInput", input)
	}

	return nil
}

// StringTemplatePart is one segment of a template literal instruction — either a static string
// chunk (IsExpr=false, Str) or an interpolated value (IsExpr=true, Value). Mirrors ast.TemplateStringPart
// but holds the already-evaluated interpolated Value; the type checker replaces it with its stringified form.
type StringTemplatePart struct {
	IsExpr bool
	Str    string
	Value  zeus_value.Value
}

// StringTemplateInstrInput is a template literal kept whole through type checking (see InstrTypeStringTemplate).
type StringTemplateInstrInput struct {
	Parts []*StringTemplatePart
}

func NewStringTemplateInstrInput(parts []*StringTemplatePart) *StringTemplateInstrInput {
	return &StringTemplateInstrInput{Parts: parts}
}

func (i StringTemplateInstrInput) String() string {
	return fmt.Sprintf("string_template(%d parts)", len(i.Parts))
}

func AsStringTemplateInstrInput(input InstrInput) *StringTemplateInstrInput {
	switch input := input.(type) {
	case *StringTemplateInstrInput:
		return input
	default:
		panicInvalidInputType("StringTemplateInstrInput", input)
	}

	return nil
}

// ReflectToStringInstrInput converts Value (a reference — object/array/interface/function/null) to
// its debug `string` via the runtime reflection printer. Codegen emits the zeus_reflect_to_string
// runtime call directly; there is no lowering pass.
type ReflectToStringInstrInput struct {
	Value zeus_value.Value
}

func NewReflectToStringInstrInput(value zeus_value.Value) *ReflectToStringInstrInput {
	return &ReflectToStringInstrInput{Value: value}
}

func (i ReflectToStringInstrInput) String() string {
	return fmt.Sprintf("reflect_to_string %s", i.Value)
}

func AsReflectToStringInstrInput(input InstrInput) *ReflectToStringInstrInput {
	switch input := input.(type) {
	case *ReflectToStringInstrInput:
		return input
	default:
		panicInvalidInputType("ReflectToStringInstrInput", input)
	}

	return nil
}

type ObjectPropertyAccessInstrInput struct {
	Object   zeus_value.Value
	Property string
	IsLValue bool
}

func NewObjectPropertyAccessInstrInput(object zeus_value.Value, property string, isLValue bool) *ObjectPropertyAccessInstrInput {
	return &ObjectPropertyAccessInstrInput{
		Object:   object,
		Property: property,
		IsLValue: isLValue,
	}
}

func AsObjectPropertyAccessInstrInput(input InstrInput) *ObjectPropertyAccessInstrInput {
	switch input := input.(type) {
	case *ObjectPropertyAccessInstrInput:
		return input
	default:
		panicInvalidInputType("ObjectPropertyAccessInstrInput", input)
	}

	return nil
}

func (i ObjectPropertyAccessInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.Object, i.Property)
}

type MethodCallInstrInput struct {
	Object     zeus_value.Value
	MethodName string
	Args       []zeus_value.Value
	// StaticClass, when set, makes this a non-virtual call: the method is resolved on (and called
	// directly from) StaticClass rather than dispatched through the receiver's vtable. This is how
	// super.method() reaches a base implementation even when the object overrides it. nil = the
	// usual virtual (vtable) dispatch.
	StaticClass *zeus_value.Class
}

func NewMethodCallInstrInput(object zeus_value.Value, methodName string, args []zeus_value.Value) *MethodCallInstrInput {
	return &MethodCallInstrInput{
		Object:     object,
		MethodName: methodName,
		Args:       args,
	}
}

// NewStaticMethodCallInstrInput builds a non-virtual (super.method()) call resolved on staticClass.
func NewStaticMethodCallInstrInput(object zeus_value.Value, methodName string, args []zeus_value.Value, staticClass *zeus_value.Class) *MethodCallInstrInput {
	return &MethodCallInstrInput{
		Object:      object,
		MethodName:  methodName,
		Args:        args,
		StaticClass: staticClass,
	}
}

func AsMethodCallInstrInput(input InstrInput) *MethodCallInstrInput {
	switch input := input.(type) {
	case *MethodCallInstrInput:
		return input
	default:
		panicInvalidInputType("MethodCallInstrInput", input)
	}

	return nil
}

func (i MethodCallInstrInput) String() string {
	args := []string{}
	for _, arg := range i.Args {
		args = append(args, arg.String())
	}
	return fmt.Sprintf("%s, %q, [%s]", i.Object, i.MethodName, strings.Join(args, ", "))
}

// SuperConstructorCallInstrInput represents `super(...)` inside a derived class's constructor.
// ThisObject is the current instance; ParentClass is the base class whose constructor runs.
type SuperConstructorCallInstrInput struct {
	ParentClass *zeus_value.Class
	ThisObject  zeus_value.Value
	Args        []zeus_value.Value
}

func NewSuperConstructorCallInstrInput(parentClass *zeus_value.Class, thisObject zeus_value.Value, args []zeus_value.Value) *SuperConstructorCallInstrInput {
	return &SuperConstructorCallInstrInput{ParentClass: parentClass, ThisObject: thisObject, Args: args}
}

func AsSuperConstructorCallInstrInput(input InstrInput) *SuperConstructorCallInstrInput {
	switch input := input.(type) {
	case *SuperConstructorCallInstrInput:
		return input
	default:
		panicInvalidInputType("SuperConstructorCallInstrInput", input)
	}

	return nil
}

func (i SuperConstructorCallInstrInput) String() string {
	args := []string{}
	for _, arg := range i.Args {
		args = append(args, arg.String())
	}
	return fmt.Sprintf("%s, [%s]", i.ParentClass.Name, strings.Join(args, ", "))
}

type GetAccessorInstrInput struct {
	Object       zeus_value.Value
	AccessorName string
}

func NewGetAccessorInstrInput(object zeus_value.Value, accessorName string) *GetAccessorInstrInput {
	return &GetAccessorInstrInput{Object: object, AccessorName: accessorName}
}

func AsGetAccessorInstrInput(input InstrInput) *GetAccessorInstrInput {
	switch input := input.(type) {
	case *GetAccessorInstrInput:
		return input
	default:
		panicInvalidInputType("GetAccessorInstrInput", input)
	}
	return nil
}

func (i GetAccessorInstrInput) String() string {
	return fmt.Sprintf("%s, %q", i.Object, i.AccessorName)
}

type SetAccessorInstrInput struct {
	Object       zeus_value.Value
	AccessorName string
	Value        zeus_value.Value
}

func NewSetAccessorInstrInput(object zeus_value.Value, accessorName string, value zeus_value.Value) *SetAccessorInstrInput {
	return &SetAccessorInstrInput{Object: object, AccessorName: accessorName, Value: value}
}

func AsSetAccessorInstrInput(input InstrInput) *SetAccessorInstrInput {
	switch input := input.(type) {
	case *SetAccessorInstrInput:
		return input
	default:
		panicInvalidInputType("SetAccessorInstrInput", input)
	}
	return nil
}

func (i SetAccessorInstrInput) String() string {
	return fmt.Sprintf("%s, %q, %s", i.Object, i.AccessorName, i.Value)
}

// InterfacePropGetInstrInput reads property PropName through an interface value (Object, typed as
// Iface). Codegen resolves the backing (field offset or accessor slot) via Iface's property itable.
type InterfacePropGetInstrInput struct {
	Object   zeus_value.Value
	Iface    *zeus_value.Interface
	PropName string
}

func NewInterfacePropGetInstrInput(object zeus_value.Value, iface *zeus_value.Interface, propName string) *InterfacePropGetInstrInput {
	return &InterfacePropGetInstrInput{Object: object, Iface: iface, PropName: propName}
}

func AsInterfacePropGetInstrInput(input InstrInput) *InterfacePropGetInstrInput {
	switch input := input.(type) {
	case *InterfacePropGetInstrInput:
		return input
	default:
		panicInvalidInputType("InterfacePropGetInstrInput", input)
	}
	return nil
}

func (i InterfacePropGetInstrInput) String() string {
	return fmt.Sprintf("%s, %s.%s", i.Object, i.Iface.Name, i.PropName)
}

// InterfacePropSetInstrInput writes Value into property PropName through an interface value.
type InterfacePropSetInstrInput struct {
	Object   zeus_value.Value
	Iface    *zeus_value.Interface
	PropName string
	Value    zeus_value.Value
}

func NewInterfacePropSetInstrInput(object zeus_value.Value, iface *zeus_value.Interface, propName string, value zeus_value.Value) *InterfacePropSetInstrInput {
	return &InterfacePropSetInstrInput{Object: object, Iface: iface, PropName: propName, Value: value}
}

func AsInterfacePropSetInstrInput(input InstrInput) *InterfacePropSetInstrInput {
	switch input := input.(type) {
	case *InterfacePropSetInstrInput:
		return input
	default:
		panicInvalidInputType("InterfacePropSetInstrInput", input)
	}
	return nil
}

func (i InterfacePropSetInstrInput) String() string {
	return fmt.Sprintf("%s.%s, %s", i.Iface.Name, i.PropName, i.Value)
}

type IndirectFuncCallInstrInput struct {
	Function zeus_value.Value
	Args     []zeus_value.Value
}

func NewIndirectFuncCallInstrInput(method zeus_value.Value, args []zeus_value.Value) *IndirectFuncCallInstrInput {
	return &IndirectFuncCallInstrInput{
		Function: method,
		Args:     args,
	}
}

func AsIndirectFuncCallInstrInput(input InstrInput) *IndirectFuncCallInstrInput {
	switch input := input.(type) {
	case *IndirectFuncCallInstrInput:
		return input
	default:
		panicInvalidInputType("IndirectFuncCallInstrInput", input)
	}

	return nil
}

func (i IndirectFuncCallInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.Function.String(), i.Args)
}

type DeclClassMethodInstrInput struct {
	Method *zeus_value.Function
	Body   *BasicBlock
	Class  *zeus_value.Class
}

func NewDeclClassMethodInstrInput(method *zeus_value.Function, body *BasicBlock, class *zeus_value.Class) *DeclClassMethodInstrInput {
	return &DeclClassMethodInstrInput{
		Method: method,
		Body:   body,
		Class:  class,
	}
}

func (i DeclClassMethodInstrInput) String() string {
	params := []string{}
	for _, param := range i.Method.Params {
		params = append(params, param.String())
	}
	return fmt.Sprintf("%s %s(%s)", i.Class.Name, i.Method.Name, strings.Join(params, ", "))
}

func AsDeclClassMethodInstrInput(input InstrInput) *DeclClassMethodInstrInput {
	switch input := input.(type) {
	case *DeclClassMethodInstrInput:
		return input
	default:
		panicInvalidInputType("DeclClassMethodInstrInput", input)
	}

	return nil
}

type GetIndexInstrInput struct {
	Array   zeus_value.Value
	Indices []zeus_value.Value
}

func (i GetIndexInstrInput) String() string {
	indices := []string{}
	for _, index := range i.Indices {
		indices = append(indices, fmt.Sprintf("[%s]", index.String()))
	}
	return fmt.Sprintf("%s%s", i.Array.String(), strings.Join(indices, ""))
}

func NewGetIndexInstrInput(array zeus_value.Value, indices []zeus_value.Value) *GetIndexInstrInput {
	return &GetIndexInstrInput{
		Array:   array,
		Indices: indices,
	}
}

func AsGetIndexInstrInput(input InstrInput) *GetIndexInstrInput {
	switch input := input.(type) {
	case *GetIndexInstrInput:
		return input
	default:
		panicInvalidInputType("GetIndexInstrInput", input)
	}

	return nil
}

type SetIndexInstrInput struct {
	Array zeus_value.Value
	Index zeus_value.Value
	Value zeus_value.Value
}

func (i SetIndexInstrInput) String() string {
	return fmt.Sprintf("%s[%s] = %s", i.Array.String(), i.Index.String(), i.Value.String())
}

func NewSetIndexInstrInput(array, index, value zeus_value.Value) *SetIndexInstrInput {
	return &SetIndexInstrInput{Array: array, Index: index, Value: value}
}

func AsSetIndexInstrInput(input InstrInput) *SetIndexInstrInput {
	switch input := input.(type) {
	case *SetIndexInstrInput:
		return input
	default:
		panicInvalidInputType("SetIndexInstrInput", input)
	}

	return nil
}

// ElemLoadInstrInput is a lowered, primitive array element read: load the element at
// Index from the raw element buffer Data (the array's opaque `data` pointer), yielding
// a value of ElemType. Emitted by IndexLoweringPass; codegen lowers it to a GEP + load.
type ElemLoadInstrInput struct {
	Data     zeus_value.Value
	Index    zeus_value.Value
	ElemType zeus_value.ValueType
}

func (i ElemLoadInstrInput) String() string {
	return fmt.Sprintf("%s[%s]", i.Data.String(), i.Index.String())
}

func NewElemLoadInstrInput(data, index zeus_value.Value, elemType zeus_value.ValueType) *ElemLoadInstrInput {
	return &ElemLoadInstrInput{Data: data, Index: index, ElemType: elemType}
}

func AsElemLoadInstrInput(input InstrInput) *ElemLoadInstrInput {
	switch input := input.(type) {
	case *ElemLoadInstrInput:
		return input
	default:
		panicInvalidInputType("ElemLoadInstrInput", input)
	}

	return nil
}

// ElemStoreInstrInput is a lowered, primitive array element write: store Value (of
// ElemType) at Index into the raw element buffer Data (the array's opaque `data`
// pointer). Emitted by IndexLoweringPass; codegen lowers it to a GEP + store.
type ElemStoreInstrInput struct {
	Data     zeus_value.Value
	Index    zeus_value.Value
	Value    zeus_value.Value
	ElemType zeus_value.ValueType
}

func (i ElemStoreInstrInput) String() string {
	return fmt.Sprintf("%s[%s] = %s", i.Data.String(), i.Index.String(), i.Value.String())
}

func NewElemStoreInstrInput(data, index, value zeus_value.Value, elemType zeus_value.ValueType) *ElemStoreInstrInput {
	return &ElemStoreInstrInput{Data: data, Index: index, Value: value, ElemType: elemType}
}

func AsElemStoreInstrInput(input InstrInput) *ElemStoreInstrInput {
	switch input := input.(type) {
	case *ElemStoreInstrInput:
		return input
	default:
		panicInvalidInputType("ElemStoreInstrInput", input)
	}

	return nil
}

type DeclPrimordialFuncInstrInput struct {
	Function *zeus_value.Function
}

func NewDeclPrimordialFuncInstrInput(function *zeus_value.Function) *DeclPrimordialFuncInstrInput {
	return &DeclPrimordialFuncInstrInput{
		Function: function,
	}
}

func (i DeclPrimordialFuncInstrInput) String() string {
	return i.Function.String()
}

func AsDeclPrimordialFuncInstrInput(input InstrInput) *DeclPrimordialFuncInstrInput {
	switch input := input.(type) {
	case *DeclPrimordialFuncInstrInput:
		return input
	default:
		panicInvalidInputType("DeclPrimordialFuncInstrInput", input)
	}

	return nil
}

// ThrowInstrInput is the input for THROW instruction
type ThrowInstrInput struct {
	ClassId    int              // Class ID of the exception type
	ObjectPtr  zeus_value.Value // Pointer to the Error object
	SourceFile string           // Source file where throw occurred
	SourceLine int              // Line number where throw occurred
}

func NewThrowInstrInput(classId int, objectPtr zeus_value.Value, sourceFile string, sourceLine int) *ThrowInstrInput {
	return &ThrowInstrInput{
		ClassId:    classId,
		ObjectPtr:  objectPtr,
		SourceFile: sourceFile,
		SourceLine: sourceLine,
	}
}

func (i ThrowInstrInput) String() string {
	return fmt.Sprintf("throw class_id=%d, obj=%s at %s:%d", i.ClassId, i.ObjectPtr, i.SourceFile, i.SourceLine)
}

func AsThrowInstrInput(input InstrInput) *ThrowInstrInput {
	switch input := input.(type) {
	case *ThrowInstrInput:
		return input
	default:
		panicInvalidInputType("ThrowInstrInput", input)
	}
	return nil
}

// PushHandlerInstrInput is the input for PUSH_HANDLER instruction
type PushHandlerInstrInput struct {
	HandlerBlock *BasicBlock // The block to jump to when exception is caught
	TryBodyBlock *BasicBlock // The block containing the try body
	ClassIds     []int       // Class IDs this handler catches (in order of catch clauses)
}

func NewPushHandlerInstrInput(handlerBlock *BasicBlock, tryBodyBlock *BasicBlock, classIds []int) *PushHandlerInstrInput {
	return &PushHandlerInstrInput{
		HandlerBlock: handlerBlock,
		TryBodyBlock: tryBodyBlock,
		ClassIds:     classIds,
	}
}

func (i PushHandlerInstrInput) String() string {
	return fmt.Sprintf("push_handler handler=%d, try_body=%d, class_ids=%v", i.HandlerBlock.Id, i.TryBodyBlock.Id, i.ClassIds)
}

func AsPushHandlerInstrInput(input InstrInput) *PushHandlerInstrInput {
	switch input := input.(type) {
	case *PushHandlerInstrInput:
		return input
	default:
		panicInvalidInputType("PushHandlerInstrInput", input)
	}
	return nil
}

// CheckExceptionInstrInput is the input for CHECK_EXCEPTION instruction
type CheckExceptionInstrInput struct {
	HandlerBlock  *BasicBlock // Block to jump to if exception is pending
	ContinueBlock *BasicBlock // Block to jump to if no exception
}

func NewCheckExceptionInstrInput(handlerBlock *BasicBlock, continueBlock *BasicBlock) *CheckExceptionInstrInput {
	return &CheckExceptionInstrInput{
		HandlerBlock:  handlerBlock,
		ContinueBlock: continueBlock,
	}
}

func (i CheckExceptionInstrInput) String() string {
	return fmt.Sprintf("check_exception handler=%d, continue=%d", i.HandlerBlock.Id, i.ContinueBlock.Id)
}

func AsCheckExceptionInstrInput(input InstrInput) *CheckExceptionInstrInput {
	switch input := input.(type) {
	case *CheckExceptionInstrInput:
		return input
	default:
		panicInvalidInputType("CheckExceptionInstrInput", input)
	}
	return nil
}

const (
	// math operations
	InstrTypeAdd InstrType = iota
	InstrTypeSub
	InstrTypeMul
	InstrTypeDiv
	InstrTypeMod   // modulo
	InstrTypePower // exponentiation (**)
	// casting
	InstrTypeCast
	// runtime downcast type test (object `as` cast): output bool = obj is-a target class id
	InstrTypeInstanceOf
	// comparison operations
	InstrTypeNeg
	InstrTypeEqEq
	InstrTypeNotEq
	InstrTypeLessThan
	InstrTypeLessThanEq
	InstrTypeGreaterThan
	InstrTypeGreaterThanEq
	// logical operations
	InstrTypeNot
	InstrTypeAnd // logical AND (&&)
	InstrTypeOr  // logical OR (||)
	// bitwise operations
	InstrTypeBitAnd // &
	InstrTypeBitOr  // |
	InstrTypeBitXor // ^
	InstrTypeBitNot // ~ (unary)
	InstrTypeShl    // <<
	InstrTypeShr    // >> (arithmetic for signed, logical for unsigned)
	// variable declaration
	InstrTypeDeclVar
	// module-level global variable declaration (uses AddGlobal in codegen)
	InstrTypeDeclGlobalVar
	// mem management
	InstrTypeLoad
	InstrTypeStore
	// function
	InstrTypeDeclFunc
	InstrTypeDeclPrimordialFunc
	InstrTypeCallFunc
	InstrTypeIndirectFuncCall
	// CALL_MODULE_INIT invokes a module's per-module init function by its stable external
	// symbol name (module.ModuleInitFuncName). Emitted only into the entry point's dispatcher;
	// codegen declares the symbol extern if the definition lives in another module.
	InstrTypeCallModuleInit
	InstrTypeReturn
	// control flow
	InstrTypeJmp
	InstrTypeCondJmp
	// import and export
	InstrTypeImport
	InstrTypeExport
	// class
	InstrTypeDeclClass
	InstrTypeDeclClassMethod
	InstrTypeNewObj
	// allocate a zeroed object of a class + install its header (mechanism half of NEW_OBJ,
	// synthesized by FactoryLoweringPass into a class's zeus_new_<Class> factory function)
	InstrTypeAllocObj
	// autobox a scalar into its boxed primordial (Number/Bool). Emitted by the type checker at
	// object boundaries; BoxLoweringPass rewrites it to ALLOC_OBJ + store of the scalar into the
	// box's `value` field. The scalar is already the box's field type (int/float pre-cast to f64).
	InstrTypeBox
	// unbox a Number/Bool back to its scalar `value`. Emitted by the type checker when a box flows
	// into a primitive slot or an arithmetic operand; BoxLoweringPass rewrites it to
	// OBJECT_PROPERTY_ACCESS + LOAD. Output is the box's field type (f64/boolean).
	InstrTypeUnbox
	// a template literal `a ${x} b` kept as one node (static chunks + interpolated values) through
	// type checking so interpolation errors are template-specific; StringTemplateLoweringPass rewrites
	// it to the `+`/concat chain. Output is always `string`.
	InstrTypeStringTemplate
	// convert a reference value (object/array/interface/function/null) to its `string` debug
	// representation via the runtime reflection printer (zeus_reflect_to_string), which walks the
	// value's emitted type-info metadata. Emitted by the type checker's emitToString when the value
	// has no user-defined toString; codegen calls the runtime fn directly (no lowering pass).
	InstrTypeReflectToString
	// object property access
	InstrTypeObjectPropertyAccess
	// object method call (explicit receiver + method name + args)
	InstrTypeMethodCall
	// super(...) — direct (non-virtual) call to the base class constructor with `this`
	InstrTypeSuperConstructorCall
	// accessor invocation (HIR - lowered before codegen)
	InstrTypeGetAccessor // read via getter: obj.name
	InstrTypeSetAccessor // write via setter: obj.name = value
	// interface property access (LIR only). Emitted by InterfacePropertyLoweringPass from an
	// OBJECT_PROPERTY_ACCESS on an interface receiver plus its consuming LOAD/STORE. Codegen
	// dispatches each through the interface's tagged property itable (field offset OR accessor
	// vtable slot), so the read/write shape — not a plain lvalue pointer — is explicit in the IR.
	InstrTypeInterfacePropGet // read a property through an interface value (value-producing)
	InstrTypeInterfacePropSet // write a property through an interface value (value-consuming)
	// array indexing (HIR - lowered before codegen)
	InstrTypeGetIndex
	// array element assignment (HIR - lowered before codegen)
	InstrTypeSetIndex
	// exception handling
	InstrTypeThrow          // Throw exception: calls zeus_throw(class_id, obj_ptr)
	InstrTypePushHandler    // Push catch handler onto handler stack at try entry
	InstrTypePopHandler     // Pop handler from stack at try exit
	InstrTypeCheckException // Check if exception pending after call in try block
	InstrTypeGetException   // Get current exception object for catch binding
	InstrTypeClearException // Clear exception after successful catch
	// type coercion: ObjectType value whose __call__ is compatible with the target FunctionType;
	// output has the source ObjectType so FunctorCallLoweringPass handles downstream calls naturally.
	InstrTypeCoerce
	// primitive array element read: value = data[index] (LIR only). Emitted by
	// IndexLoweringPass for primitive-element arrays to skip the get() method call,
	// vtable dispatch and runtime round-trip; codegen lowers it to a GEP + load.
	InstrTypeElemLoad
	// primitive array element write: data[index] = value (LIR only). Emitted by
	// IndexLoweringPass on the in-bounds branch of a primitive-element array write;
	// codegen lowers it to a GEP + store.
	InstrTypeElemStore
)

func (i InstrType) String() string {
	switch i {
	case InstrTypeAdd:
		return "ADD"
	case InstrTypeSub:
		return "SUB"
	case InstrTypeMul:
		return "MUL"
	case InstrTypeDiv:
		return "DIV"
	case InstrTypeMod:
		return "MOD"
	case InstrTypePower:
		return "POWER"
	case InstrTypeNeg:
		return "NEG"
	case InstrTypeEqEq:
		return "EQ_EQ"
	case InstrTypeNotEq:
		return "NOT_EQ"
	case InstrTypeLessThan:
		return "LESS_THAN"
	case InstrTypeLessThanEq:
		return "LESS_THAN_EQ"
	case InstrTypeGreaterThan:
		return "GREATER_THAN"
	case InstrTypeGreaterThanEq:
		return "GREATER_THAN_EQ"
	case InstrTypeNot:
		return "NOT"
	case InstrTypeAnd:
		return "AND"
	case InstrTypeOr:
		return "OR"
	case InstrTypeBitAnd:
		return "BIT_AND"
	case InstrTypeBitOr:
		return "BIT_OR"
	case InstrTypeBitXor:
		return "BIT_XOR"
	case InstrTypeBitNot:
		return "BIT_NOT"
	case InstrTypeShl:
		return "SHL"
	case InstrTypeShr:
		return "SHR"
	case InstrTypeDeclVar:
		return "DECLARE_VAR"
	case InstrTypeDeclGlobalVar:
		return "DECLARE_GLOBAL_VAR"
	case InstrTypeDeclFunc:
		return "DECLARE_FUNC"
	case InstrTypeDeclPrimordialFunc:
		return "DECLARE_PRIMORDIAL_FUNC"
	case InstrTypeCallFunc:
		return "CALL_FUNC"
	case InstrTypeCallModuleInit:
		return "CALL_MODULE_INIT"
	case InstrTypeReturn:
		return "RETURN"
	case InstrTypeJmp:
		return "JMP"
	case InstrTypeCondJmp:
		return "COND_JMP"
	case InstrTypeLoad:
		return "LOAD"
	case InstrTypeStore:
		return "STORE"
	case InstrTypeCast:
		return "CAST"
	case InstrTypeInstanceOf:
		return "INSTANCEOF"
	case InstrTypeImport:
		return "IMPORT"
	case InstrTypeExport:
		return "EXPORT"
	case InstrTypeDeclClass:
		return "DECLARE_CLASS"
	case InstrTypeNewObj:
		return "NEW_OBJ"
	case InstrTypeAllocObj:
		return "ALLOC_OBJ"
	case InstrTypeBox:
		return "BOX"
	case InstrTypeUnbox:
		return "UNBOX"
	case InstrTypeStringTemplate:
		return "STRING_TEMPLATE"
	case InstrTypeReflectToString:
		return "REFLECT_TO_STRING"
	case InstrTypeIndirectFuncCall:
		return "CALL_INDIRECT_FUNC"
	case InstrTypeObjectPropertyAccess:
		return "OBJECT_PROPERTY_ACCESS"
	case InstrTypeMethodCall:
		return "CALL_METHOD"
	case InstrTypeSuperConstructorCall:
		return "CALL_SUPER_CONSTRUCTOR"
	case InstrTypeDeclClassMethod:
		return "DECLARE_CLASS_METHOD"
	case InstrTypeGetAccessor:
		return "GET_ACCESSOR"
	case InstrTypeSetAccessor:
		return "SET_ACCESSOR"
	case InstrTypeInterfacePropGet:
		return "INTERFACE_PROP_GET"
	case InstrTypeInterfacePropSet:
		return "INTERFACE_PROP_SET"
	case InstrTypeGetIndex:
		return "GET_INDEX"
	case InstrTypeSetIndex:
		return "SET_INDEX"
	case InstrTypeElemLoad:
		return "ELEM_LOAD"
	case InstrTypeElemStore:
		return "ELEM_STORE"
	case InstrTypeThrow:
		return "THROW"
	case InstrTypePushHandler:
		return "PUSH_HANDLER"
	case InstrTypePopHandler:
		return "POP_HANDLER"
	case InstrTypeCheckException:
		return "CHECK_EXCEPTION"
	case InstrTypeGetException:
		return "GET_EXCEPTION"
	case InstrTypeClearException:
		return "CLEAR_EXCEPTION"
	case InstrTypeCoerce:
		return "COERCE"
	default:
		panic("unknown instruction type")
	}
}

func IsControlFlowInstr(instrType InstrType) bool {
	return instrType == InstrTypeJmp || instrType == InstrTypeCondJmp || instrType == InstrTypeReturn || instrType == InstrTypeThrow || instrType == InstrTypeCheckException || instrType == InstrTypePushHandler
}

func IsFunctionDeclInstr(instrType InstrType) bool {
	return instrType == InstrTypeDeclFunc
}

func IsPrimordialFunctionDeclInstr(instrType InstrType) bool {
	return instrType == InstrTypeDeclPrimordialFunc
}

func IsClassMethodDeclInstr(instrType InstrType) bool {
	return instrType == InstrTypeDeclClassMethod
}

func IsClassDeclInstr(instrType InstrType) bool {
	return instrType == InstrTypeDeclClass
}

func IsExportInstr(instrType InstrType) bool {
	return instrType == InstrTypeExport
}

func IsImportInstr(instrType InstrType) bool {
	return instrType == InstrTypeImport
}

type Instr struct {
	Id     int
	Type   InstrType
	Output *zeus_value.Var
	Input  InstrInput
	Span   *token.Span
}

func (i *Instr) String() string {
	if i.Output != nil && i.Input != nil {
		return fmt.Sprintf("%s = %s %s", i.Output, i.Type, i.Input)
	} else if i.Input != nil {
		return fmt.Sprintf("%s %s", i.Type, i.Input)
	} else if i.Output != nil {
		return fmt.Sprintf("%s = %s", i.Output, i.Type)
	} else {
		return i.Type.String()
	}
}

type BasicBlock struct {
	Id         int
	Instrs     []*Instr
	Successors []*BasicBlock
}

func NewBasicBlock(id int) *BasicBlock {
	return &BasicBlock{
		Id:         id,
		Instrs:     []*Instr{},
		Successors: []*BasicBlock{},
	}
}
