package token

import "fmt"

type TokenType string

const (
	TokenTypeSemicolon TokenType = ";"
	TokenTypeColon TokenType = ":"
	TokenTypeLeftParen TokenType = "("
	TokenTypeRightParen TokenType = ")"
	TokenTypeLeftBrace TokenType = "{"
	TokenTypeRightBrace TokenType = "}"
	TokenTypeLeftBracket TokenType = "["
	TokenTypeRightBracket TokenType = "]"
	TokenTypeComma TokenType = ","
	TokenTypeDot TokenType = "."
	TokenTypeKeyword TokenType = "keyword"
	TokenTypeDataType TokenType = "datatype"
	// operators
	TokenTypePlus TokenType = "+"
	TokenTypeMinus TokenType = "-"
	TokenTypeStar TokenType = "*"
	TokenTypeSlash TokenType = "/"
	TokenTypeEqual TokenType = "="
	TokenTypeBang TokenType = "!"
	TokenTypeEqualEqual TokenType = "=="
	TokenTypeBangEqual TokenType = "!="
	TokenTypeGreaterThan TokenType = ">"
	TokenTypeGreaterThanEqual TokenType = ">="
	TokenTypeLessThan TokenType = "<"
	TokenTypeLessThanEqual TokenType = "<="
	// literals
	TokenTypeNumber TokenType = "number"
	TokenTypeIdentifier TokenType = "identifier"
	// EOF
	TokenTypeEOF TokenType = "EOF"
)

type Position struct {
	Line int
	Column int
}

func NewPosition(line int, column int) *Position {
	return &Position{Line: line, Column: column}
}

func (p *Position) String() string {
	return fmt.Sprintf("{Line: %d, Column: %d}", p.Line, p.Column)
}

func (p *Position) IsEqual(other *Position) bool {
	return p.Line == other.Line && p.Column == other.Column
}

type Span struct {
	Start Position
	End Position
}

func (s *Span) String() string {
	return fmt.Sprintf("{Start: %s, End: %s}", &s.Start, &s.End)
}

func (s *Span) IsEqual(other *Span) bool {
	return s.Start.IsEqual(&other.Start) && s.End.IsEqual(&other.End)
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

func NewSpan(start Position, end Position) *Span {
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

	return t.Type == other.Type && isValueEqual && t.Span.IsEqual(other.Span)
}
