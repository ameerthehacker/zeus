package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const mathlibSrc = `export function add(a: i32, b: i32): i32 { return a + b; }
export class Vec {
  public x: i32;
  constructor(x: i32) { this.x = x; }
}
`

// writeModule writes a .zs file into dir and returns its path.
func writeModule(t *testing.T, dir, name, src string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func TestImportResolution(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "mathlib.zs", mathlibSrc)
	mainSrc := "import { add } from \"./mathlib\";\nfunction main(): i32 { return add(1, 2); }\n"
	mainPath := writeModule(t, dir, "main.zs", mainSrc)

	s := NewServer()
	irModule, errs := s.parseDocument(mainPath, mainSrc)

	for _, e := range errs {
		if strings.Contains(e.Message, "not found") || strings.Contains(e.Message, "does not export") {
			t.Errorf("unexpected import error (import should resolve): %q", e.Message)
		}
	}
	if symbolByName(irModule, "add") == nil {
		t.Errorf("imported symbol 'add' was not resolved into the document's scope")
	}
}

func TestImportMissingSymbolStillErrors(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "mathlib.zs", mathlibSrc)
	mainSrc := "import { nope } from \"./mathlib\";\n"
	mainPath := writeModule(t, dir, "main.zs", mainSrc)

	_, errs := s0().parseDocument(mainPath, mainSrc)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "does not export") {
			found = true
		}
	}
	if !found {
		t.Errorf("importing a non-exported symbol should still report 'does not export'")
	}
}

func TestImportPathCompletion(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "mathlib.zs", "export function add(): i32 { return 0; }\n")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	mainPath := writeModule(t, dir, "main.zs", "")

	got := labels(s0().importPathCompletions(mainPath, "./"))
	if _, ok := got["mathlib"]; !ok {
		t.Errorf("path completion should list module 'mathlib', got %v", keysOf(got))
	}
	if _, ok := got["sub/"]; !ok {
		t.Errorf("path completion should list directory 'sub/', got %v", keysOf(got))
	}
	if _, ok := got["main"]; ok {
		t.Errorf("path completion must not offer the importing file itself")
	}
	// The file entry should be extension-less (imports omit `.zs`).
	if _, ok := got["mathlib.zs"]; ok {
		t.Errorf("path completion should strip the .zs extension")
	}
}

func TestImportSymbolCompletion(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "mathlib.zs", mathlibSrc)
	mainPath := filepath.Join(dir, "main.zs")

	got := labels(s0().importSymbolCompletions(mainPath, "./mathlib"))
	for _, want := range []string{"add", "Vec"} {
		if _, ok := got[want]; !ok {
			t.Errorf("symbol completion should list exported %q, got %v", want, keysOf(got))
		}
	}
}

func TestDetectImportContext(t *testing.T) {
	pathLine := `import { add } from "./ma`
	if ctx := detectImportContext(pathLine, len(pathLine)); ctx.kind != importPath || ctx.pathPrefix != "./ma" {
		t.Errorf("path context: got kind=%d prefix=%q", ctx.kind, ctx.pathPrefix)
	}

	symLine := `import { ad`
	if ctx := detectImportContext(symLine, len(symLine)); ctx.kind != importSymbol {
		t.Errorf("symbol context (no module yet): got kind=%d", ctx.kind)
	}

	symWithModule := `import {  } from "./mathlib"`
	if ctx := detectImportContext(symWithModule, len("import { ")); ctx.kind != importSymbol || ctx.moduleStr != "./mathlib" {
		t.Errorf("symbol context with module: got kind=%d module=%q", ctx.kind, ctx.moduleStr)
	}

	// A plain string that is not an import must not be treated as an import path.
	if ctx := detectImportContext(`let s = "hello`, len(`let s = "hello`)); ctx.kind != importNone {
		t.Errorf("non-import line should be importNone, got kind=%d", ctx.kind)
	}
	// `important` is not the `import` keyword.
	if ctx := detectImportContext(`importantValue`, 5); ctx.kind != importNone {
		t.Errorf("identifier starting with 'import' should be importNone, got kind=%d", ctx.kind)
	}
}

// TestImportCompletionViaURI exercises the full getCompletions path, including the
// URI -> filesystem path conversion that resolves the sibling module.
func TestImportCompletionViaURI(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "mathlib.zs", mathlibSrc)
	content := "import {  } from \"./mathlib\"\n"
	mainPath := writeModule(t, dir, "main.zs", content)
	fileURI := uri.File(mainPath)

	s := NewServer()
	irModule, errs := s.parseDocument(mainPath, content)
	s.documents[string(fileURI)] = &DocumentInfo{Content: content, IRModule: irModule, Errors: errs}

	// Cursor between the braces: `import { | } from "./mathlib"`.
	list := s.getCompletions(fileURI, protocol.Position{Line: 0, Character: 9})
	got := labels(list.Items)
	if _, ok := got["add"]; !ok {
		t.Errorf("import-symbol completion via URI should list 'add', got %v", keysOf(got))
	}
	// It must not fall through to keyword completions inside an import.
	if _, ok := got["function"]; ok {
		t.Errorf("import completion should not include keywords")
	}
}

func s0() *Server { return NewServer() }
