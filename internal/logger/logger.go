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
	colorBold    = color.New(color.Bold)
	colorError   = color.New(color.FgRed, color.Bold)
	colorWarning = color.New(color.FgYellow, color.Bold)
	colorInfo    = color.New(color.FgGreen, color.Bold)
	colorMessage = color.New(color.FgYellow, color.Bold)
	colorGutter  = color.New(color.FgHiBlack)
	colorArrow   = color.New(color.FgHiBlack)
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

		startLine := err.Span.Start.Line
		endLine := err.Span.End.Line
		startCol := err.Span.Start.Column - 1
		endCol := err.Span.End.Column - 1

		pipe := colorGutter.Sprint("|")
		arrow := colorArrow.Sprint("-->")
		sc := severityColor(err.Severity)

		// Gutter width based on the largest line number we'll print.
		lastShown := min(endLine+1, len(sourceLines))
		gutterW := len(strconv.Itoa(lastShown)) + 1
		gutterPad := strings.Repeat(" ", gutterW)

		fmt.Fprintf(os.Stderr, "%s %s\n", severityLabel(err.Severity), colorMessage.Sprint(err.Message))
		fmt.Fprintf(os.Stderr, "  %s %s:%d:%d\n", arrow, filePath, startLine, err.Span.Start.Column)
		fmt.Fprintf(os.Stderr, "%s %s\n", gutterPad, pipe)

		if endLine <= startLine {
			// ── single-line span ───────────────────────────────────────────
			indicator := "^"
			if endCol > startCol {
				indicator = strings.Repeat("~", endCol-startCol+1)
			}
			indicator = sc.Sprint(indicator)

			for ctx := max(1, startLine-2); ctx < startLine; ctx++ {
				printSourceLine(ctx, gutterW, pipe)
			}
			printSourceLine(startLine, gutterW, pipe)
			fmt.Fprintf(os.Stderr, "%s %s %s%s\n", gutterPad, pipe, strings.Repeat(" ", startCol), indicator)
			for ctx := startLine + 1; ctx <= min(startLine+2, len(sourceLines)); ctx++ {
				printSourceLine(ctx, gutterW, pipe)
			}
		} else {
			// ── multi-line span ────────────────────────────────────────────
			printSpanLine := func(lineNo int, bracket string) {
				if lineNo < 1 || lineNo-1 >= len(sourceLines) {
					return
				}
				label := colorGutter.Sprintf("%*d", gutterW, lineNo)
				fmt.Fprintf(os.Stderr, "%s %s %s %s\n", label, pipe, sc.Sprint(bracket), highlightZeusLine(sourceLines[lineNo-1]))
			}

			// Up to 2 context lines before the start line.
			for ctx := max(1, startLine-2); ctx < startLine; ctx++ {
				printSourceLine(ctx, gutterW, pipe)
			}

			printSpanLine(startLine, "╭")

			// Intermediate lines: show up to 3; collapse longer ranges with "...".
			intermediateCount := endLine - startLine - 1
			const maxIntermediate = 3
			if intermediateCount > 0 && intermediateCount <= maxIntermediate {
				for mid := startLine + 1; mid < endLine; mid++ {
					printSpanLine(mid, "│")
				}
			} else if intermediateCount > maxIntermediate {
				printSpanLine(startLine+1, "│")
				skipped := intermediateCount - 2
				label := colorGutter.Sprintf("%*s", gutterW, "...")
				fmt.Fprintf(os.Stderr, "%s %s %s %s\n", label, pipe, sc.Sprint("│"), colorGutter.Sprintf("... %d lines not shown ...", skipped))
				printSpanLine(endLine-1, "│")
			}

			printSpanLine(endLine, "╰")

			// One context line after the end.
			if endLine+1 <= len(sourceLines) {
				printSourceLine(endLine+1, gutterW, pipe)
			}
		}

		fmt.Fprintf(os.Stderr, "%s %s\n", gutterPad, pipe)
	}
}
