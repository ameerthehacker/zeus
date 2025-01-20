package test

import (
	"ameerthehacker/zeus/internal/common/token"
	"ameerthehacker/zeus/internal/lexer"
	"testing"
)

func TestZeusLexer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []token.Token
		hasError bool
	}{
		{
			name:  "empty input",
			input: "",
			expected: []token.Token{
				token.NewToken(token.TokenTypeEOF, token.NewSpan(0, 0)),
			},
		},
		{
			name:  "whitespace only",
			input: " \t\n\r",
			expected: []token.Token{
				token.NewToken(token.TokenTypeEOF, token.NewSpan(4, 4)),
			},
		},
		{
			name:  "single tokens",
			input: "({[)}];,",
			expected: []token.Token{
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
			expected: []token.Token{
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
			expected: []token.Token{
				token.NewToken(token.TokenTypeEOF, token.NewSpan(1, 1)),
			},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.NewLexer(tt.input)
			tokens, errors := l.Lex()

			if tt.hasError && len(errors) == 0 {
				t.Error("expected errors but got none")
			}

			if !tt.hasError && len(errors) > 0 {
				t.Errorf("unexpected errors: %v", errors)
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("expected %d tokens, got %d", len(tt.expected), len(tokens))
				t.Errorf("expected %v, got %v", tt.expected, tokens)
				return
			}

			for i, token := range tokens {
				expected := tt.expected[i]
				if token.Type != expected.Type {
					t.Errorf("token %v: expected type %s, got %s", token, expected.Type, token.Type)
				}
				if token.Span.Start != expected.Span.Start {
					t.Errorf("token %v: expected start %d, got %d", token, expected.Span.Start, token.Span.Start)
				}
				if token.Span.End != expected.Span.End {
					t.Errorf("token %v: expected end %d, got %d", token, expected.Span.End, token.Span.End)
				}
			}
		})
	}
}
