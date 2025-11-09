package codegen

const VTABLE_STRUCT_INDEX = 0
const OBJ_HEADER_STRUCT_INDEX = 0
const ZEUS_PRIMORDIALS_MODULE_PATH = "@zeus/primordials"

func GetVTableStructName(className string) string {
	return className + ".vTable"
}

func GetVTableStructPtrName(className string) string {
	return className + ".vTablePtr"
}

func GetObjectHeaderStructName(className string) string {
	return className + ".header"
}

func GetObjectHeaderStructPtrName(className string) string {
	return className + ".headerPtr"
}
