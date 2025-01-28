package expr_test

import (
	"fmt"
	"testing"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/error"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/test_utils"
	"github.com/ameerthehacker/zeus/internal/token"
)

type testCase struct {
	name     string
	input    string
	expected ast.ExprNode
	errors   []*error.ZeusError
}

var BinaryOperatorPrecedence = map[token.TokenType]int{
	token.TokenTypeStar:             4,
	token.TokenTypeSlash:            4,
	token.TokenTypePlus:             3,
	token.TokenTypeMinus:            3,
	token.TokenTypeGreaterThan:      3,
	token.TokenTypeGreaterThanEqual: 3,
	token.TokenTypeLessThan:         3,
	token.TokenTypeLessThanEqual:    3,
	token.TokenTypeEqualEqual:       2,
	token.TokenTypeBangEqual:        2,
	token.TokenTypeEqual:            1,
}

// getHigherPrecedenceOperatorTestCase generates an AST node representing a binary expression
// where the higher precedence operator binds more tightly than the lower precedence operator.
// For example, given operators + and *, it would generate an AST for "1 + 2 * 3" where
// the multiplication binds 2 and 3 first, then adds 1.
//
// Returns an AST node structured as:
//
//	(1 lowerOp (2 higherOp 3))
//
// with proper token positions calculated based on operator lengths.
//
// The function carefully tracks column positions to ensure accurate source mapping,
// accounting for the varying lengths of different operators and spacing between tokens.
func getHigherPrecedenceOperatorTestCase(higherPrecedenceOp, lowerPrecedenceOp token.TokenType) ast.ExprNode {
	lowerPrecedenceOpLen := len(lowerPrecedenceOp.String())
	higherPrecedenceOpLen := len(higherPrecedenceOp.String())

	// Calculate column positions based on operator lengths
	leftNumCol := 1
	lowerOpCol := leftNumCol + 2                           // After 1 and space
	middleNumCol := lowerOpCol + lowerPrecedenceOpLen + 1  // After operator and space
	higherOpCol := middleNumCol + 2                        // After 2 and space
	rightNumCol := higherOpCol + higherPrecedenceOpLen + 1 // After operator and space

	return &ast.BinaryExprNode{
		Left: &ast.NumberExprNode{
			Value: token.NewTokenWithValue(token.TokenTypeNumber, "1",
				token.NewSpan(token.Position{Line: 1, Column: leftNumCol}, token.Position{Line: 1, Column: leftNumCol})),
		},
		Right: &ast.BinaryExprNode{
			Left: &ast.NumberExprNode{
				Value: token.NewTokenWithValue(token.TokenTypeNumber, "2",
					token.NewSpan(token.Position{Line: 1, Column: middleNumCol}, token.Position{Line: 1, Column: middleNumCol})),
			},
			Right: &ast.NumberExprNode{
				Value: token.NewTokenWithValue(token.TokenTypeNumber, "3",
					token.NewSpan(token.Position{Line: 1, Column: rightNumCol}, token.Position{Line: 1, Column: rightNumCol})),
			},
			Operator: token.NewToken(higherPrecedenceOp,
				token.NewSpan(token.Position{Line: 1, Column: higherOpCol}, token.Position{Line: 1, Column: higherOpCol})),
		},
		Operator: token.NewToken(lowerPrecedenceOp,
			token.NewSpan(token.Position{Line: 1, Column: lowerOpCol}, token.Position{Line: 1, Column: lowerOpCol})),
	}
}

