package lsp

import (
	"strings"

	"github.com/ameerthehacker/zeus/internal/formatter"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"go.lsp.dev/protocol"
)

// formatDocument formats an open document and returns a single whole-document
// text edit. It returns nil (no edits) when the document is unknown, does not
// lex/parse cleanly, is already formatted, or when the formatted result would
// not itself parse (a safety net so a formatter bug can never corrupt a buffer).
func (s *Server) formatDocument(uri protocol.DocumentURI) []protocol.TextEdit {
	doc, ok := s.documents[string(uri)]
	if !ok {
		return nil
	}
	source := doc.Content

	// Formatting only needs the lexer + parser; never format code with errors.
	lex := lexer.NewLexer(source)
	tokens, lexErrs := lex.Lex()
	if len(lexErrs) > 0 {
		return nil
	}
	program, parseErrs := parser.NewParser(tokens).ParseProgram()
	if len(parseErrs) > 0 {
		return nil
	}

	formatted := formatter.Format(program, lex.Comments(), formatter.DefaultOptions())
	if formatted == source {
		return nil
	}

	// Safety net: never hand the editor an edit that would not re-parse.
	checkTokens, checkLexErrs := lexer.NewLexer(formatted).Lex()
	if _, checkParseErrs := parser.NewParser(checkTokens).ParseProgram(); len(checkLexErrs) > 0 || len(checkParseErrs) > 0 {
		return nil
	}

	return []protocol.TextEdit{{
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   documentEnd(source),
		},
		NewText: formatted,
	}}
}

// documentEnd returns the LSP position just past the last character of source,
// so a text edit spanning (0,0) to it replaces the entire document.
func documentEnd(source string) protocol.Position {
	lines := strings.Split(source, "\n")
	last := lines[len(lines)-1]
	return protocol.Position{
		Line:      uint32(len(lines) - 1),
		Character: uint32(len([]rune(last))),
	}
}
