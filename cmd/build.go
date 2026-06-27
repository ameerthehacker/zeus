package cmd

import (
	"path/filepath"
	"strings"

	"github.com/ameerthehacker/zeus/internal/debug"
	"github.com/ameerthehacker/zeus/internal/zeus_compiler"

	"github.com/spf13/cobra"
)

const (
	FlagOutputPath = "out"
	FlagTargetDir  = "target-dir"
	FlagRelease    = "release"
	FlagEmitIR     = "internal-emit-ir"
)

func buildCmd() *cobra.Command {
	buildCmd := &cobra.Command{
		Use:   "build [file]",
		Short: "Build the zeus file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			folderPath := getCurDir()
			fileName := filepath.Base(filePath)
			outputFileName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
			targetBase := folderPath

			isRelease := cmd.Flag(FlagRelease).Changed
			mode := zeus_compiler.BuildModeDebug
			if isRelease {
				mode = zeus_compiler.BuildModeRelease
			}

			if cmd.Flag(FlagTargetDir).Changed {
				targetBase = cmd.Flag(FlagTargetDir).Value.String()
			}
			objDir := getModeDir(targetBase, mode, OBJ_DIR_NAME)

			var outputPath string
			if cmd.Flag(FlagOutputPath).Changed {
				outputPath = cmd.Flag(FlagOutputPath).Value.String()
			} else {
				outputPath = getModeDir(targetBase, mode, BIN_DIR_NAME, outputFileName)
			}

			compiler := mustNewCompiler(mode)

			if debug.IsDebug() {
				if f := cmd.Flags().Lookup(FlagEmitIR); f != nil && f.Changed {
					irDir := getModeDir(targetBase, mode, IR_DIR_NAME)
					compiler.EnableIREmission(irDir, folderPath)
				}
			}

			compiler.Compile(filePath, objDir, outputPath)
		},
	}

	buildCmd.Flags().StringP(FlagOutputPath, "o", "", "the output path")
	buildCmd.Flags().String(FlagTargetDir, "", "the directory where the target/ folder is created (default: current directory)")
	buildCmd.Flags().Bool(FlagRelease, false, "build an optimized release binary")
	if debug.IsDebug() {
		buildCmd.Flags().Bool(FlagEmitIR, false, "emit .zhir/.zlir/.ll IR files for debugging")
	}

	return buildCmd
}