func TestParseExpression(t *testing.T) {
	tests := []testCase{
		{
			name:  "unary operation",
			input: "-name",
			expected: &ast.UnaryExprNode{
				Operator: token.NewToken(token.TokenTypeMinus, token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1})),
				Expr:     &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "name", token.NewSpan(token.Position{Line: 1, Column: 2}, token.Position{Line: 1, Column: 5}))},
			},
			errors: []*error.ZeusError{},
		},
		{
			name:  "binary operation",
			input: "1 + 2",
			expected: &ast.BinaryExprNode{
				Left:     &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right:    &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5}, token.Position{Line: 1, Column: 5}))},
				Operator: token.NewToken(token.TokenTypePlus, token.NewSpan(token.Position{Line: 1, Column: 3}, token.Position{Line: 1, Column: 3})),
			},
			errors: []*error.ZeusError{},
		},
		{
			name:  "function call has higher precedence than equality",
			input: "name(1) == 2",
			expected: &ast.BinaryExprNode{
				Left:     &ast.FunctionCallExprNode{
					Callee: &ast.IdentifierExprNode{
						Name: token.NewTokenWithValue(
							token.TokenTypeIdentifier, "name", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 4})),
					},
					Params: []ast.ExprNode{
						&ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 6}, token.Position{Line: 1, Column: 6}))},
					},
					Span: token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 7}),
				},
				Right:    &ast.NumberExprNode{
					Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 12}, token.Position{Line: 1, Column: 12})),
				},
				Operator: token.NewToken(token.TokenTypeEqualEqual, token.NewSpan(token.Position{Line: 1, Column: 8}, token.Position{Line: 1, Column: 9})),
			},
			errors: []*error.ZeusError{},
		},
	}

	leftAssociativeOperators := []token.TokenType{token.TokenTypePlus, token.TokenTypeMinus, token.TokenTypeStar, token.TokenTypeSlash, token.TokenTypeEqualEqual, token.TokenTypeBangEqual, token.TokenTypeGreaterThan, token.TokenTypeGreaterThanEqual, token.TokenTypeLessThan, token.TokenTypeLessThanEqual}

	for _, operator := range leftAssociativeOperators {
		input := fmt.Sprintf("1 %s 2 %s 3", operator, operator)
		positionOffset := len(operator.String()) - 1
		tests = append(tests, testCase{
			name:  fmt.Sprintf("left associativity (%s)", input),
			input: input,
			expected: &ast.BinaryExprNode{
				Left: &ast.BinaryExprNode{
					Left:     &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
					Right:    &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5 + positionOffset}, token.Position{Line: 1, Column: 5 + positionOffset}))},
					Operator: token.NewToken(operator, token.NewSpan(token.Position{Line: 1, Column: 3 + positionOffset}, token.Position{Line: 1, Column: 3 + positionOffset})),
				},
				Right:    &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "3", token.NewSpan(token.Position{Line: 1, Column: 9 + 2*positionOffset}, token.Position{Line: 1, Column: 9 + 2*positionOffset}))},
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
			name:  fmt.Sprintf("right associativity (%s)", input),
			input: input,
			expected: &ast.BinaryExprNode{
				Left: &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right: &ast.BinaryExprNode{
					Left:     &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5 + positionOffset}, token.Position{Line: 1, Column: 5 + positionOffset}))},
					Right:    &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "3", token.NewSpan(token.Position{Line: 1, Column: 9 + 2*positionOffset}, token.Position{Line: 1, Column: 9 + 2*positionOffset}))},
					Operator: token.NewToken(operator, token.NewSpan(token.Position{Line: 1, Column: 7 + positionOffset}, token.Position{Line: 1, Column: 7 + positionOffset})),
				},
				Operator: token.NewToken(operator, token.NewSpan(token.Position{Line: 1, Column: 3 + positionOffset}, token.Position{Line: 1, Column: 3 + positionOffset})),
			},
			errors: []*error.ZeusError{},
		})
	}

	// Build test cases for operator precedence pairs
	for firstOp, firstPrec := range BinaryOperatorPrecedence {
		for secondOp, secondPrec := range BinaryOperatorPrecedence {
			// Only test pairs where first operator has lower precedence
			if firstPrec < secondPrec {
				input := fmt.Sprintf("1 %s 2 %s 3", firstOp, secondOp)
				tests = append(tests, testCase{
					name:     fmt.Sprintf("%s has lower precedence than %s", firstOp, secondOp),
					input:    input,
					expected: getHigherPrecedenceOperatorTestCase(secondOp, firstOp),
					errors:   []*error.ZeusError{},
				})
			}
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
func compareExprNodes(t *testing.T, value, expectation ast.ExprNode) {
	logExprNotEqualError := func(value, expectation ast.ExprNode) {
		t.Errorf("expected %s, got %s", expectation.PrettyString(), value.PrettyString())
	}
	switch aNode := value.(type) {
	case *ast.NumberExprNode:
		bNode, ok := expectation.(*ast.NumberExprNode)
		if !ok {
			logExprNotEqualError(aNode, expectation)
			return
		}
		if aNode.Value.Value != bNode.Value.Value {
			logExprNotEqualError(aNode, expectation)
		}
	case *ast.IdentifierExprNode:
		bNode, ok := expectation.(*ast.IdentifierExprNode)
		if !ok {
			logExprNotEqualError(aNode, expectation)
			return
		}
		if aNode.Name.Value != bNode.Name.Value {
			logExprNotEqualError(aNode, expectation)
		}
	case *ast.UnaryExprNode:
		bNode, ok := expectation.(*ast.UnaryExprNode)
		if !ok {
			logExprNotEqualError(aNode, expectation)
			return
		}
		compareExprNodes(t, aNode.Expr, bNode.Expr)
	case *ast.BinaryExprNode:
		bNode, ok := expectation.(*ast.BinaryExprNode)
		if !ok {
			logExprNotEqualError(aNode, expectation)
			return
		}
		compareExprNodes(t, aNode.Left, bNode.Left)
		compareExprNodes(t, aNode.Right, bNode.Right)
		if aNode.Operator.Type != bNode.Operator.Type {
			logExprNotEqualError(aNode, bNode)
		}
	case *ast.GroupingExprNode:
		bNode, ok := expectation.(*ast.GroupingExprNode)
		if !ok {
			logExprNotEqualError(aNode, expectation)
			return
		}
		compareExprNodes(t, aNode.Expr, bNode.Expr)
	case *ast.FunctionCallExprNode:
		bNode, ok := expectation.(*ast.FunctionCallExprNode)
		if !ok {
			logExprNotEqualError(aNode, expectation)
			return
		}
		compareExprNodes(t, aNode.Callee, bNode.Callee)
		// compare the params
		if len(aNode.Params) != len(bNode.Params) {
			logExprNotEqualError(aNode, expectation)
		}
		for i := range aNode.Params {
			compareExprNodes(t, aNode.Params[i], bNode.Params[i])
		}
	default:
		panic(fmt.Sprintf("unsupported node type: %T", aNode))
	}

	// at last compare the expression spans
	if !value.GetSpan().IsEqual(expectation.GetSpan()) {
		t.Errorf("expected expressions %s , %s spans to be equal, expected: %s got: %s", value.PrettyString(), expectation.PrettyString(), value.GetSpan(), expectation.GetSpan())
		return
	}
}
