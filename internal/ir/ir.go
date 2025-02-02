package ir

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/constant"
	"github.com/ameerthehacker/zeus/internal/token"
)

type InstrType int

type InstrInput interface {
	String() string
}

type TempVariable struct {
	Name string
}

func (t *TempVariable) String() string {
	return t.Name
}

func panicInvalidInputType(expected string, actual InstrInput) {
	panic(fmt.Sprintf("invalid input type: %s, but found: %+v", expected, actual))
}

type BinaryOpInstrInput struct {
	Left constant.Value
	Right constant.Value
}

func (i BinaryOpInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.Left, i.Right)
}

func ToBinaryOpInstrInput(input InstrInput) *BinaryOpInstrInput {
	switch input := input.(type) {
	case BinaryOpInstrInput:
		return &input
	default:
		panicInvalidInputType("BinaryOpInstrInput", input)
	}

	return nil
}

type UnaryOpInstrInput struct {
	Value constant.Value
}

func (i UnaryOpInstrInput) String() string {
	return i.Value.String()
}

func ToUnaryOpInstrInput(input InstrInput) *UnaryOpInstrInput {
	switch input := input.(type) {
	case UnaryOpInstrInput:
		return &input
	default:
		panicInvalidInputType("UnaryOpInstrInput", input)
	}

	return nil
}

type DeclareVarInstrInput struct {
	Name string
	ValueType constant.ValueType
	IsConst bool
}

func (i DeclareVarInstrInput) String() string {
	return fmt.Sprintf("%s %s", i.ValueType, i.Name)
}

func ToDeclareVarInstrInput(input InstrInput) *DeclareVarInstrInput {
	switch input := input.(type) {
	case DeclareVarInstrInput:
		return &input
	default:
		panicInvalidInputType("DeclareVarInstrInput", input)
	}

	return nil
}

type LoadInstrInput struct {
	Addr string
}

func (i LoadInstrInput) String() string {
	return i.Addr
}

func ToLoadInstrInput(input InstrInput) *LoadInstrInput {
	switch input := input.(type) {
	case LoadInstrInput:
		return &input
	default:
		panicInvalidInputType("LoadInstrInput", input)
	}

	return nil
}

type StoreInstrInput struct {
	Addr string
	Value constant.Value
}

func (i StoreInstrInput) String() string {
	return fmt.Sprintf("%s, %s", i.Addr, i.Value)
}

func ToStoreInstrInput(input InstrInput) *StoreInstrInput {
	switch input := input.(type) {
	case StoreInstrInput:
		return &input
	default:
		panicInvalidInputType("StoreInstrInput", input)
	}

	return nil
}

type CallFuncInstrInput struct {
	Callee constant.Value
	Args []constant.Value
}

func (i CallFuncInstrInput) String() string {
	args := []string{}
	for _, arg := range i.Args {
		args = append(args, arg.String())
	}
	return fmt.Sprintf("%s(%s)", i.Callee, strings.Join(args, ", "))
}

func ToCallFuncInstrInput(input InstrInput) *CallFuncInstrInput {
	switch input := input.(type) {
	case CallFuncInstrInput:
		return &input
	default:
		panicInvalidInputType("CallFuncInstrInput", input)
	}

	return nil
}

type ReturnInstrInput struct {
	Value constant.Value
}

func (i ReturnInstrInput) String() string {
	return i.Value.String()
}

func ToReturnInstrInput(input InstrInput) *ReturnInstrInput {
	switch input := input.(type) {
	case ReturnInstrInput:
		return &input
	default:
		panicInvalidInputType("ReturnInstrInput", input)
	}

	return nil
}

type DeclFuncInstrInput struct {
	Name string
	Args []Variable
	Body *BasicBlock
	ReturnType constant.ValueType
}

func ToDeclFuncInstrInput(input InstrInput) *DeclFuncInstrInput {
	switch input := input.(type) {
	case DeclFuncInstrInput:
		return &input
	default:
		panicInvalidInputType("DeclFuncInstrInput", input)
	}

	return nil
}

func (i DeclFuncInstrInput) String() string {
	args := []string{}
	for _, arg := range i.Args {
		args = append(args, fmt.Sprintf("%s %s", arg.ValueType, arg.Name))
	}

	return fmt.Sprintf("%s(%s) %s", i.Name, strings.Join(args, ", "), i.ReturnType)
}

type JmpInstrInput struct {
	Target *BasicBlock
}

func (i JmpInstrInput) String() string {
	return fmt.Sprintf("%d", i.Target.Id)
}

