package ir

type InstrType int

type Value interface {}

type Constant struct {
	Value string
	ValueType ValueType
}

type Variable struct {
	Name string
	ValueType ValueType
}

const (
	InstrTypeAdd InstrType = iota
	// math operations
	InstrTypeSub
	InstrTypeMul
	InstrTypeDiv
	InstrTypeNeq
	InstrTypeEq
	InstrTypeNotEq
	InstrTypeLessThan
	InstrTypeLessThanEq
	InstrTypeGreaterThan
	InstrTypeGreaterThanEq
	// declarations
	InstrTypeDeclareVar
	InstrTypeDeclareFunc
)


func (i InstrType) String() string {
	switch i {
	case InstrTypeAdd:
		return "ADD"
	case InstrTypeSub:
		return "SUB"
	case InstrTypeMul:
		return "MUL"
	case InstrTypeDiv:
		return "DIV"
	case InstrTypeNeq:
		return "NEQ"
	case InstrTypeEq:
		return "EQ"
	case InstrTypeNotEq:
		return "NOT_EQ"
	case InstrTypeLessThan:
		return "LESS_THAN"
	case InstrTypeLessThanEq:
		return "LESS_THAN_EQ"
	case InstrTypeGreaterThan:
		return "GREATER_THAN"
	case InstrTypeGreaterThanEq:
		return "GREATER_THAN_EQ"
	case InstrTypeDeclareVar:
		return "DECLARE_VAR"
	case InstrTypeDeclareFunc:
		return "DECLARE_FUNC"
	default:
		panic("unknown instruction type")
	}
}

type Instr struct {
	Type InstrType
	Result Value
	Left   Value
	Right  Value
}

type BasicBlock struct {
	Instructions []*Instr
}
