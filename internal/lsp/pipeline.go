package lsp

import (
	"os"

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
// The IR generator resolves the import path to an absolute file path before calling this, so
// the callback just reads, parses, and IR-generates that file on demand. Results (including
// failures) are cached, and a module is cached before its body is generated so an import cycle
// resolves to the in-progress module instead of recursing forever. Errors inside imported
// modules are intentionally not surfaced on the importing document.
//
// A cache is created per call, so every edit re-resolves against the current files on disk.
// When baseDocPath is empty (fuzzing), resolution is disabled and no files are read.
func (s *Server) makeModuleResolver(baseDocPath string) func(string) *ir.IRModule {
	if baseDocPath == "" {
		return func(string) *ir.IRModule { return nil }
	}
	cache := map[string]*ir.IRModule{}
	var resolve func(path string) *ir.IRModule
	resolve = func(path string) *ir.IRModule {
		if m, ok := cache[path]; ok {
			return m
		}
		// Reserve the slot before generating so a cyclic import terminates.
		cache[path] = nil

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		tokens, _ := lexer.NewLexer(string(data)).Lex()
		if len(tokens) == 0 {
			return nil
		}
		program, _ := parser.NewParser(tokens).ParseProgram()
		if program == nil {
			return nil
		}
		builder := ir.NewIRBuilder()
		m := ir.NewIRModule(builder, path, false, resolve)
		cache[path] = m
		m.Generate(program)
		return m
	}
	return resolve
}
