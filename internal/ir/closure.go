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
	ValueType    zeus_value.ValueType // type of the captured value (ObjectType for ref cells)
	Source       zeus_value.Value     // *zeus_value.Var, *zeus_value.Object (for "this"), or ref cell *Var
	IsRefCell    bool                 // true when Source is a GC ref cell object (capture-by-reference)
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

func (f *freeVarCollector) VisitNumber(*ast.NumberExprNode) zeus_value.Value   { return nil }
func (f *freeVarCollector) VisitChar(*ast.CharExprNode) zeus_value.Value       { return nil }
func (f *freeVarCollector) VisitBoolean(*ast.BooleanExprNode) zeus_value.Value { return nil }
func (f *freeVarCollector) VisitNull(*ast.NullExprNode) zeus_value.Value       { return nil }
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
	case *zeus_value.RefCellVar:
		// Variable promoted to a heap ref cell — capture the cell pointer itself so
		// both the outer scope and the closure operate on the same GC object.
		f.captured[name] = &CapturedVar{
			OriginalName: name,
			PropertyName: "__cap_" + name + "__",
			ValueType:    zeus_value.GetValueType(v.Cell),
			Source:       v.Cell,
			IsRefCell:    true,
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

// ─────────────────────────────────────────────────────────────────────────────
// collectEscapedVarNames: pure-AST pre-pass for capture-by-reference
// ─────────────────────────────────────────────────────────────────────────────

// escapedNameCollector walks a function body AST to discover which variable/param
// names declared at the outermost (depth=0) scope are referenced inside any nested
// function body. These variables need to be promoted to heap ref cells so that
// closures can share the same mutable binding.
type escapedNameCollector struct {
	topLevelNames map[string]bool   // params + var names declared at depth=0
	nestedRefs    map[string]bool   // identifiers referenced at depth>0
	nestedScopes  []map[string]bool // scope stack for nested functions' own locals
	depth         int               // 0=top-level, >0=inside nested function body
}

func newEscapedNameCollector(params []*ast.VarDeclNode) *escapedNameCollector {
	topLevel := make(map[string]bool, len(params))
	for _, p := range params {
		topLevel[p.Identifier.Name.Value] = true
	}
	return &escapedNameCollector{
		topLevelNames: topLevel,
		nestedRefs:    make(map[string]bool),
	}
}

func (c *escapedNameCollector) isLocalToNested(name string) bool {
	for _, scope := range c.nestedScopes {
		if scope[name] {
			return true
		}
	}
	return false
}

func (c *escapedNameCollector) pushNestedScope(params []*ast.VarDeclNode) {
	scope := make(map[string]bool, len(params))
	for _, p := range params {
		scope[p.Identifier.Name.Value] = true
	}
	c.nestedScopes = append(c.nestedScopes, scope)
}

func (c *escapedNameCollector) popNestedScope() {
	c.nestedScopes = c.nestedScopes[:len(c.nestedScopes)-1]
}

func (c *escapedNameCollector) declareNestedLocal(name string) {
	if len(c.nestedScopes) > 0 {
		c.nestedScopes[len(c.nestedScopes)-1][name] = true
	}
}

func (c *escapedNameCollector) walkExpr(expr ast.ExprNode) {
	if expr != nil {
		expr.Accept(c)
	}
}

func (c *escapedNameCollector) walkStmt(stmt ast.StmtNode) {
	if stmt != nil {
		stmt.Accept(c)
	}
}

// ---- ast.StmtVisitor ----

func (c *escapedNameCollector) VisitExprStmt(stmt *ast.ExprStmtNode) { c.walkExpr(stmt.Expr) }

func (c *escapedNameCollector) VisitVarDeclStmt(stmt *ast.VarDeclStmtNode) {
	for _, decl := range stmt.Decls {
		if decl.Initializer != nil {
			c.walkExpr(decl.Initializer)
		}
		name := decl.Identifier.Name.Value
		if c.depth == 0 {
			c.topLevelNames[name] = true
		} else {
			c.declareNestedLocal(name)
		}
	}
}

func (c *escapedNameCollector) VisitBlockStmt(stmt *ast.BlockStmtNode) {
	for _, s := range stmt.Statements {
		c.walkStmt(s)
	}
}

func (c *escapedNameCollector) VisitReturnStmt(stmt *ast.ReturnStmtNode) {
	if stmt.Expr != nil {
		c.walkExpr(stmt.Expr)
	}
}

func (c *escapedNameCollector) VisitIfStmt(stmt *ast.IfStmtNode) {
	c.walkExpr(stmt.Condition)
	c.walkStmt(stmt.ThenStmt)
	if stmt.ElseStmt != nil {
		c.walkStmt(stmt.ElseStmt)
	}
}

func (c *escapedNameCollector) VisitWhileStmt(stmt *ast.WhileStmtNode) {
	c.walkExpr(stmt.Condition)
	c.walkStmt(stmt.Body)
}

func (c *escapedNameCollector) VisitForStmt(stmt *ast.ForStmtNode) {
	if stmt.Init != nil {
		c.walkStmt(stmt.Init)
	}
	if stmt.Condition != nil {
		c.walkExpr(stmt.Condition)
	}
	if stmt.Update != nil {
		c.walkExpr(stmt.Update)
	}
	c.walkStmt(stmt.Body)
}

func (c *escapedNameCollector) VisitImportStmt(*ast.ImportStmtNode)     {}
func (c *escapedNameCollector) VisitExportStmt(*ast.ExportStmtNode)     {}
func (c *escapedNameCollector) VisitBreakStmt(*ast.BreakStmtNode)       {}
func (c *escapedNameCollector) VisitContinueStmt(*ast.ContinueStmtNode) {}

func (c *escapedNameCollector) VisitTryCatchStmt(stmt *ast.TryCatchStmtNode) {
	c.walkStmt(stmt.TryBody)
	for _, clause := range stmt.CatchClauses {
		if clause.ErrorVar != nil {
			name := clause.ErrorVar.Name.Value
			if c.depth == 0 {
				c.topLevelNames[name] = true
			} else {
				c.declareNestedLocal(name)
			}
		}
		c.walkStmt(clause.Body)
	}
}

func (c *escapedNameCollector) VisitThrowStmt(stmt *ast.ThrowStmtNode) { c.walkExpr(stmt.Expr) }

// ---- ast.ExprVisitor[zeus_value.Value] ----

func (c *escapedNameCollector) VisitBinaryExpr(node *ast.BinaryExprNode) zeus_value.Value {
	c.walkExpr(node.Left)
	c.walkExpr(node.Right)
	return nil
}

func (c *escapedNameCollector) VisitIndexingExpression(node *ast.IndexingExprNode) zeus_value.Value {
	c.walkExpr(node.Array)
	for _, idx := range node.IndexingMeta.IndexingExprs {
		c.walkExpr(idx)
	}
	return nil
}

func (c *escapedNameCollector) VisitNumber(*ast.NumberExprNode) zeus_value.Value   { return nil }
func (c *escapedNameCollector) VisitChar(*ast.CharExprNode) zeus_value.Value       { return nil }
func (c *escapedNameCollector) VisitBoolean(*ast.BooleanExprNode) zeus_value.Value { return nil }
func (c *escapedNameCollector) VisitNull(*ast.NullExprNode) zeus_value.Value       { return nil }
func (c *escapedNameCollector) VisitStringConstant(*ast.StringConstantExprNode) zeus_value.Value {
	return nil
}
func (c *escapedNameCollector) VisitValueType(*ast.ValueTypeNode) zeus_value.Value { return nil }

func (c *escapedNameCollector) VisitUnaryExpr(node *ast.UnaryExprNode) zeus_value.Value {
	c.walkExpr(node.Expr)
	return nil
}

func (c *escapedNameCollector) VisitPostfixExpr(node *ast.PostfixExprNode) zeus_value.Value {
	c.walkExpr(node.Expr)
	return nil
}

func (c *escapedNameCollector) VisitGroupingExpr(node *ast.GroupingExprNode) zeus_value.Value {
	c.walkExpr(node.Expr)
	return nil
}

func (c *escapedNameCollector) VisitFunctionCallExpr(node *ast.FunctionCallExprNode) zeus_value.Value {
	c.walkExpr(node.Callee)
	for _, arg := range node.Params {
		c.walkExpr(arg)
	}
	return nil
}

func (c *escapedNameCollector) VisitIdentifier(node *ast.IdentifierExprNode) zeus_value.Value {
	if c.depth > 0 && !c.isLocalToNested(node.Name.Value) {
		c.nestedRefs[node.Name.Value] = true
	}
	return nil
}

func (c *escapedNameCollector) VisitFunctionDeclExpr(node *ast.FunctionDeclExprNode) zeus_value.Value {
	// Push a scope for the nested function's own params, then walk its body at a deeper depth.
	// This covers both fat-arrow expressions and named function expressions.
	c.pushNestedScope(node.Params)
	c.depth++
	c.walkStmt(node.Body)
	c.depth--
	c.popNestedScope()
	return nil
}

func (c *escapedNameCollector) VisitClassDeclExpr(*ast.ClassDeclExprNode) zeus_value.Value {
	return nil // class bodies have their own scope; don't walk method bodies
}

func (c *escapedNameCollector) VisitNewExpr(node *ast.NewExprNode) zeus_value.Value {
	c.walkExpr(node.Callee)
	for _, arg := range node.Args {
		c.walkExpr(arg)
	}
	return nil
}

func (c *escapedNameCollector) VisitObjectPropertyAccessExpr(node *ast.ObjectPropertyAccessExprNode) zeus_value.Value {
	c.walkExpr(node.Object)
	return nil
}

func (c *escapedNameCollector) VisitTernaryExpr(node *ast.TernaryExprNode) zeus_value.Value {
	c.walkExpr(node.Condition)
	c.walkExpr(node.Then)
	c.walkExpr(node.Else)
	return nil
}

// collectEscapedVarNames returns the set of variable/param names declared at the
// outermost scope of a function that are referenced inside any nested closure.
// Must be called before IR generation for the body begins.
func collectEscapedVarNames(params []*ast.VarDeclNode, body *ast.BlockStmtNode) map[string]bool {
	c := newEscapedNameCollector(params)
	c.VisitBlockStmt(body)
	escaped := make(map[string]bool)
	for name := range c.nestedRefs {
		if c.topLevelNames[name] {
			escaped[name] = true
		}
	}
	return escaped
}
