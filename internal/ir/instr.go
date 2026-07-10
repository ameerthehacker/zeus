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
		properties = append(properties, fmt.Sprintf("%s: %s", property.Property.Name, property.Property.ValueType))
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
}

func NewMethodCallInstrInput(object zeus_value.Value, methodName string, args []zeus_value.Value) *MethodCallInstrInput {
	return &MethodCallInstrInput{
		Object:     object,
		MethodName: methodName,
		Args:       args,
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
	// object property access
	InstrTypeObjectPropertyAccess
	// object method call (explicit receiver + method name + args)
	InstrTypeMethodCall
	// accessor invocation (HIR - lowered before codegen)
	InstrTypeGetAccessor // read via getter: obj.name
	InstrTypeSetAccessor // write via setter: obj.name = value
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
	case InstrTypeImport:
		return "IMPORT"
	case InstrTypeExport:
		return "EXPORT"
	case InstrTypeDeclClass:
		return "DECLARE_CLASS"
	case InstrTypeNewObj:
		return "NEW_OBJ"
	case InstrTypeIndirectFuncCall:
		return "CALL_INDIRECT_FUNC"
	case InstrTypeObjectPropertyAccess:
		return "OBJECT_PROPERTY_ACCESS"
	case InstrTypeMethodCall:
		return "CALL_METHOD"
	case InstrTypeDeclClassMethod:
		return "DECLARE_CLASS_METHOD"
	case InstrTypeGetAccessor:
		return "GET_ACCESSOR"
	case InstrTypeSetAccessor:
		return "SET_ACCESSOR"
	case InstrTypeGetIndex:
		return "GET_INDEX"
	case InstrTypeSetIndex:
		return "SET_INDEX"
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
