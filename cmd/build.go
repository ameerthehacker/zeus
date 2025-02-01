package cmd

import (
	"os"

	"github.com/ameerthehacker/zeus/internal/compiler"
	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/zeus_error"

	"github.com/spf13/cobra"
)

func buildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build [file]",
		Short: "Build the zeus file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			content, err := os.ReadFile(filePath)
			if err != nil {
				logger.Log(zeus_error.ErrorSeverityError, err.Error())
				os.Exit(1)
			} else {
				_compiler := compiler.NewCompiler()
				sourceFile := _compiler.CompileFile(compiler.Input{
					Path: filePath,
					Source: string(content),
				})

				// log the compiler errors
				if len(sourceFile.Errors) > 0 {
					for _, err := range sourceFile.Errors {
						logger.LogZeusError(filePath, err)
					}
					os.Exit(1)
				}
			}
		},
	}
}
