package ir

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/token"
)

type InstrType int

type Value interface {
	String() string
}

type InstrInput interface {}

type TempVariable struct {
	Name string
}

func (t *TempVariable) String() string {
	return t.Name
}

func panicInvalidInputType(expected string, actual InstrInput) {
	panic(fmt.Sprintf("invalid input type: %s, but found: %s", expected, actual))
}

type BinaryOpInstrInput struct {
	Left Value
	Right Value
}

func (i *BinaryOpInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.Left, i.Right)
}

func ToBinaryOpInstrInput(input InstrInput) *BinaryOpInstrInput {
	switch input := input.(type) {
	case *BinaryOpInstrInput:
		return input
	default:
		panicInvalidInputType("BinaryOpInstrInput", input)
	}

	return nil
}

type UnaryOpInstrInput struct {
	Value Value
}

func (i *UnaryOpInstrInput) String() string {
	return i.Value.String()
}

func ToUnaryOpInstrInput(input InstrInput) *UnaryOpInstrInput {
	switch input := input.(type) {
	case *UnaryOpInstrInput:
		return input
	default:
		panicInvalidInputType("UnaryOpInstrInput", input)
	}

	return nil
}

type DeclareVarInstrInput struct {
	Name string
	ValueType ValueType
}

func (i *DeclareVarInstrInput) String() string {
	return fmt.Sprintf("%s %s", i.ValueType, i.Name)
}

func ToDeclareVarInstrInput(input InstrInput) *DeclareVarInstrInput {
	switch input := input.(type) {
	case *DeclareVarInstrInput:
		return input
	default:
		panicInvalidInputType("DeclareVarInstrInput", input)
	}

	return nil
}

type LoadInstrInput struct {
	Addr string
}

func (i *LoadInstrInput) String() string {
	return i.Addr
}

func ToLoadInstrInput(input InstrInput) *LoadInstrInput {
	switch input := input.(type) {
	case *LoadInstrInput:
		return input
	default:
		panicInvalidInputType("LoadInstrInput", input)
	}

	return nil
}

type StoreInstrInput struct {
	Addr string
	Value Value
}

func (i *StoreInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.Addr, i.Value)
}

func ToStoreInstrInput(input InstrInput) *StoreInstrInput {
	switch input := input.(type) {
	case *StoreInstrInput:
		return input
	default:
		panicInvalidInputType("StoreInstrInput", input)
	}

	return nil
}

type CallFuncInstrInput struct {
	FuncName string
	Args []Value
}

func (i *CallFuncInstrInput) String() string {
	args := []string{}
	for _, arg := range i.Args {
		args = append(args, arg.String())
	}
	return fmt.Sprintf("%s(%s)", i.FuncName, strings.Join(args, ", "))
}

func ToCallFuncInstrInput(input InstrInput) *CallFuncInstrInput {
	switch input := input.(type) {
	case *CallFuncInstrInput:
		return input
	default:
		panicInvalidInputType("CallFuncInstrInput", input)
	}

	return nil
}

type ReturnInstrInput struct {
	Value Value
}

func (i *ReturnInstrInput) String() string {
	return i.Value.String()
}

func ToReturnInstrInput(input InstrInput) *ReturnInstrInput {
	switch input := input.(type) {
	case *ReturnInstrInput:
		return input
	default:
		panicInvalidInputType("ReturnInstrInput", input)
	}

	return nil
}

type DeclFuncInstrInput struct {
	Name string
	Args []Variable
	Body *BasicBlock
	ReturnType ValueType
}

func toDeclFuncInstrInput(input InstrInput) *DeclFuncInstrInput {
	switch input := input.(type) {
	case *DeclFuncInstrInput:
		return input
	default:
		panicInvalidInputType("DeclFuncInstrInput", input)
	}

	return nil
}

func (i *DeclFuncInstrInput) String() string {
	args := []string{}
	for _, arg := range i.Args {
		args = append(args, fmt.Sprintf("%s %s", arg.ValueType, arg.Name))
	}

	return fmt.Sprintf("%s(%s) %s", i.Name, strings.Join(args, ", "), i.ReturnType)
}

type JmpInstrInput struct {
	Target *BasicBlock
}

func (i *JmpInstrInput) String() string {
	return fmt.Sprintf("%d", i.Target.Id)
}

func toJmpInstrInput(input InstrInput) *JmpInstrInput {
	switch input := input.(type) {
	case *JmpInstrInput:
		return input
	default:
		panicInvalidInputType("JmpInstrInput", input)
	}

	return nil
}

