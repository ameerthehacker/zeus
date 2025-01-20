package token

import "fmt"

type TokenType string

const (
	TokenTypeNumber TokenType = "number"
	TokenTypeOperator TokenType = "operator"
	TokenTypeSemicolon TokenType = ";"
	TokenTypeLeftParen TokenType = "("
	TokenTypeRightParen TokenType = ")"
	TokenTypeLeftBrace TokenType = "{"
	TokenTypeRightBrace TokenType = "}"
	TokenTypeLeftBracket TokenType = "["
	TokenTypeRightBracket TokenType = "]"
	TokenTypeIdentifier TokenType = "identifier"
	TokenTypeComma TokenType = ","
	TokenTypeDataType TokenType = "datatype"
	TokenTypeEOF TokenType = "EOF"
)

type Span struct {
	Start int
	End int
}

func (s *Span) String() string {
	return fmt.Sprintf("{Start: %d, End: %d}", s.Start, s.End)
}

type Token struct {
	Type TokenType
	Value string
	Span *Span
}

func NewTokenWithValue(tokenType TokenType, value string, span *Span) *Token {
	return &Token{Type: tokenType, Value: value, Span: span}
}

func NewToken(tokenType TokenType, span *Span) *Token {
	return &Token{Type: tokenType, Value: "", Span: span}
}

func NewSpan(start int, end int) *Span {
	return &Span{Start: start, End: end}
}

func (t *Token) String() string {
	return fmt.Sprintf("{Type: %s, Value: %s, Span: %s}", t.Type, t.Value, t.Span)
}

func (t *Token) IsEqual(other *Token) bool {
	return t.Type == other.Type && t.Value == other.Value && t.Span.Start == other.Span.Start && t.Span.End == other.Span.End
}
