package token

const (
	OperatorTypePlus string = "+"
	OperatorTypeMinus string = "-"
	OperatorTypeStar string = "*"
	OperatorTypeSlash string = "/"
)

var Operators = map[string]bool{
	OperatorTypePlus: true,
	OperatorTypeMinus: true,
	OperatorTypeStar: true,
	OperatorTypeSlash: true,
}

func IsOperator(tokenType TokenType) bool {
	return Operators[string(tokenType)]
}
