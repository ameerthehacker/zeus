package lsp

import (
	"sort"
	"strings"
	"testing"

	"github.com/ameerthehacker/zeus/internal/analysis"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// shadowSrc has an inner `x: string` shadowing an outer `x: i32`, used by the scope-precision tests.
const shadowSrc = `function main(): i32 {
  let x: i32 = 100;
  if (true) {
    let x: string = "hello";
    console.log(x);
  }
  return x;
}
`

func refLines(refs []protocol.Location) []int {
	lines := []int{}
	for _, r := range refs {
		lines = append(lines, int(r.Range.Start.Line))
	}
	sort.Ints(lines)
	return lines
}

func equalInts(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

const sampleProgram = `class Animal {
  public name: string;
  protected legs: i32;

  constructor(name: string) {
    this.name = name;
    this.legs = 4;
  }

  public speak(): string {
    return this.name;
  }

  public legCount(): i32 {
    return this.legs;
  }
}

class Dog extends Animal {
  public breed: string;

  constructor(name: string, breed: string) {
    super(name);
    this.breed = breed;
  }

  public bark(): i32 {
    return this.legs;
  }
}

function main(): i32 {
  let d: Dog = new Dog("Rex", "Lab");
  d.bark();
  return 0;
}
`

// openDoc parses the given source into a Server as if the editor opened it.
func openDoc(t *testing.T, src string) (*Server, protocol.DocumentURI) {
	t.Helper()
	s := NewServer()
	docURI := protocol.DocumentURI("file:///sample.zs")
	res := analysis.Analyze(docURI.Filename(), src, s.makeModuleResolver(docURI.Filename()))
	s.documents[string(docURI)] = &DocumentInfo{Content: src, AST: res.AST, IRModule: res.Module, Semantic: res.Model, Errors: res.Diagnostics}
	return s, docURI
}

// openDocPath analyzes an on-disk file into a Server as if the editor opened it, keyed by its file
// URI so imports resolve relative to it. Used by cross-file navigation tests.
func openDocPath(t *testing.T, s *Server, path, src string) protocol.DocumentURI {
	t.Helper()
	fileURI := uri.File(path)
	res := analysis.Analyze(path, src, s.makeModuleResolver(path))
	s.documents[string(fileURI)] = &DocumentInfo{Content: src, AST: res.AST, IRModule: res.Module, Semantic: res.Model, Errors: res.Diagnostics}
	return fileURI
}

// posAfter returns the 0-based (line, character) position immediately after the first
// occurrence of marker in src.
func posAfter(t *testing.T, src, marker string) protocol.Position {
	t.Helper()
	idx := strings.Index(src, marker)
	if idx == -1 {
		t.Fatalf("marker %q not found in source", marker)
	}
	end := idx + len(marker)
	line := strings.Count(src[:end], "\n")
	lastNL := strings.LastIndex(src[:end], "\n")
	col := end - (lastNL + 1)
	return protocol.Position{Line: uint32(line), Character: uint32(col)}
}

func labels(items []protocol.CompletionItem) map[string]protocol.CompletionItem {
	m := make(map[string]protocol.CompletionItem, len(items))
	for _, it := range items {
		m[it.Label] = it
	}
	return m
}

// TestPreludeAmbientGlobalsResolveInLSP guards the regression where deleting the Go-built
// primordial globals made `console`/`Math` resolve only via the compiler's injected globals.zs —
// which the language server never compiles. The prelude ambient-global tier must make them resolve
// on the LSP's single-document path too. Run twice to exercise the process-reuse / reset path.
func TestPreludeAmbientGlobalsResolveInLSP(t *testing.T) {
	src := `function main(): i32 {
  console.log("hi");
  let r: f64 = Math.sqrt(4.0);
  return 0;
}
`
	for i := 0; i < 2; i++ {
		_, errs := NewServer().parseDocument("/sample.zs", src)
		for _, e := range errs {
			if strings.Contains(e.Message, "undefined identifier 'console'") ||
				strings.Contains(e.Message, "undefined identifier 'Math'") {
				t.Fatalf("iteration %d: ambient global did not resolve in LSP: %s", i, e.Message)
			}
		}
	}
}

func TestMemberCompletionInstance(t *testing.T) {
	s, uri := openDoc(t, sampleProgram)
	// Complete on `d.` (d is a Dog instance).
	pos := posAfter(t, sampleProgram, "d.bark")
	// Move cursor to just after the dot: `d.` — posAfter("d.bark") is after "bark",
	// so instead target the dot directly.
	pos = posAfter(t, sampleProgram, "  d.")
	list := s.getCompletions(uri, pos)
	got := labels(list.Items)

	// Inherited + own instance members should be present.
	for _, want := range []string{"bark", "speak", "legCount", "name", "breed"} {
		if _, ok := got[want]; !ok {
			t.Errorf("member completion missing %q; got %v", want, keysOf(got))
		}
	}
	// The constructor and any language keyword must NOT appear after a '.'.
	if _, ok := got["constructor"]; ok {
		t.Errorf("member completion should not include constructor")
	}
	if _, ok := got["function"]; ok {
		t.Errorf("member completion should not include keywords")
	}
	// Protected/private members are inaccessible from external access and must be hidden.
	if _, ok := got["legs"]; ok {
		t.Errorf("member completion should not include protected field 'legs' (external access)")
	}
	// A method item should carry a signature detail.
	if bark, ok := got["bark"]; ok && !strings.Contains(bark.Detail, "i32") {
		t.Errorf("bark detail should include return type, got %q", bark.Detail)
	}
}

func TestGeneralCompletionHasKeywordsAndSymbols(t *testing.T) {
	s, uri := openDoc(t, sampleProgram)
	// A non-member position (start of the return line inside main).
	pos := posAfter(t, sampleProgram, "  return 0")
	list := s.getCompletions(uri, pos)
	got := labels(list.Items)
	if _, ok := got["function"]; !ok {
		t.Errorf("general completion should include keyword 'function'")
	}
	if _, ok := got["Dog"]; !ok {
		t.Errorf("general completion should include class 'Dog'")
	}
	if _, ok := got["i32"]; !ok {
		t.Errorf("general completion should include type 'i32'")
	}
}

func TestHoverVariableClassAndMember(t *testing.T) {
	s, uri := openDoc(t, sampleProgram)

	// Hover on the `d` variable use in `d.bark()`.
	h := s.getHover(uri, posAfter(t, sampleProgram, "  d"))
	if h == nil || !strings.Contains(hoverText(h), "Dog") {
		t.Errorf("hover on 'd' should mention Dog, got %v", h)
	}

	// Hover on the class name `Dog` in its declaration.
	h = s.getHover(uri, posAfter(t, sampleProgram, "class Do"))
	if h == nil || !strings.Contains(hoverText(h), "class Dog") {
		t.Errorf("hover on class Dog should mention 'class Dog', got %v", h)
	}

	// Hover on the method `bark` in `d.bark()`.
	h = s.getHover(uri, posAfter(t, sampleProgram, "d.bar"))
	if h == nil || !strings.Contains(hoverText(h), "bark") {
		t.Errorf("hover on member bark should mention 'bark', got %v", h)
	}
}

// TestScopeCorrectShadowing is the Phase 2 payoff: hover and go-to-definition resolve the symbol
// actually in scope at the cursor, via the binding index recorded during IR generation — not the
// flat, scope-blind name table, which would return the same `x` for both uses.
func TestScopeCorrectShadowing(t *testing.T) {
	src := `function main(): i32 {
  let x: i32 = 100;
  if (true) {
    let x: string = "hello";
    console.log(x);
  }
  return x;
}
`
	s, uri := openDoc(t, src)

	// Hover reflects the in-scope binding: inner x is a string, outer x is an i32.
	if h := s.getHover(uri, posAfter(t, src, "log(")); h == nil || !strings.Contains(hoverText(h), "string") {
		t.Errorf("hover on inner x should be string, got %v", h)
	}
	if h := s.getHover(uri, posAfter(t, src, "return ")); h == nil || !strings.Contains(hoverText(h), "i32") {
		t.Errorf("hover on outer x should be i32, got %v", h)
	}

	// Go-to-definition jumps to the correct scope-local declaration (inner let on line 3, outer
	// let on line 1; both 0-based).
	if def := s.getDefinition(uri, posAfter(t, src, "log(")); len(def) == 0 || def[0].Range.Start.Line != 3 {
		t.Errorf("inner x definition should be the inner let (line 3), got %v", def)
	}
	if def := s.getDefinition(uri, posAfter(t, src, "return ")); len(def) == 0 || def[0].Range.Start.Line != 1 {
		t.Errorf("outer x definition should be the outer let (line 1), got %v", def)
	}
}

// TestReferencesAreScopePrecise: references use the binding index, so a shadowed local's
// references include only its own occurrences — not the same-named variable in another scope.
func TestReferencesAreScopePrecise(t *testing.T) {
	s, uri := openDoc(t, shadowSrc)

	// Inner x: its declaration (line 3) and its use in console.log (line 4). Not the outer x.
	if got := refLines(s.getReferences(uri, posAfter(t, shadowSrc, "log("))); !equalInts(got, []int{3, 4}) {
		t.Errorf("inner x references should be lines {3,4}, got %v", got)
	}
	// Outer x: its declaration (line 1) and its use in `return x` (line 6).
	if got := refLines(s.getReferences(uri, posAfter(t, shadowSrc, "return "))); !equalInts(got, []int{1, 6}) {
		t.Errorf("outer x references should be lines {1,6}, got %v", got)
	}
}

// TestRenameLocalVariable: renaming a function-local rewrites exactly its own occurrences.
func TestRenameLocalVariable(t *testing.T) {
	s, uri := openDoc(t, shadowSrc)

	if r := s.prepareRename(uri, posAfter(t, shadowSrc, "log(")); r == nil {
		t.Fatalf("a function-local variable should be renameable")
	}

	edit := s.rename(uri, posAfter(t, shadowSrc, "log("), "y")
	if edit == nil {
		t.Fatalf("rename should produce edits")
	}
	edits := edit.Changes[uri]
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits for the inner x (decl + use), got %d", len(edits))
	}
	for _, e := range edits {
		if e.NewText != "y" {
			t.Errorf("edit should insert 'y', got %q", e.NewText)
		}
		if e.Range.Start.Line != 3 && e.Range.Start.Line != 4 {
			t.Errorf("rename touched line %d, outside the inner x's scope (expected 3 or 4)", e.Range.Start.Line)
		}
	}
}

// TestRenameRefusesNonLocal: only function-locals are renameable (single-file complete). A
// function name — which could be referenced from other files — is refused rather than
// partially/incorrectly renamed.
func TestRenameRefusesNonLocal(t *testing.T) {
	s, uri := openDoc(t, "function main(): i32 {\n  return 0;\n}\n")
	if r := s.prepareRename(uri, posAfter(t, "function main", "function ma")); r != nil {
		t.Errorf("a function name should not be renameable, got %v", r)
	}
}

// TestCapturedVariableRenameRefusedAndRefsComplete guards the review fix: a closure-captured
// variable is escape-boxed into multiple ref-cell symbols, so the binding index cannot rewrite all
// its occurrences. Rename must therefore be REFUSED (never a corrupting partial edit), and
// references must still be COMPLETE via the name-scan fallback.
func TestCapturedVariableRenameRefusedAndRefsComplete(t *testing.T) {
	src := `function make(seed: i32): i32 {
  let f = () => { return seed; };
  return seed + f();
}
`
	s, uri := openDoc(t, src)

	if r := s.prepareRename(uri, posAfter(t, src, "return seed")); r != nil {
		t.Errorf("rename of a closure-captured variable must be refused, got %v", r)
	}
	if e := s.rename(uri, posAfter(t, src, "return seed"), "renamed"); e != nil {
		t.Errorf("rename of a closure-captured variable must produce no edit, got %v", e)
	}

	// `seed` occurs 3 times (param decl, closure use, body use); the fallback must find them all.
	if refs := s.getReferences(uri, posAfter(t, src, "return seed")); len(refs) != 3 {
		t.Errorf("captured-variable references should find all 3 occurrences, got %d", len(refs))
	}
}

// TestRenameToInvalidNameRefused guards the newName validation: renaming to a keyword or a
// non-identifier must be refused so a rename never writes source that fails to parse.
func TestRenameToInvalidNameRefused(t *testing.T) {
	s, uri := openDoc(t, shadowSrc)
	for _, bad := range []string{"return", "class", "123", "x y", ""} {
		if e := s.rename(uri, posAfter(t, shadowSrc, "log("), bad); e != nil {
			t.Errorf("rename to invalid name %q must produce no edit, got %v", bad, e)
		}
	}
	if e := s.rename(uri, posAfter(t, shadowSrc, "log("), "y"); e == nil {
		t.Errorf("rename to a valid identifier should still work")
	}
}

// TestInterfaceFeatures checks the LSP is interface-aware: hover/definition on interface names and
// their members, member completion on interface-typed values, and interfaces in the outline.
func TestInterfaceFeatures(t *testing.T) {
	src := `interface Shape {
  name: string;
  area(): f64;
}
function describe(s: Shape): f64 {
  return s.area();
}
`
	sv, uri := openDoc(t, src)

	// Hover on the interface name in its declaration.
	if h := sv.getHover(uri, posAfter(t, src, "interface Sha")); h == nil || !strings.Contains(hoverText(h), "interface Shape") {
		t.Errorf("hover on interface name should say 'interface Shape', got %v", h)
	}
	// Hover on the interface used as a type annotation.
	if h := sv.getHover(uri, posAfter(t, src, "s: Sha")); h == nil || !strings.Contains(hoverText(h), "interface Shape") {
		t.Errorf("hover on interface type annotation should say 'interface Shape', got %v", h)
	}
	// Hover on a member of an interface-typed value.
	if h := sv.getHover(uri, posAfter(t, src, "s.are")); h == nil || !strings.Contains(hoverText(h), "area") {
		t.Errorf("hover on interface member should mention 'area', got %v", h)
	}
	// Go-to-definition on the interface member resolves to its signature.
	if def := sv.getDefinition(uri, posAfter(t, src, "s.are")); len(def) == 0 {
		t.Errorf("go-to-definition on interface member should resolve")
	}
	// Member completion on an interface-typed value lists the interface's members.
	got := labels(sv.getCompletions(uri, posAfter(t, src, "return s.")).Items)
	for _, want := range []string{"name", "area"} {
		if _, ok := got[want]; !ok {
			t.Errorf("interface member completion missing %q; got %v", want, keysOf(got))
		}
	}
	// The document outline includes the interface.
	foundIface := false
	for _, ds := range sv.getDocumentSymbols(uri) {
		if ds.Name == "Shape" && ds.Kind == protocol.SymbolKindInterface {
			foundIface = true
		}
	}
	if !foundIface {
		t.Errorf("document symbols should include the Shape interface")
	}
}

func TestHoverKeywordAndType(t *testing.T) {
	s, uri := openDoc(t, sampleProgram)
	h := s.getHover(uri, posAfter(t, sampleProgram, "i3"))
	if h == nil || !strings.Contains(hoverText(h), "integer") {
		t.Errorf("hover on i32 should describe integer type, got %v", h)
	}
}

func TestDefinitionResolvesDeclaration(t *testing.T) {
	s, uri := openDoc(t, sampleProgram)
	locs := s.getDefinition(uri, posAfter(t, sampleProgram, "  d.bar"))
	// bark is declared in the Dog class; its span line is where `public bark` appears.
	if len(locs) == 0 {
		t.Fatalf("expected a definition location for bark")
	}
	declLine := uint32(strings.Count(sampleProgram[:strings.Index(sampleProgram, "public bark")], "\n"))
	if locs[0].Range.Start.Line != declLine {
		t.Errorf("bark definition line = %d, want %d", locs[0].Range.Start.Line, declLine)
	}
}

func TestDocumentSymbolsOutline(t *testing.T) {
	s, uri := openDoc(t, sampleProgram)
	syms := s.getDocumentSymbols(uri)

	names := map[string]protocol.DocumentSymbol{}
	for _, sym := range syms {
		names[sym.Name] = sym
	}
	for _, want := range []string{"Animal", "Dog", "main"} {
		if _, ok := names[want]; !ok {
			t.Errorf("document symbols missing %q; got %v", want, keysOfSym(names))
		}
	}
	// Class symbols should nest their members.
	if dog, ok := names["Dog"]; ok {
		childNames := map[string]bool{}
		for _, c := range dog.Children {
			childNames[c.Name] = true
		}
		if !childNames["bark"] {
			t.Errorf("Dog outline should include method 'bark', got children %v", dog.Children)
		}
	}
	// Primordials must not leak into the outline.
	if _, ok := names["Console"]; ok {
		t.Errorf("document symbols should not include primordial 'Console'")
	}
}

func TestDocumentHighlightAndReferences(t *testing.T) {
	s, uri := openDoc(t, sampleProgram)
	// Cursor on `bark`, which occurs twice: the declaration and the call `d.bark()`.
	pos := posAfter(t, sampleProgram, "d.bar")

	hl := s.getDocumentHighlights(uri, pos)
	if len(hl) != 2 {
		t.Errorf("expected 2 highlights for 'bark', got %d", len(hl))
	}
	refs := s.getReferences(uri, pos)
	if len(refs) != 2 {
		t.Errorf("expected 2 references for 'bark', got %d", len(refs))
	}
	// A reference occurrence inside a string/comment must not be matched: `name` appears only
	// as identifiers here, so use it to sanity-check that highlights are token-based.
	if len(refs) > 0 && refs[0].URI != uri {
		t.Errorf("reference should point at the same document")
	}
}

func TestSignatureHelpConstructor(t *testing.T) {
	s, uri := openDoc(t, sampleProgram)
	// Cursor just after the comma inside `new Dog("Rex", "Lab")` → second parameter.
	pos := posAfter(t, sampleProgram, `new Dog("Rex", `)
	sh := s.getSignatureHelp(uri, pos)
	if sh == nil || len(sh.Signatures) == 0 {
		t.Fatalf("expected signature help inside the constructor call")
	}
	sig := sh.Signatures[0]
	if !strings.Contains(sig.Label, "constructor") {
		t.Errorf("signature label = %q, want it to mention the constructor", sig.Label)
	}
	if len(sig.Parameters) != 2 {
		t.Errorf("constructor should have 2 parameters, got %d", len(sig.Parameters))
	}
	if sh.ActiveParameter != 1 {
		t.Errorf("active parameter = %d, want 1 (second arg)", sh.ActiveParameter)
	}
}

func TestSignatureHelpMethod(t *testing.T) {
	s, uri := openDoc(t, sampleProgram)
	// Cursor just after the '(' in `d.bark()`.
	pos := posAfter(t, sampleProgram, "d.bark(")
	sh := s.getSignatureHelp(uri, pos)
	if sh == nil || len(sh.Signatures) == 0 {
		t.Fatalf("expected signature help for method call")
	}
	if !strings.Contains(sh.Signatures[0].Label, "bark") {
		t.Errorf("signature label = %q, want it to mention 'bark'", sh.Signatures[0].Label)
	}
}

func TestInlayHints(t *testing.T) {
	// `count` is un-annotated (gets a hint); `d` has an explicit type (no hint). Inlay hints
	// are best-effort: they are shown only when the variable's inferred type is resolvable, and
	// never show a wrong type.
	src := "function main(): i32 {\n  let d: i32 = 5;\n  let count = 42;\n  return d + count;\n}\n"
	s, uri := openDoc(t, src)
	hints := s.getInlayHints(inlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Range:        protocol.Range{End: protocol.Position{Line: 1000, Character: 0}},
	})
	if len(hints) != 1 {
		t.Fatalf("expected exactly 1 inlay hint (for `let count = 42`), got %d: %+v", len(hints), hints)
	}
	if !strings.HasPrefix(hints[0].Label, ": ") {
		t.Errorf("inlay hint label = %q, want it to start with ': '", hints[0].Label)
	}
	countLine := uint32(strings.Count(src[:strings.Index(src, "let count")], "\n"))
	if hints[0].Position.Line != countLine {
		t.Errorf("inlay hint on line %d, want %d (the `let count` line)", hints[0].Position.Line, countLine)
	}
}

