package semantics

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

func ident(name string) *ast.IdentifierExprNode {
	return &ast.IdentifierExprNode{Name: &token.Token{Value: name}}
}

func TestModelRecordAndQuery(t *testing.T) {
	m := NewModel()
	node := ident("x")
	sym := zeus_value.NewVar("x", nil, false, nil)

	m.RecordBinding(node, sym)
	if got, ok := m.SymbolAt(node); !ok || got != sym {
		t.Fatalf("SymbolAt(node) = %v, %v; want the recorded symbol", got, ok)
	}
	// A different node (distinct pointer) must not resolve.
	if _, ok := m.SymbolAt(ident("x")); ok {
		t.Fatalf("SymbolAt should key on node identity, not name")
	}

	typ := zeus_value.ValueType(zeus_value.IntType{Size: zeus_value.I32, Signed: true})
	m.RecordType(node, typ)
	if got, ok := m.TypeAt(node); !ok || got != typ {
		t.Fatalf("TypeAt(node) = %v, %v; want the recorded type", got, ok)
	}
}

// Recording ignores nil node/symbol/type; querying an absent node misses. Also, a nil *Model is
// fully usable (all methods no-op), so the ir package can record unconditionally.
func TestModelNilSafety(t *testing.T) {
	m := NewModel()
	m.RecordBinding(nil, zeus_value.NewVar("x", nil, false, nil))
	m.RecordBinding(ident("x"), nil)
	if _, ok := m.SymbolAt(ident("missing")); ok {
		t.Fatalf("absent node should miss")
	}

	var nilModel *Model
	nilModel.RecordBinding(ident("x"), zeus_value.NewVar("x", nil, false, nil)) // must not panic
	nilModel.RecordType(ident("x"), nil)
	if _, ok := nilModel.SymbolAt(ident("x")); ok {
		t.Fatalf("nil model SymbolAt should be false")
	}
	if _, ok := nilModel.TypeAt(ident("x")); ok {
		t.Fatalf("nil model TypeAt should be false")
	}
}
