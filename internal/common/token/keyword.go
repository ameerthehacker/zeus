package token

const (
	KeywordTypeIf string = "if"
	KeywordTypeElse string = "else"
	KeywordTypeFunction string = "function"
)

var Keywords = map[string]bool{
	KeywordTypeIf: true,
	KeywordTypeElse: true,
	KeywordTypeFunction: true,
}

func IsKeyword(tokenValue string) bool {
	return Keywords[tokenValue]
}
