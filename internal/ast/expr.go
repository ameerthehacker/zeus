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

func (v *ValueTypeNode) GetSpan() *token.Span {
	return v.Span
}

func (v *ValueTypeNode) PrettyString() string {
	return v.ValueType.String()
}

func (v *ValueTypeNode) String() string {
	return fmt.Sprintf("{ type: ValueTypeNode, ValueType: %s, Span: %s }", v.ValueType.String(), v.GetSpan())
}

func (v *ValueTypeNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitValueType(v)
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

// PostfixExprNode represents postfix operators like i++ and i--
type PostfixExprNode struct {
	Expr     ExprNode
	Operator *token.Token
}

func (p *PostfixExprNode) GetSpan() *token.Span {
	startPosition := p.Expr.GetSpan().Start
	endPosition := p.Operator.Span.End
	return &token.Span{Start: startPosition, End: endPosition}
}

func (p *PostfixExprNode) PrettyString() string {
	return fmt.Sprintf("(%s%s)", p.Expr.PrettyString(), p.Operator.Type)
}

func (p *PostfixExprNode) String() string {
	return fmt.Sprintf("{ type: PostfixExprNode, Expr: %s, Operator: %s, Span: %s }", p.Expr.String(), p.Operator.Type, p.GetSpan())
}

func (p *PostfixExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitPostfixExpr(p)
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
	Name         *IdentifierExprNode
	Params       []*VarDeclNode
	Body         *BlockStmtNode // nil for extern functions
	ReturnType   *ValueTypeNode
	ExternSymbol string
	IsCExtern    bool
	Span         *token.Span
}

func (f *FunctionDeclExprNode) GetSpan() *token.Span {
	return f.Span
}

func (f *FunctionDeclExprNode) PrettyString() string {
	nameStr := ""
	if f.Name != nil {
		nameStr = f.Name.PrettyString()
	}
	head := fmt.Sprintf("function %s(%s): %s", nameStr, varDeclsString(f.Params, true), f.ReturnType.ValueType)
	body := f.Body.PrettyString()

	return fmt.Sprintf("%s\n%s\n", head, body)
}

func (f *FunctionDeclExprNode) String() string {
	nameStr := "nil"
	if f.Name != nil {
		nameStr = f.Name.String()
	}
	return fmt.Sprintf("{ type: FunctionDeclExprNode, Name: %s, Params: [%s], ReturnType: %s, Body: %s, Span: %s }", nameStr, varDeclsString(f.Params, false), f.ReturnType.ValueType, f.Body.String(), f.GetSpan())
}

func (f *FunctionDeclExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitFunctionDeclExpr(f)
}

type IndexingMeta struct {
	IndexingExprs []ExprNode
}

func (i *IndexingMeta) String() string {
	indexingExprsStr := []string{}
	for _, indexingExpr := range i.IndexingExprs {
		if indexingExpr == nil {
			indexingExprsStr = append(indexingExprsStr, "[]")
			continue
		}
		indexingExprsStr = append(indexingExprsStr, fmt.Sprintf("[%s]", indexingExpr.String()))
	}
	return strings.Join(indexingExprsStr, "")
}

type IndexingExprNode struct {
	Array        ExprNode
	IndexingMeta IndexingMeta
	Span         *token.Span
}

func (t *IndexingExprNode) GetSpan() *token.Span {
	return t.Span
}

func (t *IndexingExprNode) PrettyString() string {
	if t.IndexingMeta.IndexingExprs != nil {
		indexingExprsStr := []string{}
		for _, indexingExpr := range t.IndexingMeta.IndexingExprs {
			// An index can be nil for empty brackets (e.g. `arr[]` in type position).
			if indexingExpr == nil {
				indexingExprsStr = append(indexingExprsStr, "[]")
				continue
			}
			indexingExprsStr = append(indexingExprsStr, fmt.Sprintf("[%s]", indexingExpr.PrettyString()))
		}
		return fmt.Sprintf("%s%s", t.Array.PrettyString(), strings.Join(indexingExprsStr, ""))
	}
	return t.Array.PrettyString()
}

func (t *IndexingExprNode) String() string {
	if t.IndexingMeta.IndexingExprs != nil {
		return fmt.Sprintf("{ type: IndexingExprNode, Array: %s, IndexingMeta: %s, Span: %s }", t.Array.String(), t.IndexingMeta.String(), t.GetSpan())
	}
	return fmt.Sprintf("{ type: IndexingExprNode, Array: %s, Span: %s }", t.Array.String(), t.GetSpan())
}

func (t *IndexingExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitIndexingExpression(t)
}

// ClassDeclExprNode and its methods
type ClassDeclExprNode struct {
	Name        *IdentifierExprNode
	ParentClass *IdentifierExprNode   // Optional parent class for inheritance (extends)
	Implements  []*IdentifierExprNode // Interfaces this class declares it implements
	Properties  []*ClassProperty
	Methods     []*ClassMethod
	Span        *token.Span
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
	extendsStr := ""
	if c.ParentClass != nil {
		extendsStr = fmt.Sprintf(" extends %s", c.ParentClass.PrettyString())
	}
	nameStr := ""
	if c.Name != nil {
		nameStr = c.Name.PrettyString()
	}
	return fmt.Sprintf("class %s%s {\n%s\n%s\n}", nameStr, extendsStr, strings.Join(properties, "\n"), strings.Join(methods, "\n"))
}

func (c *ClassDeclExprNode) String() string {
	parentStr := "nil"
	if c.ParentClass != nil {
		parentStr = c.ParentClass.String()
	}
	nameStr := "nil"
	if c.Name != nil {
		nameStr = c.Name.String()
	}
	return fmt.Sprintf("{ type: ClassDeclExprNode, Name: %s, ParentClass: %s, Properties: [%s], Methods: [%s], Span: %s }", nameStr, parentStr, classPropertiesString(c.Properties), classMethodsString(c.Methods), c.GetSpan())
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

// ArrayLiteralExprNode and its methods — an inline array literal like [1, 2, 3] or [[1], []].
type ArrayLiteralExprNode struct {
	Elements []ExprNode
	Span     *token.Span
}

func (a *ArrayLiteralExprNode) GetSpan() *token.Span {
	return a.Span
}

func (a *ArrayLiteralExprNode) PrettyString() string {
	return fmt.Sprintf("[%s]", exprNodesString(a.Elements, true))
}

func (a *ArrayLiteralExprNode) String() string {
	return fmt.Sprintf("{ type: ArrayLiteralExprNode, Elements: [%s], Span: %s }", exprNodesString(a.Elements, false), a.GetSpan())
}

func (a *ArrayLiteralExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitArrayLiteral(a)
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
	IsReadonly     bool
	IsStatic       bool
	// Initializer is the optional `= <expr>` default value (nil when absent). For instance
	// properties it runs during construction; for static properties it initializes the backing
	// global (constant initializers are materialized per-module for primordials).
	Initializer ExprNode
	Span        *token.Span
}

type AccessorKind int

const (
	AccessorKindNone   AccessorKind = iota // ordinary method
	AccessorKindGetter                     // get name(): T { ... }
	AccessorKindSetter                     // set name(value: T) { ... }
)

type ClassMethod struct {
	Name           *IdentifierExprNode
	Params         []*VarDeclNode
	Body           *BlockStmtNode // nil for extern methods
	ReturnType     *ValueTypeNode
	AccessModifier *token.Token
	Accessor       AccessorKind
	IsStatic       bool
	// ExternSymbol, when non-empty, marks this as an extern method whose body forwards to the
	// named Zig runtime symbol (no Zeus body). Body is nil in that case.
	ExternSymbol string
	Span         *token.Span
}

// InterfacePropertySignature is a `name: Type;` member of an interface (no default value).
type InterfacePropertySignature struct {
	Name       *IdentifierExprNode
	ValueType  *ValueTypeNode
	IsReadonly bool
	Span       *token.Span
}

// InterfaceMethodSignature is a `name(params): Ret;` member of an interface (no body).
type InterfaceMethodSignature struct {
	Name       *IdentifierExprNode
	Params     []*VarDeclNode
	ReturnType *ValueTypeNode
	Span       *token.Span
}

// InterfaceDeclExprNode is a TypeScript-style `interface` declaration. It is a
// purely type-level construct: it produces no runtime code, only a type used for
// structural conformance checking (see internal/zeus_value InterfaceType).
type InterfaceDeclExprNode struct {
	Name       *IdentifierExprNode
	Parents    []*IdentifierExprNode // interfaces this one extends (structural union)
	Properties []*InterfacePropertySignature
	Methods    []*InterfaceMethodSignature
	Span       *token.Span
}

func (i *InterfaceDeclExprNode) GetSpan() *token.Span {
	return i.Span
}

func (i *InterfaceDeclExprNode) PrettyString() string {
	members := []string{}
	for _, property := range i.Properties {
		ro := ""
		if property.IsReadonly {
			ro = "readonly "
		}
		members = append(members, fmt.Sprintf("%s%s: %s", ro, property.Name.PrettyString(), property.ValueType.ValueType))
	}
	for _, method := range i.Methods {
		members = append(members, fmt.Sprintf("%s()", method.Name.PrettyString()))
	}
	extendsStr := ""
	if len(i.Parents) > 0 {
		parents := []string{}
		for _, parent := range i.Parents {
			parents = append(parents, parent.PrettyString())
		}
		extendsStr = fmt.Sprintf(" extends %s", strings.Join(parents, ", "))
	}
	nameStr := ""
	if i.Name != nil {
		nameStr = i.Name.PrettyString()
	}
	return fmt.Sprintf("interface %s%s {\n%s\n}", nameStr, extendsStr, strings.Join(members, "\n"))
}

func (i *InterfaceDeclExprNode) String() string {
	nameStr := "nil"
	if i.Name != nil {
		nameStr = i.Name.String()
	}
	return fmt.Sprintf("{ type: InterfaceDeclExprNode, Name: %s, Parents: %d, Properties: %d, Methods: %d, Span: %s }", nameStr, len(i.Parents), len(i.Properties), len(i.Methods), i.GetSpan())
}

func (i *InterfaceDeclExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitInterfaceDeclExpr(i)
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

func AsIndexingExpr(expr ExprNode) *IndexingExprNode {
	switch expr := expr.(type) {
	case *IndexingExprNode:
		return expr
	default:
		return nil
	}
}

type CharExprNode struct {
	Value *token.Token
}

func (c *CharExprNode) GetSpan() *token.Span {
	return c.Value.Span
}

func (c *CharExprNode) PrettyString() string {
	return c.Value.Value
}

func (c *CharExprNode) String() string {
	return fmt.Sprintf("{ type: CharExprNode, Value: %s, Span: %s }", c.Value, c.GetSpan())
}

func (c *CharExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitChar(c)
}

type StringConstantExprNode struct {
	Value *token.Token
}

func (s *StringConstantExprNode) PrettyString() string {
	return s.Value.Value
}

func (s *StringConstantExprNode) String() string {
	return fmt.Sprintf("{ type: StringConstantExprNode, Value: %s, Span: %s }", s.Value, s.GetSpan())
}

func (s *StringConstantExprNode) GetSpan() *token.Span {
	return s.Value.Span
}

func (s *StringConstantExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitStringConstant(s)
}

// TemplateStringPart is one segment of a template literal — either a static string or an interpolated expression.
type TemplateStringPart struct {
	IsExpr bool
	Str    string
	Expr   ExprNode
}

// TemplateStringExprNode represents a backtick template literal: `Hello ${name}!`
type TemplateStringExprNode struct {
	Parts []*TemplateStringPart
	Span  *token.Span
}

func (t *TemplateStringExprNode) GetSpan() *token.Span {
	return t.Span
}

func (t *TemplateStringExprNode) PrettyString() string {
	var sb strings.Builder
	sb.WriteRune('`')
	for _, p := range t.Parts {
		if p.IsExpr {
			sb.WriteString("${")
			sb.WriteString(p.Expr.PrettyString())
			sb.WriteRune('}')
		} else {
			sb.WriteString(p.Str)
		}
	}
	sb.WriteRune('`')
	return sb.String()
}

func (t *TemplateStringExprNode) String() string {
	return fmt.Sprintf("{ type: TemplateStringExprNode, Span: %s }", t.Span)
}

func (t *TemplateStringExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitTemplateString(t)
}

// TernaryExprNode represents the ternary conditional: cond ? then : else
type TernaryExprNode struct {
	Condition ExprNode
	Then      ExprNode
	Else      ExprNode
	Span      *token.Span
}

func (t *TernaryExprNode) GetSpan() *token.Span {
	return t.Span
}

func (t *TernaryExprNode) PrettyString() string {
	return fmt.Sprintf("(%s ? %s : %s)", t.Condition.PrettyString(), t.Then.PrettyString(), t.Else.PrettyString())
}

func (t *TernaryExprNode) String() string {
	return fmt.Sprintf("{ type: TernaryExprNode, Condition: %s, Then: %s, Else: %s, Span: %s }", t.Condition.String(), t.Then.String(), t.Else.String(), t.Span)
}

func (t *TernaryExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitTernaryExpr(t)
}

// CastExprNode represents an explicit cast: expr as Type
type CastExprNode struct {
	Expr   ExprNode
	AsType *ValueTypeNode
	Span   *token.Span
}

func (c *CastExprNode) GetSpan() *token.Span {
	return c.Span
}

func (c *CastExprNode) PrettyString() string {
	return fmt.Sprintf("(%s as %s)", c.Expr.PrettyString(), c.AsType.ValueType.String())
}

func (c *CastExprNode) String() string {
	return fmt.Sprintf("{ type: CastExprNode, Expr: %s, AsType: %s, Span: %s }", c.Expr.String(), c.AsType.String(), c.Span)
}

func (c *CastExprNode) Accept(visitor ExprVisitor[zeus_value.Value]) zeus_value.Value {
	return visitor.VisitCastExpr(c)
}

// ExprVisitor interface
type ExprVisitor[T zeus_value.Value] interface {
	VisitBinaryExpr(node *BinaryExprNode) T
	VisitIndexingExpression(node *IndexingExprNode) T
	VisitNumber(node *NumberExprNode) T
	VisitChar(node *CharExprNode) T
	VisitUnaryExpr(node *UnaryExprNode) T
	VisitPostfixExpr(node *PostfixExprNode) T
	VisitIdentifier(node *IdentifierExprNode) T
	VisitBoolean(node *BooleanExprNode) T
	VisitGroupingExpr(node *GroupingExprNode) T
	VisitFunctionCallExpr(node *FunctionCallExprNode) T
	VisitFunctionDeclExpr(node *FunctionDeclExprNode) T
	VisitClassDeclExpr(node *ClassDeclExprNode) T
	VisitInterfaceDeclExpr(node *InterfaceDeclExprNode) T
	VisitNewExpr(node *NewExprNode) T
	VisitArrayLiteral(node *ArrayLiteralExprNode) T
	VisitObjectPropertyAccessExpr(node *ObjectPropertyAccessExprNode) T
	VisitNull(node *NullExprNode) T
	VisitStringConstant(node *StringConstantExprNode) T
	VisitTemplateString(node *TemplateStringExprNode) T
	VisitValueType(node *ValueTypeNode) T
	VisitTernaryExpr(node *TernaryExprNode) T
	VisitCastExpr(node *CastExprNode) T
}
