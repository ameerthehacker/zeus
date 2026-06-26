package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/zeus_compiler"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

func getCurDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to get current directory: %s", err.Error()))
		os.Exit(1)
	}

	return cwd
}

func mustNewCompiler(mode zeus_compiler.BuildMode) *zeus_compiler.Compiler {
	compiler, err := zeus_compiler.NewCompiler(mode)
	if err != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to initialize compiler: %s", err.Error()))
		os.Exit(1)
	}
	return compiler
}

func getModeDir(basePath string, mode zeus_compiler.BuildMode, elem ...string) string {
	modeDir := "debug"

	if mode == zeus_compiler.BuildModeRelease {
		modeDir = "release"
	}

	parts := slices.Concat([]string{
		basePath,
		TARGET_DIR_NAME,
		modeDir,
	}, elem)

	return filepath.Join(parts...)
}
