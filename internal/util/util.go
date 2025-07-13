package util

import (
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

func GetClassMethodName(className string, methodName string) string {
	return className + "." + methodName
}

func GetPropertyIndex(class zeus_value.Class, propertyName string) int {
	for index, property := range class.Properties {
		if property.Property.Name == propertyName {
			return index + 1 // skip the vtable struct
		}
	}
	return -1
}

func GetMethodIndex(class zeus_value.Class, methodName string) int {
	methodIndex := 0
	for _, method := range class.Methods {
		if method.Method.Name == token.CONSTRUCTOR_METHOD_NAME {
			continue
		}
		if method.Method.Name == methodName {
			return methodIndex
		}
		methodIndex += 1
	}
	return -1
}
