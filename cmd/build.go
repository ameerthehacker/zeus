package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/zeus_compiler"
	"github.com/ameerthehacker/zeus/internal/zeus_error"

	"github.com/spf13/cobra"
)

const (
	FlagOutputPath = "out"
	FlagTargetDir  = "target-dir"
	FlagRelease    = "release"
)

func buildCmd() *cobra.Command {
	buildCmd := &cobra.Command{
		Use:   "build [file]",
		Short: "Build the zeus file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			folderPath, err := os.Getwd()
			if err != nil {
				logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to get current directory: %s", err.Error()))
				os.Exit(1)
			}
			fileName := filepath.Base(filePath)
			outputFileName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

			isRelease := cmd.Flag(FlagRelease).Changed
			mode := zeus_compiler.BuildModeDebug
			modeDir := "debug"
			if isRelease {
				mode = zeus_compiler.BuildModeRelease
				modeDir = "release"
			}

			targetBase := folderPath
			if cmd.Flag(FlagTargetDir).Changed {
				targetBase = cmd.Flag(FlagTargetDir).Value.String()
			}
			objDir := filepath.Join(targetBase, "target", modeDir, "obj")

			var outputPath string
			if cmd.Flag(FlagOutputPath).Changed {
				outputPath = cmd.Flag(FlagOutputPath).Value.String()
			} else {
				outputPath = filepath.Join(targetBase, "target", modeDir, "bin", outputFileName)
			}

			compiler, err := zeus_compiler.NewCompiler(objDir, mode)
			if err != nil {
				logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to initialize compiler: %s", err.Error()))
				os.Exit(1)
			}
			compiler.Compile(filePath, outputPath)
		},
	}

	buildCmd.Flags().StringP(FlagOutputPath, "o", "", "the output path")
	buildCmd.Flags().String(FlagTargetDir, "", "the directory where the target/ folder is created (default: current directory)")
	buildCmd.Flags().Bool(FlagRelease, false, "build an optimized release binary")

	return buildCmd
}
