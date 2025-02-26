package ast

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
)

type VarDeclType int

const (
	VarDeclTypeLet VarDeclType = iota
	VarDeclTypeConst
)

func (v VarDeclType) String() string {
	switch v {
	case VarDeclTypeLet:
		return "let"
	case VarDeclTypeConst:
		return "const"
	}

	panic("unknown var decl type")
}

type StmtNode interface {
	String() string
	PrettyString() string
	GetSpan() *token.Span
	Accept(visitor StmtVisitor)
}

type VarDeclNode struct {
	Identifier *IdentifierExprNode
	DataType *token.Token
	DeclType VarDeclType
	Initializer ExprNode
}

type VarDeclStmtNode struct {
	Decls []VarDeclNode
	Span *token.Span
}

type BlockStmtNode struct {
	Statements []StmtNode
	Span *token.Span
}

type ReturnStmtNode struct {
	Expr ExprNode
	Span *token.Span
}

type IfStmtNode struct {
	Condition ExprNode
	ThenStmt StmtNode
	ElseStmt StmtNode
	Span *token.Span
}

type WhileStmtNode struct {
	Condition ExprNode
	Body StmtNode
	Span *token.Span
}

type ImportStmtNode struct {
	Source *token.Token
	Imports []ExprNode
	Span *token.Span
}

type ExportStmtNode struct {
	Expr ExprNode
	Span *token.Span
}

type StmtVisitor interface {
	VisitExprStmt(stmt *ExprStmtNode)
	VisitVarDeclStmt(stmt *VarDeclStmtNode)
	VisitBlockStmt(stmt *BlockStmtNode)
	VisitReturnStmt(stmt *ReturnStmtNode)
	VisitIfStmt(stmt *IfStmtNode)
	VisitWhileStmt(stmt *WhileStmtNode)
	VisitImportStmt(stmt *ImportStmtNode)
	VisitExportStmt(stmt *ExportStmtNode)
}

type ExprStmtNode struct {
	Expr ExprNode
}

func (e *ExprStmtNode) String() string {
	return fmt.Sprintf("{ type: ExprStmtNode, Expr: %s, Span: %s }", e.Expr.String(), e.Expr.GetSpan())
}

func (e *ExprStmtNode) PrettyString() string {
	return e.Expr.PrettyString()
}

func (e *ExprStmtNode) GetSpan() *token.Span {
	return e.Expr.GetSpan()
}

func (e *ExprStmtNode) Accept(visitor StmtVisitor) {
	visitor.VisitExprStmt(e)
}

func (v *VarDeclNode) String() string {
	if v.Initializer != nil {
		return fmt.Sprintf("{ type: VarDeclNode, Identifier: %s, DeclType: %s, Initializer: %s, DataType: %s, Span: %s }", v.Identifier.String(), v.DeclType, v.Initializer.String(), v.DataType.String(), v.Identifier.GetSpan())
	}

	return fmt.Sprintf("{ type: VarDeclNode, Identifier: %s, DeclType: %s, DataType: %s, Span: %s }", v.Identifier.String(), v.DeclType, v.DataType.String(), v.Identifier.GetSpan())
}

func (v *VarDeclNode) PrettyString() string {
	if v.Initializer != nil {
		return fmt.Sprintf("%s %s: %s = %s", v.DeclType, v.Identifier.PrettyString(), v.DataType.Type, v.Initializer.PrettyString())
	}

	return fmt.Sprintf("%s %s: %s", v.DeclType, v.Identifier.PrettyString(), v.DataType.Type)
}

func (v *VarDeclNode) GetSpan() *token.Span {
	startPosition := v.Identifier.GetSpan().Start
	endPosition := v.DataType.Span.End

	if v.Initializer != nil {
		endPosition = v.Initializer.GetSpan().End
	}

	return &token.Span{Start: startPosition, End: endPosition}
}

func (b *BlockStmtNode) String() string {
	statements := []string{}
	for _, stmt := range b.Statements {
		statements = append(statements, stmt.String())
	}

	return fmt.Sprintf("{ type: BlockStmtNode, Statements: %s, Span: %s }", strings.Join(statements, ", "), b.Span)
}

func (b *BlockStmtNode) PrettyString() string {
	statements := []string{}
	for _, stmt := range b.Statements {
		statements = append(statements, "\t" + stmt.PrettyString())
	}
	return fmt.Sprintf("{\n%s\n}", strings.Join(statements, "\n"))
}

func (b *BlockStmtNode) GetSpan() *token.Span {
	return b.Span
}

func (b *BlockStmtNode) Accept(visitor StmtVisitor) {
	visitor.VisitBlockStmt(b)
}