type CondJmpInstrInput struct {
	Target *BasicBlock
	Condition Value
}

func (i *CondJmpInstrInput) String() string {
	return fmt.Sprintf("%d, %s", i.Target.Id, i.Condition)
}

func toCondJmpInstrInput(input InstrInput) *CondJmpInstrInput {
	switch input := input.(type) {
	case *CondJmpInstrInput:
		return input
	default:
		panicInvalidInputType("CondJmpInstrInput", input)
	}

	return nil
}

type Constant struct {
	Value string
	ValueType ValueType
	Span *token.Span
}

func (i *Constant) String() string {
	return fmt.Sprintf("%s %s", i.ValueType, i.Value)
}

type Variable struct {
	Name string
	ValueType ValueType
	Span *token.Span
}

func (v *Variable) String() string {
	return fmt.Sprintf("%s %s", v.ValueType, v.Name)
}

const (
	InstrTypeAdd InstrType = iota
	// math operations
	InstrTypeSub
	InstrTypeMul
	InstrTypeDiv
	InstrTypeNeq
	InstrTypeNotEq
	InstrTypeLessThan
	InstrTypeLessThanEq
	InstrTypeGreaterThan
	InstrTypeGreaterThanEq
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
	case InstrTypeNeq:
		return "NEQ"
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
	default:
		panic("unknown instruction type")
	}
}

type Instr struct {
	Type InstrType
	Output Value
	Input InstrInput
	Span   *token.Span
}

func (i *Instr) String() string {
	if i.Output != nil && i.Input != nil {
		return fmt.Sprintf("%s %s = %s", i.Type, i.Output, i.Input)
	} else if i.Input != nil {
		return fmt.Sprintf("%s %s", i.Type, i.Input)
	} else if i.Output != nil {
		return fmt.Sprintf("%s %s", i.Type, i.Output)
	} else {
		return i.Type.String()
	}
}

type BasicBlock struct {
	Id int
	Instrs []*Instr
	Parent *BasicBlock
	tempVariablesCount int
}

func NewBasicBlock(id int, parent *BasicBlock) *BasicBlock {
	return &BasicBlock{
		Id: id,
		Instrs: []*Instr{},
		Parent: parent,
		tempVariablesCount: 0,
	}
}

func (b *BasicBlock) CreateTempVariable() *TempVariable {
	temp_variable_name := "%" + strconv.Itoa(b.tempVariablesCount)
	b.tempVariablesCount++

	return &TempVariable{
		Name: temp_variable_name,
	}
}

type IRGen struct {
	ir_builder *IRBuilder
}

func NewIRGen() *IRGen {
	return &IRGen{
		ir_builder: NewIRBuilder(),
	}
}

func (g *IRGen) Generate(program *ast.ProgramNode) {
	for _, stmt := range program.Statements {
		stmt.Accept(g)
	}
}

func (g *IRGen) VisitBlockStmt(stmt *ast.BlockStmtNode) any {
	block := g.ir_builder.BuildBasicBlock()
	g.ir_builder.SetInsertionBlock(block)

	for _, stmt := range stmt.Statements {
		stmt.Accept(g)
	}

	g.ir_builder.EndBasicBlock()

	return nil
}

func (g *IRGen) VisitVarDeclStmt(stmt *ast.VarDeclStmtNode) any {
	

	return nil
}

func (g *IRGen) VisitExprStmt(stmt *ast.ExprStmtNode) any {
	return stmt.Expr.Accept(g)	
}

func (g *IRGen) VisitReturnStmt(stmt *ast.ReturnStmtNode) any {
	return nil	
}

func (g *IRGen) VisitIfStmt(stmt *ast.IfStmtNode) any {
	return nil
}

func (g *IRGen) VisitBinaryExpr(expr *ast.BinaryExprNode) any {
	return nil
}

func (g *IRGen) VisitGroupingExpr(expr *ast.GroupingExprNode) any {
	return nil
}

func (g *IRGen) VisitNumberExpr(expr *ast.NumberExprNode) any {
	return nil
}

func (g *IRGen) VisitFunctionCallExpr(expr *ast.FunctionCallExprNode) any {
	return nil
}

func (g *IRGen) VisitFunctionDeclExpr(expr *ast.FunctionDeclExprNode) any {
	return nil
}

func (g *IRGen) VisitIdentifier(expr *ast.IdentifierExprNode) any {
	return nil
}

func (g *IRGen) VisitNumber(expr *ast.NumberExprNode) any {
	return nil
}

func (g *IRGen) VisitUnaryExpr(expr *ast.UnaryExprNode) any {
	return nil
}
