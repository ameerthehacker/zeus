package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

type IRModule struct {
	irBuilder *IRBuilder
	isLValueExpr bool
	symbolTable *symbol_table.SymbolTable[zeus_value.Value]
	errors []*zeus_error.ZeusError
}

func NewIRModule(ir_builder *IRBuilder) *IRModule {
	return &IRModule{
		irBuilder: ir_builder,
		isLValueExpr: false,
		symbolTable: symbol_table.NewSymbolTable[zeus_value.Value](),
	}
}

func (g *IRModule) pushError(err *zeus_error.ZeusError) {
	g.errors = append(g.errors, err)
}

func (g *IRModule) Generate(program *ast.ProgramNode) []*zeus_error.ZeusError {
	g.symbolTable.EnterScope()
	for _, stmt := range program.Statements {
		stmt.Accept(g)
	}
	g.symbolTable.ExitScope()
	g.irBuilder.Optimize()

	return g.errors
}

func (g *IRModule) VisitBlockStmt(stmt *ast.BlockStmtNode) { 
	g.symbolTable.EnterScope()
	for _, stmt := range stmt.Statements {
		stmt.Accept(g)
	}
	g.symbolTable.ExitScope()
}

func (g *IRModule) VisitVarDeclStmt(stmt *ast.VarDeclStmtNode) {
	for _, decl := range stmt.Decls {
		if _, ok := g.symbolTable.GetSymbolInCurrentScope(decl.Identifier.Name.Value); ok {
			g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("cannot redeclare identifier '%s' in the same scope", decl.Identifier.Name.Value), decl.Identifier.Name.Span))
			return
		}

		var initializer zeus_value.Value
		isConst := false

		if decl.Initializer != nil {
			initializer = decl.Initializer.Accept(g)
		}

		if decl.DeclType == ast.VarDeclTypeConst {
			isConst = true
		}

		variable := g.irBuilder.BuildVarDecl(NewVarDecl(
			decl.Identifier.Name.Value,
			zeus_value.ToValueType(decl.DataType),
			isConst,
			initializer,
			decl.Identifier.Name.Span,
		))

		g.symbolTable.DeclareSymbol(decl.Identifier.Name.Value, variable)
	}
}

func (g *IRModule) VisitExprStmt(stmt *ast.ExprStmtNode) {
	stmt.Expr.Accept(g)
}

func (g *IRModule) VisitReturnStmt(stmt *ast.ReturnStmtNode) {
	if stmt.Expr == nil {
		g.irBuilder.BuildReturn(nil, stmt.GetSpan())
	} else {
		g.irBuilder.BuildReturn(stmt.Expr.Accept(g), stmt.GetSpan())
	}
}

func (g *IRModule) VisitIfStmt(stmt *ast.IfStmtNode) {
	// create the condition
	condition := stmt.Condition.Accept(g)
	// create the required blocks
	then_block := g.irBuilder.BuildSuccessorBlock()
	else_block := g.irBuilder.BuildSuccessorBlock()
	merge_block := g.irBuilder.BuildSuccessorBlock()

	// build jump to if block
	g.irBuilder.BuildCondJmp(then_block, else_block, condition, stmt.Condition.GetSpan())

	// generate the then block
	g.irBuilder.SetInsertionBlock(then_block)
	stmt.ThenStmt.Accept(g)
	// jump to the merge block
	g.irBuilder.BuildJmp(merge_block, nil)
	
	// generate the else block
	if stmt.ElseStmt != nil {
		g.irBuilder.SetInsertionBlock(else_block)
		stmt.ElseStmt.Accept(g)
	}

	g.irBuilder.SetInsertionBlock(merge_block)
}

func (g *IRModule) VisitWhileStmt(stmt *ast.WhileStmtNode) {
	
	// create the required blocks
	condition_block := g.irBuilder.BuildSuccessorBlock()
	body_block := g.irBuilder.BuildSuccessorBlock()
	merge_block := g.irBuilder.BuildSuccessorBlock()
	g.irBuilder.BuildJmp(condition_block, nil)

	// build condition block
	g.irBuilder.SetInsertionBlock(condition_block)
	// create the condition
	condition := stmt.Condition.Accept(g)
	g.irBuilder.BuildCondJmp(body_block, merge_block, condition, stmt.Condition.GetSpan())

	// generate the body block
	g.irBuilder.SetInsertionBlock(body_block)
	stmt.Body.Accept(g)
	g.irBuilder.BuildJmp(condition_block, nil)

	// generate the merge block
	g.irBuilder.SetInsertionBlock(merge_block)
}

func (g *IRModule) VisitBinaryExpr(expr *ast.BinaryExprNode) zeus_value.Value {
	g.isLValueExpr = expr.Operator.Type == token.TokenTypeEqual
	left := expr.Left.Accept(g)
	g.isLValueExpr = false
	right := expr.Right.Accept(g)
	
	switch expr.Operator.Type {
	case token.TokenTypePlus:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeAdd, expr.GetSpan())
	case token.TokenTypeMinus:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeSub, expr.GetSpan())
	case token.TokenTypeStar:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeMul, expr.GetSpan())
	case token.TokenTypeSlash:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeDiv, expr.GetSpan())
	case token.TokenTypeBangEqual:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeNotEq, expr.GetSpan())
	case token.TokenTypeEqualEqual:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeEqEq, expr.GetSpan())
	case token.TokenTypeLessThan:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeLessThan, expr.GetSpan())
	case token.TokenTypeLessThanEqual:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeLessThanEq, expr.GetSpan())
	case token.TokenTypeGreaterThan:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeGreaterThan, expr.GetSpan())
	case token.TokenTypeGreaterThanEqual:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeGreaterThanEq, expr.GetSpan())
	case token.TokenTypeEqual:
		addr := zeus_value.AsVar(left)
		if addr == nil {
			panic(fmt.Sprintf("invalid lvalue: %s", left))
		}

		g.irBuilder.BuildStore(addr, right, expr.GetSpan())
		return g.irBuilder.BuildLoad(addr, expr.GetSpan())

	default:
		panic(fmt.Sprintf("unknown binary operator: %s", expr.Operator.Type))
	}
}

