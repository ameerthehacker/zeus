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

// ModuleInitFuncPrefix marks the per-module initializer function that runs a module's top-level
// statements (global/const/let initializers) at program startup. It uses the same `$` module-symbol
// convention as GetModuleScopedName — link-safe across object files, unlike `#`, which the macOS
// linker cannot resolve for cross-TU `call`s (these init functions are called from `main`, which
// lives in a different object file). The name embeds the module's own path, so it cannot collide
// across modules; codegen gives these functions external linkage so `main` can call them.
const ModuleInitFuncPrefix = "$module_init$"

// ModuleInitFuncName returns the stable, program-wide symbol for a module's init function.
// Both the defining module and the entry point's dispatcher derive it from the module path,
// so the two agree without any shared state.
func ModuleInitFuncName(modulePath string) string {
	return ModuleInitFuncPrefix + GetModulePrefix(modulePath)
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
	} else if err != nil {
		// Path doesn't exist as-is — try appending the .zs extension
		withExt := resolvedPath + ".zs"
		if _, err2 := os.Stat(withExt); err2 == nil {
			resolvedPath = withExt
		}
	}

	return resolvedPath
}
