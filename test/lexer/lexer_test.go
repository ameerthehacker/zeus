package lexer_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/error"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/test_utils"
	"github.com/ameerthehacker/zeus/internal/token"
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
			name: "ignores comments",
			input: "// this is a comment",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 21), *token.NewPosition(1, 21))),
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
				token.NewToken(token.TokenTypeBangEqual, token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 2))),
				token.NewToken(token.TokenTypeEqualEqual, token.NewSpan(*token.NewPosition(1, 4), *token.NewPosition(1, 5))), 
				token.NewToken(token.TokenTypeEqual, token.NewSpan(*token.NewPosition(1, 7), *token.NewPosition(1, 7))),
				token.NewToken(token.TokenTypeGreaterThan, token.NewSpan(*token.NewPosition(1, 9), *token.NewPosition(1, 9))),
				token.NewToken(token.TokenTypeLessThan, token.NewSpan(*token.NewPosition(1, 11), *token.NewPosition(1, 11))),
				token.NewToken(token.TokenTypeGreaterThanEqual, token.NewSpan(*token.NewPosition(1, 13), *token.NewPosition(1, 14))),
				token.NewToken(token.TokenTypeLessThanEqual, token.NewSpan(*token.NewPosition(1, 16), *token.NewPosition(1, 17))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 18), *token.NewPosition(1, 18))),
			},
		},
		{
			name:     "unknown token error",
			input:    "@",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 2), *token.NewPosition(1, 2))),
			},
			errors: []*error.ZeusError{error.NewZeusError(error.ErrorSeverityError, "unknown token '@'", token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 1)))},
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
				token.NewToken(token.TokenTypeIf, token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 2))),
				token.NewToken(token.TokenTypeElse, token.NewSpan(*token.NewPosition(1, 4), *token.NewPosition(1, 7))),
				token.NewToken(token.TokenTypeFunction, token.NewSpan(*token.NewPosition(1, 9), *token.NewPosition(1, 16))),
				token.NewToken(token.TokenTypeLet, token.NewSpan(*token.NewPosition(1, 18), *token.NewPosition(1, 20))),
				token.NewToken(token.TokenTypeConst, token.NewSpan(*token.NewPosition(1, 22), *token.NewPosition(1, 26))),
				token.NewToken(token.TokenTypeWhile, token.NewSpan(*token.NewPosition(1, 28), *token.NewPosition(1, 32))),
				token.NewToken(token.TokenTypeReturn, token.NewSpan(*token.NewPosition(1, 34), *token.NewPosition(1, 39))),
				token.NewToken(token.TokenTypeTrue, token.NewSpan(*token.NewPosition(1, 41), *token.NewPosition(1, 44))),
				token.NewToken(token.TokenTypeFalse, token.NewSpan(*token.NewPosition(1, 46), *token.NewPosition(1, 50))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 51), *token.NewPosition(1, 51))),
			},
		},
		{
			name: "integer datatype",
			input: "i8 i16 i32 i64 i128",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeInt8, token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 2))),
				token.NewToken(token.TokenTypeInt16, token.NewSpan(*token.NewPosition(1, 4), *token.NewPosition(1, 6))),
				token.NewToken(token.TokenTypeInt32, token.NewSpan(*token.NewPosition(1, 8), *token.NewPosition(1, 10))),
				token.NewToken(token.TokenTypeInt64, token.NewSpan(*token.NewPosition(1, 12), *token.NewPosition(1, 14))),
				token.NewToken(token.TokenTypeInt128, token.NewSpan(*token.NewPosition(1, 16), *token.NewPosition(1, 19))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 20), *token.NewPosition(1, 20))),
			},
		},
		{
			name: "float datatype",
			input: "f32 f64",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeFloat32, token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 3))),
				token.NewToken(token.TokenTypeFloat64, token.NewSpan(*token.NewPosition(1, 5), *token.NewPosition(1, 7))),
				token.NewToken(token.TokenTypeEOF, token.NewSpan(*token.NewPosition(1, 8), *token.NewPosition(1, 8))),
			},
		},
		{
			name: "boolean datatype",
			input: "boolean",
			expected: []*token.Token{
				token.NewToken(token.TokenTypeBoolean, token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 7))),
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			l := lexer.NewLexer(test.input)
			tokens, errors := l.Lex()

			test_utils.CompareZeusErrors(t, errors, test.errors)

			for i, token := range tokens {
				expected := test.expected[i]
				if !token.IsEqual(expected) {
					t.Errorf("expected token %s, got %s", expected, token)
				}
			}
		})
	}
}
