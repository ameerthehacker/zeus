package token

type DataType TokenType

const (
	// signed integer types
	DataTypeInt DataType = "int"
	DataTypeInt8 DataType = "i8"
	DataTypeInt16 DataType = "i16"
	DataTypeInt32 DataType = "i32"
	DataTypeInt64 DataType = "i64"
	DataTypeInt128 DataType = "i128"
	// unsigned integer types
	DataTypeUInt8 DataType = "u8"
	DataTypeUInt16 DataType = "u16"
	DataTypeUInt32 DataType = "u32"
	DataTypeUInt64 DataType = "u64"
	DataTypeUInt128 DataType = "u128"
	// floating point types
	DataTypeFloat DataType = "float"
	DataTypeFloat32 DataType = "f32"
	DataTypeFloat64 DataType = "f64"
	// boolean type
	DataTypeBoolean DataType = "boolean"
)

var DataTypes = map[DataType]bool{
	DataTypeInt:     true,
	DataTypeInt8:    true,
	DataTypeInt16:   true,
	DataTypeInt32:   true,
	DataTypeInt64:   true,
	DataTypeInt128:  true,
	DataTypeUInt8:   true,
	DataTypeUInt16:  true,
	DataTypeUInt32:  true,
	DataTypeUInt64:  true,
	DataTypeUInt128: true,
	DataTypeFloat:   true,
	DataTypeFloat32: true,
	DataTypeFloat64: true,
	DataTypeBoolean: true,
}

func IsDataType(tokenValue string) bool {
	return DataTypes[DataType(tokenValue)]
}
