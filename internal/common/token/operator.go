package token

const (
	OperatorTypePlus string = "+"
	OperatorTypeMinus string = "-"
	OperatorTypeStar string = "*"
	OperatorTypeSlash string = "/"
	OperatorTypeEqual string = "="
	OperatorTypeBang string = "!"
	OperatorTypeEqualEqual string = "=="
	OperatorTypeBangEqual string = "!="
	OperatorTypeGreaterThan string = ">"
	OperatorTypeGreaterThanEqual string = ">="
	OperatorTypeLessThan string = "<"
	OperatorTypeLessThanEqual string = "<="
)

var Operators = map[string]bool{
	OperatorTypePlus: true,
	OperatorTypeMinus: true,
	OperatorTypeStar: true,
	OperatorTypeSlash: true,
	OperatorTypeEqual: true,
	OperatorTypeBang: true,
	OperatorTypeEqualEqual: true,
	OperatorTypeBangEqual: true,
	OperatorTypeGreaterThan: true,
	OperatorTypeGreaterThanEqual: true,
	OperatorTypeLessThan: true,
	OperatorTypeLessThanEqual: true,
}

func IsOperator(tokenType TokenType) bool {
	return Operators[string(tokenType)]
}
