package token

type OperatorType TokenType

const (
	OperatorTypePlus OperatorType = "+"
	OperatorTypeMinus OperatorType = "-"
	OperatorTypeStar OperatorType = "*"
	OperatorTypeSlash OperatorType = "/"
)

var Operators = map[OperatorType]bool{
	OperatorTypePlus: true,
	OperatorTypeMinus: true,
	OperatorTypeStar: true,
	OperatorTypeSlash: true,
}

func IsOperator(tokenType TokenType) bool {
	return Operators[OperatorType(tokenType)]
}
