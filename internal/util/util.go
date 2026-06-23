package util

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

func GetClassMethodName(className string, methodName string) string {
	return className + "." + methodName
}

func GetPropertyIndex(class zeus_value.Class, propertyName string) int {
	for index, property := range class.Properties {
		if property.Property.Name == propertyName {
			return index + 1 // skip the obj header struct
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
		// Skip lowered methods - they are not in the vtable
		if method.IsLowered {
			continue
		}
		if method.Method.Name == methodName {
			return methodIndex
		}
		methodIndex += 1
	}
	return -1
}

func GetMacOSVersion() (string, error) {
	// sw_vers -productVersion explicitly extracts just the version number
	cmd := exec.Command("sw_vers", "-productVersion")

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get macOS version: %w", err)
	}

	// Clean up whitespace/newlines from the command output
	version := strings.TrimSpace(out.String())
	return version, nil
}
