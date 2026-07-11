package util

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

func GetClassMethodName(className string, methodName string) string {
	return className + "." + methodName
}

// FactoryFunctionPrefix is the naming prefix for every class factory function. The Zig runtime
// calls the primordial ones by name (e.g. zeus_new_string), so this must stay in sync with the
// runtime's extern declarations. Shared here so both codegen and the IR lowering pass that
// synthesizes user-class factories agree on the symbol name.
const FactoryFunctionPrefix = "zeus_new_"

// GetFactoryFunctionName returns the factory function name for a class.
// e.g. "u8[]" -> "zeus_new_u8_array", "string" -> "zeus_new_string", "MyClass" -> "zeus_new_MyClass".
func GetFactoryFunctionName(className string) string {
	// Replace [] with _array for array types
	mangledName := strings.ReplaceAll(className, "[]", "_array")
	return FactoryFunctionPrefix + mangledName
}

func GetPropertyIndex(class *zeus_value.Class, propertyName string) int {
	// Layout().Fields is base-first (and excludes statics, which live in globals, not the
	// instance struct), so the field's position is its instance struct slot (after the header).
	for instanceIndex, property := range class.Layout().Fields {
		if property.Property.Name == propertyName {
			return instanceIndex + 1 // skip the obj header struct
		}
	}
	return -1
}

func GetMethodIndex(class *zeus_value.Class, methodName string) int {
	// Layout().VTable yields slots in physical order across the inheritance chain (base methods
	// first, overrides in the base's slot), so an inherited/overridden method resolves to the
	// same index it occupies in a base class's vtable.
	for methodIndex, entry := range class.Layout().VTable {
		if entry.Method.Method.SourceName() == methodName {
			return methodIndex
		}
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
