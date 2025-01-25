package test

import (
	"ameerthehacker/zeus/internal/error"
	"ameerthehacker/zeus/internal/lexer"
	"ameerthehacker/zeus/internal/token"
	"testing"
)

func TestZeusLexer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []*token.Token
		errors   []*error.ZeusError
	}{
		{
			name:  "empty input",
			input: "",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 1))),
			},
		},
		{
			name:  "whitespace only",
			input: " \t\n\r",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(3, 1), *token.NewPosition(3, 1))),
			},
		},
		{
			name:  "semantic tokens",
			input: "({[)\n}];,:.",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeLeftParen, token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 1))),
				token.NewToken(token.TokenTypeLeftBrace, token.NewSpan(*token.NewPosition(1, 2), *token.NewPosition(1, 2))),
				token.NewToken(token.TokenTypeLeftBracket, token.NewSpan(*token.NewPosition(1, 3), *token.NewPosition(1, 3))),
				token.NewToken(token.TokenTypeRightParen, token.NewSpan(*token.NewPosition(1, 4), *token.NewPosition(1, 4))),
				token.NewToken(token.TokenTypeRightBrace, token.NewSpan(*token.NewPosition(2, 1), *token.NewPosition(2, 1))),
				token.NewToken(token.TokenTypeRightBracket, token.NewSpan(*token.NewPosition(2, 2), *token.NewPosition(2, 2))),
				token.NewToken(token.TokenTypeSemicolon, token.NewSpan(*token.NewPosition(2, 3), *token.NewPosition(2, 3))),
				token.NewToken(token.TokenTypeComma, token.NewSpan(*token.NewPosition(2, 4), *token.NewPosition(2, 4))),
				token.NewToken(token.TokenTypeColon, token.NewSpan(*token.NewPosition(2, 5), *token.NewPosition(2, 5))),
				token.NewToken(token.TokenTypeDot, token.NewSpan(*token.NewPosition(2, 6), *token.NewPosition(2, 6))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(2, 7), *token.NewPosition(2, 7))),
			},
		},
		{
			name:  "operators with whitespace",
			input: "!= == = > < >= <=",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeBangEqual, token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 2))),
				token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeEqualEqual, token.NewSpan(*token.NewPosition(1, 4), *token.NewPosition(1, 5))), 
				token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeEqual, token.NewSpan(*token.NewPosition(1, 7), *token.NewPosition(1, 7))),
				token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeGreaterThan, token.NewSpan(*token.NewPosition(1, 9), *token.NewPosition(1, 9))),
				token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeLessThan, token.NewSpan(*token.NewPosition(1, 11), *token.NewPosition(1, 11))),
				token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeGreaterThanEqual, token.NewSpan(*token.NewPosition(1, 13), *token.NewPosition(1, 14))),
				token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeLessThanEqual, token.NewSpan(*token.NewPosition(1, 16), *token.NewPosition(1, 17))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 18), *token.NewPosition(1, 18))),
			},
		},
		{
			name:     "unknown token error",
			input:    "@",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 2), *token.NewPosition(1, 2))),
			},
			errors: []*error.ZeusError{error.NewZeusError(error.ErrorSeverityError, "unknown token: '@'", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 1)))},
		},
		{
			name: "identifier",
			input: "hello_world _123 $123 hello_world$",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeIdentifier, "hello_world", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 11))),
				token.NewTokenWithValue(token.TokenTypeIdentifier, "_123", token.NewSpan(*token.NewPosition(1, 13), *token.NewPosition(1, 16))), 
				token.NewTokenWithValue(token.TokenTypeIdentifier, "$123", token.NewSpan(*token.NewPosition(1, 18), *token.NewPosition(1, 21))),
				token.NewTokenWithValue(token.TokenTypeIdentifier, "hello_world$", token.NewSpan(*token.NewPosition(1, 23), *token.NewPosition(1, 34))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 35), *token.NewPosition(1, 35))),
			},
		},
		{
			name: "keyword",
			input: "if else function let const while return true false",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeKeyword, "if", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 2))),
				token.NewTokenWithValue(token.TokenTypeKeyword, "else", token.NewSpan(*token.NewPosition(1, 4), *token.NewPosition(1, 7))),
				token.NewTokenWithValue(token.TokenTypeKeyword, "function", token.NewSpan(*token.NewPosition(1, 9), *token.NewPosition(1, 16))),
				token.NewTokenWithValue(token.TokenTypeKeyword, "let", token.NewSpan(*token.NewPosition(1, 18), *token.NewPosition(1, 20))),
				token.NewTokenWithValue(token.TokenTypeKeyword, "const", token.NewSpan(*token.NewPosition(1, 22), *token.NewPosition(1, 26))),
				token.NewTokenWithValue(token.TokenTypeKeyword, "while", token.NewSpan(*token.NewPosition(1, 28), *token.NewPosition(1, 32))),
				token.NewTokenWithValue(token.TokenTypeKeyword, "return", token.NewSpan(*token.NewPosition(1, 34), *token.NewPosition(1, 39))),
				token.NewTokenWithValue(token.TokenTypeKeyword, "true", token.NewSpan(*token.NewPosition(1, 41), *token.NewPosition(1, 44))),
				token.NewTokenWithValue(token.TokenTypeKeyword, "false", token.NewSpan(*token.NewPosition(1, 46), *token.NewPosition(1, 50))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 51), *token.NewPosition(1, 51))),
			},
		},
		{
			name: "integer datatype",
			input: "i8 i16 i32 i64 i128",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeDataType, "i8", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 2))),
				token.NewTokenWithValue(token.TokenTypeDataType, "i16", token.NewSpan(*token.NewPosition(1, 4), *token.NewPosition(1, 6))),
				token.NewTokenWithValue(token.TokenTypeDataType, "i32", token.NewSpan(*token.NewPosition(1, 8), *token.NewPosition(1, 10))),
				token.NewTokenWithValue(token.TokenTypeDataType, "i64", token.NewSpan(*token.NewPosition(1, 12), *token.NewPosition(1, 14))),
				token.NewTokenWithValue(token.TokenTypeDataType, "i128", token.NewSpan(*token.NewPosition(1, 16), *token.NewPosition(1, 19))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 20), *token.NewPosition(1, 20))),
			},
		},
		{
			name: "float datatype",
			input: "f32 f64",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeDataType, "f32", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 3))),
				token.NewTokenWithValue(token.TokenTypeDataType, "f64", token.NewSpan(*token.NewPosition(1, 5), *token.NewPosition(1, 7))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 8), *token.NewPosition(1, 8))),
			},
		},
		{
			name: "boolean datatype",
			input: "boolean",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeDataType, "boolean", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 7))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 8), *token.NewPosition(1, 8))),
			},
		},
		{
			name: "integer number",
			input: "123",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeNumber, "123", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 3))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 4), *token.NewPosition(1, 4))),
			},
		},
		{
			name: "float number",
			input: "123.456",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeNumber, "123.456", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 7))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 8), *token.NewPosition(1, 8))),
			},
		},
		{
			name: "numerical separator",
			input: "123_000_000",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeNumber, "123_000_000", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 11))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 12), *token.NewPosition(1, 12))),
			},
		},
		{
			name: "numerical separator error",
			input: "123_",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeNumber, "123_", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 4))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 5), *token.NewPosition(1, 5))),
			},
			errors: []*error.ZeusError{error.NewZeusError(error.ErrorSeverityError, "numerical separator must be followed by a digit", token.NewSpan(*token.NewPosition(1, 4), *token.NewPosition(1, 4)))},
		},
		{
			name: "binary octal hexadecimal number",
			input: "0b1010 0o123 0x123 0B1010 0O123 0X123",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeNumber, "0b1010", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 6))),
				token.NewTokenWithValue(token.TokenTypeNumber, "0o123", token.NewSpan(*token.NewPosition(1, 8), *token.NewPosition(1, 12))),
				token.NewTokenWithValue(token.TokenTypeNumber, "0x123", token.NewSpan(*token.NewPosition(1, 14), *token.NewPosition(1, 18))),
				token.NewTokenWithValue(token.TokenTypeNumber, "0B1010", token.NewSpan(*token.NewPosition(1, 20), *token.NewPosition(1, 25))),
				token.NewTokenWithValue(token.TokenTypeNumber, "0O123", token.NewSpan(*token.NewPosition(1, 27), *token.NewPosition(1, 31))),
				token.NewTokenWithValue(token.TokenTypeNumber, "0X123", token.NewSpan(*token.NewPosition(1, 33), *token.NewPosition(1, 37))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 38), *token.NewPosition(1, 38))),
			},
		},
		{
			name: "binary octal hexadecimal number with decimal point",
			input: "0b1010.",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeNumber, "0b1010", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 6))),
				token.NewToken(token.TokenTypeDot, token.NewSpan(*token.NewPosition(1, 7), *token.NewPosition(1, 7))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 8), *token.NewPosition(1, 8))),
			},
		},
		{
			name: "decimal point error",
			input: "123.3.4",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeNumber, "123.3.4", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 7))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 8), *token.NewPosition(1, 8))),
			},
			errors: []*error.ZeusError{error.NewZeusError(error.ErrorSeverityError, "invalid decimal point", token.NewSpan(*token.NewPosition(1, 6), *token.NewPosition(1, 6)))},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.NewLexer(tt.input)
			tokens, errors := l.Lex()

			if len(tt.errors) > 0 && len(errors) == 0 {
				t.Error("expected errors but got none")
			} else if len(tt.errors) == 0 && len(errors) > 0 {
				t.Errorf("unexpected errors: %v", errors)
			} else if len(tt.errors) != len(errors) {
				t.Errorf("expected %d errors, got %d", len(tt.errors), len(errors))
			} else {
				for i, error := range errors {
					expected := tt.errors[i]
					if !error.IsEqual(expected) {
						t.Errorf("error %s: expected %s, got %s", error, expected, error)
					}
				}
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("expected %d tokens, got %d", len(tt.expected), len(tokens))
				return
			}

			for i, token := range tokens {
				expected := tt.expected[i]
				if !token.IsEqual(expected) {
					t.Errorf("expected token %s, got %s", expected, token)
				}
			}
		})
	}
}