func hoverText(h *protocol.Hover) string {
	if h == nil {
		return ""
	}
	return h.Contents.Value
}

func keysOf(m map[string]protocol.CompletionItem) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func keysOfSym(m map[string]protocol.DocumentSymbol) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestInStringOrComment unit-tests the cursor-context state machine that gates completion.
func TestInStringOrComment(t *testing.T) {
	cases := []struct {
		src  string
		col  int
		want bool
	}{
		{`let s = "hello`, len(`let s = "hello`), true},   // inside an (unterminated) string
		{`let s = "hi";`, len(`let s = "hi";`), false},    // after the string closes
		{`let s = "a\"b`, len(`let s = "a\"b`), true},     // escaped quote stays in the string
		{`// comment`, len(`// comment`), true},           // line comment
		{`let x = 1; // c`, len(`let x = 1; // c`), true}, // trailing line comment
		{`let x = 1;`, len(`let x = 1;`), false},          // plain code
		{`let c = 'a`, len(`let c = 'a`), true},           // char literal
		{"let s = `tmpl ", len("let s = `tmpl "), true},   // template literal text
		{"let s = `a${b", len("let s = `a${b"), false},    // interpolation is code
		{"let s = `a${b}c", len("let s = `a${b}c"), true}, // back to template text after }
	}
	for _, c := range cases {
		if got := inStringOrComment(c.src, 0, c.col); got != c.want {
			t.Errorf("inStringOrComment(%q, col=%d) = %v, want %v", c.src, c.col, got, c.want)
		}
	}
	// Multi-line block comment: the cursor on line 1 is still inside the comment opened on line 0.
	if !inStringOrComment("/* line one\nstill inside", 1, 5) {
		t.Errorf("cursor inside a multi-line block comment should be suppressed")
	}
}

