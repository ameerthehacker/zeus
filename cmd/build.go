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
	FlagInternalZeusIR = "internal-zeus-ir"
	FlagInternalLLVMIR = "internal-llvm-ir"
	FlagFileType = "file-type"
	FlagOutputPath = "out"
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
				_compiler := zeus_compiler.NewCompiler()
				sourceFile := _compiler.CompileFile(zeus_compiler.Input{
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

				folderPath, err := os.Getwd()
				if err != nil {
					logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to get current directory: %s", err.Error()))
					os.Exit(1)
				}
				fileName := filepath.Base(filePath)
				fileNameWithoutExtension := strings.TrimSuffix(fileName, filepath.Ext(fileName))
				getOutputFileNameWithExtension := func(fileType zeus_compiler.EmitFileType) string {
					switch fileType {
					case zeus_compiler.EmitFileTypeLLVMIR:
						return fileNameWithoutExtension + ".ll"
					case zeus_compiler.EmitFileTypeObject:
						return fileNameWithoutExtension + ".o"
					case zeus_compiler.EmitFileTypeASM:
						return fileNameWithoutExtension + ".s"
					default:
						return fileNameWithoutExtension
					}
				}
				getEmitFileType := func(fileType string) zeus_compiler.EmitFileType {
					switch fileType {
					case "ll":
						return zeus_compiler.EmitFileTypeLLVMIR
					case "obj":
						return zeus_compiler.EmitFileTypeObject
					case "asm":
						return zeus_compiler.EmitFileTypeASM
					default:
						return zeus_compiler.EmitFileTypeEXE
					}
				}
				emitFileType := getEmitFileType(cmd.Flag(FlagFileType).Value.String())
				outputFileName := getOutputFileNameWithExtension(emitFileType)

				var outputPath string
				if cmd.Flag(FlagOutputPath).Changed {
					outputPath = cmd.Flag(FlagOutputPath).Value.String()
				} else {
					outputPath = filepath.Join(folderPath, outputFileName)
				}
				err = _compiler.EmitFile(sourceFile, emitFileType, outputPath)

				if err != nil {
					logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to emit file %s: %s", outputFileName, err.Error()))
					os.Exit(1)
				}
			}
		},
	}

	buildCmd.Flags().Bool(FlagInternalZeusIR, false, "print the zeus IR")
	buildCmd.Flags().Bool(FlagInternalLLVMIR, false, "print the llvm IR")
	buildCmd.Flags().StringP(FlagOutputPath, "o", "", "the output path")
	buildCmd.Flags().String(FlagFileType, "", "file type to emit")

	return buildCmd
}
