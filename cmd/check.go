package cmd

import (
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
			compiler.Check(filePath)
		},
	}

	return runCmd
}
