package lsp

import (
	"os"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

// This file runs the compiler front-end (lex -> parse -> IR -> type check) for the language
// server, tolerating errors so IDE features still work on incomplete code, and resolves imports
// on demand.

// parseDocument parses a document and returns the IR module and any errors.
// Returns partial results even when there are errors to support IDE features. docPath is the
// filesystem path of the document, used to resolve imports relative to it; an empty docPath
// disables import resolution (used by fuzzing so no files are read).
func (s *Server) parseDocument(docPath, content string) (*ir.IRModule, []*zeus_error.ZeusError) {
	allErrors := []*zeus_error.ZeusError{}

	// Lexer phase
	lexer := lexer.NewLexer(content)
	tokens, lexerErrors := lexer.Lex()

	if len(lexerErrors) > 0 {
		allErrors = append(allErrors, lexerErrors...)
		// Continue even with lexer errors - we may have partial tokens
	}

	// If we have no tokens at all, we can't proceed
	if len(tokens) == 0 {
		return nil, allErrors
	}

	// Parser phase
	parser := parser.NewParser(tokens)
	program, parserErrors := parser.ParseProgram()

	if len(parserErrors) > 0 {
		allErrors = append(allErrors, parserErrors...)
		// Continue with partial AST - parser returns partial results
	}

	// If we have no program AST, we can't proceed
	if program == nil {
		return nil, allErrors
	}

	// IR Generation phase
	irBuilder := ir.NewIRBuilder()
	modulePath := docPath
	if modulePath == "" {
		modulePath = "lsp-document"
	}
	irModule := ir.NewIRModule(irBuilder, modulePath, true, s.makeModuleResolver(docPath))
	irErrors := irModule.Generate(program)

	if len(irErrors) > 0 {
		allErrors = append(allErrors, irErrors...)
		// Continue to type checking even if IR has some errors
	}

	// Type checking phase
	// Only run type checking if we have a valid IR builder
	if irBuilder != nil {
		typeChecker := ir.NewTypeChecker(irBuilder, true)
		typeErrors := typeChecker.TypeCheck()

		if len(typeErrors) > 0 {
			allErrors = append(allErrors, typeErrors...)
		}
	}

	return irModule, allErrors
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