func (g *IRModule) VisitGroupingExpr(expr *ast.GroupingExprNode) zeus_value.Value {
	return expr.Expr.Accept(g)
}

func (g *IRModule) VisitFunctionCallExpr(expr *ast.FunctionCallExprNode) zeus_value.Value {
	callee := expr.Callee.Accept(g)
	params := []zeus_value.Value{}
	for _, arg := range expr.Params {
		params = append(params, arg.Accept(g))
	}

	addr := zeus_value.AsFunction(callee)

	return g.irBuilder.BuildCallFunc(addr, params, expr.GetSpan())
}

func (g *IRModule) VisitFunctionDeclExpr(expr *ast.FunctionDeclExprNode) zeus_value.Value {
	params := []*VarDecl{}

	for _, param := range expr.Params {
		params = append(params, NewVarDecl(
			param.Identifier.Name.Value,
			zeus_value.ToValueType(param.DataType),
			true,
			nil,
			param.Identifier.Name.Span,
		))
	}

	current_block := g.irBuilder.GetInsertionBlock()
	// functions are global
	g.irBuilder.SetInsertionBlock(nil)
	body := g.irBuilder.BuildBasicBlock()
	fn := g.irBuilder.BuildFuncDecl(expr.Name.Name.Value, params, body, zeus_value.ToValueType(expr.ReturnType), expr.Name.Name.Span)
	g.symbolTable.DeclareSymbol(expr.Name.Name.Value, fn)
	g.symbolTable.EnterScope()

	for index, param := range expr.Params {
		if _, ok := g.symbolTable.GetSymbolInCurrentScope(param.Identifier.Name.Value); ok {
			g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("cannot redeclare parameter '%s' in the same scope", param.Identifier.Name.Value), param.Identifier.Name.Span))
			return nil
		}
		g.symbolTable.DeclareSymbol(param.Identifier.Name.Value, fn.Params[index])
	}

	g.irBuilder.SetInsertionBlock(body)
	expr.Body.Accept(g)
	g.irBuilder.SetInsertionBlock(current_block)

	g.symbolTable.ExitScope()

	return zeus_value.NewVar(expr.Name.Name.Value, zeus_value.ToFunctionType(*fn), false, expr.Name.Name.Span)
}

func (g *IRModule) VisitIdentifier(expr *ast.IdentifierExprNode) zeus_value.Value {
	variable, ok := g.symbolTable.GetSymbol(expr.Name.Value)

	if !ok {
		g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("undefined identifier '%s'", expr.Name.Value), expr.Name.Span))
		return nil
	}

	asFn := zeus_value.AsFunction(variable)

	if asFn != nil {
		return asFn
	}

	if g.isLValueExpr {
		return variable
	}

	asVar := zeus_value.AsVar(variable)

	if asVar.IsPtr {
		return g.irBuilder.BuildLoad(asVar, expr.Name.Span)
	} else {
		return asVar
	}
}

func (g *IRModule) VisitNumber(expr *ast.NumberExprNode) zeus_value.Value {
	if zeus_value.IsFloat(expr.Value.Value) {
		return zeus_value.NewConstant(
			expr.Value.Value,
			zeus_value.FloatType{
				Size: zeus_value.F64,
			},
			expr.Value.Span,
		)
	} else {
		return zeus_value.NewConstant(
			expr.Value.Value,
			zeus_value.IntType{
				Signed: false,
				Size: zeus_value.GetSignedIntSize(expr.Value.Value),
			},
			expr.Value.Span,
		)
	}
}

func (g *IRModule) VisitUnaryExpr(expr *ast.UnaryExprNode) zeus_value.Value {
	value := expr.Expr.Accept(g)

	switch expr.Operator.Type {
	case token.TokenTypeMinus:
		return g.irBuilder.BuildUnaryOp(value, InstrTypeNeg, expr.Operator.Span)
	case token.TokenTypeBang:
		return g.irBuilder.BuildUnaryOp(value, InstrTypeNot, expr.Operator.Span)
	default:
		panic(fmt.Sprintf("unknown unary operator: %s", expr.Operator.Type))
	}
}

func (g *IRModule) VisitBoolean(expr *ast.BooleanExprNode) zeus_value.Value {
	if expr.Value.Type == token.TokenTypeTrue {
		return zeus_value.NewConstant(
			"true",
			zeus_value.BoolType{},
			expr.Value.Span,
		)
	}
	return zeus_value.NewConstant(
		"false",
		zeus_value.BoolType{},
		expr.Value.Span,
	)
}

func (g *IRModule) VisitExportStmt(stmt *ast.ExportStmtNode) {
	g.irBuilder.BuildExport(stmt.Expr.Accept(g), stmt.GetSpan())
}

func (g *IRModule) VisitImportStmt(stmt *ast.ImportStmtNode) {}
