package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ameerthehacker/zeus/internal/analysis"
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
	// Valid programs that drive the semantic model + rename (locals, params, shadowing, capture).
	"function f(a: i32): i32 { let b: i32 = a; return b; }",
	"function f(a: i32, b: i32): i32 { return a + b; }",
	"function make(seed: i32): i32 { let g = () => { return seed; }; return seed + g(); }",
	"function outer(): i32 { let x: i32 = 1; { let x: i32 = 2; return x; } }",
	"class Point { public x: i32; getX(): i32 { return this.x; } }\nlet p = new Point(); let v = p.getX();",
	"function f(n: i32): i32 { let acc: i32 = 0; for (let i: i32 = 0; i < n; i = i + 1) { acc = acc + i; } return acc; }",
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

// exerciseLSP runs the full editor-facing surface of the LSP against a document and asserts the
// responses are well-formed. It checks two things: (1) no stage panics, and (2) nothing
// misbehaves — every returned range is ordered (start <= end), locations point at the document,
// completion items are labelled, rename agrees with prepareRename and never emits invalid/
// overlapping edits. Failures are reported through t, so the fuzzer minimizes the offending input.
func exerciseLSP(t testing.TB, content string) {
	s := NewServer()
	// A URI under a nonexistent directory: import path/symbol completion resolves against
	// uri.Filename(), so this keeps fuzzing from reading real files off disk.
	const uri = protocol.DocumentURI("file:///zeus-nonexistent-fuzz-dir/fuzz.zs")

	// Analyze with no import resolver so fuzzing never reads files off disk, and populate the full
	// DocumentInfo (AST + semantic model) exactly as the real didOpen/didChange path does — so the
	// position index, binding index, and rename are all exercised, not only the text fallbacks.
	res := analysis.Analyze("", content, nil)
	s.documents[string(uri)] = &DocumentInfo{
		Content:  content,
		AST:      res.AST,
		IRModule: res.Module,
		Semantic: res.Model,
		Errors:   res.Diagnostics,
	}

	// Diagnostics must convert to ordered ranges.
	for _, d := range s.convertToLSPDiagnostics(res.Diagnostics) {
		checkRange(t, content, "diagnostic", d.Range)
	}

	// Position-independent features.
	for _, ds := range s.getDocumentSymbols(uri) {
		checkRange(t, content, "documentSymbol", ds.Range)
		checkRange(t, content, "documentSymbol.selection", ds.SelectionRange)
		if ds.Name == "" {
			t.Errorf("document symbol with empty name on %q", truncate(content, 60))
		}
	}
	_ = s.getInlayHints(inlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Range:        protocol.Range{End: protocol.Position{Line: 1 << 20, Character: 1 << 20}},
	})

	lines := strings.Split(content, "\n")
	probe := func(pos protocol.Position) {
		if cl := s.getCompletions(uri, pos); cl != nil {
			for _, it := range cl.Items {
				if it.Label == "" {
					t.Errorf("completion item with empty label at %v on %q", pos, truncate(content, 60))
				}
			}
		}
		if h := s.getHover(uri, pos); h != nil && strings.TrimSpace(h.Contents.Value) == "" {
			t.Errorf("hover returned empty content at %v on %q", pos, truncate(content, 60))
		}
		for _, loc := range s.getDefinition(uri, pos) {
			checkRange(t, content, "definition", loc.Range)
			if loc.URI != uri {
				t.Errorf("definition points at a foreign URI %q at %v", loc.URI, pos)
			}
		}
		for _, loc := range s.getReferences(uri, pos) {
			checkRange(t, content, "references", loc.Range)
			if loc.URI != uri {
				t.Errorf("reference points at a foreign URI %q at %v", loc.URI, pos)
			}
		}
		for _, hl := range s.getDocumentHighlights(uri, pos) {
			checkRange(t, content, "highlight", hl.Range)
		}
		_ = s.getSignatureHelp(uri, pos)
		checkRename(t, s, uri, content, pos)
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

	// Illegal rename targets must always be refused (position-independent, so check once).
	for _, bad := range []string{"", "class", "return", "1abc", "a b", "i32"} {
		if s.rename(uri, protocol.Position{}, bad) != nil {
			t.Errorf("rename to illegal name %q was not refused on %q", bad, truncate(content, 60))
		}
	}
}

// checkRange fails if r is out of order: a range whose start is after its end would make an editor
// select or replace a nonsensical region.
func checkRange(t testing.TB, content, feature string, r protocol.Range) {
	if posBefore(r.End, r.Start) {
		t.Errorf("%s range start after end: %v on %q", feature, r, truncate(content, 60))
	}
}

// checkRename verifies rename never misbehaves at pos: it agrees with prepareRename, edits only the
// document under the cursor, always inserts exactly the requested name, and produces ordered,
// non-overlapping edit ranges (an overlapping WorkspaceEdit is invalid and would corrupt on apply).
func checkRename(t testing.TB, s *Server, uri protocol.DocumentURI, content string, pos protocol.Position) {
	const newName = "zzz_fuzz_rename"
	pr := s.prepareRename(uri, pos)
	edit := s.rename(uri, pos, newName)

	if (pr != nil) != (edit != nil) {
		t.Errorf("prepareRename/rename disagree at %v (prepare=%v rename=%v) on %q",
			pos, pr != nil, edit != nil, truncate(content, 60))
	}
	if pr != nil {
		checkRange(t, content, "prepareRename", *pr)
	}
	if edit == nil {
		return
	}
	if len(edit.Changes) != 1 {
		t.Errorf("rename edited %d files, expected exactly the current document, at %v", len(edit.Changes), pos)
	}
	edits, ok := edit.Changes[uri]
	if !ok || len(edits) == 0 {
		t.Errorf("rename produced no edit for the document URI at %v on %q", pos, truncate(content, 60))
		return
	}
	for _, e := range edits {
		checkRange(t, content, "rename edit", e.Range)
		if e.NewText != newName {
			t.Errorf("rename edit inserts %q, expected %q, at %v", e.NewText, newName, pos)
		}
	}
	if editsOverlap(edits) {
		t.Errorf("rename produced overlapping edits at %v on %q", pos, truncate(content, 60))
	}
}

func posBefore(p, q protocol.Position) bool {
	if p.Line != q.Line {
		return p.Line < q.Line
	}
	return p.Character < q.Character
}

func editsOverlap(edits []protocol.TextEdit) bool {
	for i := range edits {
		for j := i + 1; j < len(edits); j++ {
			a, b := edits[i].Range, edits[j].Range
			// Overlap iff a starts before b ends AND b starts before a ends (touching is allowed).
			if posBefore(a.Start, b.End) && posBefore(b.Start, a.End) {
				return true
			}
		}
	}
	return false
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
		exerciseLSP(t, content)
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
			exerciseLSP(t, seed)
		}()
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
