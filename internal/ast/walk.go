package ast

import (
	"reflect"

	"github.com/ameerthehacker/zeus/internal/token"
)

// This file gives the AST a positional query. Every syntactic construct carries a source span
// (GetSpan), but nothing let a caller ask "what node is at line X, column Y?". Walk/Children
// provide a uniform traversal, and NodeAt/EnclosingPath answer that question — so tooling (the
// language server) can map a cursor to the smallest enclosing node instead of scraping raw text.

// Node is any AST construct that carries a source span. Statements, expressions, the program
// root, and the declaration sub-nodes (params, class/interface members, catch clauses) all
// satisfy it, so a single traversal can cover the whole tree.
type Node interface {
	GetSpan() *token.Span
}

// GetSpan methods for the member sub-nodes that only stored a Span field, so the position
// walker can treat them as first-class Nodes and descend into them.
func (c *ClassProperty) GetSpan() *token.Span              { return c.Span }
func (c *ClassMethod) GetSpan() *token.Span                { return c.Span }
func (i *InterfacePropertySignature) GetSpan() *token.Span { return i.Span }
func (i *InterfaceMethodSignature) GetSpan() *token.Span   { return i.Span }

// isNilNode reports whether n is a nil interface or an interface wrapping a nil pointer. The
// parser leaves typed-nil children behind during error recovery, so every descent must guard
// against them; reflection keeps this correct without enumerating every concrete node type.
func isNilNode(n Node) bool {
	if n == nil {
		return true
	}
	v := reflect.ValueOf(n)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

// Walk traverses the tree rooted at n in preorder, calling visit for each (non-nil) node. If
// visit returns false, Walk does not descend into that node's children.
func Walk(n Node, visit func(Node) bool) {
	if isNilNode(n) {
		return
	}
	if !visit(n) {
		return
	}
	for _, c := range Children(n) {
		Walk(c, visit)
	}
}

// Children returns the direct child nodes of n in source order, with nil/typed-nil children
// omitted. Container structs that do not themselves carry meaningful position semantics beyond
// their members (e.g. IndexingMeta, template parts) are flattened into those members.
func Children(n Node) []Node {
	var out []Node
	add := func(c Node) {
		if !isNilNode(c) {
			out = append(out, c)
		}
	}

	switch e := n.(type) {
	// ----- program root -----
	case *ProgramNode:
		for _, s := range e.Statements {
			add(s)
		}

	// ----- expressions -----
	case *GroupingExprNode:
		add(e.Expr)
	case *BinaryExprNode:
		add(e.Left)
		add(e.Right)
	case *UnaryExprNode:
		add(e.Expr)
	case *PostfixExprNode:
		add(e.Expr)
	case *FunctionCallExprNode:
		add(e.Callee)
		for _, p := range e.Params {
			add(p)
		}
	case *FunctionDeclExprNode:
		add(e.Name)
		for _, p := range e.Params {
			add(p)
		}
		add(e.ReturnType)
		add(e.Body)
	case *IndexingExprNode:
		add(e.Array)
		for _, idx := range e.IndexingMeta.IndexingExprs {
			add(idx)
		}
	case *ClassDeclExprNode:
		add(e.Name)
		add(e.ParentClass)
		for _, impl := range e.Implements {
			add(impl)
		}
		for _, p := range e.Properties {
			add(p)
		}
		for _, m := range e.Methods {
			add(m)
		}
	case *NewExprNode:
		add(e.Callee)
		for _, a := range e.Args {
			add(a)
		}
	case *ArrayLiteralExprNode:
		for _, el := range e.Elements {
			add(el)
		}
	case *ObjectPropertyAccessExprNode:
		add(e.Object)
		add(e.Property)
	case *InterfaceDeclExprNode:
		add(e.Name)
		for _, parent := range e.Parents {
			add(parent)
		}
		for _, p := range e.Properties {
			add(p)
		}
		for _, m := range e.Methods {
			add(m)
		}
	case *TemplateStringExprNode:
		for _, part := range e.Parts {
			if part.IsExpr {
				add(part.Expr)
			}
		}
	case *TernaryExprNode:
		add(e.Condition)
		add(e.Then)
		add(e.Else)
	case *CastExprNode:
		add(e.Expr)
		add(e.AsType)

	// ----- statements -----
	case *ExprStmtNode:
		add(e.Expr)
	case *VarDeclStmtNode:
		for i := range e.Decls {
			add(&e.Decls[i])
		}
	case *BlockStmtNode:
		for _, s := range e.Statements {
			add(s)
		}
	case *ReturnStmtNode:
		add(e.Expr)
	case *IfStmtNode:
		add(e.Condition)
		add(e.ThenStmt)
		add(e.ElseStmt)
	case *WhileStmtNode:
		add(e.Condition)
		add(e.Body)
	case *ForStmtNode:
		add(e.Init)
		add(e.Condition)
		add(e.Update)
		add(e.Body)
	case *ImportStmtNode:
		for _, imp := range e.Imports {
			add(imp)
		}
	case *ExportStmtNode:
		add(e.Expr)
	case *TryCatchStmtNode:
		add(e.TryBody)
		for _, clause := range e.CatchClauses {
			add(clause)
		}
	case *ThrowStmtNode:
		add(e.Expr)

	// ----- declaration sub-nodes -----
	case *VarDeclNode:
		add(e.Identifier)
		add(e.ValueType)
		add(e.Initializer)
	case *CatchClause:
		add(e.ErrorVar)
		add(e.ErrorType)
		add(e.Body)
	case *ClassProperty:
		add(e.Name)
		add(e.ValueType)
		add(e.Initializer)
	case *ClassMethod:
		add(e.Name)
		for _, p := range e.Params {
			add(p)
		}
		add(e.ReturnType)
		add(e.Body)
	case *InterfacePropertySignature:
		add(e.Name)
		add(e.ValueType)
	case *InterfaceMethodSignature:
		add(e.Name)
		for _, p := range e.Params {
			add(p)
		}
		add(e.ReturnType)

		// leaves (Number, Boolean, Identifier, Null, Char, StringConstant, ValueType, Break,
		// Continue) have no child nodes and fall through to the empty result.
	}

	return out
}

// posLess reports whether position a is strictly before b.
func posLess(a, b token.Position) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Column < b.Column
}

