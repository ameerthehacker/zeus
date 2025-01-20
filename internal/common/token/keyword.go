package token

type KeywordType TokenType

const (
	KeywordTypeIf KeywordType = "if"
	KeywordTypeElse KeywordType = "else"
	KeywordTypeFunction KeywordType = "function"
)

var Keywords = map[KeywordType]bool{
	KeywordTypeIf: true,
	KeywordTypeElse: true,
	KeywordTypeFunction: true,
}

func IsKeyword(tokenType TokenType) bool {
	return Keywords[KeywordType(tokenType)]
}
