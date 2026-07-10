package lexer

import (
	"fmt"
	"unicode"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

type Lexer struct {
	source              []rune
	cursor              int
	tokens              []*token.Token
	errors              []*zeus_error.ZeusError
	line                int
	column              int
	lastToken           *token.Token
	// templateBraceStack tracks open-brace depth inside template string expressions.
	// Each entry corresponds to one active interpolation; the value is the depth of
	// nested '{' seen so far. When it hits 0 and we see '}', the interpolation ends.
	templateBraceStack  []int
}

func NewLexer(source string) *Lexer {
	return &Lexer{source: []rune(source), cursor: 0, tokens: []*token.Token{}, line: 1, column: 1}
}

func (l *Lexer) advance() {
	l.cursor++
	l.column++
}

func (l *Lexer) newLine() {
	l.line++
	l.column = 1
}

func (l *Lexer) isEOF(offset int) bool {
	return l.cursor+offset >= len(l.source)
}

func (l *Lexer) pushToken(t *token.Token) {
	l.tokens = append(l.tokens, t)
	l.lastToken = t
}

func (l *Lexer) shouldInsertSemicolon() bool {
	if l.lastToken == nil {
		return false
	}
	switch l.lastToken.Type {
	case token.TokenTypeIdentifier,
		token.TokenTypeNumber,
		token.TokenTypeString,
		token.TokenTypeChar,
		token.TokenTypeTrue,
		token.TokenTypeFalse,
		token.TokenTypeNull,
		token.TokenTypeRightParen,
		token.TokenTypeRightBracket,
		token.TokenTypePlusPlus,
		token.TokenTypeMinusMinus,
		token.TokenTypeTemplateLiteralEnd:
		return true
	}
	return false
}

func (l *Lexer) isNewLine() bool {
	return l.source[l.cursor] == '\n' || l.source[l.cursor] == '\r'
}

func (l *Lexer) consumeLine() {
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

	isDigit := func() bool {
		if l.isEOF(0) {
			return false
		}
		index := l.cursor - start
		char := unicode.ToLower(l.source[l.cursor])
		isRadixPrefix := char == 'b' || char == 'o' || char == 'x'

		if index == 1 && (l.matchPreviousRune('0') && isRadixPrefix) {
			isRadix10 = false
			return true
		}

		if !isRadix10 {
			// Allow hex digits a-f when in non-decimal mode (0x...)
			return unicode.IsDigit(char) || ('a' <= char && char <= 'f')
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

func (l *Lexer) eatStringOrChar(isChar bool) {
	startPosition := l.getCurrentPosition()
	l.advance() // consume opening quote
	quote := '"'
	if isChar {
		quote = '\''
	}

	var result []rune
	for !l.isEOF(0) && l.source[l.cursor] != quote {
		if l.source[l.cursor] == '\\' && !l.isEOF(1) {
			// Handle escape sequences
			l.advance() // consume backslash
			escaped := l.processEscapeSequence()
			result = append(result, escaped)
		} else {
			result = append(result, l.source[l.cursor])
			l.advance()
		}
	}

	tokenType := token.TokenTypeString
	if isChar {
		tokenType = token.TokenTypeChar
	}

	if l.isEOF(0) {
		l.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("unterminated %s literal", tokenType), token.NewSpan(*startPosition, *startPosition)))
		return
	}

	endPosition := l.getCurrentPosition()
	l.advance() // consume closing quote

	stringLiteral := string(result)

	if isChar && len(stringLiteral) != 1 {
		l.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "char literal must be a single byte", token.NewSpan(*startPosition, *startPosition)))
		return
	}

	l.pushToken(token.NewTokenWithValue(tokenType, stringLiteral, token.NewSpan(*startPosition, *endPosition)))
}

func (l *Lexer) eatTemplateLiteral() {
	l.advance() // consume opening backtick
	l.eatTemplateLiteralParts()
}

