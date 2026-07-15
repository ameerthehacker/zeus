package lsp

import (
	"fmt"
	"os"
	"strings"

	"github.com/ameerthehacker/zeus/internal/analysis"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"go.lsp.dev/protocol"
)

// This file turns compiler errors into LSP diagnostics, drives (re)validation of a document,
// and surfaces recovered internal errors to the editor.

// validateDocument analyzes a document, caches the result (AST + IR + diagnostics), and
// publishes diagnostics. It keeps the AST so positional features (hover, definition) can query
// it directly.
func (s *Server) validateDocument(uri protocol.DocumentURI, content string) error {
	// uri.Filename() gives the on-disk path, used to resolve imports relative to this file.
	res := analysis.Analyze(uri.Filename(), content, s.makeModuleResolver(uri.Filename()))

	s.documents[string(uri)] = &DocumentInfo{
		Content:  content,
		AST:      res.AST,
		IRModule: res.Module,
		Errors:   res.Diagnostics,
	}

	diagnostics := s.convertToLSPDiagnostics(res.Diagnostics)
	fmt.Fprintf(os.Stderr, "Generated %d diagnostics for %s\n", len(diagnostics), uri)
	return s.sendDiagnostics(uri, diagnostics)
}

// convertToLSPDiagnostics converts Zeus errors to LSP diagnostics.
func (s *Server) convertToLSPDiagnostics(errors []*zeus_error.ZeusError) []protocol.Diagnostic {
	diagnostics := make([]protocol.Diagnostic, 0, len(errors))

	for _, err := range errors {
		var severity protocol.DiagnosticSeverity
		switch err.Severity {
		case zeus_error.ErrorSeverityError:
			severity = protocol.DiagnosticSeverityError
		case zeus_error.ErrorSeverityWarning:
			severity = protocol.DiagnosticSeverityWarning
		case zeus_error.ErrorSeverityInfo:
			severity = protocol.DiagnosticSeverityInformation
		default:
			severity = protocol.DiagnosticSeverityError
		}

		// Convert Zeus Span to LSP Range.
		// Zeus uses 1-based line/column and INCLUSIVE end positions; LSP uses 0-based and an
		// EXCLUSIVE end. So Start subtracts 1 on both axes, and End subtracts 1 on the line but
		// not the column (which makes the inclusive end exclusive).
		var startLine, startChar, endLine, endChar uint32
		if err.Span != nil {
			startLine = uint32(err.Span.Start.Line - 1)
			startChar = uint32(err.Span.Start.Column - 1)
			endLine = uint32(err.Span.End.Line - 1)
			endChar = uint32(err.Span.End.Column)
		}

		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: startLine, Character: startChar},
				End:   protocol.Position{Line: endLine, Character: endChar},
			},
			Severity: severity,
			Source:   "zeus",
			Message:  err.Message,
		})
	}

	return diagnostics
}

// reportInternalError surfaces a recovered panic to the editor via window/showMessage so it is
// visible to the user (and reportable) rather than only living in the server's stderr log.
// Identical failures are shown once per session to avoid a popup storm during rapid edits.
func (s *Server) reportInternalError(method string, r interface{}, stack []byte) {
	signature := fmt.Sprintf("%s|%v", method, r)
	if s.reportedPanics[signature] {
		return
	}
	s.reportedPanics[signature] = true

	message := fmt.Sprintf(
		"Zeus LSP hit an internal error and recovered (the server is still running). "+
			"Please report this at github.com/ameerthehacker/zeus with a snippet that reproduces it.\n\n"+
			"While handling %s: %v%s",
		method, r, firstZeusFrame(stack),
	)

	_ = s.sendNotification("window/showMessage", protocol.ShowMessageParams{
		Type:    protocol.MessageTypeError,
		Message: message,
	})
}

// firstZeusFrame extracts the first Zeus compiler frame from a runtime stack so the popup
// points roughly at the failing code. It matches the module path on the function-name lines
// (which are worktree-path independent) rather than the absolute file path. Returns "" if
// none is found.
func firstZeusFrame(stack []byte) string {
	const pkgRoot = "ameerthehacker/zeus/internal/"
	for _, line := range strings.Split(string(stack), "\n") {
		trimmed := strings.TrimSpace(line)
		if idx := strings.Index(trimmed, pkgRoot); idx != -1 {
			// Keep the "internal/pkg.Func(...)" tail, dropping the module prefix.
			return "\n  at " + trimmed[idx+len("ameerthehacker/zeus/"):]
		}
	}
	return ""
}
