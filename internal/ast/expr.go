package ast

import (
	"ameerthehacker/zeus/internal/token"
	"fmt"
)

type ExprNode interface {
	Accept(visitor Visitor[any]) any
	String() string
}

type BinaryExprNode struct {
	Left     *ExprNode
	Right    *ExprNode
	Operator token.TokenType
	Span     *token.Span
}

func (b *BinaryExprNode) String() string {
	return fmt.Sprintf("(%s %s %s)", (*b.Left).String(), b.Operator, (*b.Right).String())
}

func (b *BinaryExprNode) Accept(visitor Visitor[any]) any {
	return visitor.VisitBinaryExpr(b)
}

type NumberNode struct {
	Value string
	Span  *token.Span
}

func (n *NumberNode) Accept(visitor Visitor[any]) any {
	return visitor.VisitNumber(n)
}

func (n *NumberNode) String() string {
	return n.Value
}

type UnaryExprNode struct {
	Operator token.TokenType
	Operand  *ExprNode
	Span     *token.Span
}

func (u *UnaryExprNode) String() string {
	return fmt.Sprintf("(%s %s)", u.Operator, (*u.Operand).String())
}

func (u *UnaryExprNode) Accept(visitor Visitor[any]) any {
	return visitor.VisitUnaryExpr(u)
}

type IdentifierNode struct {
	Value string
	Span   *token.Span
}

func (i *IdentifierNode) String() string {
	return i.Value
}

func (i *IdentifierNode) Accept(visitor Visitor[any]) any {
	return visitor.VisitIdentifier(i)
}

type Visitor[T any] interface {
	VisitBinaryExpr(node *BinaryExprNode) T
	VisitNumber(node *NumberNode) T
	VisitUnaryExpr(node *UnaryExprNode) T
	VisitIdentifier(node *IdentifierNode) T
}
