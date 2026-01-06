package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

const modZeusFile = "index.zs"
const stdZeusModulePrefix = "@"
const stdZeusModuleDir = "lib"
const ZeusHomeEnvVar = "ZEUS_HOME"

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

func ResolveFilePath(sourcePath string, importPath string) string {
	var resolvedPath string = importPath

	if strings.HasPrefix(importPath, stdZeusModulePrefix) {
		zeusHomePath, err := os.Getwd()

		if os.Getenv(ZeusHomeEnvVar) != "" {
			zeusHomePath = os.Getenv(ZeusHomeEnvVar)
		} else if err != nil {
			logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to get zeus home path: %s", err.Error()))
			os.Exit(1)
		}

		resolvedPath = filepath.Join(zeusHomePath, stdZeusModuleDir, strings.TrimPrefix(importPath, stdZeusModulePrefix))
	} else {
		resolvedPath = filepath.Join(filepath.Dir(sourcePath), importPath)
	}

	stat, err := os.Stat(resolvedPath)
	if err == nil && stat.IsDir() {
		resolvedPath = filepath.Join(resolvedPath, modZeusFile)
	}

	return resolvedPath
}
