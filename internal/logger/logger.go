package logger

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/fatih/color"
)

const zeus = "zeus"

var (
	colorBold       = color.New(color.Bold)
	colorError      = color.New(color.FgRed, color.Bold)
	colorWarning    = color.New(color.FgYellow, color.Bold)
	colorInfo       = color.New(color.FgGreen, color.Bold)
	colorMessage    = color.New(color.FgYellow, color.Bold)
	colorGutter     = color.New(color.FgHiBlack)
	colorArrow      = color.New(color.FgHiBlack)
)

func severityColor(severity zeus_error.ErrorSeverity) *color.Color {
	switch severity {
	case zeus_error.ErrorSeverityError:
		return colorError
	case zeus_error.ErrorSeverityWarning:
		return colorWarning
	default:
		return colorInfo
	}
}

func severityLabel(severity zeus_error.ErrorSeverity) string {
	switch severity {
	case zeus_error.ErrorSeverityError:
		return colorError.Sprint("error:")
	case zeus_error.ErrorSeverityWarning:
		return colorWarning.Sprint("warning:")
	default:
		return colorInfo.Sprint("info:")
	}
}

func formatError(severity zeus_error.ErrorSeverity, prefix string, message string) string {
	return fmt.Sprintf("%s: %s %s", colorBold.Sprint(prefix), severityLabel(severity), colorMessage.Sprint(message))
}

func Log(severity zeus_error.ErrorSeverity, message string) {
	fmt.Fprintln(os.Stderr, formatError(severity, zeus, message))
}

func LogZeusError(filePath string, error *zeus_error.ZeusError) {
	prefix := fmt.Sprintf("%s:%d:%d", filePath, error.Span.Start.Line, error.Span.Start.Column)
	fmt.Fprintln(os.Stderr, formatError(error.Severity, prefix, error.Message))
}

func PrettyPrintError(filePath string, source string, errors []*zeus_error.ZeusError) {
	sourceLines := strings.Split(source, "\n")

	printSourceLine := func(lineNo, gutterW int, pipe string) {
		if lineNo < 1 || lineNo-1 >= len(sourceLines) {
			return
		}
		label := colorGutter.Sprintf("%*d", gutterW, lineNo)
		fmt.Fprintf(os.Stderr, "%s %s %s\n", label, pipe, highlightZeusLine(sourceLines[lineNo-1]))
	}

	for _, err := range errors {
		if err.Span == nil {
			fmt.Fprintln(os.Stderr, formatError(err.Severity, filePath, err.Message))
			continue
		}

		lineNum := err.Span.Start.Line // 1-indexed
		col := err.Span.Start.Column - 1

		indicator := "^"
		if err.Span.End.Column > err.Span.Start.Column {
			indicator = strings.Repeat("~", err.Span.End.Column-err.Span.Start.Column+1)
		}
		indicator = severityColor(err.Severity).Sprint(indicator)

		// Gutter width: accommodate the largest line number shown (up to lineNum+2)
		gutterW := len(strconv.Itoa(min(lineNum+2, len(sourceLines)))) + 1
		gutterPad := strings.Repeat(" ", gutterW)
		pipe := colorGutter.Sprint("|")
		arrow := colorArrow.Sprint("-->")

		fmt.Fprintf(os.Stderr, "%s %s\n", severityLabel(err.Severity), colorMessage.Sprint(err.Message))
		fmt.Fprintf(os.Stderr, "  %s %s:%d:%d\n", arrow, filePath, lineNum, err.Span.Start.Column)
		fmt.Fprintf(os.Stderr, "%s %s\n", gutterPad, pipe)

		// 2 context lines before
		for ctx := max(1, lineNum-2); ctx < lineNum; ctx++ {
			printSourceLine(ctx, gutterW, pipe)
		}

		// Error line
		printSourceLine(lineNum, gutterW, pipe)

		// Indicator
		fmt.Fprintf(os.Stderr, "%s %s %s%s\n", gutterPad, pipe, strings.Repeat(" ", col), indicator)

		// 2 context lines after
		for ctx := lineNum + 1; ctx <= min(lineNum+2, len(sourceLines)); ctx++ {
			printSourceLine(ctx, gutterW, pipe)
		}

		fmt.Fprintf(os.Stderr, "%s %s\n", gutterPad, pipe)
	}
}
