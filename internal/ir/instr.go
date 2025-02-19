package ir

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/value"
)

type InstrType int

type InstrInput interface {
	String() string
}

type VarDecl struct {
	Name string
	ValueType value.ValueType
	IsConst bool
	Initializer value.Value
	Span *token.Span
}

func (v VarDecl) String() string {
	return fmt.Sprintf("%s %s", v.ValueType, v.Name)
}

func panicInvalidInputType(expected string, actual InstrInput) {
	panic(fmt.Sprintf("invalid input type: %s, but found: %+v", expected, actual))
}

type BinaryOpInstrInput struct {
	Left value.Value
	Right value.Value
}

func (i BinaryOpInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.Left, i.Right)
}

func AsBinaryOpInstrInput(input InstrInput) *BinaryOpInstrInput {
	switch input := input.(type) {
	case BinaryOpInstrInput:
		return &input
	default:
		panicInvalidInputType("BinaryOpInstrInput", input)
	}

	return nil
}

type UnaryOpInstrInput struct {
	Value value.Value
}

func (i UnaryOpInstrInput) String() string {
	return i.Value.String()
}

func AsUnaryOpInstrInput(input InstrInput) *UnaryOpInstrInput {
	switch input := input.(type) {
	case UnaryOpInstrInput:
		return &input
	default:
		panicInvalidInputType("UnaryOpInstrInput", input)
	}

	return nil
}

type DeclareVarInstrInput struct {
	Variable *value.Var
	Initializer value.Value
	IsConst bool
}

func (i DeclareVarInstrInput) String() string {
	if i.Initializer != nil {
		return fmt.Sprintf("%s %s = %s", i.Variable.ValueType, i.Variable.Name, i.Initializer)
	}
	return fmt.Sprintf("%s %s", i.Variable.ValueType, i.Variable.Name)
}

func AsDeclVarInstrInput(input InstrInput) *DeclareVarInstrInput {
	switch input := input.(type) {
	case DeclareVarInstrInput:
		return &input
	default:
		panicInvalidInputType("DeclareVarInstrInput", input)
	}

	return nil
}

type LoadInstrInput struct {
	Addr *value.Var
}

func (i LoadInstrInput) String() string {
	return i.Addr.String()
}

func AsLoadInstrInput(input InstrInput) *LoadInstrInput {
	switch input := input.(type) {
	case LoadInstrInput:
		return &input
	default:
		panicInvalidInputType("LoadInstrInput", input)
	}

	return nil
}

type StoreInstrInput struct {
	Addr *value.Var
	Value value.Value
}

func (i StoreInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.Addr, i.Value)
}

func AsStoreInstrInput(input InstrInput) *StoreInstrInput {
	switch input := input.(type) {
	case StoreInstrInput:
		return &input
	default:
		panicInvalidInputType("StoreInstrInput", input)
	}

	return nil
}

type CallFuncInstrInput struct {
	Callee value.Value
	Args []value.Value
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
	case CallFuncInstrInput:
		return &input
	default:
		panicInvalidInputType("CallFuncInstrInput", input)
	}

	return nil
}

type ReturnInstrInput struct {
	Value value.Value
}

func (i ReturnInstrInput) String() string {
	if i.Value != nil {
		return i.Value.String()
	}
	return "void"
}

func AsReturnInstrInput(input InstrInput) *ReturnInstrInput {
	switch input := input.(type) {
	case ReturnInstrInput:
		return &input
	default:
		panicInvalidInputType("ReturnInstrInput", input)
	}

	return nil
}

type DeclFuncInstrInput struct {
	Function value.Function
	Body *BasicBlock
}

func AsDeclFuncInstrInput(input InstrInput) *DeclFuncInstrInput {
	switch input := input.(type) {
	case DeclFuncInstrInput:
		return &input
	default:
		panicInvalidInputType("DeclFuncInstrInput", input)
	}

	return nil
}

func (i DeclFuncInstrInput) String() string {
	return i.Function.String()
}

type JmpInstrInput struct {
	Target *BasicBlock
}

func (i JmpInstrInput) String() string {
	return fmt.Sprintf("%d", i.Target.Id)
}

func AsJmpInstrInput(input InstrInput) *JmpInstrInput {
	switch input := input.(type) {
	case JmpInstrInput:
		return &input
	default:
		panicInvalidInputType("JmpInstrInput", input)
	}

	return nil
}

type CondJmpInstrInput struct {
	TrueTarget *BasicBlock
	FalseTarget *BasicBlock
	Condition value.Value
}

func (i CondJmpInstrInput) String() string {
	return fmt.Sprintf("%s, %d, %d", i.Condition, i.TrueTarget.Id, i.FalseTarget.Id)
}

func AsCondJmpInstrInput(input InstrInput) *CondJmpInstrInput {
	switch input := input.(type) {
	case CondJmpInstrInput:
		return &input
	default:
		panicInvalidInputType("CondJmpInstrInput", input)
	}

	return nil
}

const (
	InstrTypeAdd InstrType = iota
	// math operations
	InstrTypeSub
	InstrTypeMul
	InstrTypeDiv
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
	InstrTypeReturn
	// control flow
	InstrTypeJmp
	InstrTypeCondJmp
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
	default:
		panic("unknown instruction type")
	}
}

type Instr struct {
	Type InstrType
	Output *value.Var
	Input InstrInput
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
	Id int
	Instrs []*Instr
	Successors []*BasicBlock
}

func NewBasicBlock(id int) *BasicBlock {
	return &BasicBlock{
		Id: id,
		Instrs: []*Instr{},
		Successors: []*BasicBlock{},
	}
}
