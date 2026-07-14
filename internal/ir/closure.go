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

// ─────────────────────────────────────────────────────────────────────────────
// astWalker — shared traversal base for closure analysis passes
// ─────────────────────────────────────────────────────────────────────────────

// closureVisitor combines the two visitor interfaces so astWalker can dispatch
// through the concrete outer type, giving correct virtual-method behaviour.
type closureVisitor interface {
	ast.StmtVisitor
	ast.ExprVisitor[zeus_value.Value]
}

// astWalker provides default visitor implementations that recurse into children.
// Concrete types embed astWalker and set self = themselves after construction,
// then override only the methods that carry analysis-specific logic.
// All walkExpr / walkStmt calls go through self so overridden methods are reached.
type astWalker struct {
	self closureVisitor
}

func (w *astWalker) walkExpr(expr ast.ExprNode) {
	if expr != nil {
		expr.Accept(w.self)
	}
}

func (w *astWalker) walkStmt(stmt ast.StmtNode) {
	if stmt != nil {
		stmt.Accept(w.self)
	}
}

// ---- ast.StmtVisitor defaults ----

func (w *astWalker) VisitExprStmt(stmt *ast.ExprStmtNode) { w.walkExpr(stmt.Expr) }

func (w *astWalker) VisitBlockStmt(stmt *ast.BlockStmtNode) {
	for _, s := range stmt.Statements {
		w.walkStmt(s)
	}
}

func (w *astWalker) VisitReturnStmt(stmt *ast.ReturnStmtNode) {
	if stmt.Expr != nil {
		w.walkExpr(stmt.Expr)
	}
}

func (w *astWalker) VisitIfStmt(stmt *ast.IfStmtNode) {
	w.walkExpr(stmt.Condition)
	w.walkStmt(stmt.ThenStmt)
	if stmt.ElseStmt != nil {
		w.walkStmt(stmt.ElseStmt)
	}
}

func (w *astWalker) VisitWhileStmt(stmt *ast.WhileStmtNode) {
	w.walkExpr(stmt.Condition)
	w.walkStmt(stmt.Body)
}

func (w *astWalker) VisitForStmt(stmt *ast.ForStmtNode) {
	if stmt.Init != nil {
		w.walkStmt(stmt.Init)
	}
	if stmt.Condition != nil {
		w.walkExpr(stmt.Condition)
	}
	if stmt.Update != nil {
		w.walkExpr(stmt.Update)
	}
	w.walkStmt(stmt.Body)
}

func (w *astWalker) VisitThrowStmt(stmt *ast.ThrowStmtNode) { w.walkExpr(stmt.Expr) }

func (w *astWalker) VisitImportStmt(*ast.ImportStmtNode)     {}
func (w *astWalker) VisitExportStmt(*ast.ExportStmtNode)     {}
func (w *astWalker) VisitBreakStmt(*ast.BreakStmtNode)       {}
func (w *astWalker) VisitContinueStmt(*ast.ContinueStmtNode) {}

// ---- ast.ExprVisitor[zeus_value.Value] defaults ----

func (w *astWalker) VisitBinaryExpr(node *ast.BinaryExprNode) zeus_value.Value {
	w.walkExpr(node.Left)
	w.walkExpr(node.Right)
	return nil
}

func (w *astWalker) VisitIndexingExpression(node *ast.IndexingExprNode) zeus_value.Value {
	w.walkExpr(node.Array)
	for _, idx := range node.IndexingMeta.IndexingExprs {
		w.walkExpr(idx)
	}
	return nil
}

func (w *astWalker) VisitUnaryExpr(node *ast.UnaryExprNode) zeus_value.Value {
	w.walkExpr(node.Expr)
	return nil
}

func (w *astWalker) VisitPostfixExpr(node *ast.PostfixExprNode) zeus_value.Value {
	w.walkExpr(node.Expr)
	return nil
}

func (w *astWalker) VisitGroupingExpr(node *ast.GroupingExprNode) zeus_value.Value {
	w.walkExpr(node.Expr)
	return nil
}

func (w *astWalker) VisitFunctionCallExpr(node *ast.FunctionCallExprNode) zeus_value.Value {
	w.walkExpr(node.Callee)
	for _, arg := range node.Params {
		w.walkExpr(arg)
	}
	return nil
}

func (w *astWalker) VisitNewExpr(node *ast.NewExprNode) zeus_value.Value {
	w.walkExpr(node.Callee)
	for _, arg := range node.Args {
		w.walkExpr(arg)
	}
	return nil
}

