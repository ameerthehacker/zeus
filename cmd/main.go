package cmd

import (
	"github.com/spf13/cobra"
)

func Execute() {
	rootCmd := &cobra.Command{
		Use:     "zeus",
		Short:   "Zeus language compiler",
		Version: Version,
	}
	rootCmd.SetVersionTemplate("zeus {{.Version}}\n")
	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(checkCmd())
	rootCmd.AddCommand(lspCmd())
	rootCmd.Execute()
}
