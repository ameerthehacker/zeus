package parser_test

import (
	"fmt"
	"testing"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/test_utils"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

type testCase struct {
	name     string
	input    string
	expected ast.ExprNode
	errors   []*zeus_error.ZeusError
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
			name:  "unary expression",
			input: "-name",
			expected: &ast.UnaryExprNode{
				Operator: token.NewToken(token.TokenTypeMinus, token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1})),
				Expr:     &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "name", token.NewSpan(token.Position{Line: 1, Column: 2}, token.Position{Line: 1, Column: 5}))},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			name:  "binary expression",
			input: "1 + 2",
			expected: &ast.BinaryExprNode{
				Left:     &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}))},
				Right:    &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 5}, token.Position{Line: 1, Column: 5}))},
				Operator: token.NewToken(token.TokenTypePlus, token.NewSpan(token.Position{Line: 1, Column: 3}, token.Position{Line: 1, Column: 3})),
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			name:  "function call has higher precedence than equality",
			input: "name(1) == 2",
			expected: &ast.BinaryExprNode{
				Left: &ast.FunctionCallExprNode{
					Callee: &ast.IdentifierExprNode{
						Name: token.NewTokenWithValue(
							token.TokenTypeIdentifier, "name", token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 4})),
					},
					Params: []ast.ExprNode{
						&ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 6}, token.Position{Line: 1, Column: 6}))},
					},
					Span: token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 7}),
				},
				Right: &ast.NumberExprNode{
					Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 12}, token.Position{Line: 1, Column: 12})),
				},
				Operator: token.NewToken(token.TokenTypeEqualEqual, token.NewSpan(token.Position{Line: 1, Column: 8}, token.Position{Line: 1, Column: 9})),
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			name:  "parenthesized expression has the highest precedence",
			input: "(1 + 2) * 3",
			expected: &ast.BinaryExprNode{
				Left: &ast.GroupingExprNode{
					Expr: &ast.BinaryExprNode{
						Left:     &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "1", token.NewSpan(token.Position{Line: 1, Column: 2}, token.Position{Line: 1, Column: 2}))},
						Right:    &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "2", token.NewSpan(token.Position{Line: 1, Column: 6}, token.Position{Line: 1, Column: 6}))},
						Operator: token.NewToken(token.TokenTypePlus, token.NewSpan(token.Position{Line: 1, Column: 4}, token.Position{Line: 1, Column: 4})),
					},
					Span: token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 7}),
				},
				Right:    &ast.NumberExprNode{Value: token.NewTokenWithValue(token.TokenTypeNumber, "3", token.NewSpan(token.Position{Line: 1, Column: 11}, token.Position{Line: 1, Column: 11}))},
				Operator: token.NewToken(token.TokenTypeStar, token.NewSpan(token.Position{Line: 1, Column: 9}, token.Position{Line: 1, Column: 9})),
			},
		},
		{
			name:  "parser error on invalid expression",
			input: "1 + + 2",
			errors: []*zeus_error.ZeusError{
				zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "expected expression, but found +", token.NewSpan(token.Position{Line: 1, Column: 5}, token.Position{Line: 1, Column: 5})),
			},
		},
		{
			name:  "parser error on unclosed parenthesis",
			input: "(1 + 2",
			errors: []*zeus_error.ZeusError{
				zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "expected ) to close grouped expression, but found ;", token.NewSpan(token.Position{Line: 1, Column: 6}, token.Position{Line: 1, Column: 6})),
			},
		},
		{
			name:  "function declaration",
			input: "function name(a: i8, b: i8): i8 { return a + b; }",
			expected: &ast.FunctionDeclExprNode{
				Name: &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "name", token.NewSpan(token.Position{Line: 1, Column: 10}, token.Position{Line: 1, Column: 13}))},
				Params: []*ast.VarDeclNode{
					{
						Identifier: &ast.IdentifierExprNode{
							Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "a", token.NewSpan(token.Position{Line: 1, Column: 15}, token.Position{Line: 1, Column: 15})),
						},
						ValueType: &ast.ValueTypeNode{
							ValueType: zeus_value.IntType{Signed: true, Size: zeus_value.I8},
							Span:      token.NewSpan(token.Position{Line: 1, Column: 18}, token.Position{Line: 1, Column: 18}),
						},
						DeclType:    ast.VarDeclTypeLet,
						Initializer: nil,
					},
					{
						Identifier: &ast.IdentifierExprNode{
							Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "b", token.NewSpan(token.Position{Line: 1, Column: 22}, token.Position{Line: 1, Column: 22})),
						},
						ValueType: &ast.ValueTypeNode{
							ValueType: zeus_value.IntType{Signed: true, Size: zeus_value.I8},
							Span:      token.NewSpan(token.Position{Line: 1, Column: 25}, token.Position{Line: 1, Column: 26}),
						},
						DeclType:    ast.VarDeclTypeLet,
						Initializer: nil,
					},
				},
				ReturnType: &ast.ValueTypeNode{
					ValueType: zeus_value.IntType{Signed: true, Size: zeus_value.I8},
					Span:      token.NewSpan(token.Position{Line: 1, Column: 30}, token.Position{Line: 1, Column: 30}),
				},
				Body: &ast.BlockStmtNode{
					Statements: []ast.StmtNode{
						&ast.ReturnStmtNode{
							Expr: &ast.BinaryExprNode{
								Left:     &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "a", token.NewSpan(token.Position{Line: 1, Column: 42}, token.Position{Line: 1, Column: 42}))},
								Right:    &ast.IdentifierExprNode{Name: token.NewTokenWithValue(token.TokenTypeIdentifier, "b", token.NewSpan(token.Position{Line: 1, Column: 46}, token.Position{Line: 1, Column: 46}))},
								Operator: token.NewToken(token.TokenTypePlus, token.NewSpan(token.Position{Line: 1, Column: 44}, token.Position{Line: 1, Column: 44})),
							},
							Span: token.NewSpan(token.Position{Line: 1, Column: 35}, token.Position{Line: 1, Column: 46}),
						},
					},
				},
				Span: token.NewSpan(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 49}),
			},
		},
		{
			name:     "function name missing",
			input:    "function",
			expected: nil,
			errors: []*zeus_error.ZeusError{
				zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "expected identifier for function name", token.NewSpan(token.Position{Line: 1, Column: 9}, token.Position{Line: 1, Column: 9})),
			},
		},
		{
			name:     "function open parenthesis missing",
			input:    "function name",
			expected: nil,
			errors: []*zeus_error.ZeusError{
				zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "expected ( after function name, but found ;", token.NewSpan(token.Position{Line: 1, Column: 13}, token.Position{Line: 1, Column: 13})),
			},
		},
		{
			name:     "function closing parenthesis missing",
			input:    "function name(",
			expected: nil,
			errors: []*zeus_error.ZeusError{
				zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "expected ) after function parameters", token.NewSpan(token.Position{Line: 1, Column: 15}, token.Position{Line: 1, Column: 15})),
			},
		},
		{
			name:     "function parameter missing : after identifier",
			input:    "function name(a)",
			expected: nil,
			errors: []*zeus_error.ZeusError{
				zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "expected : after identifier in function parameter, but found )", token.NewSpan(token.Position{Line: 1, Column: 16}, token.Position{Line: 1, Column: 16})),
			},
		},
		{
			name:     "function parameter data type missing",
			input:    "function name(a:)",
			expected: nil,
			errors: []*zeus_error.ZeusError{
				zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "expected data type in function parameter, but found )", token.NewSpan(token.Position{Line: 1, Column: 17}, token.Position{Line: 1, Column: 17})),
			},
		},
		{
			name:     "function return type missing :",
			input:    "function name(a: i8) {}",
			expected: nil,
			errors: []*zeus_error.ZeusError{
				zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "expected : after function parameters, but found {", token.NewSpan(token.Position{Line: 1, Column: 22}, token.Position{Line: 1, Column: 22})),
			},
		},
		{
			name:     "function return type missing",
			input:    "function name(a: i8): {}",
			expected: nil,
			errors: []*zeus_error.ZeusError{
				zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "expected return type in function declaration, but found {", token.NewSpan(token.Position{Line: 1, Column: 23}, token.Position{Line: 1, Column: 23})),
			},
		},
		{
			name:     "open brace missing in function body",
			input:    "function name(a: i8): i8",
			expected: nil,
			errors: []*zeus_error.ZeusError{
				zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "expected { to begin block", token.NewSpan(token.Position{Line: 1, Column: 25}, token.Position{Line: 1, Column: 25})),
			},
		},
		{
			name:     "closing brace missing in function body",
			input:    "function name(a: i8): i8 {",
			expected: nil,
			errors: []*zeus_error.ZeusError{
				zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "expected } to close block", token.NewSpan(token.Position{Line: 1, Column: 27}, token.Position{Line: 1, Column: 27})),
			},
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
			errors: []*zeus_error.ZeusError{},
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
			errors: []*zeus_error.ZeusError{},
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
					errors:   []*zeus_error.ZeusError{},
				})
			}
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lexer := lexer.NewLexer(test.input)
			tokens, _ := lexer.Lex()
			parser := parser.NewParser(tokens)

			defer func() {
				if r := recover(); r != nil {
					// when error happens, we need to compare the errors
					test_utils.CompareZeusErrors(t, parser.GetErrors(), test.errors)
				}
			}()

			result := parser.ParseExpr()

			if test.expected != nil {
				test_utils.CompareExprNodes(t, result, test.expected)
			}
		})
	}
}
