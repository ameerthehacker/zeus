package cmd

import (
	"fmt"
	"os"

	"github.com/ameerthehacker/zeus/internal/compiler"
	"github.com/ameerthehacker/zeus/internal/logger"

	"github.com/spf13/cobra"
)

func buildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build [file]",
		Short: "Build the zeus file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fileName := args[0]
			content, err := os.ReadFile(fileName)
			if err != nil {
				logger.Log(logger.LogSeverityError, err.Error())
				os.Exit(1)
			} else {
				compiler := compiler.NewCompiler(string(content))
				errors := compiler.Compile()
				for _, err := range errors {
					fmt.Println(err)
				}
			}
		},
	}
}
