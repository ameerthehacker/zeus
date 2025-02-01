package logger

import (
	"fmt"

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
	fmt.Println(formatError(error.Severity, prefix, error.Message))
}