// eatTemplateLiteralParts scans the static portion of a template literal, emitting
// a TemplateLiteralStr token (possibly empty) followed by either ExprStart+push or End.
// It is also called after each ExprEnd to continue scanning the rest of the literal.
func (l *Lexer) eatTemplateLiteralParts() {
	startPosition := l.getCurrentPosition()
	var result []rune

	for {
		if l.isEOF(0) {
			l.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "unterminated template literal", token.NewSpan(*startPosition, *startPosition)))
			return
		}
		ch := l.source[l.cursor]
		if ch == '`' {
			endPosition := l.getCurrentPosition()
			l.pushToken(token.NewTokenWithValue(token.TokenTypeTemplateLiteralStr, string(result), token.NewSpan(*startPosition, *endPosition)))
			l.advance() // consume closing backtick
			closePosition := l.getCurrentPosition()
			l.pushToken(token.NewToken(token.TokenTypeTemplateLiteralEnd, token.NewSpan(*closePosition, *closePosition)))
			return
		}
		if ch == '$' && !l.isEOF(1) && l.source[l.cursor+1] == '{' {
			endPosition := l.getCurrentPosition()
			l.pushToken(token.NewTokenWithValue(token.TokenTypeTemplateLiteralStr, string(result), token.NewSpan(*startPosition, *endPosition)))
			l.advance() // consume '$'
			l.advance() // consume '{'
			exprStartPos := l.getCurrentPosition()
			l.pushToken(token.NewToken(token.TokenTypeTemplateLiteralExprStart, token.NewSpan(*exprStartPos, *exprStartPos)))
			l.templateBraceStack = append(l.templateBraceStack, 0)
			return // main Lex() loop takes over for the expression tokens
		}
		if ch == '\\' && !l.isEOF(1) {
			l.advance() // consume backslash
			result = append(result, l.processEscapeSequence())
		} else {
			result = append(result, ch)
			l.advance()
			if l.isNewLine() {
				l.newLine()
			}
		}
	}
}

// processEscapeSequence processes an escape sequence after the backslash has been consumed
func (l *Lexer) processEscapeSequence() rune {
	if l.isEOF(0) {
		return '\\'
	}

	char := l.source[l.cursor]
	l.advance()

	switch char {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case '\\':
		return '\\'
	case '"':
		return '"'
	case '\'':
		return '\''
	case '0':
		return '\x00'
	default:
		// Unknown escape sequence - return the character as-is
		return char
	}
}

func (l *Lexer) pushError(err *zeus_error.ZeusError) {
	l.errors = append(l.errors, err)
}

func (l *Lexer) matchRune(char rune) bool {
	return !l.isEOF(0) && l.source[l.cursor] == char
}

func (l *Lexer) matchNextRune(char rune) bool {
	return !l.isEOF(1) && l.source[l.cursor+1] == char
}

func (l *Lexer) matchPreviousRune(char rune) bool {
	return l.cursor > 0 && l.source[l.cursor-1] == char
}

func (l *Lexer) matchNext(fn func(rune) bool) bool {
	return !l.isEOF(1) && fn(l.source[l.cursor+1])
}

