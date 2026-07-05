package ir

import (
	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// CapturedVar describes a variable that must be captured from an enclosing scope
// into a functor's properties so the closure works after the enclosing stack frame
// is gone.
type CapturedVar struct {
	OriginalName string               // source name as written (e.g. "x" or "this")
	PropertyName string               // functor struct field name (e.g. "__cap_x__")
	ValueType    zeus_value.ValueType // type of the captured value
	Source       zeus_value.Value     // *zeus_value.Var or *zeus_value.Object (for "this")
}

// freeVarCollector walks a function body AST and collects all variables that are
// defined in an outer (non-global) scope and referenced inside the body.
// It implements ast.StmtVisitor and ast.ExprVisitor[zeus_value.Value].
//
// Must be created and run while the symbol table still reflects the enclosing scope
// (before emitFunctorClass pushes a new scope for the functor body).
type freeVarCollector struct {
	module      *IRModule
	localScopes []map[string]bool // stack; level 0 = current fn's own params + locals
	captured    map[string]*CapturedVar
}

func newFreeVarCollector(module *IRModule, ownParams []*ast.VarDeclNode) *freeVarCollector {
	top := make(map[string]bool, len(ownParams))
	for _, p := range ownParams {
		top[p.Identifier.Name.Value] = true
	}
	return &freeVarCollector{
		module:      module,
		localScopes: []map[string]bool{top},
		captured:    make(map[string]*CapturedVar),
	}
}

func (f *freeVarCollector) isLocal(name string) bool {
	for _, scope := range f.localScopes {
		if scope[name] {
			return true
		}
	}
	return false
}

func (f *freeVarCollector) declareLocal(name string) {
	f.localScopes[len(f.localScopes)-1][name] = true
}

func (f *freeVarCollector) pushScope(params []*ast.VarDeclNode) {
	scope := make(map[string]bool, len(params))
	for _, p := range params {
		scope[p.Identifier.Name.Value] = true
	}
	f.localScopes = append(f.localScopes, scope)
}

func (f *freeVarCollector) popScope() {
	f.localScopes = f.localScopes[:len(f.localScopes)-1]
}

// result returns captured vars with non-this vars first, this last.
// The preamble in __call__ must load all functor properties before shadowing
// "this", so "this" captures must be processed last.
func (f *freeVarCollector) result() []*CapturedVar {
	out := make([]*CapturedVar, 0, len(f.captured))
	for _, cap := range f.captured {
		if cap.OriginalName != token.THIS_KEYWORD {
			out = append(out, cap)
		}
	}
	for _, cap := range f.captured {
		if cap.OriginalName == token.THIS_KEYWORD {
			out = append(out, cap)
		}
	}
	return out
}

func (f *freeVarCollector) walkExpr(expr ast.ExprNode) {
	if expr != nil {
		expr.Accept(f)
	}
}

func (f *freeVarCollector) walkStmt(stmt ast.StmtNode) {
	if stmt != nil {
		stmt.Accept(f)
	}
}

// ---- ast.StmtVisitor ----

func (f *freeVarCollector) VisitExprStmt(stmt *ast.ExprStmtNode) {
	f.walkExpr(stmt.Expr)
}

func (f *freeVarCollector) VisitVarDeclStmt(stmt *ast.VarDeclStmtNode) {
	for _, decl := range stmt.Decls {
		if decl.Initializer != nil {
			f.walkExpr(decl.Initializer)
		}
		f.declareLocal(decl.Identifier.Name.Value)
	}
}

func (f *freeVarCollector) VisitBlockStmt(stmt *ast.BlockStmtNode) {
	for _, s := range stmt.Statements {
		f.walkStmt(s)
	}
}

func (f *freeVarCollector) VisitReturnStmt(stmt *ast.ReturnStmtNode) {
	if stmt.Expr != nil {
		f.walkExpr(stmt.Expr)
	}
}

func (f *freeVarCollector) VisitIfStmt(stmt *ast.IfStmtNode) {
	f.walkExpr(stmt.Condition)
	f.walkStmt(stmt.ThenStmt)
	if stmt.ElseStmt != nil {
		f.walkStmt(stmt.ElseStmt)
	}
}

func (f *freeVarCollector) VisitWhileStmt(stmt *ast.WhileStmtNode) {
	f.walkExpr(stmt.Condition)
	f.walkStmt(stmt.Body)
}

func (f *freeVarCollector) VisitForStmt(stmt *ast.ForStmtNode) {
	if stmt.Init != nil {
		f.walkStmt(stmt.Init)
	}
	if stmt.Condition != nil {
		f.walkExpr(stmt.Condition)
	}
	if stmt.Update != nil {
		f.walkExpr(stmt.Update)
	}
	f.walkStmt(stmt.Body)
}

func (f *freeVarCollector) VisitImportStmt(*ast.ImportStmtNode)     {}
func (f *freeVarCollector) VisitExportStmt(*ast.ExportStmtNode)     {}
func (f *freeVarCollector) VisitBreakStmt(*ast.BreakStmtNode)       {}
func (f *freeVarCollector) VisitContinueStmt(*ast.ContinueStmtNode) {}

func (f *freeVarCollector) VisitTryCatchStmt(stmt *ast.TryCatchStmtNode) {
	f.walkStmt(stmt.TryBody)
	for _, clause := range stmt.CatchClauses {
		if clause.ErrorVar != nil {
			f.declareLocal(clause.ErrorVar.Name.Value)
		}
		f.walkStmt(clause.Body)
	}
}

func (f *freeVarCollector) VisitThrowStmt(stmt *ast.ThrowStmtNode) {
	f.walkExpr(stmt.Expr)
}

// ---- ast.ExprVisitor[zeus_value.Value] ----

func (f *freeVarCollector) VisitBinaryExpr(node *ast.BinaryExprNode) zeus_value.Value {
	f.walkExpr(node.Left)
	f.walkExpr(node.Right)
	return nil
}

func (f *freeVarCollector) VisitIndexingExpression(node *ast.IndexingExprNode) zeus_value.Value {
	f.walkExpr(node.Array)
	for _, idx := range node.IndexingMeta.IndexingExprs {
		f.walkExpr(idx)
	}
	return nil
}

func (f *freeVarCollector) VisitNumber(*ast.NumberExprNode) zeus_value.Value         { return nil }
func (f *freeVarCollector) VisitChar(*ast.CharExprNode) zeus_value.Value              { return nil }
func (f *freeVarCollector) VisitBoolean(*ast.BooleanExprNode) zeus_value.Value        { return nil }
func (f *freeVarCollector) VisitNull(*ast.NullExprNode) zeus_value.Value              { return nil }
func (f *freeVarCollector) VisitStringConstant(*ast.StringConstantExprNode) zeus_value.Value {
	return nil
}
func (f *freeVarCollector) VisitValueType(*ast.ValueTypeNode) zeus_value.Value { return nil }

func (f *freeVarCollector) VisitUnaryExpr(node *ast.UnaryExprNode) zeus_value.Value {
	f.walkExpr(node.Expr)
	return nil
}

func (f *freeVarCollector) VisitPostfixExpr(node *ast.PostfixExprNode) zeus_value.Value {
	f.walkExpr(node.Expr)
	return nil
}

func (f *freeVarCollector) VisitGroupingExpr(node *ast.GroupingExprNode) zeus_value.Value {
	f.walkExpr(node.Expr)
	return nil
}

func (f *freeVarCollector) VisitFunctionCallExpr(node *ast.FunctionCallExprNode) zeus_value.Value {
	f.walkExpr(node.Callee)
	for _, arg := range node.Params {
		f.walkExpr(arg)
	}
	return nil
}

func (f *freeVarCollector) VisitIdentifier(node *ast.IdentifierExprNode) zeus_value.Value {
	name := node.Name.Value
	if f.isLocal(name) {
		return nil
	}
	sym, ok := f.module.symbolTable().GetSymbol(name)
	if !ok || f.module.symbolTable().IsSymbolGlobal(name) {
		return nil
	}
	if _, already := f.captured[name]; already {
		return nil
	}
	switch v := sym.(type) {
	case *zeus_value.Var:
		f.captured[name] = &CapturedVar{
			OriginalName: name,
			PropertyName: "__cap_" + name + "__",
			ValueType:    v.ValueType,
			Source:       v,
		}
	case *zeus_value.Object:
		// Only "this" is an Object in the symbol table; capture it so the closure
		// can access the outer class instance after the enclosing method returns.
		f.captured[name] = &CapturedVar{
			OriginalName: name,
			PropertyName: "__cap_this__",
			ValueType:    v.ValueType,
			Source:       v,
		}
	// *zeus_value.Function and *zeus_value.Class are referenced as raw pointers —
	// they don't live on any stack frame, so no capture is needed.
	}
	return nil
}

func (f *freeVarCollector) VisitFunctionDeclExpr(node *ast.FunctionDeclExprNode) zeus_value.Value {
	// Declare the function name as a local so it isn't incorrectly flagged as a
	// free var inside the parent scope (e.g. for a named recursive function).
	if node.Name != nil {
		f.declareLocal(node.Name.Name.Value)
	}
	// Push the inner function's own params so they shadow any outer vars with
	// the same name, but continue walking the body — required so that free vars
	// referenced only inside inner closures are still propagated to this level.
	f.pushScope(node.Params)
	f.walkStmt(node.Body)
	f.popScope()
	return nil
}

func (f *freeVarCollector) VisitClassDeclExpr(node *ast.ClassDeclExprNode) zeus_value.Value {
	// Class bodies run in a separate self context; their methods cannot capture
	// outer locals the same way. Skip walking class method bodies here.
	return nil
}

func (f *freeVarCollector) VisitNewExpr(node *ast.NewExprNode) zeus_value.Value {
	f.walkExpr(node.Callee)
	for _, arg := range node.Args {
		f.walkExpr(arg)
	}
	return nil
}

func (f *freeVarCollector) VisitObjectPropertyAccessExpr(node *ast.ObjectPropertyAccessExprNode) zeus_value.Value {
	f.walkExpr(node.Object)
	return nil
}

func (f *freeVarCollector) VisitTernaryExpr(node *ast.TernaryExprNode) zeus_value.Value {
	f.walkExpr(node.Condition)
	f.walkExpr(node.Then)
	f.walkExpr(node.Else)
	return nil
}

// collectFreeVars analyzes a function body AST and returns the variables that
// must be captured from the enclosing scope. Must be called before any IR is
// emitted for the body (symbol table still reflects the enclosing scope).
func (g *IRModule) collectFreeVars(fnParams []*ast.VarDeclNode, body *ast.BlockStmtNode) []*CapturedVar {
	collector := newFreeVarCollector(g, fnParams)
	collector.VisitBlockStmt(body)
	return collector.result()
}
