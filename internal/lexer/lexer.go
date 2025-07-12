package lexer

import (
	"fmt"
	"unicode"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

	type Lexer struct {
		source []rune
		cursor int
		tokens []*token.Token
		errors []*zeus_error.ZeusError
		line int
		column int
	}

	func NewLexer(source string) *Lexer {
		return &Lexer{source: []rune(source), cursor: 0, tokens: []*token.Token{}, line: 1, column: 1}
	}

	func (l *Lexer) advance() {
		l.cursor++;
		l.column++;
	}

	func (l *Lexer) newLine() {
		l.line++;
		l.column = 1;
	}

	func (l *Lexer) isEOF(offset int) bool {
		return l.cursor + offset >= len(l.source)
	}

	func (l *Lexer) pushToken(token *token.Token) {
		l.tokens = append(l.tokens, token)
	}

	func (l* Lexer) isNewLine() bool {
		return l.source[l.cursor] == '\n' || l.source[l.cursor] == '\r'
	}

	func (l* Lexer) consumeLine() {
		for !l.isEOF(0) && !l.isNewLine() {
			l.advance()
		}
	}

	func isIdentifierRune(char rune) bool {
		return unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_' || char == '$'
	}

	func (l *Lexer) eatIdentifierOrKeywordOrDatatype() {
		startPosition := l.getCurrentPosition()
		start := l.cursor
		var endPosition *token.Position = nil

		for !l.isEOF(0) && isIdentifierRune(l.source[l.cursor]) {
			endPosition = l.getCurrentPosition()
			l.advance()
		}

		identifier := string(l.source[start:l.cursor])
		span := token.NewSpan(*startPosition, *endPosition)
		keywordTokenType, isKeyword := token.Keywords[identifier]
		dataTypeTokenType, isDataType := token.DataTypes[identifier]

		if isKeyword {
			l.pushToken(token.NewToken(keywordTokenType, span))
		} else if isDataType {
			l.pushToken(token.NewToken(dataTypeTokenType, span))
		} else {
			l.pushToken(token.NewTokenWithValue(token.TokenTypeIdentifier, identifier, span))
		}
	}

	func (l *Lexer) eatNumber() {
		isFloat := false
		start := l.cursor
		startPosition := l.getCurrentPosition()
		var endPosition *token.Position = nil
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
			endPosition = l.getCurrentPosition()
			l.advance()

			if l.matchRune('.') && isRadix10 {
				if isFloat {
					errorPosition := l.getCurrentPosition()
					l.pushError(zeus_error.NewZeusError(
						zeus_error.ErrorSeverityError,
						"invalid decimal point",
						token.NewSpan(*errorPosition, *errorPosition),
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
					errorPosition := l.getCurrentPosition()
					endPosition = errorPosition
					l.pushError(zeus_error.NewZeusError(
						zeus_error.ErrorSeverityError,
						"numerical separator must be followed by a digit",
						token.NewSpan(*errorPosition, *errorPosition),
					))
				}
				l.advance()
			}
		}

		l.pushToken(token.NewTokenWithValue(token.TokenTypeNumber, string(l.source[start:l.cursor]), token.NewSpan(*startPosition, *endPosition)))
	}

	func (l *Lexer) eatString() {
		start := l.cursor
		startPosition := l.getCurrentPosition()
		l.advance()
		for !l.isEOF(0) && l.source[l.cursor] != '"' {
			l.advance()
		}

		if l.isEOF(0) {
			l.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "unterminated string", token.NewSpan(*startPosition, *startPosition)))
			return
		}

		endPosition := l.getCurrentPosition()
		l.advance()

		l.pushToken(token.NewTokenWithValue(token.TokenTypeString, string(l.source[start + 1:l.cursor - 1]), token.NewSpan(*startPosition, *endPosition)))
	}

	func (l *Lexer) pushError(err *zeus_error.ZeusError) {
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

	func (l* Lexer) getCurrentPosition() *token.Position {
		return token.NewPosition(l.line, l.column)
	}

	func (l *Lexer) Lex() ([]*token.Token, []*zeus_error.ZeusError) {
		l.tokens = []*token.Token{}
		l.errors = []*zeus_error.ZeusError{}

		for !l.isEOF(0) {
			char := l.source[l.cursor]
			position := l.getCurrentPosition()

			switch {
			case l.isNewLine():
				l.advance()
				l.newLine()
			case unicode.IsSpace(char):
				l.advance()
			case char == '[':
				l.pushToken(token.NewToken(token.TokenTypeLeftBracket, token.NewSpan(*position, *position)))
				l.advance()
			case char == ']':
				l.pushToken(token.NewToken(token.TokenTypeRightBracket, token.NewSpan(*position, *position)))
				l.advance()
			case char == '(':
				l.pushToken(token.NewToken(token.TokenTypeLeftParen, token.NewSpan(*position, *position)))
				l.advance()
			case char == ')':
				l.pushToken(token.NewToken(token.TokenTypeRightParen, token.NewSpan(*position, *position)))
				l.advance()
			case char == '{':
				l.pushToken(token.NewToken(token.TokenTypeLeftBrace, token.NewSpan(*position, *position)))
				l.advance()
			case char == '}':
				l.pushToken(token.NewToken(token.TokenTypeRightBrace, token.NewSpan(*position, *position)))
				l.advance()
			case char == ';':
				l.pushToken(token.NewToken(token.TokenTypeSemicolon, token.NewSpan(*position, *position)))
				l.advance()
			case char == ',':
				l.pushToken(token.NewToken(token.TokenTypeComma, token.NewSpan(*position, *position)))
				l.advance()
			case char == '+':
				l.pushToken(token.NewToken(token.TokenTypePlus, token.NewSpan(*position, *position)))
				l.advance()
			case char == '-':
				l.pushToken(token.NewToken(token.TokenTypeMinus, token.NewSpan(*position, *position)))
				l.advance()
			case char == '*':
				l.pushToken(token.NewToken(token.TokenTypeStar, token.NewSpan(*position, *position)))
				l.advance()
			case char == '/':
				if l.matchNextRune('/') {
					l.consumeLine()
				} else {
					l.pushToken(token.NewToken(token.TokenTypeSlash, token.NewSpan(*position, *position)))
					l.advance()
				}
			case char == ':':
				l.pushToken(token.NewToken(token.TokenTypeColon, token.NewSpan(*position, *position)))
				l.advance()
			case char == '.':
				l.pushToken(token.NewToken(token.TokenTypeDot, token.NewSpan(*position, *position)))
				l.advance()
			case char == '=':
				startPosition := position
				if l.matchNextRune('=') {
					l.advance()
					endPosition := l.getCurrentPosition()
					l.pushToken(token.NewToken(token.TokenTypeEqualEqual, token.NewSpan(*startPosition, *endPosition)))
				} else {
					l.pushToken(token.NewToken(token.TokenTypeEqual, token.NewSpan(*startPosition, *startPosition)))
				}
				l.advance()
			case char == '!':
				startPosition := position
				if l.matchNextRune('=') {
					l.advance()
					endPosition := l.getCurrentPosition()
					l.pushToken(token.NewToken(token.TokenTypeBangEqual, token.NewSpan(*startPosition, *endPosition)))
				} else {
					l.pushToken(token.NewToken(token.TokenTypeBang, token.NewSpan(*startPosition, *startPosition)))
				}
				l.advance()
			case char == '>':
				startPosition := position
				if l.matchNextRune('=') {
					l.advance()
					endPosition := l.getCurrentPosition()
					l.pushToken(token.NewToken(token.TokenTypeGreaterThanEqual, token.NewSpan(*startPosition, *endPosition)))
				} else {
					l.pushToken(token.NewToken(token.TokenTypeGreaterThan, token.NewSpan(*startPosition, *startPosition)))
				}
				l.advance()
			case char == '<':
				startPosition := position
				if l.matchNextRune('=') {
					l.advance()
					endPosition := l.getCurrentPosition()
					l.pushToken(token.NewToken(token.TokenTypeLessThanEqual, token.NewSpan(*startPosition, *endPosition)))
				} else {
					l.pushToken(token.NewToken(token.TokenTypeLessThan, token.NewSpan(*startPosition, *startPosition)))
				}
				l.advance()
			case unicode.IsDigit(char):
				l.eatNumber()
			case char == '"':
				l.eatString()
			case isIdentifierRune(char):
				l.eatIdentifierOrKeywordOrDatatype()
			default:
				l.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("unknown token '%c'", char), token.NewSpan(*position, *position)))
				l.advance()
			}
		}

		position := l.getCurrentPosition()
		l.pushToken(token.NewToken(token.TokenTypeEOF, token.NewSpan(*position, *position)))

		return l.tokens, l.errors
	}
