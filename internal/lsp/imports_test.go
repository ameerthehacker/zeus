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

	// Cursor just after the typed `./` (2 runes) inside the quotes.
	pos := protocol.Position{Line: 0, Character: 2}
	got := labels(s0().importPathCompletions(mainPath, "./", pos))
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
	// The inserted specifier must preserve the typed `./` and the TextEdit must replace the whole
	// typed prefix (start at the opening quote, i.e. character 0 here).
	if edit := got["mathlib"].TextEdit; edit == nil || edit.NewText != "./mathlib" {
		t.Errorf("file insert text should be \"./mathlib\", got %+v", got["mathlib"].TextEdit)
	} else if edit.Range.Start.Character != 0 || edit.Range.End.Character != 2 {
		t.Errorf("file insert should replace the whole typed prefix, got range %+v", edit.Range)
	}
}

// TestImportPathCompletionAddsRelativePrefix verifies that a bare sibling (no `./` typed yet) is
// inserted as an explicit relative specifier `./relative`, not `relative`.
func TestImportPathCompletionAddsRelativePrefix(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "relative.zs", "export function f(): i32 { return 0; }\n")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	mainPath := writeModule(t, dir, "main.zs", "")

	// Empty prefix: cursor sits right after the opening quote (`from "|"`).
	got := labels(s0().importPathCompletions(mainPath, "", protocol.Position{Line: 0, Character: 0}))
	if edit := got["relative"].TextEdit; edit == nil || edit.NewText != "./relative" {
		t.Errorf("bare sibling should insert \"./relative\", got %+v", got["relative"].TextEdit)
	}
	if edit := got["sub/"].TextEdit; edit == nil || edit.NewText != "./sub/" {
		t.Errorf("bare directory should insert \"./sub/\", got %+v", got["sub/"].TextEdit)
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

// TestDefinitionCrossFileImport verifies go-to-definition on a use of an imported symbol jumps to
// the symbol's declaration in the *source* module, not the importing file.
func TestDefinitionCrossFileImport(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "mathlib.zs", mathlibSrc)
	mainSrc := "import { add } from \"./mathlib\";\nfunction main(): i32 { return add(1, 2); }\n"
	mainPath := writeModule(t, dir, "main.zs", mainSrc)

	s := NewServer()
	fileURI := openDocPath(t, s, mainPath, mainSrc)

	// Cursor on the usage `add(1, 2)`.
	locs := s.getDefinition(fileURI, posAfter(t, mainSrc, "return ad"))
	if len(locs) != 1 {
		t.Fatalf("expected 1 definition location, got %d", len(locs))
	}
	wantURI := uri.File(filepath.Join(dir, "mathlib.zs"))
	if locs[0].URI != wantURI {
		t.Errorf("definition should point to mathlib.zs (%s), got %s", wantURI, locs[0].URI)
	}
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("`add` is declared on line 0 of mathlib, got line %d", locs[0].Range.Start.Line)
	}

	// The same must hold when clicking the name inside the import clause itself.
	clauseLocs := s.getDefinition(fileURI, posAfter(t, mainSrc, "import { ad"))
	if len(clauseLocs) != 1 || clauseLocs[0].URI != wantURI {
		t.Errorf("definition on the imported name in the clause should open mathlib.zs, got %v", clauseLocs)
	}
}

// TestDefinitionLocalShadowBeatsImport verifies that a local variable shadowing an imported name
// resolves to the local declaration (same file), not the import's source module.
func TestDefinitionLocalShadowBeatsImport(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "mathlib.zs", mathlibSrc)
	mainSrc := "import { add } from \"./mathlib\";\n" +
		"function main(): i32 {\n" +
		"  let add: i32 = 5;\n" +
		"  return add;\n" +
		"}\n"
	mainPath := writeModule(t, dir, "main.zs", mainSrc)

	s := NewServer()
	fileURI := openDocPath(t, s, mainPath, mainSrc)

	// Cursor on the shadowing use `return add` — must stay in main.zs at the local `let add` (line 2).
	locs := s.getDefinition(fileURI, posAfter(t, mainSrc, "return ad"))
	if len(locs) != 1 {
		t.Fatalf("expected 1 definition location, got %d", len(locs))
	}
	if locs[0].URI != fileURI {
		t.Errorf("shadowed local should resolve within main.zs, got %s", locs[0].URI)
	}
	if locs[0].Range.Start.Line != 2 {
		t.Errorf("local `add` is declared on line 2, got line %d", locs[0].Range.Start.Line)
	}
}

// TestDefinitionOnImportPath verifies go-to-definition on the `from "./x"` path string opens the
// resolved module file.
func TestDefinitionOnImportPath(t *testing.T) {
	dir := t.TempDir()
	mathPath := writeModule(t, dir, "mathlib.zs", mathlibSrc)
	mainSrc := "import { add } from \"./mathlib\";\nfunction main(): i32 { return add(1, 2); }\n"
	mainPath := writeModule(t, dir, "main.zs", mainSrc)

	s := NewServer()
	fileURI := openDocPath(t, s, mainPath, mainSrc)

	locs := s.getDefinition(fileURI, posAfter(t, mainSrc, `"./math`))
	if len(locs) != 1 {
		t.Fatalf("expected 1 location for the import path, got %d", len(locs))
	}
	if locs[0].URI != uri.File(mathPath) {
		t.Errorf("import path definition should open mathlib.zs (%s), got %s", uri.File(mathPath), locs[0].URI)
	}
}

func s0() *Server { return NewServer() }
