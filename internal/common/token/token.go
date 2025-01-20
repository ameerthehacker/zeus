package token

type TokenType string

const (
	TokenTypeNumber TokenType = "number"
	TokenOperator TokenType = "operator"
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

type Token struct {
	Type TokenType
	Value string
	Span Span
}

func NewTokenWithValue(tokenType TokenType, value string, span Span) Token {
	return Token{Type: tokenType, Value: value, Span: span}
}

func NewToken(tokenType TokenType, span Span) Token {
	return Token{Type: tokenType, Value: "", Span: span}
}

func NewSpan(start int, end int) Span {
	return Span{Start: start, End: end}
}
