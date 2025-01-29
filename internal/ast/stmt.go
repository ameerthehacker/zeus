package ast

import (
	"fmt"

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
	Accept(visitor StmtVisitor[any]) any
}

type StmtVisitor[T any] interface {
	VisitExprStmt(stmt *ExprStmtNode) T
	VisitVarDecl(stmt *VarDeclNode) T
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

func (e *ExprStmtNode) Accept(visitor StmtVisitor[any]) any {
	return visitor.VisitExprStmt(e)
}

type VarDeclNode struct {
	Identifier *IdentifierExprNode
	DataType *token.Token
	DeclType VarDeclType
	Initializer ExprNode
	Span *token.Span
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
	return v.Span
}

func (v *VarDeclNode) Accept(visitor StmtVisitor[any]) any {
	return visitor.VisitVarDecl(v)
}
