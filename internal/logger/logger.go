package logger

import (
	"fmt"

	"github.com/fatih/color"
)

const zeus = "zeus"

type LogSeverity int

const (
	LogSeverityError LogSeverity = iota
	LogSeverityWarning
	LogSeverityInfo
)

func Log(severity LogSeverity, message string) {
	severityString := ""
	switch severity {
	case LogSeverityError:
		severityString = color.New(color.FgRed, color.Bold).Sprint("error:")
	case LogSeverityWarning:
		severityString = color.New(color.FgYellow, color.Bold).Sprint("warning:")
	case LogSeverityInfo:
		severityString = color.New(color.FgGreen, color.Bold).Sprint("info:")
	}
	fmt.Printf("%s: %s %s\n", color.New(color.Bold).Sprint(zeus), severityString, color.New(color.FgYellow, color.Bold).Sprint(message))
}
