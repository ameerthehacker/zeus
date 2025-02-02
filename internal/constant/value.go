package constant

import (
	"fmt"
	"strconv"
	"strings"
)

type Value interface {
	String() string
}

func GetIntSize(number string) IntSize {
	value, err := strconv.ParseInt(number, 10, 64)

	if err != nil {
		panic(fmt.Sprintf("failed to parse int: %s", err))
	}

	switch {
	case value >= -128 && value <= 127:
		return I8
	case value >= -32768 && value <= 32767:
		return I16
	case value >= -2147483648 && value <= 2147483647:
		return I32
	case value >= -9223372036854775808 && value <= 9223372036854775807:
		return I64
	default:
		return I128
	}
}

func IsFloat(number string) bool {
	return strings.Contains(number, ".")
}