func (l *Lexer) getCurrentPosition() *token.Position {
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
			if l.shouldInsertSemicolon() {
				pos := l.lastToken.Span.End
				l.pushToken(token.NewToken(token.TokenTypeSemicolon, token.NewSpan(pos, pos)))
			}
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
			if len(l.templateBraceStack) > 0 {
				l.templateBraceStack[len(l.templateBraceStack)-1]++
			}
			l.advance()
		case char == '}':
			if len(l.templateBraceStack) > 0 {
				top := l.templateBraceStack[len(l.templateBraceStack)-1]
				if top > 0 {
					l.templateBraceStack[len(l.templateBraceStack)-1]--
					l.pushToken(token.NewToken(token.TokenTypeRightBrace, token.NewSpan(*position, *position)))
					l.advance()
				} else {
					l.templateBraceStack = l.templateBraceStack[:len(l.templateBraceStack)-1]
					l.advance()
					l.pushToken(token.NewToken(token.TokenTypeTemplateLiteralExprEnd, token.NewSpan(*position, *position)))
					l.eatTemplateLiteralParts()
				}
			} else {
				l.pushToken(token.NewToken(token.TokenTypeRightBrace, token.NewSpan(*position, *position)))
				l.advance()
			}
		case char == '`':
			l.eatTemplateLiteral()
		case char == ';':
			l.pushToken(token.NewToken(token.TokenTypeSemicolon, token.NewSpan(*position, *position)))
			l.advance()
		case char == ',':
			l.pushToken(token.NewToken(token.TokenTypeComma, token.NewSpan(*position, *position)))
			l.advance()
		case char == '+':
			startPosition := position
			if l.matchNextRune('+') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypePlusPlus, token.NewSpan(*startPosition, *endPosition)))
			} else if l.matchNextRune('=') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypePlusEqual, token.NewSpan(*startPosition, *endPosition)))
			} else {
				l.pushToken(token.NewToken(token.TokenTypePlus, token.NewSpan(*startPosition, *startPosition)))
			}
			l.advance()
		case char == '-':
			startPosition := position
			if l.matchNextRune('-') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeMinusMinus, token.NewSpan(*startPosition, *endPosition)))
			} else if l.matchNextRune('=') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeMinusEqual, token.NewSpan(*startPosition, *endPosition)))
			} else {
				l.pushToken(token.NewToken(token.TokenTypeMinus, token.NewSpan(*startPosition, *startPosition)))
			}
			l.advance()
		case char == '*':
			startPosition := position
			if l.matchNextRune('*') {
				l.advance()
				if l.matchNextRune('=') {
					l.advance()
					endPosition := l.getCurrentPosition()
					l.pushToken(token.NewToken(token.TokenTypeDoubleStarEqual, token.NewSpan(*startPosition, *endPosition)))
				} else {
					endPosition := l.getCurrentPosition()
					l.pushToken(token.NewToken(token.TokenTypeDoubleStar, token.NewSpan(*startPosition, *endPosition)))
				}
			} else if l.matchNextRune('=') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeStarEqual, token.NewSpan(*startPosition, *endPosition)))
			} else {
				l.pushToken(token.NewToken(token.TokenTypeStar, token.NewSpan(*startPosition, *startPosition)))
			}
			l.advance()
		case char == '/':
			startPosition := position
			if l.matchNextRune('/') {
				l.consumeLine()
			} else if l.matchNextRune('=') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeSlashEqual, token.NewSpan(*startPosition, *endPosition)))
				l.advance()
			} else {
				l.pushToken(token.NewToken(token.TokenTypeSlash, token.NewSpan(*startPosition, *startPosition)))
				l.advance()
			}
		case char == '%':
			startPosition := position
			if l.matchNextRune('=') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypePercentEqual, token.NewSpan(*startPosition, *endPosition)))
			} else {
				l.pushToken(token.NewToken(token.TokenTypePercent, token.NewSpan(*startPosition, *startPosition)))
			}
			l.advance()
		case char == '&':
			startPosition := position
			if l.matchNextRune('&') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeAmpAmp, token.NewSpan(*startPosition, *endPosition)))
				l.advance()
			} else if l.matchNextRune('=') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeBitwiseAndEqual, token.NewSpan(*startPosition, *endPosition)))
				l.advance()
			} else {
				l.pushToken(token.NewToken(token.TokenTypeBitwiseAnd, token.NewSpan(*startPosition, *startPosition)))
				l.advance()
			}
		case char == '|':
			startPosition := position
			if l.matchNextRune('|') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypePipePipe, token.NewSpan(*startPosition, *endPosition)))
				l.advance()
			} else if l.matchNextRune('=') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeBitwiseOrEqual, token.NewSpan(*startPosition, *endPosition)))
				l.advance()
			} else {
				l.pushToken(token.NewToken(token.TokenTypeBitwiseOr, token.NewSpan(*startPosition, *startPosition)))
				l.advance()
			}
		case char == '^':
			startPosition := position
			if l.matchNextRune('=') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeBitwiseXorEqual, token.NewSpan(*startPosition, *endPosition)))
			} else {
				l.pushToken(token.NewToken(token.TokenTypeBitwiseXor, token.NewSpan(*startPosition, *startPosition)))
			}
			l.advance()
		case char == '~':
			startPosition := position
			l.pushToken(token.NewToken(token.TokenTypeBitwiseNot, token.NewSpan(*startPosition, *startPosition)))
			l.advance()
		case char == '?':
			l.pushToken(token.NewToken(token.TokenTypeQuestion, token.NewSpan(*position, *position)))
			l.advance()
		case char == ':':
			l.pushToken(token.NewToken(token.TokenTypeColon, token.NewSpan(*position, *position)))
			l.advance()
		case char == '.':
			startPosition := position
			if l.matchNextRune('.') {
				l.advance()
				if l.matchNextRune('.') {
					l.advance()
					endPosition := l.getCurrentPosition()
					l.pushToken(token.NewToken(token.TokenTypeEllipsis, token.NewSpan(*startPosition, *endPosition)))
				} else {
					endPosition := l.getCurrentPosition()
					l.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "unexpected '..'; did you mean '...'?", token.NewSpan(*startPosition, *endPosition)))
				}
			} else {
				l.pushToken(token.NewToken(token.TokenTypeDot, token.NewSpan(*startPosition, *startPosition)))
			}
			l.advance()
		case char == '=':
			startPosition := position
			if l.matchNextRune('=') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeEqualEqual, token.NewSpan(*startPosition, *endPosition)))
			} else if l.matchNextRune('>') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeArrow, token.NewSpan(*startPosition, *endPosition)))
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
			if l.matchNextRune('>') {
				l.advance()
				if l.matchNextRune('=') {
					l.advance()
					endPosition := l.getCurrentPosition()
					l.pushToken(token.NewToken(token.TokenTypeRightShiftEqual, token.NewSpan(*startPosition, *endPosition)))
				} else {
					endPosition := l.getCurrentPosition()
					l.pushToken(token.NewToken(token.TokenTypeRightShift, token.NewSpan(*startPosition, *endPosition)))
				}
			} else if l.matchNextRune('=') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeGreaterThanEqual, token.NewSpan(*startPosition, *endPosition)))
			} else {
				l.pushToken(token.NewToken(token.TokenTypeGreaterThan, token.NewSpan(*startPosition, *startPosition)))
			}
			l.advance()
		case char == '<':
			startPosition := position
			if l.matchNextRune('<') {
				l.advance()
				if l.matchNextRune('=') {
					l.advance()
					endPosition := l.getCurrentPosition()
					l.pushToken(token.NewToken(token.TokenTypeLeftShiftEqual, token.NewSpan(*startPosition, *endPosition)))
				} else {
					endPosition := l.getCurrentPosition()
					l.pushToken(token.NewToken(token.TokenTypeLeftShift, token.NewSpan(*startPosition, *endPosition)))
				}
			} else if l.matchNextRune('=') {
				l.advance()
				endPosition := l.getCurrentPosition()
				l.pushToken(token.NewToken(token.TokenTypeLessThanEqual, token.NewSpan(*startPosition, *endPosition)))
			} else {
				l.pushToken(token.NewToken(token.TokenTypeLessThan, token.NewSpan(*startPosition, *startPosition)))
			}
			l.advance()
		case char == '\'':
			l.eatStringOrChar(true)
		case unicode.IsDigit(char):
			l.eatNumber()
		case char == '"':
			l.eatStringOrChar(false)
		case isIdentifierRune(char):
			l.eatIdentifierOrKeywordOrDatatype()
		default:
			l.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("unknown token '%c'", char), token.NewSpan(*position, *position)))
			l.advance()
		}
	}

	if l.shouldInsertSemicolon() {
		pos := l.lastToken.Span.End
		l.pushToken(token.NewToken(token.TokenTypeSemicolon, token.NewSpan(pos, pos)))
	}
	position := l.getCurrentPosition()
	l.pushToken(token.NewToken(token.TokenTypeEOF, token.NewSpan(*position, *position)))

	return l.tokens, l.errors
}
