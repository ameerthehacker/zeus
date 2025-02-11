package logger

import (
	"fmt"
	"os"
	"strings"

	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/fatih/color"
)

const zeus = "zeus"

func formatError(severity zeus_error.ErrorSeverity, prefix string, message string) string {
	severityString := ""
	switch severity {
	case zeus_error.ErrorSeverityError:
		severityString = color.New(color.FgRed, color.Bold).Sprint("error:")
	case zeus_error.ErrorSeverityWarning:
		severityString = color.New(color.FgYellow, color.Bold).Sprint("warning:")
	case zeus_error.ErrorSeverityInfo:
		severityString = color.New(color.FgGreen, color.Bold).Sprint("info:")
	}

	return fmt.Sprintf("%s: %s %s", color.New(color.Bold).Sprint(prefix), severityString, color.New(color.FgYellow, color.Bold).Sprint(message))
}

func Log(severity zeus_error.ErrorSeverity, message string) {
	fmt.Println(formatError(severity, zeus, message))
}

func LogZeusError(filePath string, error *zeus_error.ZeusError) {
	prefix := fmt.Sprintf("%s:%d:%d", filePath, error.Span.Start.Line, error.Span.Start.Column)
	fmt.Fprintln(os.Stderr, formatError(error.Severity, prefix, error.Message))
}

func PrettyPrintError(filePath string, source string, errors []*zeus_error.ZeusError) {
	lines := strings.Split(source, "\n")

	for _, err := range errors {
		line, col := err.Span.Start.Line - 1, err.Span.Start.Column - 1
		errorIndicator := "^"
		if err.Span.End.Column > err.Span.Start.Column {
			errorIndicator = strings.Repeat("~", err.Span.End.Column - err.Span.Start.Column + 1)
		}

		LogZeusError(filePath, err)
		if line < len(lines) {
			fmt.Fprintln(os.Stderr, lines[line])
			fmt.Fprintln(os.Stderr, strings.Repeat(" ", col)+errorIndicator)
		}
	}

	errorCount := 0
	warningCount := 0
	for _, err := range errors {
		if err.Severity == zeus_error.ErrorSeverityError {
			errorCount++
		} else if err.Severity == zeus_error.ErrorSeverityWarning {
			warningCount++
		}
	}

	var errorMessage string
	if errorCount > 0 {
		s := "s"
		if errorCount == 1 {
			s = ""
		}
		errorMessage = color.New(color.FgRed, color.Bold).Sprintf("%d error%s", errorCount, s)
	}

	var warningMessage string
	if warningCount > 0 {
		s := "s"
		if warningCount == 1 {
			s = ""
		}
		warningMessage = color.New(color.FgYellow, color.Bold).Sprintf("%d warning%s", warningCount, s)
	}

	conjunction := ""
	if errorCount > 0 && warningCount > 0 {
		conjunction = " and "
	}

	if errorCount > 0 || warningCount > 0 {
		fmt.Fprintf(os.Stderr, "%s%s%s %s\n", errorMessage, conjunction, warningMessage, color.New(color.Bold).Sprint("generated."))
	}
}
