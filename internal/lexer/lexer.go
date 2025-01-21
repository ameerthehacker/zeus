package lexer

import (
	"ameerthehacker/zeus/internal/common/error"
	"ameerthehacker/zeus/internal/common/token"
	"fmt"
	"unicode"
)

	type Lexer struct {
		source []rune
		cursor int
		tokens []*token.Token
		errors []*error.ZeusError
	}

	func NewLexer(source string) *Lexer {
		return &Lexer{source: []rune(source), cursor: 0, tokens: []*token.Token{}}
	}

	func (l *Lexer) advance() {
		l.cursor++;
	}

	func (l *Lexer) isEOF(offset int) bool {
		return l.cursor + offset >= len(l.source)
	}

	func (l *Lexer) pushToken(token *token.Token) {
		l.tokens = append(l.tokens, token)
	}

	func isIdentifierRune(char rune) bool {
		return unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_' || char == '$'
	}

	func (l *Lexer) eatIdentifierOrKeywordOrDatatype() {
		start := l.cursor
		l.advance()

		for !l.isEOF(0) && isIdentifierRune(l.source[l.cursor]) {
			l.advance()
		}

		identifier := string(l.source[start:l.cursor])

		if token.IsKeyword(identifier) {
			l.pushToken(token.NewTokenWithValue(token.TokenTypeKeyword, identifier, token.NewSpan(start, l.cursor - 1)))
		} else if token.IsDataType(identifier) {
			l.pushToken(token.NewTokenWithValue(token.TokenTypeDataType, identifier, token.NewSpan(start, l.cursor - 1)))
		} else {
			l.pushToken(token.NewTokenWithValue(token.TokenTypeIdentifier, identifier, token.NewSpan(start, l.cursor - 1)))
		}
	}

	func (l *Lexer) eatNumber() {
		isFloat := false
		start := l.cursor
		isRadix10 := true

		isDigit := func () bool {
			if l.isEOF(0) {
				return false
			}
			index := l.cursor - start
			char := unicode.ToLower(l.source[l.cursor])
			isRadixPrefix := char == 'b' || char == 'o' || char == 'x'
		
			if index == 1 && (l.matchPreviousRune('0') && isRadixPrefix){
				isRadix10 = false
				return true
			}
		
			return unicode.IsDigit(char)
		}

		for isDigit() {
			l.advance()

			if l.matchRune('.') && isRadix10 {
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
			// support for numerical separator // 2_00_00_000
			if l.matchRune('_') {
				if !l.matchNext(func(char rune) bool {
					return unicode.IsDigit(char)
				}) {
					l.pushError(error.NewZeusError(
						error.ErrorSeverityError,
						"numerical separator must be followed by a digit",
						token.NewSpan(l.cursor, l.cursor),
					))
				}
				l.advance()
			}
		}
		end := l.cursor - 1

		l.pushToken(token.NewTokenWithValue(token.TokenTypeNumber, string(l.source[start:l.cursor]), token.NewSpan(start, end)))
	}

	func (l *Lexer) pushError(err *error.ZeusError) {
		l.errors = append(l.errors, err)
	}

	func (l* Lexer) matchRune(char rune) bool {
		return !l.isEOF(0) && l.source[l.cursor] == char
	}

	func (l *Lexer) matchNextRune(char rune) bool {
		return !l.isEOF(1) && l.source[l.cursor + 1] == char
	}

	func (l *Lexer) matchPreviousRune(char rune) bool {
		return l.cursor > 0 && l.source[l.cursor - 1] == char
	}

	func (l *Lexer) matchNext(fn func(rune) bool) bool {
		return !l.isEOF(1) && fn(l.source[l.cursor + 1])
	}

	func (l *Lexer) Lex() ([]*token.Token, []*error.ZeusError) {
		l.tokens = []*token.Token{}
		l.errors = []*error.ZeusError{}

		for !l.isEOF(0) {
			char := l.source[l.cursor]

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
			} else if char == ':' {
				l.pushToken(token.NewToken(token.TokenTypeColon, token.NewSpan(l.cursor, l.cursor)))
				l.advance()
			} else if char == '.' {
				l.pushToken(token.NewToken(token.TokenTypeDot, token.NewSpan(l.cursor, l.cursor)))
				l.advance()
			} else if char == '=' {
				if l.matchNextRune('=') {
					l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeEqualEqual, token.NewSpan(l.cursor, l.cursor + 1)))
					l.advance()
				} else {
					l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeEqual, token.NewSpan(l.cursor, l.cursor)))
				}
				l.advance()
			} else if char == '!' {
				if l.matchNextRune('=') {
					l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeBangEqual, token.NewSpan(l.cursor, l.cursor + 1)))
					l.advance()
				} else {
					l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeBang, token.NewSpan(l.cursor, l.cursor)))
				}
				l.advance()
			} else if char == '>' {
				if l.matchNextRune('=') {
					l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeGreaterThanEqual, token.NewSpan(l.cursor, l.cursor + 1)))
					l.advance()
				} else {
					l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeGreaterThan, token.NewSpan(l.cursor, l.cursor)))
				}
				l.advance()
			} else if char == '<' {
				if l.matchNextRune('=') {
					l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeLessThanEqual, token.NewSpan(l.cursor, l.cursor + 1)))
					l.advance()
				} else {
					l.pushToken(token.NewTokenWithValue(token.TokenTypeOperator, token.OperatorTypeLessThan, token.NewSpan(l.cursor, l.cursor)))
				}
				l.advance()
			} else if unicode.IsDigit(char) {
				l.eatNumber()
			} else if isIdentifierRune(char) {
				l.eatIdentifierOrKeywordOrDatatype()
			} else {
				l.pushError(error.NewZeusError(error.ErrorSeverityError, fmt.Sprintf("unknown token: '%c'", char), token.NewSpan(l.cursor, l.cursor)))
				l.advance()
			}
		}

		l.pushToken(token.NewToken(token.TokenTypeEOF, token.NewSpan(l.cursor, l.cursor)))

		return l.tokens, l.errors
	}
