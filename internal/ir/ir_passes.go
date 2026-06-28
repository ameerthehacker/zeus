package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// IRGenPass is a pass that runs over the AST before IR emission.
// Passes run in order; if a pass returns errors, subsequent passes are skipped.
type IRGenPass interface {
	Name() string
	Run(g *IRModule, program *ast.ProgramNode) []*zeus_error.ZeusError
}

// IREmitPass visits every AST statement and emits IR instructions.
// It runs after DeclCheckPass, so it can assume declarations are unique and
// that top-level stubs are already registered in the global registry.
type IREmitPass struct{}

func NewIREmitPass() *IREmitPass { return &IREmitPass{} }

func (p *IREmitPass) Name() string { return "IREmitPass" }

func (p *IREmitPass) Run(g *IRModule, program *ast.ProgramNode) []*zeus_error.ZeusError {
	for _, fn := range zeus_value.Registry.GetAllFunctions() {
		g.irBuilder.BuildDeclPrimordialFunc(fn, fn.Span)
	}
	for _, stmt := range program.Statements {
		stmt.Accept(g)
	}
	g.irBuilder.Optimize()
	return g.errors
}

// DeclCheckPass walks the full AST, validates declaration uniqueness at every
// scope level, and pre-registers top-level function/class stubs in the IRBuilder
// global registry so forward references resolve during IR emission.
//
// It uses the existing SymbolTable to track scopes — the value stored is the
// declaration span, used only for error reporting.
type DeclCheckPass struct {
	st     *symbol_table.SymbolTable[*token.Span]
	errors []*zeus_error.ZeusError
}

func NewDeclCheckPass() *DeclCheckPass { return &DeclCheckPass{} }

func (p *DeclCheckPass) Name() string { return "DeclCheckPass" }

func (p *DeclCheckPass) Run(g *IRModule, program *ast.ProgramNode) []*zeus_error.ZeusError {
	p.st = symbol_table.NewSymbolTable[*token.Span]()
	p.errors = nil
	p.st.EnterScope()
	for _, stmt := range program.Statements {
		p.walkStmt(g, stmt, true)
	}
	p.st.ExitScope()
	return p.errors
}

// checkAndDeclare registers name in the current scope.
// Reports an error and returns false if the name is already declared there.
func (p *DeclCheckPass) checkAndDeclare(name string, span *token.Span) bool {
	if _, exists := p.st.GetSymbolInCurrentScope(name); exists {
		p.errors = append(p.errors, zeus_error.NewZeusError(
			zeus_error.ErrorSeverityError,
			fmt.Sprintf("cannot redeclare identifier '%s' in the same scope", name),
			span,
		))
		return false
	}
	p.st.DeclareSymbol(name, span)
	return true
}

func (p *DeclCheckPass) walkStmt(g *IRModule, stmt ast.StmtNode, isTopLevel bool) {
	switch s := stmt.(type) {
	case *ast.ExprStmtNode:
		p.walkExpr(g, s.Expr, isTopLevel)

	case *ast.VarDeclStmtNode:
		for _, decl := range s.Decls {
			p.checkAndDeclare(decl.Identifier.Name.Value, decl.Identifier.Name.Span)
			if decl.Initializer != nil {
				p.walkExpr(g, decl.Initializer, false)
			}
		}

	case *ast.BlockStmtNode:
		p.st.EnterScope()
		for _, inner := range s.Statements {
			p.walkStmt(g, inner, false)
		}
		p.st.ExitScope()

	case *ast.IfStmtNode:
		if s.Condition != nil {
			p.walkExpr(g, s.Condition, false)
		}
		p.walkStmt(g, s.ThenStmt, false)
		if s.ElseStmt != nil {
			p.walkStmt(g, s.ElseStmt, false)
		}

	case *ast.WhileStmtNode:
		if s.Condition != nil {
			p.walkExpr(g, s.Condition, false)
		}
		p.walkStmt(g, s.Body, false)

	case *ast.ForStmtNode:
		p.st.EnterScope()
		if s.Init != nil {
			p.walkStmt(g, s.Init, false)
		}
		if s.Condition != nil {
			p.walkExpr(g, s.Condition, false)
		}
		if s.Update != nil {
			p.walkExpr(g, s.Update, false)
		}
		p.walkStmt(g, s.Body, false)
		p.st.ExitScope()

	case *ast.ReturnStmtNode:
		if s.Expr != nil {
			p.walkExpr(g, s.Expr, false)
		}

	case *ast.ExportStmtNode:
		p.walkExpr(g, s.Expr, isTopLevel)

	case *ast.TryCatchStmtNode:
		p.walkStmt(g, s.TryBody, false)
		for _, clause := range s.CatchClauses {
			p.st.EnterScope()
			p.checkAndDeclare(clause.ErrorVar.Name.Value, clause.ErrorVar.GetSpan())
			p.walkStmt(g, clause.Body, false)
			p.st.ExitScope()
		}

	case *ast.ThrowStmtNode:
		if s.Expr != nil {
			p.walkExpr(g, s.Expr, false)
		}

	case *ast.ImportStmtNode, *ast.BreakStmtNode, *ast.ContinueStmtNode:
		// no declarations to check
	}
}

