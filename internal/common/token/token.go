package token

import "fmt"

type TokenType string

const (
	TokenTypeNumber TokenType = "number"
	TokenTypeOperator TokenType = "operator"
	TokenTypeSemicolon TokenType = ";"
	TokenTypeColon TokenType = ":"
	TokenTypeLeftParen TokenType = "("
	TokenTypeRightParen TokenType = ")"
	TokenTypeLeftBrace TokenType = "{"
	TokenTypeRightBrace TokenType = "}"
	TokenTypeLeftBracket TokenType = "["
	TokenTypeRightBracket TokenType = "]"
	TokenTypeIdentifier TokenType = "identifier"
	TokenTypeComma TokenType = ","
	TokenTypeDot TokenType = "."
	TokenTypeKeyword TokenType = "keyword"
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
	Value *string
	Span *Span
}

func NewTokenWithValue(tokenType TokenType, value string, span *Span) *Token {
	return &Token{Type: tokenType, Value: &value, Span: span}
}

func NewToken(tokenType TokenType, span *Span) *Token {
	return &Token{Type: tokenType, Value: nil, Span: span}
}

func NewSpan(start int, end int) *Span {
	return &Span{Start: start, End: end}
}

func (t *Token) String() string {
	if t.Value == nil {
		return fmt.Sprintf("{Type: %s, Span: %s}", t.Type, t.Span)
	}
	return fmt.Sprintf("{Type: %s, Value: %s, Span: %s}", t.Type, *t.Value, t.Span)
}

func (t *Token) IsEqual(other *Token) bool {
	isValueEqual := true

	if t.Value != nil && other.Value != nil {
		isValueEqual = *t.Value == *other.Value
	}

	return t.Type == other.Type && isValueEqual && t.Span.Start == other.Span.Start && t.Span.End == other.Span.End
}
