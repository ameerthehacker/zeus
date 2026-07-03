package cmd

import (
	"github.com/ameerthehacker/zeus/internal/debug"
	"github.com/ameerthehacker/zeus/internal/zeus_compiler"
	"github.com/spf13/cobra"
)

func checkCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "check [file]",
		Short: "Check the zeus file for compilation errors",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			compiler := mustNewCompiler(zeus_compiler.BuildModeDebug)
			if debug.IsDebug() {
				if f := cmd.Flags().Lookup(FlagEmitIR); f != nil && f.Changed {
					basePath := f.Value.String()
					if basePath == "" {
						basePath = getCurDir()
					}
					irDir := getModeDir(basePath, zeus_compiler.BuildModeDebug, IR_DIR_NAME)
					compiler.EnableIREmission(irDir, getCurDir())
				}
			}
			compiler.Check(filePath)
		},
	}

	if debug.IsDebug() {
		runCmd.Flags().String(FlagEmitIR, "", "emit .zhir IR files for debugging; optionally specify base path (default: current directory)")
		runCmd.Flags().Lookup(FlagEmitIR).NoOptDefVal = ""
	}

	return runCmd
}
