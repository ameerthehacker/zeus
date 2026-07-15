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

// TestModuleCacheTransitiveFreshness guards the review fix: a cached module is stale when one of
// its transitive dependencies changes on disk, even if the module's own file is untouched.
func TestModuleCacheTransitiveFreshness(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "c.zs", "export function cee(): i32 { return 1; }\n")
	writeModule(t, dir, "b.zs", "import { cee } from \"./c\";\nexport function bee(): i32 { return cee(); }\n")
	mainSrc := "import { bee } from \"./b\";\nfunction main(): i32 { return bee(); }\n"
	mainPath := writeModule(t, dir, "main.zs", mainSrc)

	s := NewServer()
	s.parseDocument(mainPath, mainSrc)
	if s.modules.builds != 2 {
		t.Fatalf("expected 2 import builds (b and c), got %d", s.modules.builds)
	}
	// Re-analyzing reuses both.
	s.parseDocument(mainPath, mainSrc)
	if s.modules.builds != 2 {
		t.Fatalf("unchanged imports should be reused, got %d builds", s.modules.builds)
	}

	// Change C on disk (only C; B's own file is untouched). B must be treated as stale because it
	// transitively imports C, so both regenerate.
	cPath := writeModule(t, dir, "c.zs", "export function cee(): i32 { return 2; }\n")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(cPath, future, future); err != nil {
		t.Fatal(err)
	}
	s.parseDocument(mainPath, mainSrc)
	if s.modules.builds != 4 {
		t.Errorf("changing transitive dep C should rebuild both B and C (builds 2->4), got %d", s.modules.builds)
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
