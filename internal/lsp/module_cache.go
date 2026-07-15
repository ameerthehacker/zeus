package lsp

import (
	"os"
	"time"

	"github.com/ameerthehacker/zeus/internal/ir"
)

// moduleCache is the server's persistent, invalidation-aware cache of generated import modules.
// Without it every keystroke re-reads, re-parses, and re-generates every imported file; with it an
// unchanged import is generated once and reused until it — or a module that imports it — changes.
//
// The language server processes messages sequentially, so the cache needs no locking.
type moduleCache struct {
	entries map[string]*moduleEntry // absolute path -> entry
	builds  int                     // total modules generated; observed by tests
}

type moduleEntry struct {
	module  *ir.IRModule
	modTime time.Time // file mtime when the module was generated
	imports []string  // direct import paths, for reverse (dependent) invalidation
}

func newModuleCache() *moduleCache {
	return &moduleCache{entries: map[string]*moduleEntry{}}
}

// getFresh returns the cached module for path if it is still fresh — the file exists and its
// modification time matches what it was when the module was generated.
func (c *moduleCache) getFresh(path string) (*ir.IRModule, bool) {
	e, ok := c.entries[path]
	if !ok {
		return nil, false
	}
	info, err := os.Stat(path)
	if err != nil || !info.ModTime().Equal(e.modTime) {
		return nil, false
	}
	return e.module, true
}

// put stores a freshly generated module along with its direct imports and the file's mtime.
func (c *moduleCache) put(path string, module *ir.IRModule, modTime time.Time, imports []string) {
	c.entries[path] = &moduleEntry{module: module, modTime: modTime, imports: imports}
	c.builds++
}

// invalidate drops path from the cache along with every module that (transitively) imports it, so
// a change to one file refreshes everything depending on it. Unrelated modules stay cached.
func (c *moduleCache) invalidate(path string) {
	removed := map[string]bool{path: true}
	delete(c.entries, path)
	for {
		changed := false
		for p, e := range c.entries {
			for _, imp := range e.imports {
				if removed[imp] {
					delete(c.entries, p)
					removed[p] = true
					changed = true
					break
				}
			}
		}
		if !changed {
			break
		}
	}
}

// readModuleFile reads a module's source and its modification time together, so the mtime recorded
// in the cache corresponds to the bytes that were generated.
func readModuleFile(path string) (data []byte, modTime time.Time, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	data, err = os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	return data, info.ModTime(), nil
}

// appendUnique appends v to s only if it is not already present.
func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
