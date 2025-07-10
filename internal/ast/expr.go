package ast

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

type ExprNode interface {
	GetSpan() *token.Span
	Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value
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

type BooleanExprNode struct {
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

type ClassProperty struct {
	Name *IdentifierExprNode
	ValueType *token.Token
	AccessModifier *token.Token
	Span *token.Span
}

type ClassMethod struct {
	Name *IdentifierExprNode
	Params []*VarDeclNode
	Body *BlockStmtNode
	ReturnType *token.Token
	AccessModifier *token.Token
	Span *token.Span
}

type ClassDeclExprNode struct {
	Name *IdentifierExprNode
	Properties []*ClassProperty
	Methods []*ClassMethod
	Span *token.Span
}

type NewExprNode struct {
	Callee ExprNode
	Args []ExprNode
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

func (g *GroupingExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
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

func (b *BinaryExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitBinaryExpr(b)
}

func (n *NumberExprNode) GetSpan() *token.Span {
	return n.Value.Span
}

func (n *NumberExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
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

func (u *UnaryExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
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

func (i *IdentifierExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
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

func (f *FunctionCallExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
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

func (f *FunctionDeclExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitFunctionDeclExpr(f)
}

func (b *BooleanExprNode) GetSpan() *token.Span {
	return b.Value.Span
}

func (b *BooleanExprNode) PrettyString() string {
	return b.Value.Value
}

func (b *BooleanExprNode) String() string {
	return fmt.Sprintf("{ type: BooleanNode, Value: %s, Span: %s }", b.Value, b.GetSpan())
}

func (b *BooleanExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitBoolean(b)
}

func (c *ClassDeclExprNode) GetSpan() *token.Span {
	return c.Span
}

func classPropertiesString(properties []*ClassProperty) string {
	propertiesStr := []string{}
	for _, property := range properties {
		propertiesStr = append(propertiesStr, property.Name.String())
	}
	return strings.Join(propertiesStr, ", ")
}

func classMethodsString(methods []*ClassMethod) string {
	methodsStr := []string{}
	for _, method := range methods {
		methodsStr = append(methodsStr, method.Name.String())
	}
	return strings.Join(methodsStr, ", ")
}

func (c *ClassDeclExprNode) String() string {
	return fmt.Sprintf("{ type: ClassDeclExprNode, Name: %s, Properties: [%s], Methods: [%s], Span: %s }", c.Name.String(), classPropertiesString(c.Properties), classMethodsString(c.Methods), c.GetSpan())
}

func (c *ClassDeclExprNode) PrettyString() string {
	properties := []string{}
	for _, property := range c.Properties {
		properties = append(properties, fmt.Sprintf("%s: %s", property.Name.PrettyString(), property.ValueType.Type))
	}
	methods := []string{}
	for _, method := range c.Methods {
		methods = append(methods, method.Name.PrettyString())
	}
	return fmt.Sprintf("class %s {\n%s\n%s\n}", c.Name.PrettyString(), strings.Join(properties, "\n"), strings.Join(methods, "\n"))
}

func (c *ClassDeclExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitClassDeclExpr(c)
}

func (n *NewExprNode) GetSpan() *token.Span {
	return n.Span
}

func (n *NewExprNode) PrettyString() string {
	return fmt.Sprintf("new %s(%s)", n.Callee.PrettyString(), exprNodesString(n.Args, true))
}

func (n *NewExprNode) String() string {
	return fmt.Sprintf("{ type: NewExprNode, Callee: %s, Args: [%s], Span: %s }", n.Callee.String(), exprNodesString(n.Args, false), n.GetSpan())
}

func (n *NewExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitNewExpr(n)
}

type ExprVisitor[T zeus_value.Value] interface {
	VisitBinaryExpr(node *BinaryExprNode) T
	VisitNumber(node *NumberExprNode) T
	VisitUnaryExpr(node *UnaryExprNode) T
	VisitIdentifier(node *IdentifierExprNode) T
	VisitBoolean(node *BooleanExprNode) T
	VisitGroupingExpr(node *GroupingExprNode) T
	VisitFunctionCallExpr(node *FunctionCallExprNode) T
	VisitFunctionDeclExpr(node *FunctionDeclExprNode) T
	VisitClassDeclExpr(node *ClassDeclExprNode) T
	VisitNewExpr(node *NewExprNode) T
}
