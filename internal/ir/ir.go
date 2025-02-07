package ir

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ast"
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
	Name string
	ValueType value.ValueType
	Initializer value.Value
	IsConst bool
}

func (i DeclareVarInstrInput) String() string {
	if i.Initializer != nil {
		return fmt.Sprintf("%s %s = %s", i.ValueType, i.Name, i.Initializer)
	}
	return fmt.Sprintf("%s %s", i.ValueType, i.Name)
}

func AsDeclareVarInstrInput(input InstrInput) *DeclareVarInstrInput {
	switch input := input.(type) {
	case DeclareVarInstrInput:
		return &input
	default:
		panicInvalidInputType("DeclareVarInstrInput", input)
	}

	return nil
}

type LoadInstrInput struct {
	Addr value.Value
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
	Addr value.Value
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
	return fmt.Sprintf("%s(%s)", i.Callee, strings.Join(args, ", "))
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
	return i.Value.String()
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
	Name string
	Args []VarDecl
	Body *BasicBlock
	ReturnType value.ValueType
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
	default:
		panic("unknown instruction type")
	}
}

type Instr struct {
	Type InstrType
	Output value.Value
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

type IRGen struct {
	ir_builder *IRBuilder
	is_lvalue_expr bool
}

func NewIRGen(ir_builder *IRBuilder) *IRGen {
	return &IRGen{
		ir_builder: ir_builder,
		is_lvalue_expr: false,
	}
}

func (g *IRGen) Generate(program *ast.ProgramNode) {
	for _, stmt := range program.Statements {
		stmt.Accept(g)
	}
}

func (g *IRGen) VisitBlockStmt(stmt *ast.BlockStmtNode) {
	for _, stmt := range stmt.Statements {
		stmt.Accept(g)
	}
}

func (g *IRGen) VisitVarDeclStmt(stmt *ast.VarDeclStmtNode) {
	for _, decl := range stmt.Decls {
		var initializer value.Value
		isConst := false

		if decl.Initializer != nil {
			initializer = decl.Initializer.Accept(g)
		}

		if decl.DeclType == ast.VarDeclTypeConst {
			isConst = true
		}

		g.ir_builder.BuildVarDecl(&VarDecl{
			Name: decl.Identifier.Name.Value,
			ValueType: value.ToValueType(decl.DataType),
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
	// create the condition
	condition := stmt.Condition.Accept(g)
	// create the required blocks
	then_block := g.ir_builder.BuildSuccessorBlock()
	else_block := g.ir_builder.BuildSuccessorBlock()
	merge_block := g.ir_builder.BuildSuccessorBlock()

	// build jump to if block
	g.ir_builder.BuildCondJmp(then_block, else_block, condition, stmt.Condition.GetSpan())

	// generate the then block
	g.ir_builder.SetInsertionBlock(then_block)
	stmt.ThenStmt.Accept(g)
	// jump to the merge block
	g.ir_builder.BuildJmp(merge_block, nil)
	
	// generate the else block
	g.ir_builder.SetInsertionBlock(else_block)
	if stmt.ElseStmt != nil {
		stmt.ElseStmt.Accept(g)
	}

	g.ir_builder.SetInsertionBlock(merge_block)
}

func (g *IRGen) VisitWhileStmt(stmt *ast.WhileStmtNode) {
	
	// create the required blocks
	condition_block := g.ir_builder.BuildSuccessorBlock()
	body_block := g.ir_builder.BuildSuccessorBlock()
	merge_block := g.ir_builder.BuildSuccessorBlock()

	// build condition block
	g.ir_builder.SetInsertionBlock(condition_block)
	// create the condition
	condition := stmt.Condition.Accept(g)
	g.ir_builder.BuildCondJmp(body_block, merge_block, condition, stmt.Condition.GetSpan())

	// generate the body block
	g.ir_builder.SetInsertionBlock(body_block)
	stmt.Body.Accept(g)
	g.ir_builder.BuildJmp(condition_block, nil)

	// generate the merge block
	g.ir_builder.SetInsertionBlock(merge_block)
}

func (g *IRGen) VisitBinaryExpr(expr *ast.BinaryExprNode) value.Value {
	g.is_lvalue_expr = expr.Operator.Type == token.TokenTypeEqual
	left := expr.Left.Accept(g)
	g.is_lvalue_expr = false
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
	case token.TokenTypeEqual:
		addr := value.AsVar(left)
		if addr == nil {
			panic(fmt.Sprintf("invalid lvalue: %s", left))
		}

		g.ir_builder.BuildStore(addr, right, expr.GetSpan())
		return g.ir_builder.BuildLoad(addr, expr.GetSpan())

	default:
		panic(fmt.Sprintf("unknown binary operator: %s", expr.Operator.Type))
	}
}

func (g *IRGen) VisitGroupingExpr(expr *ast.GroupingExprNode) value.Value {
	return expr.Expr.Accept(g)
}

func (g *IRGen) VisitFunctionCallExpr(expr *ast.FunctionCallExprNode) value.Value {
	callee := expr.Callee.Accept(g)
	params := []value.Value{}
	for _, arg := range expr.Params {
		params = append(params, arg.Accept(g))
	}

	addr := value.AsVar(callee)
	if addr == nil {
		panic(fmt.Sprintf("invalid callee: %s", callee))
	}

	return g.ir_builder.BuildCallFunc(addr, params, expr.GetSpan())
}

func (g *IRGen) VisitFunctionDeclExpr(expr *ast.FunctionDeclExprNode) value.Value {
	params := []VarDecl{}
	param_types := []value.ValueType{}

	for _, param := range expr.Params {
		params = append(params, VarDecl{
			Name: param.Identifier.Name.Value,
			ValueType: value.ToValueType(param.DataType),
			IsConst: true,
			Initializer: nil,
			Span: param.Identifier.Name.Span,
		})
		param_types = append(param_types, value.ToValueType(param.DataType))
	}

	current_block := g.ir_builder.GetInsertionBlock()
	// functions are global
	g.ir_builder.SetInsertionBlock(nil)
	body := g.ir_builder.BuildBasicBlock()
	g.ir_builder.BuildFuncDecl(expr.Name.Name.Value, params, body, value.ToValueType(expr.ReturnType), expr.Name.Name.Span)
	g.ir_builder.SetInsertionBlock(body)
	expr.Body.Accept(g)
	g.ir_builder.SetInsertionBlock(current_block)

	return &VarDecl{
		Name: expr.Name.Name.Value,
		ValueType: value.FunctionType{
			ReturnType: value.ToValueType(expr.ReturnType),
			ParamTypes: param_types,
		},
		IsConst: true,
		Initializer: nil,
		Span: expr.Name.Name.Span,
	}
}

func (g *IRGen) VisitIdentifier(expr *ast.IdentifierExprNode) value.Value {
	if g.is_lvalue_expr {
		return &value.Var{
			Name: expr.Name.Value,
			Span: expr.Name.Span,
		}
	}

	return g.ir_builder.BuildLoad(&value.Var{
		Name: expr.Name.Value,
		Span: expr.Name.Span,
	}, expr.Name.Span)
}

func (g *IRGen) VisitNumber(expr *ast.NumberExprNode) value.Value {
	if value.IsFloat(expr.Value.Value) {
		return &value.Constant{
			Value: expr.Value.Value,
			ValueType: value.FloatType{
				Size: value.F64,
			},
			Span: expr.Value.Span,
		}
	} else {
		return &value.Constant{
			Value: expr.Value.Value,
			ValueType: value.IntType{
				Signed: true,
				Size: value.GetIntSize(expr.Value.Value),
			},
			Span: expr.Value.Span,
		}
	}
}

func (g *IRGen) VisitUnaryExpr(expr *ast.UnaryExprNode) value.Value {
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
