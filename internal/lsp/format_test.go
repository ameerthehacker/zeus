package lsp

import (
	"testing"

	"go.lsp.dev/protocol"
)

func TestDocumentEnd(t *testing.T) {
	cases := []struct {
		src  string
		want protocol.Position
	}{
		{"a\nb", protocol.Position{Line: 1, Character: 1}},
		{"a\nb\n", protocol.Position{Line: 2, Character: 0}},
		{"", protocol.Position{Line: 0, Character: 0}},
		{"abc", protocol.Position{Line: 0, Character: 3}},
	}
	for _, c := range cases {
		if got := documentEnd(c.src); got != c.want {
			t.Errorf("documentEnd(%q) = %+v, want %+v", c.src, got, c.want)
		}
	}
}

func TestFormatDocument(t *testing.T) {
	const uri = "file:///x.zs"
	formatted := "function main(): i32 {\n    return 0;\n}\n"

	s := NewServer()
	s.documents[uri] = &DocumentInfo{Content: "function   main():i32{return 0;}"}
	edits := s.formatDocument(protocol.DocumentURI(uri))
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].NewText != formatted {
		t.Errorf("NewText = %q, want %q", edits[0].NewText, formatted)
	}
	if edits[0].Range.Start != (protocol.Position{Line: 0, Character: 0}) {
		t.Errorf("edit does not start at document start: %+v", edits[0].Range.Start)
	}

	// An already-formatted document yields no edits.
	s.documents[uri] = &DocumentInfo{Content: formatted}
	if got := s.formatDocument(protocol.DocumentURI(uri)); got != nil {
		t.Errorf("expected no edits for already-formatted doc, got %+v", got)
	}

	// A document with errors is never formatted (don't corrupt a broken buffer).
	s.documents[uri] = &DocumentInfo{Content: "let x: i32 = \"unterminated"}
	if got := s.formatDocument(protocol.DocumentURI(uri)); got != nil {
		t.Errorf("expected no edits for document with errors, got %+v", got)
	}

	// An unknown document yields no edits.
	if got := s.formatDocument(protocol.DocumentURI("file:///missing.zs")); got != nil {
		t.Errorf("expected no edits for unknown doc, got %+v", got)
	}
}
