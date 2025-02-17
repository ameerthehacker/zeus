package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/value"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

type IRGen struct {
	ir_builder *IRBuilder
	is_lvalue_expr bool
	symbol_table *symbol_table.SymbolTable[value.Value]
	errors []*zeus_error.ZeusError
}

func NewIRGen(ir_builder *IRBuilder) *IRGen {
	return &IRGen{
		ir_builder: ir_builder,
		is_lvalue_expr: false,
		symbol_table: symbol_table.NewSymbolTable[value.Value](),
	}
}

func (g *IRGen) pushError(err *zeus_error.ZeusError) {
	g.errors = append(g.errors, err)
}

func (g *IRGen) Generate(program *ast.ProgramNode) []*zeus_error.ZeusError {
	g.symbol_table.EnterScope()
	for _, stmt := range program.Statements {
		stmt.Accept(g)
	}
	g.symbol_table.ExitScope()

	return g.errors
}

func (g *IRGen) VisitBlockStmt(stmt *ast.BlockStmtNode) {
	g.symbol_table.EnterScope()
	for _, stmt := range stmt.Statements {
		stmt.Accept(g)
	}
	g.symbol_table.ExitScope()
}

func (g *IRGen) VisitVarDeclStmt(stmt *ast.VarDeclStmtNode) {
	for _, decl := range stmt.Decls {
		if _, ok := g.symbol_table.GetSymbolInCurrentScope(decl.Identifier.Name.Value); ok {
			g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("cannot redeclare identifier '%s' in the same scope", decl.Identifier.Name.Value), decl.Identifier.Name.Span))
			return
		}

		var initializer value.Value
		isConst := false

		if decl.Initializer != nil {
			initializer = decl.Initializer.Accept(g)
		}

		if decl.DeclType == ast.VarDeclTypeConst {
			isConst = true
		}

		variable := g.ir_builder.BuildVarDecl(&VarDecl{
			Name: decl.Identifier.Name.Value,
			ValueType: value.ToValueType(decl.DataType),
			Span: decl.Identifier.Name.Span,
			IsConst: isConst,
			Initializer: initializer,
		})

		g.symbol_table.DeclareSymbol(decl.Identifier.Name.Value, variable)
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

	addr := value.AsFunction(callee)

	return g.ir_builder.BuildCallFunc(addr, params, expr.GetSpan())
}

func (g *IRGen) VisitFunctionDeclExpr(expr *ast.FunctionDeclExprNode) value.Value {
	params := []VarDecl{}

	for _, param := range expr.Params {
		params = append(params, VarDecl{
			Name: param.Identifier.Name.Value,
			ValueType: value.ToValueType(param.DataType),
			IsConst: true,
			Initializer: nil,
			Span: param.Identifier.Name.Span,
		})
	}

	current_block := g.ir_builder.GetInsertionBlock()
	// functions are global
	g.ir_builder.SetInsertionBlock(nil)
	body := g.ir_builder.BuildBasicBlock()
	fn := g.ir_builder.BuildFuncDecl(expr.Name.Name.Value, params, body, value.ToValueType(expr.ReturnType), expr.Name.Name.Span)
	g.symbol_table.DeclareSymbol(expr.Name.Name.Value, fn)
	g.symbol_table.EnterScope()

	for index, param := range expr.Params {
		if _, ok := g.symbol_table.GetSymbolInCurrentScope(param.Identifier.Name.Value); ok {
			g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("cannot redeclare parameter '%s' in the same scope", param.Identifier.Name.Value), param.Identifier.Name.Span))
			return nil
		}
		g.symbol_table.DeclareSymbol(param.Identifier.Name.Value, fn.Params[index])
	}

	g.ir_builder.SetInsertionBlock(body)
	expr.Body.Accept(g)
	g.ir_builder.SetInsertionBlock(current_block)

	g.symbol_table.ExitScope()

	return &value.Var{
		Name: expr.Name.Name.Value,
		ValueType: value.ToFunctionType(*fn),
		Span: expr.Name.Name.Span,
	}
}

func (g *IRGen) VisitIdentifier(expr *ast.IdentifierExprNode) value.Value {
	variable, ok := g.symbol_table.GetSymbol(expr.Name.Value)

	if !ok {
		g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("undefined identifier '%s'", expr.Name.Value), expr.Name.Span))
		return nil
	}

	asFn := value.AsFunction(variable)

	if asFn != nil {
		return asFn
	}

	if g.is_lvalue_expr {
		return variable
	}

	asVar := value.AsVar(variable)

	return g.ir_builder.BuildLoad(asVar, expr.Name.Span)
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

func (g *IRGen) VisitBoolean(expr *ast.BooleanExprNode) value.Value {
	if expr.Value.Type == token.TokenTypeTrue {
		return &value.Constant{
			Value: "true",
			ValueType: value.BoolType{},
			Span: expr.Value.Span,
		}
	}
	return &value.Constant{
		Value: "false",
		ValueType: value.BoolType{},
		Span: expr.Value.Span,
	}
}

