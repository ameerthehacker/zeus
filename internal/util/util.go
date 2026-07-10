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

func GetPropertyIndex(class *zeus_value.Class, propertyName string) int {
	// Static properties are not in the instance struct; only count instance properties.
	instanceIndex := 0
	for _, property := range class.Properties {
		if property.IsStatic {
			continue
		}
		if property.Property.Name == propertyName {
			return instanceIndex + 1 // skip the obj header struct
		}
		instanceIndex++
	}
	return -1
}

func GetMethodIndex(class *zeus_value.Class, methodName string) int {
	methodIndex := 0
	for _, method := range class.Methods {
		if method.Method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
			continue
		}
		// Skip lowered methods - they are not in the vtable
		if method.IsLowered {
			continue
		}
		// Static methods are not in the vtable
		if method.IsStatic {
			continue
		}
		if method.Method.SourceName() == methodName {
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
