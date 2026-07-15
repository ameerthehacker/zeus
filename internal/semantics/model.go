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
}

// NewModel returns an empty, ready-to-populate model.
func NewModel() *Model {
	return &Model{
		bindings: map[ast.Node]zeus_value.Value{},
		types:    map[ast.Node]zeus_value.ValueType{},
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
