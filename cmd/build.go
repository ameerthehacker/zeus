package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ameerthehacker/zeus/internal/compiler"
	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/zeus_error"

	"github.com/spf13/cobra"
)

const (
	FlagInternalZeusIR = "internal-zeus-ir"
	FlagInternalLLVMIR = "internal-llvm-ir"
)

func buildCmd() *cobra.Command {
	buildCmd := &cobra.Command{
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
					logger.PrettyPrintError(sourceFile.Path, sourceFile.Source, sourceFile.Errors)
					os.Exit(1)
				}

				if cmd.Flag(FlagInternalZeusIR).Changed {
					fmt.Println("---:ZEUS IR:---")
					sourceFile.IRBuilder.Print()
				}

				if cmd.Flag(FlagInternalLLVMIR).Changed {
					fmt.Println("---:LLVM IR:---")
					sourceFile.Module.Dump()
				}

				folderPath := filepath.Dir(filePath)
				fileName := filepath.Base(filePath)
				fileNameWithoutExtension := strings.TrimSuffix(fileName, filepath.Ext(fileName))
				err := os.WriteFile(filepath.Join(folderPath, fmt.Sprintf("%s.ll", fileNameWithoutExtension)), []byte(sourceFile.Module.String()), 0644)

				if err != nil {
					logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to write file %s: %s", fileNameWithoutExtension, err.Error()))
					os.Exit(1)
				}
			}
		},
	}

	buildCmd.Flags().Bool(FlagInternalZeusIR, false, "print the zeus IR")
	buildCmd.Flags().Bool(FlagInternalLLVMIR, false, "print the llvm IR")

	return buildCmd
}
