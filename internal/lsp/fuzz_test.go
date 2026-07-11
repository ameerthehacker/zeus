package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

// brokenSeeds is a hand-curated corpus of malformed / partial Zeus programs.
// These are the kinds of inputs an editor produces mid-keystroke: unterminated
// literals, dangling operators, half-written declarations, unbalanced braces, etc.
// The LSP must survive every one of them without panicking.
var brokenSeeds = []string{
	``,
	` `,
	"\n\n\n",
	"\x00",
	"�",
	"log(",
	"log(\"unterminated",
	"log('unterminated",
	"log(`unterminated",
	"function main(): void {\n  log(\"Hello World!)\n}",
	"function",
	"function main",
	"function main(",
	"function main()",
	"function main():",
	"function main(): void",
	"function main(): void {",
	"function main(): void {}",
	"class",
	"class Foo",
	"class Foo {",
	"class Foo extends",
	"class Foo extends Bar {",
	"class Foo { x: }",
	"class Foo { x: i32 }",
	"class Foo { constructor( }",
	"class Foo { get }",
	"class Foo { get x() }",
	"class Foo { set x( }",
	"class Foo { static }",
	"let",
	"let x",
	"let x =",
	"let x = ",
	"let x: ",
	"let x: i32 =",
	"const x =",
	"let x = 1 +",
	"let x = 1 + * 2",
	"let x = (((",
	"let x = )))",
	"let x = [",
	"let x = ]",
	"let x = {",
	"let x = [1, 2,",
	"let x = 0x",
	"let x = 0b",
	"let x = 0o",
	"let x = 0xZZ",
	"let x = 999999999999999999999999999999999",
	"let x = 1.2.3.4",
	"let x = .",
	"let x = ..",
	"let x = ...",
	"return",
	"return;",
	"if",
	"if (",
	"if ()",
	"if (true)",
	"if (true) {",
	"else",
	"while",
	"while (",
	"while (true) {",
	"for",
	"for (",
	"for (;;) {",
	"new",
	"new Foo",
	"new Foo(",
	"new Foo()",
	"new [",
	"import",
	"import {",
	"import { x } from",
	"import { x } from \"",
	"export",
	"export const",
	".",
	"..",
	"...",
	",",
	";",
	":",
	"=",
	"==",
	"=>",
	"->",
	"()",
	"(){}",
	"{}",
	"[]",
	"a.",
	"a.b.",
	"a..b",
	"a.b().c.",
	"this.",
	"super.",
	"super(",
	"foo.bar.baz.qux.",
	"let x = foo.",
	"let x: i32 = foo.bar",
	"1 . 2",
	"true.",
	"\"str\".",
	"[].",
	"[1,2,3].",
	"function f() { return f(); }",
	"class A extends A {}",
	"class A extends B {} class B extends A {}",
	"let x: Unknown = 1",
	"let x: i32[] = [",
	"let x = () =>",
	"let x = (a) => a.",
	"let x = (a: ) => a",
	"function f(a: i32, ) {}",
	"function f(...) {}",
	"function f(...args) {}",
	"function f(...args: ) {}",
	"@",
	"#",
	"$",
	"`",
	"'",
	"\"",
	"\\",
	"/* unterminated",
	"// comment only",
	"/**/",
	"log(\"\\",
	"log(\"\\u",
	"log(\"\\u{",
	"log(`${`",
	"log(`${}`)",
	"log(`${x`)",
	"let x = 1 ? 2",
	"let x = 1 ? 2 :",
	"let x = a && ",
	"let x = a || ",
	"let x = !",
	"let x = ~",
	"let x = -",
	"let x = --",
	"let x = a ** ",
	"let x = a **= ",
	strings.Repeat("(", 500),
	strings.Repeat("a.", 500),
	strings.Repeat("[", 200),
	strings.Repeat("class A {", 100),
	strings.Repeat("\n", 1000),
	// User declarations shadowing primordial classes must not crash the primordial getters.
	"let string = 0; let y = \"hi\"",
	"function Error() {} let z = 1; z.foo",
	"let Console = 5; console.log(\"x\")",
	// Undefined identifiers feeding operators/args/ternaries.
	"let x = undef ? 1 : 2",
	"let x = undef ** 2",
	"f(undef1, undef2)",
	"new Undefined(a, b)",
	"for (let ;;) {}",
	"try {} catch (e: Undefined) {}",
	"i32; 0",
	"new (0)",
	"arr[]",
	"let arr\nif (arr[0].A = 0) {}",
}