// TestCompletionSuppressedInStringAndComment checks the end-to-end completion gate: no items are
// offered inside a string literal or comment, but code positions still complete.
func TestCompletionSuppressedInStringAndComment(t *testing.T) {
	src := "function main(): i32 {\n" +
		"  let s: string = \"hello world\";\n" +
		"  // a comment here\n" +
		"  return 0;\n" +
		"}\n"
	s, docURI := openDoc(t, src)

	if items := s.getCompletions(docURI, posAfter(t, src, `"hello `)).Items; len(items) != 0 {
		t.Errorf("expected no completions inside a string, got %d", len(items))
	}
	if items := s.getCompletions(docURI, posAfter(t, src, "// a comment ")).Items; len(items) != 0 {
		t.Errorf("expected no completions inside a comment, got %d", len(items))
	}
	if items := s.getCompletions(docURI, posAfter(t, src, "  return ")).Items; len(items) == 0 {
		t.Errorf("expected completions in code position, got 0")
	}
}

// TestCompletionOffersAmbientGlobals guards that `console`/`Math` appear in completion while typing,
// even though they only enter a module's symbol table lazily on first reference.
func TestCompletionOffersAmbientGlobals(t *testing.T) {
	src := "function main(): i32 {\n  return 0;\n}\n"
	s, docURI := openDoc(t, src)
	got := labels(s.getCompletions(docURI, posAfter(t, src, "  return ")).Items)
	for _, name := range []string{"console", "Math"} {
		if _, ok := got[name]; !ok {
			t.Errorf("completion should offer ambient global %q, got %v", name, keysOf(got))
		}
	}
}

// TestCompletionHidesInternalSymbols asserts no compiler-internal names (%/$/# prefixed) leak into
// completion for a normal program.
func TestCompletionHidesInternalSymbols(t *testing.T) {
	s, docURI := openDoc(t, sampleProgram)
	for _, it := range s.getCompletions(docURI, posAfter(t, sampleProgram, "d.bark();\n  ")).Items {
		if isInternalSymbolName(it.Label) {
			t.Errorf("completion leaked internal symbol %q", it.Label)
		}
	}
}