// spanContains reports whether pos lies within span (inclusive of both ends — Zeus spans use a
// 1-based, inclusive-end convention).
func spanContains(span *token.Span, pos token.Position) bool {
	if span == nil {
		return false
	}
	return !posLess(pos, span.Start) && !posLess(span.End, pos)
}

// spanTighter reports whether span a is smaller (more specific) than span b, comparing line
// extent first, then column extent. Used to pick the most specific child when spans overlap.
func spanTighter(a, b *token.Span) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	if al, bl := a.End.Line-a.Start.Line, b.End.Line-b.Start.Line; al != bl {
		return al < bl
	}
	return (a.End.Column - a.Start.Column) < (b.End.Column - b.Start.Column)
}

// EnclosingPath returns the chain of nodes enclosing pos within the subtree rooted at root,
// ordered innermost-first (the tightest enclosing node at index 0). It returns nil when pos is
// outside root. At each level it descends into the tightest child that still contains pos.
func EnclosingPath(root Node, pos token.Position) []Node {
	if isNilNode(root) || !spanContains(root.GetSpan(), pos) {
		return nil
	}
	var outer []Node
	for cur := root; !isNilNode(cur); {
		outer = append(outer, cur)
		var next Node
		for _, c := range Children(cur) {
			if isNilNode(c) || !spanContains(c.GetSpan(), pos) {
				continue
			}
			if next == nil || spanTighter(c.GetSpan(), next.GetSpan()) {
				next = c
			}
		}
		cur = next
	}
	// Reverse to innermost-first.
	for i, j := 0, len(outer)-1; i < j; i, j = i+1, j-1 {
		outer[i], outer[j] = outer[j], outer[i]
	}
	return outer
}

// NodeAt returns the enclosing-node chain (innermost-first) for pos in program, or nil when the
// position falls outside every statement. The ProgramNode itself is not included — its span
// excludes leading/inter-statement whitespace — so the walk starts from the containing statement.
func NodeAt(program *ProgramNode, pos token.Position) []Node {
	if program == nil {
		return nil
	}
	for _, stmt := range program.Statements {
		if spanContains(stmt.GetSpan(), pos) {
			return EnclosingPath(stmt, pos)
		}
	}
	return nil
}

// InnermostAt returns the tightest node enclosing pos, or nil if none.
func InnermostAt(program *ProgramNode, pos token.Position) Node {
	path := NodeAt(program, pos)
	if len(path) == 0 {
		return nil
	}
	return path[0]
}
