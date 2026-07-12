package cmd

import (
	"fmt"
	"os"

	"github.com/ameerthehacker/zeus/internal/formatter"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/spf13/cobra"
)

func fmtCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fmt [file]",
		Short: "Format a zeus source file in place",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]

			content, err := os.ReadFile(filePath)
			if err != nil {
				logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to read %s: %s", filePath, err.Error()))
				os.Exit(1)
			}
			source := string(content)

			// Only the lexer + parser are needed to format — no type checking or codegen.
			lex := lexer.NewLexer(source)
			tokens, lexErrors := lex.Lex()
			if len(lexErrors) > 0 {
				logger.PrettyPrintError(filePath, source, lexErrors)
				os.Exit(1)
			}

			p := parser.NewParser(tokens)
			program, parseErrors := p.ParseProgram()
			if len(parseErrors) > 0 {
				logger.PrettyPrintError(filePath, source, parseErrors)
				os.Exit(1)
			}

			formatted := formatter.Format(program, lex.Comments(), formatter.DefaultOptions())

			// Safety net: never overwrite a file with output that no longer parses.
			// Re-lex and re-parse the formatted result; if it fails, this is a
			// formatter bug — leave the file untouched and report it.
			checkLex := lexer.NewLexer(formatted)
			checkTokens, checkLexErrors := checkLex.Lex()
			_, checkParseErrors := parser.NewParser(checkTokens).ParseProgram()
			if len(checkLexErrors) > 0 || len(checkParseErrors) > 0 {
				logger.Log(zeus_error.ErrorSeverityError,
					fmt.Sprintf("internal formatter error: formatting %s would produce invalid code; file left unchanged (please report this)", filePath))
				os.Exit(1)
			}

			if formatted == source {
				return
			}

			if err := os.WriteFile(filePath, []byte(formatted), 0644); err != nil {
				logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to write %s: %s", filePath, err.Error()))
				os.Exit(1)
			}
			logger.Log(zeus_error.ErrorSeverityInfo, fmt.Sprintf("formatted %s", filePath))
		},
	}
}