func (v *VarDeclStmtNode) GetSpan() *token.Span {
	return v.Span
}

func (v *VarDeclStmtNode) Accept(visitor StmtVisitor) {
	visitor.VisitVarDeclStmt(v)
}

func (v *VarDeclStmtNode) String() string {
	decls := []string{}
	for _, decl := range v.Decls {
		decls = append(decls, decl.String())
	}

	return fmt.Sprintf("{ type: VarDeclStmtNode, Decls: %s, Span: %s }", strings.Join(decls, ", "), v.GetSpan())
}

func (v *VarDeclStmtNode) PrettyString() string {
	decls := []string{}
	for _, decl := range v.Decls {
		decls = append(decls, decl.PrettyString())
	}

	return strings.Join(decls, "\n")
}

func (r *ReturnStmtNode) String() string {
	if r.Expr == nil {
		return fmt.Sprintf("{ type: ReturnStmtNode, Expr: nil, Span: %s }", r.Span)
	}

	return fmt.Sprintf("{ type: ReturnStmtNode, Expr: %s, Span: %s }", r.Expr.String(), r.Span)
}

func (r *ReturnStmtNode) PrettyString() string {
	if r.Expr == nil {
		return "return"
	}

	return fmt.Sprintf("return %s", r.Expr.PrettyString())
}

func (r *ReturnStmtNode) GetSpan() *token.Span {
	return r.Span
}

func (r *ReturnStmtNode) Accept(visitor StmtVisitor) {
	visitor.VisitReturnStmt(r)
}

func (i *IfStmtNode) GetSpan() *token.Span {
	return i.Span
}

func (i *IfStmtNode) Accept(visitor StmtVisitor) {
	visitor.VisitIfStmt(i)
}

func (i *IfStmtNode) String() string {
	if i.ElseStmt == nil {
		return fmt.Sprintf("{ type: IfStmtNode, Condition: %s, ThenStmt: %s, ElseStmt: nil, Span: %s }", i.Condition.String(), i.ThenStmt.String(), i.Span)
	}

	return fmt.Sprintf("{ type: IfStmtNode, Condition: %s, ThenStmt: %s, ElseStmt: %s, Span: %s }", i.Condition.String(), i.ThenStmt.String(), i.ElseStmt.String(), i.Span)
}

func (i *IfStmtNode) PrettyString() string {
	if i.ElseStmt == nil {
		return fmt.Sprintf("if (%s) {\n%s\n}", i.Condition.PrettyString(), i.ThenStmt.PrettyString())
	}

	return fmt.Sprintf("if (%s) {\n%s\n} else {\n%s\n}", i.Condition.PrettyString(), i.ThenStmt.PrettyString(), i.ElseStmt.PrettyString())
}

func (w *WhileStmtNode) GetSpan() *token.Span {
	return w.Span
}

func (w *WhileStmtNode) Accept(visitor StmtVisitor) {
	visitor.VisitWhileStmt(w)
}

func (w *WhileStmtNode) String() string {
	return fmt.Sprintf("{ type: WhileStmtNode, Condition: %s, Body: %s, Span: %s }", w.Condition.String(), w.Body.String(), w.Span)
}

func (w *WhileStmtNode) PrettyString() string {
	return fmt.Sprintf("while (%s) {\n%s\n}", w.Condition.PrettyString(), w.Body.PrettyString())
}

func (i *ImportStmtNode) GetSpan() *token.Span {
	return i.Span
}

func (i *ImportStmtNode) Accept(visitor StmtVisitor) {
	visitor.VisitImportStmt(i)
}

func (i *ImportStmtNode) String() string {
	return fmt.Sprintf("{ type: ImportStmtNode, Source: %s, Imports: %s, Span: %s }", i.Source, i.Imports, i.Span)
}

func (i *ImportStmtNode) PrettyString() string {
	imports := []string{}

	for _, _import := range i.Imports {
		imports = append(imports, _import.PrettyString())
	}

	return fmt.Sprintf("import { %s } from %s", strings.Join(imports, ", "), i.Source)
}

func (e *ExportStmtNode) GetSpan() *token.Span {
	return e.Span
}

func (e *ExportStmtNode) Accept(visitor StmtVisitor) {
	visitor.VisitExportStmt(e)
}

func (e *ExportStmtNode) String() string {
	return fmt.Sprintf("{ type: ExportStmtNode, Exprs: %s, Span: %s }", e.Expr, e.Span)
}

func (e *ExportStmtNode) PrettyString() string {
	return fmt.Sprintf("export %s", e.Expr)
}
