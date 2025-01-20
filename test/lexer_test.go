package test

import (
	"ameerthehacker/zeus/internal/common/error"
	"ameerthehacker/zeus/internal/common/token"
	"ameerthehacker/zeus/internal/lexer"
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
				token.NewToken(token.TokenTypeEOF, token.NewSpan(0, 0)),
			},
		},
		{
			name:  "whitespace only",
			input: " \t\n\r",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeEOF, token.NewSpan(4, 4)),
			},
		},
		{
			name:  "single tokens",
			input: "({[)}];,",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeLeftParen, token.NewSpan(0, 0)),
				token.NewToken(token.TokenTypeLeftBrace, token.NewSpan(1, 1)), 
				token.NewToken(token.TokenTypeLeftBracket, token.NewSpan(2, 2)),
				token.NewToken(token.TokenTypeRightParen, token.NewSpan(3, 3)),
				token.NewToken(token.TokenTypeRightBrace, token.NewSpan(4, 4)),
				token.NewToken(token.TokenTypeRightBracket, token.NewSpan(5, 5)),
				token.NewToken(token.TokenTypeSemicolon, token.NewSpan(6, 6)),
				token.NewToken(token.TokenTypeComma, token.NewSpan(7, 7)),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(8, 8)),
			},
		},
		{
			name:  "tokens with whitespace",
			input: "( { } )",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeLeftParen, token.NewSpan(0, 0)),
				token.NewToken(token.TokenTypeLeftBrace, token.NewSpan(2, 2)),
				token.NewToken(token.TokenTypeRightBrace, token.NewSpan(4, 4)),
				token.NewToken(token.TokenTypeRightParen, token.NewSpan(6, 6)),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(7, 7)),
			},
		},
		{
			name:     "invalid character",
			input:    "@",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeEOF, token.NewSpan(1, 1)),
			},
			errors: []*error.ZeusError{error.NewZeusError(error.ErrorSeverityError, "unknown token: '@'", token.NewSpan(0, 0))},
		},
		{
			name: "integer number",
			input: "123",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeNumber, "123", token.NewSpan(0, 2)),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(3, 3)),
			},
		},
		{
			name: "float number",
			input: "123.456",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeNumber, "123.456", token.NewSpan(0, 6)),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(7, 7)),
			},
		},
		{
			name: "decimal point error",
			input: "123.3.4",
			expected: []*token.Token{
				token.NewTokenWithValue(token.TokenTypeNumber, "123.3.4", token.NewSpan(0, 6)),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(7, 7)),
			},
			errors: []*error.ZeusError{error.NewZeusError(error.ErrorSeverityError, "invalid decimal point", token.NewSpan(5, 5))},
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
