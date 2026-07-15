package lsp

import (
	"github.com/ameerthehacker/zeus/internal/analysis"
	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

// This file runs the compiler front-end (lex -> parse -> IR -> type check) for the language
// server, tolerating errors so IDE features still work on incomplete code, and resolves imports
// on demand.

// parseDocument analyzes a document and returns the IR module and any diagnostics. It delegates
// to analysis.Analyze — the shared front-end that tolerates errors (returning partial results so
// IDE features still work on incomplete code) and never calls os.Exit. docPath is the filesystem
// path of the document, used to resolve imports relative to it; an empty docPath disables import
// resolution (used by fuzzing so no files are read).
func (s *Server) parseDocument(docPath, content string) (*ir.IRModule, []*zeus_error.ZeusError) {
	res := analysis.Analyze(docPath, content, s.makeModuleResolver(docPath))
	return res.Module, res.Diagnostics
}

// makeModuleResolver returns the getModule callback the IR generator uses to resolve imports.
// The IR generator resolves the import path to an absolute file path before calling this, so the
// callback reads, parses, and IR-generates that file on demand. Generated modules are stored in
// the server's persistent moduleCache and reused across edits as long as the file (by mtime) is
// unchanged, so an unchanged import is not regenerated on every keystroke. Errors inside imported
// modules are intentionally not surfaced on the importing document.
//
// When baseDocPath is empty (fuzzing), resolution is disabled and no files are read.
func (s *Server) makeModuleResolver(baseDocPath string) func(string) *ir.IRModule {
	if baseDocPath == "" {
		return func(string) *ir.IRModule { return nil }
	}
	// Per-analysis state: modules already resolved this pass (dedup + import-cycle break), the stack
	// of modules currently being generated (to attribute each resolved import to its importer), and
	// the direct imports collected for each module built this pass.
	seen := map[string]*ir.IRModule{}
	var stack []string
	directImports := map[string][]string{}

	var resolve func(path string) *ir.IRModule
	resolve = func(path string) *ir.IRModule {
		// Attribute this import to the module currently being generated, for reverse invalidation.
		if len(stack) > 0 {
			parent := stack[len(stack)-1]
			directImports[parent] = appendUnique(directImports[parent], path)
		}
		if m, ok := seen[path]; ok {
			return m
		}
		// Reuse an unchanged module instead of regenerating it.
		if m, ok := s.modules.getFresh(path); ok {
			seen[path] = m
			return m
		}

		data, modTime, err := readModuleFile(path)
		if err != nil {
			seen[path] = nil
			return nil
		}
		tokens, _ := lexer.NewLexer(string(data)).Lex()
		if len(tokens) == 0 {
			seen[path] = nil
			return nil
		}
		program, _ := parser.NewParser(tokens).ParseProgram()
		if program == nil {
			seen[path] = nil
			return nil
		}
		builder := ir.NewIRBuilder()
		m := ir.NewIRModule(builder, path, false, resolve)
		seen[path] = m // visible to cyclic imports during Generate
		stack = append(stack, path)
		m.Generate(program)
		stack = stack[:len(stack)-1]
		s.modules.put(path, m, modTime, directImports[path])
		return m
	}
	return resolve
}