func ToJmpInstrInput(input InstrInput) *JmpInstrInput {
	switch input := input.(type) {
	case JmpInstrInput:
		return &input
	default:
		panicInvalidInputType("JmpInstrInput", input)
	}

	return nil
}

type CondJmpInstrInput struct {
	Target *BasicBlock
	Condition constant.Value
}

func (i CondJmpInstrInput) String() string {
	return fmt.Sprintf("%d, %s", i.Target.Id, i.Condition)
}

func ToCondJmpInstrInput(input InstrInput) *CondJmpInstrInput {
	switch input := input.(type) {
	case CondJmpInstrInput:
		return &input
	default:
		panicInvalidInputType("CondJmpInstrInput", input)
	}

	return nil
}

type Constant struct {
	Value string
	ValueType constant.ValueType
	Span *token.Span
}

func (i Constant) String() string {
	return fmt.Sprintf("%s %s", i.ValueType, i.Value)
}

type Variable struct {
	Name string
	ValueType constant.ValueType
	IsConst bool
	Initializer constant.Value
	Span *token.Span
}

func (v Variable) String() string {
	return fmt.Sprintf("%s %s", v.ValueType, v.Name)
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
	// scoping
	InstrTypeEnterScope
	InstrTypeExitScope
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
	case InstrTypeEnterScope:
		return "ENTER_SCOPE"
	case InstrTypeExitScope:
		return "EXIT_SCOPE"
	default:
		panic("unknown instruction type")
	}
}

type Instr struct {
	Type InstrType
	Output constant.Value
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
	Parent *BasicBlock
}

func NewBasicBlock(id int, parent *BasicBlock) *BasicBlock {
	return &BasicBlock{
		Id: id,
		Instrs: []*Instr{},
		Parent: parent,
	}
}

type IRGen struct {
	ir_builder *IRBuilder
}

func NewIRGen(ir_builder *IRBuilder) *IRGen {
	return &IRGen{
		ir_builder: ir_builder,
	}
}

func (g *IRGen) Generate(program *ast.ProgramNode) {
	for _, stmt := range program.Statements {
		stmt.Accept(g)
	}
}

func (g *IRGen) VisitBlockStmt(stmt *ast.BlockStmtNode) {
	g.ir_builder.BuildEnterScope()
	for _, stmt := range stmt.Statements {
		stmt.Accept(g)
	}
	g.ir_builder.BuildExitScope()
}

func (g *IRGen) VisitVarDeclStmt(stmt *ast.VarDeclStmtNode) {
	for _, decl := range stmt.Decls {
		var initializer constant.Value
		isConst := false

		if decl.Initializer != nil {
			initializer = decl.Initializer.Accept(g)
		}

		if decl.DeclType == ast.VarDeclTypeConst {
			isConst = true
		}

		g.ir_builder.BuildVarDecl(&Variable{
			Name: decl.Identifier.Name.Value,
			ValueType: constant.ToValueType(decl.DataType),
			Span: decl.Identifier.Name.Span,
			IsConst: isConst,
			Initializer: initializer,
		})
	}
}

func (g *IRGen) VisitExprStmt(stmt *ast.ExprStmtNode) {
	stmt.Expr.Accept(g)
}

func (g *IRGen) VisitReturnStmt(stmt *ast.ReturnStmtNode) {
	value := stmt.Expr.Accept(g)
	g.ir_builder.BuildReturn(value, stmt.Expr.GetSpan())
}

func (g *IRGen) VisitIfStmt(stmt *ast.IfStmtNode) {
	condition := stmt.Condition.Accept(g)
	then_block := g.ir_builder.BuildBasicBlock()
	g.ir_builder.BuildCondJmp(then_block, condition, stmt.Condition.GetSpan())
	g.ir_builder.SetInsertionBlock(then_block)
	stmt.ThenStmt.Accept(g)
	g.ir_builder.EndBasicBlock()
	
	if stmt.ElseStmt != nil {
		not_condition := g.ir_builder.BuildUnaryOp(condition, InstrTypeNot, stmt.Condition.GetSpan())
		else_block := g.ir_builder.BuildBasicBlock()
		g.ir_builder.BuildCondJmp(else_block, not_condition, nil)
		g.ir_builder.SetInsertionBlock(else_block)
		stmt.ElseStmt.Accept(g)
		g.ir_builder.EndBasicBlock()
	}
}

