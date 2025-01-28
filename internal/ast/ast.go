package ast

import (
	"ameerthehacker/zeus/internal/token"
	"fmt"
	"strings"
)

type ExprNode interface {
	GetSpan() *token.Span
	Accept(visitor Visitor[any]) any
	PrettyString() string
	String() string
}

type GroupingExprNode struct {
	Expr   *ExprNode
	Span   *token.Span
}

func (g *GroupingExprNode) PrettyString() string {
	return fmt.Sprintf("(%s)", (*g.Expr).PrettyString())
}

func (g *GroupingExprNode) String() string {
	return fmt.Sprintf("{ type: GroupingExprNode, Expr: %s, Span: %s }", (*g.Expr).String(), g.Span)
}

func (g *GroupingExprNode) GetSpan() *token.Span {
	return g.Span
}

func (g *GroupingExprNode) Accept(visitor Visitor[any]) any {
	return visitor.VisitGroupingExpr(g)
}

type BinaryExprNode struct {
	Left     *ExprNode
	Right    *ExprNode
	Operator *token.Token
}

func (b *BinaryExprNode) GetSpan() *token.Span {
	startPosition := (*b.Left).GetSpan().Start
	endPosition := (*b.Right).GetSpan().End
	return &token.Span{Start: startPosition, End: endPosition}
}

func (b *BinaryExprNode) PrettyString() string {
	return fmt.Sprintf("(%s %s %s)", (*b.Left).PrettyString(), b.Operator.Type, (*b.Right).PrettyString())
}

func (b *BinaryExprNode) String() string {
	return fmt.Sprintf("{ type: BinaryExprNode, Left: %s, Right: %s, Operator: %s, Span: %s }", (*b.Left).String(), (*b.Right).String(), b.Operator.Type, b.GetSpan())
}

func (b *BinaryExprNode) Accept(visitor Visitor[any]) any {
	return visitor.VisitBinaryExpr(b)
}

type NumberNode struct {
	Value *token.Token
}

func (n *NumberNode) GetSpan() *token.Span {
	return n.Value.Span
}

func (n *NumberNode) Accept(visitor Visitor[any]) any {
	return visitor.VisitNumber(n)
}

func (n *NumberNode) PrettyString() string {
	return *n.Value.Value
}

func (n *NumberNode) String() string {
	return fmt.Sprintf("{ type: NumberNode, Value: %s, Span: %s }", n.Value, n.GetSpan())
}

type UnaryExprNode struct {
	Operator *token.Token
	Operand  *ExprNode
}

func (u *UnaryExprNode) PrettyString() string {
	return fmt.Sprintf("(%s%s)", u.Operator.Type, (*u.Operand).PrettyString())
}

func (u *UnaryExprNode) String() string {
	return fmt.Sprintf("{ type: UnaryExprNode, Operator: %s, Operand: %s, Span: %s }", u.Operator.Type, (*u.Operand).String(), u.GetSpan())
}

func (u *UnaryExprNode) Accept(visitor Visitor[any]) any {
	return visitor.VisitUnaryExpr(u)
}

func (u *UnaryExprNode) GetSpan() *token.Span {
	startPosition := u.Operator.Span.Start
	endPosition := (*u.Operand).GetSpan().End
	return &token.Span{Start: startPosition, End: endPosition}
}

type IdentifierNode struct {
	Name *token.Token
}

func (i *IdentifierNode) GetSpan() *token.Span {
	return i.Name.Span
}

func (i *IdentifierNode) PrettyString() string {
	return *i.Name.Value
}

func (i *IdentifierNode) String() string {
	return fmt.Sprintf("{ type: IdentifierNode, Name: %s, Span: %s }", i.Name, i.GetSpan())
}

func (i *IdentifierNode) Accept(visitor Visitor[any]) any {
	return visitor.VisitIdentifier(i)
}

type FunctionCallExprNode struct {
	Callee *ExprNode
	Params []*ExprNode
	Span   *token.Span
}

func (f *FunctionCallExprNode) GetSpan() *token.Span {
	return f.Span
}

func (f *FunctionCallExprNode) PrettyString() string {
	params := []string{}
	for _, param := range f.Params {
		params = append(params, (*param).PrettyString())
	}
	return fmt.Sprintf("%s(%s)", (*f.Callee).PrettyString(), strings.Join(params, ", "))
}

func (f *FunctionCallExprNode) String() string {
	params := []string{}
	for _, param := range f.Params {
		params = append(params, (*param).String())
	}
	return fmt.Sprintf("{ type: FunctionCallExprNode, Callee: %s, Params: [%s], Span: %s }", (*f.Callee).String(), strings.Join(params, ", "), f.GetSpan())
}

func (f *FunctionCallExprNode) Accept(visitor Visitor[any]) any {
	return visitor.VisitFunctionCallExpr(f)
}

type Visitor[T any] interface {
	VisitBinaryExpr(node *BinaryExprNode) T
	VisitNumber(node *NumberNode) T
	VisitUnaryExpr(node *UnaryExprNode) T
	VisitIdentifier(node *IdentifierNode) T
	VisitGroupingExpr(node *GroupingExprNode) T
	VisitFunctionCallExpr(node *FunctionCallExprNode) T
}
