package util

import "github.com/ameerthehacker/zeus/internal/zeus_value"

func GetClassMethodName(className string, methodName string) string {
	return className + "." + methodName
}

func GetPropertyIndex(class zeus_value.Class, propertyName string) int {
	for index, property := range class.Properties {
		if property.Property.Name == propertyName {
			return index
		}
	}
	return -1
}