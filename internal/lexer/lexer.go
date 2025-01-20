package lexer

import (
	"ameerthehacker/zeus/internal/common/error"
	"ameerthehacker/zeus/internal/common/token"
	"fmt"
)

type Lexer struct {
	source string
	cursor int
	start int
	tokens []token.Token
}

func NewLexer(source string) *Lexer {
	return &Lexer{source: source, cursor: 0, start: 0, tokens: []token.Token{}}
}

func (l *Lexer) advance() {
	l.cursor++;
}

func (l *Lexer) isEOF() bool {
	return l.cursor >= len(l.source)
}

func (l *Lexer) pushToken(token token.Token) {
	l.tokens = append(l.tokens, token)
	l.advance()
}

func (l *Lexer) Lex() ([]token.Token, []error.ZeusError) {
	l.tokens = []token.Token{}
	errors := []error.ZeusError{}

	for !l.isEOF() {
		char := l.source[l.cursor]

		switch char {
			case ' ':
				fallthrough
			case '\n':
				fallthrough
			case '\t':
				fallthrough
			case '\r':
				l.advance()
			case '[':
				l.pushToken(token.NewToken(token.TokenTypeLeftBracket, token.NewSpan(l.cursor, l.cursor)))
			case ']':
				l.pushToken(token.NewToken(token.TokenTypeRightBracket, token.NewSpan(l.cursor, l.cursor)))
			case '(':
				l.pushToken(token.NewToken(token.TokenTypeLeftParen, token.NewSpan(l.cursor, l.cursor)))
			case ')':
				l.pushToken(token.NewToken(token.TokenTypeRightParen, token.NewSpan(l.cursor, l.cursor)))
			case '{':
				l.pushToken(token.NewToken(token.TokenTypeLeftBrace, token.NewSpan(l.cursor, l.cursor)))
			case '}':
				l.pushToken(token.NewToken(token.TokenTypeRightBrace, token.NewSpan(l.cursor, l.cursor)))
			case ';':
				l.pushToken(token.NewToken(token.TokenTypeSemicolon, token.NewSpan(l.cursor, l.cursor)))
			case ',':
				l.pushToken(token.NewToken(token.TokenTypeComma, token.NewSpan(l.cursor, l.cursor)))
			// operators
			case '+':
				l.pushToken(token.NewTokenWithValue(token.TokenOperator, string(token.OperatorTypePlus), token.NewSpan(l.cursor, l.cursor)))
			case '-':
				l.pushToken(token.NewTokenWithValue(token.TokenOperator, string(token.OperatorTypeMinus), token.NewSpan(l.cursor, l.cursor)))
			case '*':
				l.pushToken(token.NewTokenWithValue(token.TokenOperator, string(token.OperatorTypeStar), token.NewSpan(l.cursor, l.cursor)))
			case '/':
				l.pushToken(token.NewTokenWithValue(token.TokenOperator, string(token.OperatorTypeSlash), token.NewSpan(l.cursor, l.cursor)))
			default:
				errors = append(errors, error.NewZeusError(error.ErrorSeverityError, fmt.Sprintf("unknown token: %c", char), token.NewSpan(l.cursor, l.cursor)))
				l.advance()
		}
	}

	l.pushToken(token.NewToken(token.TokenTypeEOF, token.NewSpan(l.cursor, l.cursor)))

	return l.tokens, errors
}
