package codegen

const VTABLE_STRUCT_INDEX = 0

func GetVTableStructName(className string) string {
	return className + ".vTable"
}

func GetVTableStructPtrName(className string) string {
	return className + ".vTablePtr"
}