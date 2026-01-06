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

// getZeusHomeDir returns the Zeus home directory.
// It first checks the ZEUS_HOME environment variable, and if not set,
// it finds the directory where the Zeus binary is located (resolving symlinks).
func getZeusHomeDir() string {
	// First check ZEUS_HOME environment variable for backward compatibility/override
	if zeusHome := os.Getenv(ZeusHomeEnvVar); zeusHome != "" {
		return zeusHome
	}

	// Find the executable path and resolve symlinks
	execPath, err := os.Executable()
	if err != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to get zeus executable path: %s", err.Error()))
		os.Exit(1)
	}

	// Resolve symlinks to get the actual binary location
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to resolve symlinks: %s", err.Error()))
		os.Exit(1)
	}

	// Return the directory containing the binary (go up from bin/ to zeus home)
	binDir := filepath.Dir(execPath)
	// Check if we're in a bin/ directory structure
	if filepath.Base(binDir) == "bin" {
		return filepath.Dir(binDir)
	}
	// Otherwise, the binary is directly in the zeus home directory
	return binDir
}

func ResolveFilePath(sourcePath string, importPath string) string {
	var resolvedPath string = importPath

	if strings.HasPrefix(importPath, stdZeusModulePrefix) {
		zeusHomePath := getZeusHomeDir()
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
