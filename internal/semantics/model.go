// Package semantics holds the queryable semantic model the compiler front-end produces for
// tooling. During IR generation every identifier is resolved against the scoped symbol table and
// every expression is evaluated to a typed value — information that was previously discarded once
// the pass ended. The Model captures it, keyed by AST node, so the language server can ask "what
// symbol does this identifier resolve to?" and "what is the type of this expression?" directly,
// with scope/shadowing correctness, instead of re-deriving answers from a flat name table.
//
// It lives in its own low-level package (depending only on ast/token/zeus_value) so the ir
// package can populate it without an import cycle, and both analysis and lsp can query it.
package semantics

import (
	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// Model maps AST nodes to the semantic facts resolved for them during IR generation. Keys are AST
// node pointers, which are stable for the lifetime of a parse. All methods are nil-safe so the ir
// package can record and callers can query without guarding a possibly-absent model.
type Model struct {
	// bindings maps an identifier/reference node to the symbol it resolved to (the actual in-scope
	// symbol, so shadowing and block scope are respected).
	bindings map[ast.Node]zeus_value.Value
	// types maps an expression node to the type it evaluated to (used to resolve member access on
	// receivers that are not plain identifier chains, e.g. `foo().bar`).
	types map[ast.Node]zeus_value.ValueType
	// defs maps a symbol to its declaration identifier node. Populated only for function-local
	// variables and parameters — symbols whose every reference is provably in this one file — so a
	// recorded def marks a symbol as safe to rename with only single-file edits.
	defs map[zeus_value.Value]ast.Node
}

// NewModel returns an empty, ready-to-populate model.
func NewModel() *Model {
	return &Model{
		bindings: map[ast.Node]zeus_value.Value{},
		types:    map[ast.Node]zeus_value.ValueType{},
		defs:     map[zeus_value.Value]ast.Node{},
	}
}

// RecordBinding records that node n resolves to symbol sym. A nil model, node, or symbol is
// ignored so callers can record unconditionally.
func (m *Model) RecordBinding(n ast.Node, sym zeus_value.Value) {
	if m == nil || n == nil || sym == nil {
		return
	}
	m.bindings[n] = sym
}

// RecordType records that expression node n has type t. Nil model/node/type is ignored.
func (m *Model) RecordType(n ast.Node, t zeus_value.ValueType) {
	if m == nil || n == nil || t == nil {
		return
	}
	m.types[n] = t
}

// SymbolAt returns the symbol node n resolved to, if recorded.
func (m *Model) SymbolAt(n ast.Node) (zeus_value.Value, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m.bindings[n]
	return v, ok
}

// TypeAt returns the type expression node n evaluated to, if recorded.
func (m *Model) TypeAt(n ast.Node) (zeus_value.ValueType, bool) {
	if m == nil {
		return nil, false
	}
	t, ok := m.types[n]
	return t, ok
}

// RecordDef records a symbol's declaration identifier node. Called only for function-local
// variables and parameters. Nil model/node/symbol is ignored.
func (m *Model) RecordDef(sym zeus_value.Value, node ast.Node) {
	if m == nil || sym == nil || node == nil {
		return
	}
	m.defs[sym] = node
}

// DefNode returns a symbol's declaration identifier node if one was recorded (i.e. the symbol is a
// function-local variable or parameter). A recorded def means the symbol is safe to rename with
// single-file edits, since all of its references live in this file.
func (m *Model) DefNode(sym zeus_value.Value) (ast.Node, bool) {
	if m == nil {
		return nil, false
	}
	n, ok := m.defs[sym]
	return n, ok
}

// SymbolUses returns every identifier node bound to sym — the declaration (if recorded via
// RecordDef, which also records a binding) and all references — as the set of occurrences to
// highlight, list as references, or rewrite on rename.
func (m *Model) SymbolUses(sym zeus_value.Value) []ast.Node {
	if m == nil || sym == nil {
		return nil
	}
	var nodes []ast.Node
	for node, bound := range m.bindings {
		if bound == sym {
			nodes = append(nodes, node)
		}
	}
	return nodes
}