// walkExpr handles the two expression types that introduce declarations.
// All other expression types are leaves for declaration-checking purposes.
func (p *DeclCheckPass) walkExpr(g *IRModule, expr ast.ExprNode, isTopLevel bool) {
	switch e := expr.(type) {
	case *ast.FunctionDeclExprNode:
		name := e.Name.Name.Value
		ok := p.checkAndDeclare(name, e.Name.GetSpan())
		if ok && isTopLevel {
			p.registerFunctionStub(g, e)
		}
		p.st.EnterScope()
		for _, param := range e.Params {
			p.checkAndDeclare(param.Identifier.Name.Value, param.Identifier.Name.Span)
		}
		p.walkStmt(g, e.Body, false)
		p.st.ExitScope()

	case *ast.ClassDeclExprNode:
		name := e.Name.Name.Value
		ok := p.checkAndDeclare(name, e.Name.GetSpan())
		if ok && isTopLevel {
			p.registerClassStub(g, e)
		}
		p.st.EnterScope()
		for _, prop := range e.Properties {
			p.checkAndDeclare(prop.Name.Name.Value, prop.Name.GetSpan())
		}
		for _, method := range e.Methods {
			p.checkAndDeclare(method.Name.Name.Value, method.Name.GetSpan())
			p.st.EnterScope()
			for _, param := range method.Params {
				p.checkAndDeclare(param.Identifier.Name.Value, param.Identifier.Name.Span)
			}
			p.walkStmt(g, method.Body, false)
			p.st.ExitScope()
		}
		p.st.ExitScope()
	}
}

func (p *DeclCheckPass) registerFunctionStub(g *IRModule, expr *ast.FunctionDeclExprNode) {
	name := expr.Name.Name.Value
	params := make([]*zeus_value.Var, 0, len(expr.Params))
	for _, param := range expr.Params {
		params = append(params, zeus_value.NewVar(
			param.Identifier.Name.Value,
			param.ValueType.ValueType,
			false,
			param.Identifier.Name.Span,
		))
	}
	var returnType zeus_value.ValueType = zeus_value.VoidType{Span: expr.GetSpan()}
	if expr.ReturnType != nil {
		returnType = expr.ReturnType.ValueType
	}
	fn := zeus_value.NewFunction(name, params, returnType, expr.Name.Name.Span)
	fn.OriginalName = name
	g.irBuilder.symbolTable.DeclareGlobalSymbol(name, fn)
}

func (p *DeclCheckPass) registerClassStub(g *IRModule, expr *ast.ClassDeclExprNode) {
	name := expr.Name.Name.Value
	properties := make([]*zeus_value.ClassProperty, 0, len(expr.Properties))
	for _, prop := range expr.Properties {
		v := zeus_value.NewVar(prop.Name.Name.Value, prop.ValueType.ValueType, false, prop.Name.GetSpan())
		properties = append(properties, zeus_value.NewClassProperty(v, prop.AccessModifier))
	}
	methods := make([]*zeus_value.ClassMethod, 0, len(expr.Methods))
	for _, method := range expr.Methods {
		mParams := make([]*zeus_value.Var, 0, len(method.Params))
		for _, mp := range method.Params {
			mParams = append(mParams, zeus_value.NewVar(
				mp.Identifier.Name.Value,
				mp.ValueType.ValueType,
				false,
				mp.Identifier.Name.Span,
			))
		}
		var returnType zeus_value.ValueType = zeus_value.VoidType{Span: method.Span}
		if method.ReturnType != nil {
			returnType = method.ReturnType.ValueType
		}
		fn := zeus_value.NewFunction(method.Name.Name.Value, mParams, returnType, method.Name.Name.Span)
		fn.OriginalName = method.Name.Name.Value
		methods = append(methods, zeus_value.NewClassMethod(fn, method.AccessModifier))
	}
	class := zeus_value.NewClass(name, properties, methods, "", nil, expr.GetSpan())
	class.OriginalName = name
	g.irBuilder.symbolTable.DeclareGlobalSymbol(name, class)
}
