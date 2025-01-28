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

type testCase struct {
	name string
	input string
	expected ast.ExprNode
	errors []*error.ZeusError
}

func TestParseExpression(t *testing.T) {
	tests := []testCase{
		{
			name: "unary operation",
			input: "-name",
			expected: &ast.UnaryExprNode{
				Operator: token.NewToken(token.TokenTypeMinus, token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1})),
				Expr: &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "name", token.NewSpan(token.Position{Line: 1, Column: 2}, token.Position{Line: 1, Column: 5}))},
			},
			errors: []*error.ZeusError{},
		},
		{
			name: "binary operation",
			input: "1 + 2",
			expected: &ast.BinaryExprNode{
				Left:     &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right:    &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5}, token.Position{Line: 1, Column: 5}))},
				Operator: token.NewToken(token.TokenTypePlus, token.NewSpan(token.Position{Line: 1, Column: 3}, token.Position{Line: 1, Column: 3})),
			},
			errors: []*error.ZeusError{},
		},
		{
			name: "* has higher precedence than +",
			input: "1 + 2 * 3",
			expected: &ast.BinaryExprNode{
				Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right: &ast.BinaryExprNode{
					Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5}, token.Position{Line: 1, Column: 5}))},
					Right: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "3", token.NewSpan(token.Position{Line: 1, Column: 9}, token.Position{Line: 1, Column: 9}))},
					Operator: token.NewToken(token.TokenTypeStar, token.NewSpan(token.Position{Line: 1, Column: 7}, token.Position{Line: 1, Column: 7})),
				},
				Operator: token.NewToken(token.TokenTypePlus, token.NewSpan(token.Position{Line: 1, Column: 3}, token.Position{Line: 1, Column: 3})),
			},
			errors: []*error.ZeusError{},
		},
		{
			name: "* has higher precedence than -",
			input: "1 - 2 * 3",
			expected: &ast.BinaryExprNode{
				Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right: &ast.BinaryExprNode{
					Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5}, token.Position{Line: 1, Column: 5}))},
					Right: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "3", token.NewSpan(token.Position{Line: 1, Column: 9}, token.Position{Line: 1, Column: 9}))},
					Operator: token.NewToken(token.TokenTypeStar, token.NewSpan(token.Position{Line: 1, Column: 7}, token.Position{Line: 1, Column: 7})),
				},
				Operator: token.NewToken(token.TokenTypeMinus, token.NewSpan(token.Position{Line: 1, Column: 3}, token.Position{Line: 1, Column: 3})),
			},
			errors: []*error.ZeusError{},
		},
		{
			name: "/ has higher precedence than +",
			input: "1 + 2 / 3",
			expected: &ast.BinaryExprNode{
				Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right: &ast.BinaryExprNode{
					Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5}, token.Position{Line: 1, Column: 5}))},
					Right: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "3", token.NewSpan(token.Position{Line: 1, Column: 9}, token.Position{Line: 1, Column: 9}))},
					Operator: token.NewToken(token.TokenTypeSlash, token.NewSpan(token.Position{Line: 1, Column: 7}, token.Position{Line: 1, Column: 7})),
				},
				Operator: token.NewToken(token.TokenTypePlus, token.NewSpan(token.Position{Line: 1, Column: 3}, token.Position{Line: 1, Column: 3})),
			},
			errors: []*error.ZeusError{},
		},
		{
			name: "/ has higher precedence than -",
			input: "1 - 2 / 3",
			expected: &ast.BinaryExprNode{
				Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right: &ast.BinaryExprNode{
					Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5}, token.Position{Line: 1, Column: 5}))},
					Right: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "3", token.NewSpan(token.Position{Line: 1, Column: 9}, token.Position{Line: 1, Column: 9}))},
					Operator: token.NewToken(token.TokenTypeSlash, token.NewSpan(token.Position{Line: 1, Column: 7}, token.Position{Line: 1, Column: 7})),
				},
				Operator: token.NewToken(token.TokenTypeMinus, token.NewSpan(token.Position{Line: 1, Column: 3}, token.Position{Line: 1, Column: 3})),
			},
			errors: []*error.ZeusError{},
		},
		{
			name: "> has higher precedence than ==",
			input: "1 == 2 > 3",
			expected: &ast.BinaryExprNode{
				Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right: &ast.BinaryExprNode{
					Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 6}, token.Position{Line: 1, Column: 6}))},
					Right: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "3", token.NewSpan(token.Position{Line: 1, Column: 10}, token.Position{Line: 1, Column: 10}))},
					Operator: token.NewToken(token.TokenTypeGreaterThan, token.NewSpan(token.Position{Line: 1, Column: 8}, token.Position{Line: 1, Column: 8})),
				},
				Operator: token.NewToken(token.TokenTypeEqualEqual, token.NewSpan(token.Position{Line: 1, Column: 3}, token.Position{Line: 1, Column: 4})),
			},
			errors: []*error.ZeusError{},
		},
		{
			name: "== has higher precedence than =",
			input: "a = b == c",
			expected: &ast.BinaryExprNode{
				Left: &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "a", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right: &ast.BinaryExprNode{
					Left: &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "b", token.NewSpan(token.Position{Line: 1, Column: 5}, token.Position{Line: 1, Column: 5}))},
					Right: &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "c", token.NewSpan(token.Position{Line: 1, Column: 10}, token.Position{Line: 1, Column: 10}))},
					Operator: token.NewToken(token.TokenTypeEqualEqual, token.NewSpan(token.Position{Line: 1, Column: 7}, token.Position{Line: 1, Column: 8})),
				},
				Operator: token.NewToken(token.TokenTypeEqual, token.NewSpan(token.Position{Line: 1, Column: 3}, token.Position{Line: 1, Column: 3})),
			},
			errors: []*error.ZeusError{},
		},
		{
			name: "!= has higher precedence than =",
			input: "a = b != c",
			expected: &ast.BinaryExprNode{
				Left: &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "a", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right: &ast.BinaryExprNode{
					Left: &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "b", token.NewSpan(token.Position{Line: 1, Column: 5}, token.Position{Line: 1, Column: 5}))},
					Right: &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "c", token.NewSpan(token.Position{Line: 1, Column: 10}, token.Position{Line: 1, Column: 10}))},
					Operator: token.NewToken(token.TokenTypeBangEqual, token.NewSpan(token.Position{Line: 1, Column: 7}, token.Position{Line: 1, Column: 8})),
				},
				Operator: token.NewToken(token.TokenTypeEqual, token.NewSpan(token.Position{Line: 1, Column: 3}, token.Position{Line: 1, Column: 3})),
			},
			errors: []*error.ZeusError{},
		},
	}

	leftAssociativeOperators := []token.TokenType{token.TokenTypePlus, token.TokenTypeMinus, token.TokenTypeStar, token.TokenTypeSlash, token.TokenTypeEqualEqual, token.TokenTypeBangEqual, token.TokenTypeGreaterThan, token.TokenTypeGreaterThanEqual, token.TokenTypeLessThan, token.TokenTypeLessThanEqual}

	for _, operator := range leftAssociativeOperators {
		input := fmt.Sprintf("1 %s 2 %s 3", operator, operator)
		positionOffset := len(operator.String()) - 1
		tests = append(tests, testCase{
			name: fmt.Sprintf("left associativity (%s)", input),
			input: input,
			expected: &ast.BinaryExprNode{
				Left:     &ast.BinaryExprNode{
					Left:     &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
					Right:    &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5 + positionOffset}, token.Position{Line: 1, Column: 5 + positionOffset}))},
					Operator: token.NewToken(operator, token.NewSpan(token.Position{Line: 1, Column: 3 + positionOffset}, token.Position{Line: 1, Column: 3 + positionOffset})),
				},
				Right: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "3", token.NewSpan(token.Position{Line: 1, Column: 9 + 2 * positionOffset}, token.Position{Line: 1, Column: 9 + 2 * positionOffset}))},
				Operator: token.NewToken(operator, token.NewSpan(token.Position{Line: 1, Column: 7 + positionOffset}, token.Position{Line: 1, Column: 7 + positionOffset})),
			},
			errors: []*error.ZeusError{},
		})
	}

	rightAssociativeOperators := []token.TokenType{token.TokenTypeEqual}

	for _, operator := range rightAssociativeOperators {
		input := fmt.Sprintf("1 %s 2 %s 3", operator, operator)
		positionOffset := len(operator.String()) - 1
		tests = append(tests, testCase{
			name: fmt.Sprintf("right associativity (%s)", input),
			input: input,
			expected: &ast.BinaryExprNode{
				Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right: &ast.BinaryExprNode{
					Left:     &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5 + positionOffset}, token.Position{Line: 1, Column: 5 + positionOffset}))},
					Right:    &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "3", token.NewSpan(token.Position{Line: 1, Column: 9 + 2 * positionOffset}, token.Position{Line: 1, Column: 9 + 2 * positionOffset}))},
					Operator: token.NewToken(operator, token.NewSpan(token.Position{Line: 1, Column: 7 + positionOffset}, token.Position{Line: 1, Column: 7 + positionOffset})),
				},
				Operator: token.NewToken(operator, token.NewSpan(token.Position{Line: 1, Column: 3 + positionOffset}, token.Position{Line: 1, Column: 3 + positionOffset})),
			},
			errors: []*error.ZeusError{},
		})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
		t.Errorf("expected expressions %s , %s spans to be equal, expected: %s got: %s", a.PrettyString(), b.PrettyString(), a.GetSpan(), b.GetSpan())
		return
	}

	logExprNotEqualError := func(a, b ast.ExprNode) {
		t.Errorf("expected %s, got %s", a.PrettyString(), b.PrettyString())
	}
	switch aNode := a.(type) {
	case *ast.NumberExprNode:
		bNode, ok := b.(*ast.NumberExprNode)
		if !ok {
			logExprNotEqualError(aNode, bNode)
			return
		}
		if aNode.Value.Value != bNode.Value.Value {
			logExprNotEqualError(aNode, bNode)
		}
	case *ast.IdentifierExprNode:
		bNode, ok := b.(*ast.IdentifierExprNode)
		if !ok {
			logExprNotEqualError(aNode, bNode)
			return
		}
		if aNode.Name.Value != bNode.Name.Value {
			logExprNotEqualError(aNode, bNode)
		}
	case *ast.UnaryExprNode:
		bNode, ok := b.(*ast.UnaryExprNode)
		if !ok {
			logExprNotEqualError(aNode, bNode)
			return
		}
		compareExprNodes(t, aNode.Expr, bNode.Expr)
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
