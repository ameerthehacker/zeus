package ast

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/value"
)

type ExprNode interface {
	GetSpan() *token.Span
	Accept(visitor ExprVisitor[value.Value]) value.Value
	PrettyString() string
	String() string
}

type GroupingExprNode struct {
	Expr   ExprNode
	Span   *token.Span
}

type BinaryExprNode struct {
	Left     ExprNode
	Right    ExprNode
	Operator *token.Token
}

type NumberExprNode struct {
	Value *token.Token
}

type UnaryExprNode struct {
	Operator *token.Token
	Expr  ExprNode
}

type IdentifierExprNode struct {
	Name *token.Token
}

type FunctionCallExprNode struct {
	Callee ExprNode
	Params []ExprNode
	Span   *token.Span
}

type FunctionDeclExprNode struct {
	Name *IdentifierExprNode
	Params []*VarDeclNode
	Body *BlockStmtNode
	ReturnType *token.Token
	Span *token.Span
}

func exprNodesString(params []ExprNode, pretty bool) string {
	paramsStr := []string{}
	for _, param := range params {
		if pretty {
			paramsStr = append(paramsStr, param.PrettyString())
		} else {
			paramsStr = append(paramsStr, param.String())
		}
	}
	return strings.Join(paramsStr, ", ")
}

func varDeclsString(params []*VarDeclNode, pretty bool) string {
	paramsStr := []string{}
	for _, param := range params {
		if pretty {
			paramsStr = append(paramsStr, param.PrettyString())
		} else {
			paramsStr = append(paramsStr, param.String())
		}
	}
	return strings.Join(paramsStr, ", ")
}


func (g *GroupingExprNode) PrettyString() string {
	return fmt.Sprintf("(%s)", g.Expr.PrettyString())
}

func (g *GroupingExprNode) String() string {
	return fmt.Sprintf("{ type: GroupingExprNode, Expr: %s, Span: %s }", g.Expr.String(), g.Span)
}

func (g *GroupingExprNode) GetSpan() *token.Span {
	return g.Span
}

func (g *GroupingExprNode) Accept(visitor ExprVisitor[value.Value]) value.Value {
	return visitor.VisitGroupingExpr(g)
}

func (b *BinaryExprNode) GetSpan() *token.Span {
	startPosition := b.Left.GetSpan().Start
	endPosition := b.Right.GetSpan().End
	return &token.Span{Start: startPosition, End: endPosition}
}

func (b *BinaryExprNode) PrettyString() string {
	return fmt.Sprintf("(%s %s %s)", b.Left.PrettyString(), b.Operator.Type, b.Right.PrettyString())
}

func (b *BinaryExprNode) String() string {
	return fmt.Sprintf("{ type: BinaryExprNode, Left: %s, Right: %s, Operator: %s, Span: %s }", b.Left.String(), b.Right.String(), b.Operator.Type, b.GetSpan())
}

func (b *BinaryExprNode) Accept(visitor ExprVisitor[value.Value]) value.Value {
	return visitor.VisitBinaryExpr(b)
}

func (n *NumberExprNode) GetSpan() *token.Span {
	return n.Value.Span
}

func (n *NumberExprNode) Accept(visitor ExprVisitor[value.Value]) value.Value {
	return visitor.VisitNumber(n)
}

func (n *NumberExprNode) PrettyString() string {
	return n.Value.Value
}

func (n *NumberExprNode) String() string {
	return fmt.Sprintf("{ type: NumberNode, Value: %s, Span: %s }", n.Value, n.GetSpan())
}

func (u *UnaryExprNode) PrettyString() string {
	return fmt.Sprintf("(%s%s)", u.Operator.Type, u.Expr.PrettyString())
}

func (u *UnaryExprNode) String() string {
	return fmt.Sprintf("{ type: UnaryExprNode, Operator: %s, Operand: %s, Span: %s }", u.Operator.Type, u.Expr.String(), u.GetSpan())
}

func (u *UnaryExprNode) Accept(visitor ExprVisitor[value.Value]) value.Value {
	return visitor.VisitUnaryExpr(u)
}

func (u *UnaryExprNode) GetSpan() *token.Span {
	startPosition := u.Operator.Span.Start
	endPosition := u.Expr.GetSpan().End
	return &token.Span{Start: startPosition, End: endPosition}
}

func (i *IdentifierExprNode) GetSpan() *token.Span {
	return i.Name.Span
}

func (i *IdentifierExprNode) PrettyString() string {
	return i.Name.Value
}

func (i *IdentifierExprNode) String() string {
	return fmt.Sprintf("{ type: IdentifierNode, Name: %s, Span: %s }", i.Name, i.GetSpan())
}

func (i *IdentifierExprNode) Accept(visitor ExprVisitor[value.Value]) value.Value {
	return visitor.VisitIdentifier(i)
}

func (f *FunctionCallExprNode) GetSpan() *token.Span {
	return f.Span
}

func (f *FunctionCallExprNode) PrettyString() string {
	return fmt.Sprintf("%s(%s)", f.Callee.PrettyString(), exprNodesString(f.Params, true))
}

func (f *FunctionCallExprNode) String() string {
	return fmt.Sprintf("{ type: FunctionCallExprNode, Callee: %s, Params: [%s], Span: %s }", f.Callee.String(), exprNodesString(f.Params, false), f.GetSpan())
}

func (f *FunctionCallExprNode) Accept(visitor ExprVisitor[value.Value]) value.Value {
	return visitor.VisitFunctionCallExpr(f)
}

func (f *FunctionDeclExprNode) GetSpan() *token.Span {
	return f.Span
}

func (f *FunctionDeclExprNode) PrettyString() string {
	head := fmt.Sprintf("function %s(%s): %s", f.Name.PrettyString(), varDeclsString(f.Params, true), f.ReturnType.Type)
	body := f.Body.PrettyString()

	return fmt.Sprintf("%s\n%s\n", head, body)
}

func (f *FunctionDeclExprNode) String() string {
	return fmt.Sprintf("{ type: FunctionDeclExprNode, Name: %s, Params: [%s], ReturnType: %s, Body: %s, Span: %s }", f.Name.String(), varDeclsString(f.Params, false), f.ReturnType.Type, f.Body.String(), f.GetSpan())
}

func (f *FunctionDeclExprNode) Accept(visitor ExprVisitor[value.Value]) value.Value {
	return visitor.VisitFunctionDeclExpr(f)
}

type ExprVisitor[T value.Value] interface {
	VisitBinaryExpr(node *BinaryExprNode) T
	VisitNumber(node *NumberExprNode) T
	VisitUnaryExpr(node *UnaryExprNode) T
	VisitIdentifier(node *IdentifierExprNode) T
	VisitGroupingExpr(node *GroupingExprNode) T
	VisitFunctionCallExpr(node *FunctionCallExprNode) T
	VisitFunctionDeclExpr(node *FunctionDeclExprNode) T
}
