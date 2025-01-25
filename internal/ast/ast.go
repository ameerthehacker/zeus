package ast

import "ameerthehacker/zeus/internal/token"

type ExprNode interface {
	Accept(visitor Visitor[any]) any
}

type BinaryExprNode struct {
	Left     ExprNode
	Right    ExprNode
	Operator string
	Span     token.Span
}

type NumberNode struct {
	Value string
}

type Visitor[T any] interface {
	VisitBinaryExpr(node *BinaryExprNode) T
}
