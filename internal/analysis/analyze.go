// Package analysis is the compiler's front door for interactive tooling. It runs the
// front-end — lex -> parse -> IR generation -> type check — over a single in-memory document,
// accumulating diagnostics instead of stopping at the first error and, crucially, never calling
// os.Exit. The batch compiler (internal/zeus_compiler) drives the same phases for a whole
// program and terminates the process on error; the language server needs the phases to run to
// completion on incomplete code and hand back whatever was resolved. Analyze is that path.
//
// Later phases attach a queryable semantic model (position index, node->symbol bindings,
// node->type map) to Result so IDE features can query resolved information directly instead of
// re-deriving it from text.
package analysis

import (
	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/semantics"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// ModuleResolver resolves an absolute import path to its already-generated IR module. It
// returns nil when the path cannot be resolved (missing file, parse failure, or a cyclic
// import still in progress). A nil ModuleResolver disables import resolution entirely.
type ModuleResolver func(path string) *ir.IRModule

// Result is the outcome of analyzing a single document. AST and Module may be nil when
// analysis cannot proceed (source produced no tokens, or no parseable statements); Diagnostics
// is always populated with everything found across all phases that ran.
type Result struct {
	AST         *ast.ProgramNode
	Module      *ir.IRModule
	Diagnostics []*zeus_error.ZeusError
	// Model is the queryable semantic model (node->symbol bindings, node->type facts) collected
	// during IR generation. It is non-nil whenever Module is non-nil.
	Model *semantics.Model
}

// Analyze runs lex -> parse -> IR gen -> type check over source, tolerating errors at every
// phase so IDE features keep working on incomplete code, and never calling os.Exit.
//
// path is the document's filesystem path; it is used as the module path and to resolve imports
// relative to it (an empty path yields a synthetic module name and, combined with a nil
// resolver, disables import resolution). resolve loads imported modules on demand and may be
// nil.
func Analyze(path, source string, resolve ModuleResolver) *Result {
	res := &Result{}

	// Preludes load lazily on the first IRBuilder; force that now so the primordial-id watermark
	// is valid, then rewind the class/interface id counters to it. Repeated analyses (every
	// keystroke) thus reuse stable, bounded ids instead of growing the counters forever.
	ir.EnsurePreludesLoaded()
	zeus_value.ResetToPrimordialIds()

	// Lex — continue past lexer errors, there may still be usable tokens.
	tokens, lexErrs := lexer.NewLexer(source).Lex()
	res.Diagnostics = append(res.Diagnostics, lexErrs...)
	if len(tokens) == 0 {
		return res
	}

	// Parse — the parser recovers from syntax errors and returns a partial AST.
	program, parseErrs := parser.NewParser(tokens).ParseProgram()
	res.Diagnostics = append(res.Diagnostics, parseErrs...)
	if program == nil {
		return res
	}
	res.AST = program

	// IR generation — continues past semantic errors, accumulating them.
	modulePath := path
	if modulePath == "" {
		modulePath = "lsp-document"
	}
	getModule := func(string) *ir.IRModule { return nil }
	if resolve != nil {
		getModule = resolve
	}
	builder := ir.NewIRBuilder()
	module := ir.NewIRModule(builder, modulePath, true, getModule)
	model := semantics.NewModel()
	module.CollectSemantics(model)
	res.Module = module
	res.Model = model
	res.Diagnostics = append(res.Diagnostics, module.Generate(program)...)

	// Type check — fills resolved types onto IR vars; continues past type errors.
	res.Diagnostics = append(res.Diagnostics, ir.NewTypeChecker(builder, true).TypeCheck()...)

	return res
}
