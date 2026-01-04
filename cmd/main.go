package cmd

import (
	"github.com/spf13/cobra"
)

func Execute() {
	rootCmd := &cobra.Command{
		Use:   "zeus",
		Short: "Zeus language compiler",
	}
	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(lspCmd())
	rootCmd.Execute()
}
