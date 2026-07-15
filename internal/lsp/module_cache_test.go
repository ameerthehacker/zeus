package lsp

import (
	"os"
	"testing"
	"time"
)

// TestModuleCacheReusesUnchangedImports is the Phase 3 payoff: an unchanged imported module is
// generated once and reused on subsequent analyses (every keystroke re-analyzes the entry), and is
// regenerated only when the file actually changes.
func TestModuleCacheReusesUnchangedImports(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "mathlib.zs", mathlibSrc)
	mainSrc := "import { add } from \"./mathlib\";\nfunction main(): i32 { return add(1, 2); }\n"
	mainPath := writeModule(t, dir, "main.zs", mainSrc)

	s := NewServer()

	// First analysis generates the imported module once.
	s.parseDocument(mainPath, mainSrc)
	if s.modules.builds != 1 {
		t.Fatalf("expected 1 import build after first analysis, got %d", s.modules.builds)
	}

	// Re-analyzing the entry reuses the cached import rather than regenerating it.
	s.parseDocument(mainPath, mainSrc)
	if s.modules.builds != 1 {
		t.Fatalf("unchanged import should be reused; builds rose to %d", s.modules.builds)
	}

	// Editing the imported file (new content + newer mtime) makes it stale, so it is rebuilt.
	libPath := writeModule(t, dir, "mathlib.zs",
		mathlibSrc+"export function sub(a: i32, b: i32): i32 { return a - b; }\n")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(libPath, future, future); err != nil {
		t.Fatal(err)
	}
	s.parseDocument(mainPath, mainSrc)
	if s.modules.builds != 2 {
		t.Fatalf("changed import should be regenerated (builds=2), got %d", s.modules.builds)
	}
}

// TestModuleCacheReverseInvalidation checks that invalidating a module drops every module that
// transitively imports it, while unrelated modules stay cached.
func TestModuleCacheReverseInvalidation(t *testing.T) {
	c := newModuleCache()
	c.entries["/leaf"] = &moduleEntry{}
	c.entries["/mid"] = &moduleEntry{imports: []string{"/leaf"}}
	c.entries["/top"] = &moduleEntry{imports: []string{"/mid"}}
	c.entries["/other"] = &moduleEntry{}

	c.invalidate("/leaf")

	for _, p := range []string{"/leaf", "/mid", "/top"} {
		if _, ok := c.entries[p]; ok {
			t.Errorf("%s should have been invalidated (it transitively imports /leaf)", p)
		}
	}
	if _, ok := c.entries["/other"]; !ok {
		t.Errorf("/other is unrelated and should stay cached")
	}
}
