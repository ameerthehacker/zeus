package lsp

import (
	"strings"
	"testing"

	"github.com/ameerthehacker/zeus/internal/analysis"
	"go.lsp.dev/protocol"
)

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
	uri := protocol.DocumentURI("file:///sample.zs")
	res := analysis.Analyze(uri.Filename(), src, s.makeModuleResolver(uri.Filename()))
	s.documents[string(uri)] = &DocumentInfo{Content: src, AST: res.AST, IRModule: res.Module, Errors: res.Diagnostics}
	return s, uri
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