func (w *astWalker) VisitArrayLiteral(node *ast.ArrayLiteralExprNode) zeus_value.Value {
	for _, el := range node.Elements {
		w.walkExpr(el)
	}
	return nil
}

func (w *astWalker) VisitObjectPropertyAccessExpr(node *ast.ObjectPropertyAccessExprNode) zeus_value.Value {
	w.walkExpr(node.Object)
	return nil
}

func (w *astWalker) VisitTernaryExpr(node *ast.TernaryExprNode) zeus_value.Value {
	w.walkExpr(node.Condition)
	w.walkExpr(node.Then)
	w.walkExpr(node.Else)
	return nil
}

func (w *astWalker) VisitCastExpr(node *ast.CastExprNode) zeus_value.Value {
	w.walkExpr(node.Expr)
	return nil
}

func (w *astWalker) VisitClassDeclExpr(*ast.ClassDeclExprNode) zeus_value.Value {
	return nil // class bodies have their own scope; don't walk method bodies
}

func (w *astWalker) VisitInterfaceDeclExpr(*ast.InterfaceDeclExprNode) zeus_value.Value {
	return nil // interfaces are type-level only; no runtime sub-expressions to walk
}

func (w *astWalker) VisitNumber(*ast.NumberExprNode) zeus_value.Value                 { return nil }
func (w *astWalker) VisitChar(*ast.CharExprNode) zeus_value.Value                     { return nil }
func (w *astWalker) VisitBoolean(*ast.BooleanExprNode) zeus_value.Value               { return nil }
func (w *astWalker) VisitNull(*ast.NullExprNode) zeus_value.Value                     { return nil }
func (w *astWalker) VisitStringConstant(*ast.StringConstantExprNode) zeus_value.Value { return nil }
func (w *astWalker) VisitValueType(*ast.ValueTypeNode) zeus_value.Value               { return nil }
func (w *astWalker) VisitTemplateString(node *ast.TemplateStringExprNode) zeus_value.Value {
	for _, part := range node.Parts {
		if part.IsExpr {
			w.walkExpr(part.Expr)
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// freeVarCollector — finds variables referenced inside a closure body that
// are defined in the enclosing (non-global) scope and must be captured.
// Must be run while the symbol table still reflects the enclosing scope.
// ─────────────────────────────────────────────────────────────────────────────

type freeVarCollector struct {
	astWalker
	module      *IRModule
	localScopes []map[string]bool // stack; level 0 = current fn's own params + locals
	captured    map[string]*CapturedVar
}

func newFreeVarCollector(module *IRModule, ownParams []*ast.VarDeclNode) *freeVarCollector {
	top := make(map[string]bool, len(ownParams))
	for _, p := range ownParams {
		top[p.Identifier.Name.Value] = true
	}
	f := &freeVarCollector{
		module:      module,
		localScopes: []map[string]bool{top},
		captured:    make(map[string]*CapturedVar),
	}
	f.astWalker.self = f
	return f
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

func (f *freeVarCollector) VisitVarDeclStmt(stmt *ast.VarDeclStmtNode) {
	for _, decl := range stmt.Decls {
		if decl.Initializer != nil {
			f.walkExpr(decl.Initializer)
		}
		f.declareLocal(decl.Identifier.Name.Value)
	}
}

func (f *freeVarCollector) VisitTryCatchStmt(stmt *ast.TryCatchStmtNode) {
	f.walkStmt(stmt.TryBody)
	for _, clause := range stmt.CatchClauses {
		if clause.ErrorVar != nil {
			f.declareLocal(clause.ErrorVar.Name.Value)
		}
		f.walkStmt(clause.Body)
	}
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

// collectFreeVars analyzes a function body AST and returns the variables that
// must be captured from the enclosing scope. Must be called before any IR is
// emitted for the body (symbol table still reflects the enclosing scope).
func (g *IRModule) collectFreeVars(fnParams []*ast.VarDeclNode, body *ast.BlockStmtNode) []*CapturedVar {
	collector := newFreeVarCollector(g, fnParams)
	collector.VisitBlockStmt(body)
	return collector.result()
}

// ─────────────────────────────────────────────────────────────────────────────
// escapedNameCollector — finds variable/param names at the outermost scope of a
// function that are referenced inside any nested closure, so they can be promoted
// to heap ref cells for shared mutable capture.
// ─────────────────────────────────────────────────────────────────────────────

type escapedNameCollector struct {
	astWalker
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
	c := &escapedNameCollector{
		topLevelNames: topLevel,
		nestedRefs:    make(map[string]bool),
	}
	c.astWalker.self = c
	return c
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
