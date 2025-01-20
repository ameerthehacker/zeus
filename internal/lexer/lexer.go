package lexer

import (
	"ameerthehacker/zeus/internal/common/error"
	"ameerthehacker/zeus/internal/common/token"
	"fmt"
	"unicode"
)

type Lexer struct {
	source string
	cursor int
	start int
	tokens []*token.Token
	errors []*error.ZeusError
}

func NewLexer(source string) *Lexer {
	return &Lexer{source: source, cursor: 0, start: 0, tokens: []*token.Token{}}
}

func (l *Lexer) advance() {
	l.cursor++;
}

func (l *Lexer) isEOF() bool {
	return l.cursor >= len(l.source)
}

func (l *Lexer) pushToken(token *token.Token) {
	l.tokens = append(l.tokens, token)
}

func (l *Lexer) eatNumber() {
	isFloat := false
	l.start = l.cursor

	for !l.isEOF() && unicode.IsDigit(rune(l.source[l.cursor])) {
		l.advance()

		if !l.isEOF() && rune(l.source[l.cursor]) == '.' {
			if isFloat {
				l.pushError(error.NewZeusError(
					error.ErrorSeverityError,
					"invalid decimal point",
					token.NewSpan(l.cursor, l.cursor),
				))
			}
			isFloat = true
			l.advance()
		}
	}
	end := l.cursor - 1

	l.pushToken(token.NewTokenWithValue(token.TokenTypeNumber, l.source[l.start:l.cursor], token.NewSpan(l.start, end)))
}

func (l *Lexer) pushError(err *error.ZeusError) {
	l.errors = append(l.errors, err)
}

func (l *Lexer) Lex() ([]*token.Token, []*error.ZeusError) {
	l.tokens = []*token.Token{}
	l.errors = []*error.ZeusError{}
	runes := []rune(l.source)

	for !l.isEOF() {
		char := runes[l.cursor]

		if unicode.IsSpace(char) || char == '\n' || char == '\t' || char == '\r' {
			l.advance()
		} else if char == '[' {
			l.pushToken(token.NewToken(token.TokenTypeLeftBracket, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if char == ']' {
			l.pushToken(token.NewToken(token.TokenTypeRightBracket, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if char == '(' {
			l.pushToken(token.NewToken(token.TokenTypeLeftParen, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if char == ')' {
			l.pushToken(token.NewToken(token.TokenTypeRightParen, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if char == '{' {
			l.pushToken(token.NewToken(token.TokenTypeLeftBrace, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if char == '}' {
			l.pushToken(token.NewToken(token.TokenTypeRightBrace, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if char == ';' {
			l.pushToken(token.NewToken(token.TokenTypeSemicolon, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if char == ',' {
			l.pushToken(token.NewToken(token.TokenTypeComma, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if char == '+' {
			l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypePlus, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if char == '-' {
			l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeMinus, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if char == '*' {
			l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeStar, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if char == '/' {
			l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeSlash, token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		} else if unicode.IsDigit(char) {
			l.eatNumber()
		} else {
			l.pushError(error.NewZeusError(error.ErrorSeverityError, fmt.Sprintf("unknown token: '%c'", char), token.NewSpan(l.cursor, l.cursor)))
			l.advance()
		}
	}

	l.pushToken(token.NewToken(token.TokenTypeEOF, token.NewSpan(l.cursor, l.cursor)))

	return l.tokens, l.errors
}
