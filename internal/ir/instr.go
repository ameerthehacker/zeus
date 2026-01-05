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
	Indices   []zeus_value.Value
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

const (
	// math operations
	InstrTypeAdd InstrType = iota
	InstrTypeSub
	InstrTypeMul
	InstrTypeDiv
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
	// variable declaration
	InstrTypeDeclVar
	// mem management
	InstrTypeLoad
	InstrTypeStore
	// function
	InstrTypeDeclFunc
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
	case InstrTypeNeg:
		return "NEQ"
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
	case InstrTypeDeclVar:
		return "DECLARE_VAR"
	case InstrTypeDeclFunc:
		return "DECLARE_FUNC"
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
	case InstrTypeDeclClassMethod:
		return "DECLARE_CLASS_METHOD"
	default:
		panic("unknown instruction type")
	}
}

func IsControlFlowInstr(instrType InstrType) bool {
	return instrType == InstrTypeJmp || instrType == InstrTypeCondJmp || instrType == InstrTypeReturn
}

func IsFunctionDeclInstr(instrType InstrType) bool {
	return instrType == InstrTypeDeclFunc
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
