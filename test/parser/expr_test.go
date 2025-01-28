package expr_test

import (
	"ameerthehacker/zeus/internal/ast"
	"ameerthehacker/zeus/internal/error"
	"ameerthehacker/zeus/internal/lexer"
	"ameerthehacker/zeus/internal/parser"
	"ameerthehacker/zeus/internal/test_utils"
	"ameerthehacker/zeus/internal/token"
	"fmt"
	"testing"
)
func TestParseExpression(t *testing.T) {
	tests := []struct {
		input       string
		expected    ast.ExprNode
		errors      []*error.ZeusError
	}{
		{
			input: "1 + 2",
			expected: &ast.BinaryExprNode{
				Left:     &ast.NumberNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right:    &ast.NumberNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5}, token.Position{Line: 1, Column: 5}))},
				Operator: token.NewToken(token.TokenTypePlus, token.NewSpan(token.Position{Line: 1, Column: 3}, token.Position{Line: 1, Column: 3})),
			},
			errors: []*error.ZeusError{},
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			// Tokenize the input
			lexer := lexer.NewLexer(test.input)
			tokens, _ := lexer.Lex()
			parser := parser.NewParser(tokens)
			result, errors := parser.ParseExpr()

			test_utils.CompareZeusErrors(t, errors, test.errors)
			compareExprNodes(t, result, test.expected) 
		})
	}
}

// compareExprNodes compares two expression nodes for equality using downcasting
func compareExprNodes(t *testing.T, a , b ast.ExprNode) {
	// at last compare the expression spans
	if !a.GetSpan().IsEqual(b.GetSpan()) {
		t.Errorf("expected expressions %s , %s spans to be equal, got %s and %s", a.PrettyString(), b.PrettyString(), a.GetSpan(), b.GetSpan())
		return
	}

	logExprNotEqualError := func(a, b ast.ExprNode) {
		t.Errorf("expected %s, got %s", a.PrettyString(), b.PrettyString())
	}
	switch aNode := a.(type) {
	case *ast.NumberNode:
		bNode, ok := b.(*ast.NumberNode)
		if !ok {
			logExprNotEqualError(aNode, bNode)
			return
		}
		if aNode.Value.Value != bNode.Value.Value {
			logExprNotEqualError(aNode, bNode)
		}
	case *ast.BinaryExprNode:
		bNode, ok := b.(*ast.BinaryExprNode)
		if !ok {
			logExprNotEqualError(aNode, bNode)
			return
		}
		compareExprNodes(t, aNode.Left, bNode.Left)
		compareExprNodes(t, aNode.Right, bNode.Right)
		if aNode.Operator.Type != bNode.Operator.Type {
			logExprNotEqualError(aNode, bNode)
		}
	case *ast.GroupingExprNode:
		bNode, ok := b.(*ast.GroupingExprNode)
		if !ok {
			logExprNotEqualError(aNode, bNode)
			return
		}
		compareExprNodes(t, aNode.Expr, bNode.Expr)
	default:
		panic(fmt.Sprintf("unsupported node type: %T", aNode))
	}
}
