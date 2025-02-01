package ir

type ValueType interface {}

type IntType struct {
	Size int
}

type FloatType struct {
	Size int
}

type BoolType struct {}

type FunctionType struct {
	ReturnType ValueType
	ParamTypes []ValueType
}