func (g *IRGen) VisitBinaryExpr(expr *ast.BinaryExprNode) constant.Value {
	left := expr.Left.Accept(g)
	right := expr.Right.Accept(g)
	
	switch expr.Operator.Type {
	case token.TokenTypePlus:
		return g.ir_builder.BuildBinaryOp(left, right, InstrTypeAdd, expr.GetSpan())
	case token.TokenTypeMinus:
		return g.ir_builder.BuildBinaryOp(left, right, InstrTypeSub, expr.GetSpan())
	case token.TokenTypeStar:
		return g.ir_builder.BuildBinaryOp(left, right, InstrTypeMul, expr.GetSpan())
	case token.TokenTypeSlash:
		return g.ir_builder.BuildBinaryOp(left, right, InstrTypeDiv, expr.GetSpan())
	case token.TokenTypeBangEqual:
		return g.ir_builder.BuildBinaryOp(left, right, InstrTypeNotEq, expr.GetSpan())
	case token.TokenTypeEqualEqual:
		return g.ir_builder.BuildBinaryOp(left, right, InstrTypeEqEq, expr.GetSpan())
	case token.TokenTypeLessThan:
		return g.ir_builder.BuildBinaryOp(left, right, InstrTypeLessThan, expr.GetSpan())
	case token.TokenTypeLessThanEqual:
		return g.ir_builder.BuildBinaryOp(left, right, InstrTypeLessThanEq, expr.GetSpan())
	case token.TokenTypeGreaterThan:
		return g.ir_builder.BuildBinaryOp(left, right, InstrTypeGreaterThan, expr.GetSpan())
	case token.TokenTypeGreaterThanEqual:
		return g.ir_builder.BuildBinaryOp(left, right, InstrTypeGreaterThanEq, expr.GetSpan())
	default:
		panic(fmt.Sprintf("unknown binary operator: %s", expr.Operator.Type))
	}
}

func (g *IRGen) VisitGroupingExpr(expr *ast.GroupingExprNode) constant.Value {
	return expr.Expr.Accept(g)
}

func (g *IRGen) VisitFunctionCallExpr(expr *ast.FunctionCallExprNode) constant.Value {
	callee := expr.Callee.Accept(g)
	params := []constant.Value{}
	for _, arg := range expr.Params {
		params = append(params, arg.Accept(g))
	}

	return g.ir_builder.BuildCallFunc(callee, params, expr.GetSpan())
}

func (g *IRGen) VisitFunctionDeclExpr(expr *ast.FunctionDeclExprNode) constant.Value {
	params := []Variable{}
	param_types := []constant.ValueType{}

	for _, param := range expr.Params {
		params = append(params, Variable{
			Name: param.Identifier.Name.Value,
			ValueType: constant.ToValueType(param.DataType),
			IsConst: true,
			Initializer: nil,
			Span: param.Identifier.Name.Span,
		})
		param_types = append(param_types, constant.ToValueType(param.DataType))
	}

	body := g.ir_builder.BuildBasicBlock()
	g.ir_builder.BuildFuncDecl(expr.Name.Name.Value, params, body, constant.ToValueType(expr.ReturnType), expr.Name.Name.Span)
	g.ir_builder.SetInsertionBlock(body)
	expr.Body.Accept(g)
	g.ir_builder.EndBasicBlock()

	return &Variable{
		Name: expr.Name.Name.Value,
		ValueType: constant.FunctionType{
			ReturnType: constant.ToValueType(expr.ReturnType),
			ParamTypes: param_types,
		},
		IsConst: true,
		Initializer: nil,
		Span: expr.Name.Name.Span,
	}
}

func (g *IRGen) VisitIdentifier(expr *ast.IdentifierExprNode) constant.Value {
	return g.ir_builder.BuildLoad(expr.Name.Value, expr.Name.Span)
}

func (g *IRGen) VisitNumber(expr *ast.NumberExprNode) constant.Value {
	if constant.IsFloat(expr.Value.Value) {
		return &Constant{
			Value: expr.Value.Value,
			ValueType: constant.FloatType{
				Size: constant.F64,
			},
			Span: expr.Value.Span,
		}
	} else {
		return &Constant{
			Value: expr.Value.Value,
			ValueType: constant.IntType{
				Signed: true,
				Size: constant.GetIntSize(expr.Value.Value),
			},
			Span: expr.Value.Span,
		}
	}
}

func (g *IRGen) VisitUnaryExpr(expr *ast.UnaryExprNode) constant.Value {
	value := expr.Expr.Accept(g)

	switch expr.Operator.Type {
	case token.TokenTypeMinus:
		return g.ir_builder.BuildUnaryOp(value, InstrTypeNeg, expr.Operator.Span)
	case token.TokenTypeBang:
		return g.ir_builder.BuildUnaryOp(value, InstrTypeNot, expr.Operator.Span)
	default:
		panic(fmt.Sprintf("unknown unary operator: %s", expr.Operator.Type))
	}
}
