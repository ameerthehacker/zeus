package ast

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

type ValueTypeNode struct {
	ValueType zeus_value.ValueType
	Span      *token.Span
}

type ExprNode interface {
	GetSpan() *token.Span
	Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value
	PrettyString() string
	String() string
}

// GroupingExprNode and its methods
type GroupingExprNode struct {
	Expr ExprNode
	Span *token.Span
}

func (g *GroupingExprNode) GetSpan() *token.Span {
	return g.Span
}

func (g *GroupingExprNode) PrettyString() string {
	return fmt.Sprintf("(%s)", g.Expr.PrettyString())
}

func (g *GroupingExprNode) String() string {
	return fmt.Sprintf("{ type: GroupingExprNode, Expr: %s, Span: %s }", g.Expr.String(), g.Span)
}

func (g *GroupingExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitGroupingExpr(g)
}

// BinaryExprNode and its methods
type BinaryExprNode struct {
	Left     ExprNode
	Right    ExprNode
	Operator *token.Token
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

// NumberExprNode and its methods
type NumberExprNode struct {
	Value *token.Token
}

func (n *NumberExprNode) GetSpan() *token.Span {
	return n.Value.Span
}

func (n *NumberExprNode) PrettyString() string {
	return n.Value.Value
}

func (n *NumberExprNode) String() string {
	return fmt.Sprintf("{ type: NumberNode, Value: %s, Span: %s }", n.Value, n.GetSpan())
}

func (n *NumberExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitNumber(n)
}

// BooleanExprNode and its methods
type BooleanExprNode struct {
	Value *token.Token
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

// UnaryExprNode and its methods
type UnaryExprNode struct {
	Operator *token.Token
	Expr     ExprNode
}

func (u *UnaryExprNode) GetSpan() *token.Span {
	startPosition := u.Operator.Span.Start
	endPosition := u.Expr.GetSpan().End
	return &token.Span{Start: startPosition, End: endPosition}
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

// IdentifierExprNode and its methods
type IdentifierExprNode struct {
	Name *token.Token
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

// NullExprNode and its methods
type NullExprNode struct {
	Span *token.Span
}

func (n *NullExprNode) GetSpan() *token.Span {
	return n.Span
}

func (n *NullExprNode) PrettyString() string {
	return "null"
}

func (n *NullExprNode) String() string {
	return fmt.Sprintf("{ type: NullExprNode, Span: %s }", n.GetSpan())
}

func (n *NullExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitNull(n)
}

// FunctionCallExprNode and its methods
type FunctionCallExprNode struct {
	Callee ExprNode
	Params []ExprNode
	Span   *token.Span
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

// FunctionDeclExprNode and its methods
type FunctionDeclExprNode struct {
	Name       *IdentifierExprNode
	Params     []*VarDeclNode
	Body       *BlockStmtNode
	ReturnType *ValueTypeNode
	Span       *token.Span
}

func (f *FunctionDeclExprNode) GetSpan() *token.Span {
	return f.Span
}

func (f *FunctionDeclExprNode) PrettyString() string {
	head := fmt.Sprintf("function %s(%s): %s", f.Name.PrettyString(), varDeclsString(f.Params, true), f.ReturnType.ValueType)
	body := f.Body.PrettyString()

	return fmt.Sprintf("%s\n%s\n", head, body)
}

func (f *FunctionDeclExprNode) String() string {
	return fmt.Sprintf("{ type: FunctionDeclExprNode, Name: %s, Params: [%s], ReturnType: %s, Body: %s, Span: %s }", f.Name.String(), varDeclsString(f.Params, false), f.ReturnType.ValueType, f.Body.String(), f.GetSpan())
}

func (f *FunctionDeclExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitFunctionDeclExpr(f)
}

type ArrayDims struct {
	CapacityExprs []ExprNode
}

type TypeExpressionNode struct {
	Type *token.Token
	ArrayDims* ArrayDims
	Span *token.Span
}

func (t *TypeExpressionNode) GetSpan() *token.Span {
	return t.Span
}

func (t *TypeExpressionNode) PrettyString() string {
	if t.ArrayDims != nil && t.ArrayDims.CapacityExprs != nil {
		capacities := []string{}

		for _, capacityExpr := range t.ArrayDims.CapacityExprs {
			capacities = append(capacities, fmt.Sprintf("[%s]", capacityExpr.PrettyString()))
		}

		return fmt.Sprintf("%s%s", t.Type.Value, strings.Join(capacities, ""))
	}
	return t.Type.Value
}

func (t *TypeExpressionNode) String() string {
	if t.ArrayDims != nil && t.ArrayDims.CapacityExprs != nil {
		capacities := []string{}

		for _, capacityExpr := range t.ArrayDims.CapacityExprs {
			capacities = append(capacities, fmt.Sprintf("[%s]", capacityExpr.String()))
		}

		return fmt.Sprintf("{ type: TypeExpressionNode, Type: %s, Dimensions: %s, Span: %s }", t.Type.Value, strings.Join(capacities, ""), t.GetSpan())
	}
	return fmt.Sprintf("{ type: TypeExpressionNode, Type: %s, Span: %s }", t.Type.Value, t.GetSpan())
}

func (t *TypeExpressionNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitTypeExpression(t)
}

// ClassDeclExprNode and its methods
type ClassDeclExprNode struct {
	Name       *IdentifierExprNode
	Properties []*ClassProperty
	Methods    []*ClassMethod
	Span       *token.Span
}

func (c *ClassDeclExprNode) GetSpan() *token.Span {
	return c.Span
}

func (c *ClassDeclExprNode) PrettyString() string {
	properties := []string{}
	for _, property := range c.Properties {
		properties = append(properties, fmt.Sprintf("%s: %s", property.Name.PrettyString(), property.ValueType.ValueType))
	}
	methods := []string{}
	for _, method := range c.Methods {
		methods = append(methods, method.Name.PrettyString())
	}
	return fmt.Sprintf("class %s {\n%s\n%s\n}", c.Name.PrettyString(), strings.Join(properties, "\n"), strings.Join(methods, "\n"))
}

func (c *ClassDeclExprNode) String() string {
	return fmt.Sprintf("{ type: ClassDeclExprNode, Name: %s, Properties: [%s], Methods: [%s], Span: %s }", c.Name.String(), classPropertiesString(c.Properties), classMethodsString(c.Methods), c.GetSpan())
}

func (c *ClassDeclExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitClassDeclExpr(c)
}

// NewExprNode and its methods
type NewExprNode struct {
	Callee ExprNode
	Args   []ExprNode
	Span   *token.Span
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

// ObjectPropertyAccessExprNode and its methods
type ObjectPropertyAccessExprNode struct {
	Object   ExprNode
	Property *IdentifierExprNode
	Span     *token.Span
}

func (o *ObjectPropertyAccessExprNode) GetSpan() *token.Span {
	return o.Span
}

func (o *ObjectPropertyAccessExprNode) PrettyString() string {
	return fmt.Sprintf("%s.%s", o.Object.PrettyString(), o.Property.PrettyString())
}

func (o *ObjectPropertyAccessExprNode) String() string {
	return fmt.Sprintf("{ type: ObjectPropertyAccessExprNode, Object: %s, Property: %s, Span: %s }", o.Object.String(), o.Property.String(), o.GetSpan())
}

func (o *ObjectPropertyAccessExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitObjectPropertyAccessExpr(o)
}

// Supporting structs (no methods)
type ClassProperty struct {
	Name           *IdentifierExprNode
	ValueType      *ValueTypeNode
	AccessModifier *token.Token
	Span           *token.Span
}

type ClassMethod struct {
	Name           *IdentifierExprNode
	Params         []*VarDeclNode
	Body           *BlockStmtNode
	ReturnType     *ValueTypeNode
	AccessModifier *token.Token
	Span           *token.Span
}

// Helper functions
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

func AsTypeExpression(expr ExprNode) *TypeExpressionNode {
	switch expr := expr.(type) {
	case *TypeExpressionNode:
		return expr
	default:
		return nil
	}
}

// ExprVisitor interface
type ExprVisitor[T zeus_value.Value] interface {
	VisitBinaryExpr(node *BinaryExprNode) T
	VisitTypeExpression(node *TypeExpressionNode) T
	VisitNumber(node *NumberExprNode) T
	VisitUnaryExpr(node *UnaryExprNode) T
	VisitIdentifier(node *IdentifierExprNode) T
	VisitBoolean(node *BooleanExprNode) T
	VisitGroupingExpr(node *GroupingExprNode) T
	VisitFunctionCallExpr(node *FunctionCallExprNode) T
	VisitFunctionDeclExpr(node *FunctionDeclExprNode) T
	VisitClassDeclExpr(node *ClassDeclExprNode) T
	VisitNewExpr(node *NewExprNode) T
	VisitObjectPropertyAccessExpr(node *ObjectPropertyAccessExprNode) T
	VisitNull(node *NullExprNode) T
}