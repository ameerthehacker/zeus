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

func GetPropertyIndex(class *zeus_value.Class, propertyName string) int {
	// Fields are laid out base-first; FlattenedInstanceProperties yields that exact order
	// (and excludes statics, which live in globals, not the instance struct).
	for instanceIndex, property := range zeus_value.FlattenedInstanceProperties(class) {
		if property.Property.Name == propertyName {
			return instanceIndex + 1 // skip the obj header struct
		}
	}
	return -1
}

// GetVTableSlot returns the vtable slot for a concrete compiled method (as opposed to a call
// site, which uses GetMethodIndex on a source name). It matches on the IR name first — the exact
// function identity — then falls back to the source name. This resolves both accessors (whose
// descriptor keeps the mangled #get_/#set_ name while the compiled function's source name is the
// property name) and extern methods (compiled under a class-scoped IR name).
func GetVTableSlot(class *zeus_value.Class, method *zeus_value.Function) int {
	flat := zeus_value.FlattenedVTableMethods(class)
	for i, m := range flat {
		if m.Method.Name == method.Name {
			return i
		}
	}
	for i, m := range flat {
		if m.Method.SourceName() == method.SourceName() {
			return i
		}
	}
	return -1
}

func GetMethodIndex(class *zeus_value.Class, methodName string) int {
	// FlattenedVTableMethods yields vtable slots in physical order across the inheritance
	// chain (base methods first, overrides in the base's slot), so an inherited/overridden
	// method resolves to the same index it occupies in a base class's vtable.
	for methodIndex, method := range zeus_value.FlattenedVTableMethods(class) {
		if method.Method.SourceName() == methodName {
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
