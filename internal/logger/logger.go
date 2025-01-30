package logger

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/error"
	"github.com/fatih/color"
)

const zeus = "zeus"

func formatError(severity error.ErrorSeverity, prefix string, message string) string {
	severityString := ""
	switch severity {
	case error.ErrorSeverityError:
		severityString = color.New(color.FgRed, color.Bold).Sprint("error:")
	case error.ErrorSeverityWarning:
		severityString = color.New(color.FgYellow, color.Bold).Sprint("warning:")
	case error.ErrorSeverityInfo:
		severityString = color.New(color.FgGreen, color.Bold).Sprint("info:")
	}

	return fmt.Sprintf("%s: %s %s", color.New(color.Bold).Sprint(prefix), severityString, color.New(color.FgYellow, color.Bold).Sprint(message))
}

func Log(severity error.ErrorSeverity, message string) {
	fmt.Println(formatError(severity, zeus, message))
}

func LogZeusError(filePath string, error *error.ZeusError) {
	prefix := fmt.Sprintf("%s:%d:%d", filePath, error.Span.Start.Line, error.Span.Start.Column)
	fmt.Println(formatError(error.Severity, prefix, error.Message))
}