// loadRealSeeds walks the e2e spec tree and returns the content of every .zs
// file so the fuzzer starts from valid programs it can mutate.
func loadRealSeeds(t testing.TB) []string {
	t.Helper()
	var seeds []string
	roots := []string{"../../test/e2e/specs", "../../playground"}
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".zs") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr == nil {
				seeds = append(seeds, string(data))
			}
			return nil
		})
	}
	return seeds
}

// exerciseLSP runs the full editor-facing surface of the LSP against a document.
// If any stage panics, the test/fuzzer records the offending input.
func exerciseLSP(content string) {
	s := NewServer()
	// A URI under a nonexistent directory: import path/symbol completion resolves against
	// uri.Filename(), so this keeps fuzzing from reading real files off disk.
	const uri = protocol.DocumentURI("file:///zeus-nonexistent-fuzz-dir/fuzz.zs")
	// This mirrors the didOpen/didChange path.
	s.documents[string(uri)] = &DocumentInfo{Content: content}
	// Empty docPath disables import resolution so fuzzing never reads files off disk.
	irModule, errs := s.parseDocument("", content)
	s.documents[string(uri)] = &DocumentInfo{Content: content, IRModule: irModule, Errors: errs}

	// Convert diagnostics (exercises span conversion).
	_ = s.convertToLSPDiagnostics(errs)

	// Document symbols and inlay hints are position-independent — probe once.
	_ = s.getDocumentSymbols(uri)
	_ = s.getInlayHints(inlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Range:        protocol.Range{End: protocol.Position{Line: 1 << 20, Character: 1 << 20}},
	})

	// Probe every position-driven feature at every line/col boundary in the document.
	lines := strings.Split(content, "\n")
	probe := func(pos protocol.Position) {
		_ = s.getCompletions(uri, pos)
		_ = s.getHover(uri, pos)
		_ = s.getDefinition(uri, pos)
		_ = s.getDocumentHighlights(uri, pos)
		_ = s.getReferences(uri, pos)
		_ = s.getSignatureHelp(uri, pos)
	}
	for lineNum, line := range lines {
		// Probe a handful of columns per line, including past the end.
		for _, col := range []int{0, len(line) / 2, len(line), len(line) + 1} {
			if col < 0 {
				col = 0
			}
			probe(protocol.Position{Line: uint32(lineNum), Character: uint32(col)})
		}
	}
	// Also probe wildly out-of-range positions.
	for _, pos := range []protocol.Position{
		{Line: 0, Character: 0},
		{Line: uint32(len(lines)), Character: 0},
		{Line: 1 << 20, Character: 1 << 20},
	} {
		probe(pos)
	}
}

// FuzzLSP throws arbitrary bytes at the whole LSP surface. A panic fails the run.
func FuzzLSP(f *testing.F) {
	for _, seed := range brokenSeeds {
		f.Add(seed)
	}
	for _, seed := range loadRealSeeds(f) {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, content string) {
		exerciseLSP(content)
	})
}

// TestBrokenSeedsDoNotPanic runs the curated broken corpus without the fuzzing
// engine so `go test` (no -fuzz) still guards against regressions.
func TestBrokenSeedsDoNotPanic(t *testing.T) {
	seeds := append([]string{}, brokenSeeds...)
	seeds = append(seeds, loadRealSeeds(t)...)
	for _, seed := range seeds {
		seed := seed
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on input %q: %v", truncate(seed, 80), r)
				}
			}()
			exerciseLSP(seed)
		}()
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
