package module

import "unicode"

func GetModulePrefix(modulePath string) string {
	moduleName := "$"
	for _, r := range modulePath {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			moduleName += string(r)
		} else {
			moduleName += "_"
		}
	}

	return moduleName
}

func GetModuleScopedName(modulePath string, name string) string {
	moduleName := GetModulePrefix(modulePath)
	return moduleName + "_" + name
}
