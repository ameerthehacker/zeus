package codegen

const VTABLE_STRUCT_INDEX = 0

func GetVTableStructName(className string) string {
	return className + ".vTable"
}
