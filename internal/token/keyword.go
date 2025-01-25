package token

const (
	// declaration
	KeywordTypeLet string = "let"
	KeywordTypeConst string = "const"
	// function
	KeywordTypeFunction string = "function"
	KeywordTypeReturn string = "return"
	// control flow
	KeywordTypeIf string = "if"
	KeywordTypeElse string = "else"
	// loop
	KeywordTypeWhile string = "while"
	// boolean
	KeywordTypeTrue string = "true"
	KeywordTypeFalse string = "false"
)

var Keywords = map[string]bool{
	KeywordTypeLet: true,
	KeywordTypeConst: true,
	KeywordTypeFunction: true,
	KeywordTypeReturn: true,
	KeywordTypeIf: true,
	KeywordTypeElse: true,
	KeywordTypeWhile: true,
	KeywordTypeTrue: true,
	KeywordTypeFalse: true,
}

func IsKeyword(tokenValue string) bool {
	return Keywords[tokenValue]
}
