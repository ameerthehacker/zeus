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
	FlagOutputPath     = "out"
	FlagTargetDir      = "target-dir"
)

func buildCmd() *cobra.Command {
	buildCmd := &cobra.Command{
		Use:   "build [file]",
		Short: "Build the zeus file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// identify the input file, output path and emit file type
			filePath := args[0]
			folderPath, err := os.Getwd()
			if err != nil {
				logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to get current directory: %s", err.Error()))
				os.Exit(1)
			}
			fileName := filepath.Base(filePath)
			fileNameWithoutExtension := strings.TrimSuffix(fileName, filepath.Ext(fileName))
			getOutputFileNameWithExtension := func(fileType zeus_compiler.EmitFileType) string {
				switch fileType {
				case zeus_compiler.EmitFileTypeObject:
					return fileNameWithoutExtension + ".o"
				default:
					return fileNameWithoutExtension
				}
			}
			outputFileName := getOutputFileNameWithExtension(zeus_compiler.EmitFileTypeEXE)

			var outputPath string
			targetDir := os.TempDir()
			if cmd.Flag(FlagOutputPath).Changed {
				outputPath = cmd.Flag(FlagOutputPath).Value.String()
			} else {
				outputPath = filepath.Join(folderPath, outputFileName)
			}
			if cmd.Flag(FlagTargetDir).Changed {
				targetDir = cmd.Flag(FlagTargetDir).Value.String()
			}

			compiler, err := zeus_compiler.NewCompiler(targetDir)
			if err != nil {
				logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to initialize compiler: %s", err.Error()))
				os.Exit(1)
			}
			compiler.Compile(filePath, zeus_compiler.EmitFileTypeEXE, outputPath)
		},
	}

	buildCmd.Flags().StringP(FlagOutputPath, "o", "", "the output path")
	buildCmd.Flags().String(FlagTargetDir, "", "the directory to store the output files")

	return buildCmd
}
