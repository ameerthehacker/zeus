package parser_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
)

// firstClassDecl parses source and returns the first top-level class declaration node.
func firstClassDecl(t *testing.T, source string) *ast.ClassDeclExprNode {
	t.Helper()
	l := lexer.NewLexer(source)
	tokens, lexErrs := l.Lex()
	if len(lexErrs) > 0 {
		t.Fatalf("lexer errors: %v", lexErrs)
	}
	program, errs := parser.NewParser(tokens).ParseProgram()
	if len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	for _, stmt := range program.Statements {
		if exprStmt, ok := stmt.(*ast.ExprStmtNode); ok {
			if cls, ok := exprStmt.Expr.(*ast.ClassDeclExprNode); ok {
				return cls
			}
		}
	}
	t.Fatal("no class declaration parsed")
	return nil
}

// TestParseExternMethod: `extern("sym")` parses to a body-less method carrying the runtime symbol.
func TestParseExternMethod(t *testing.T) {
	class := firstClassDecl(t, `class Console {
    public extern("zeus_Console_log") log(message: string): void;
}`)

	if len(class.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(class.Methods))
	}
	m := class.Methods[0]
	if m.ExternSymbol != "zeus_Console_log" {
		t.Errorf("ExternSymbol: got %q, want zeus_Console_log", m.ExternSymbol)
	}
	if m.Body != nil {
		t.Error("extern method should have a nil Body")
	}
	if m.Name.Name.Value != "log" {
		t.Errorf("method name: got %q, want log", m.Name.Name.Value)
	}
	if len(m.Params) != 1 {
		t.Errorf("expected 1 param, got %d", len(m.Params))
	}
}

// TestParseExternFunction: a top-level `extern("sym") function ...;` parses to a body-less
// FunctionDeclExprNode carrying the runtime symbol.
func TestParseExternFunction(t *testing.T) {
	l := lexer.NewLexer(`extern("zeus_setTimeout") function setTimeout(delay: i32): i32;`)
	tokens, lexErrs := l.Lex()
	if len(lexErrs) > 0 {
		t.Fatalf("lexer errors: %v", lexErrs)
	}
	program, errs := parser.NewParser(tokens).ParseProgram()
	if len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}

	var fn *ast.FunctionDeclExprNode
	for _, stmt := range program.Statements {
		if es, ok := stmt.(*ast.ExprStmtNode); ok {
			if f, ok := es.Expr.(*ast.FunctionDeclExprNode); ok {
				fn = f
				break
			}
		}
	}
	if fn == nil {
		t.Fatal("no function declaration parsed")
	}
	if fn.ExternSymbol != "zeus_setTimeout" {
		t.Errorf("ExternSymbol: got %q, want zeus_setTimeout", fn.ExternSymbol)
	}
	if fn.Body != nil {
		t.Error("extern function should have a nil Body")
	}
	if fn.Name.Name.Value != "setTimeout" {
		t.Errorf("function name: got %q, want setTimeout", fn.Name.Name.Value)
	}
}

// TestParseNonExternMethodUnaffected: a normal method (with a body) still parses with no
// ExternSymbol — the extern branch must not disturb regular method parsing.
func TestParseNonExternMethodUnaffected(t *testing.T) {
	class := firstClassDecl(t, `class Foo {
    public bar(x: i32): i32 { return x; }
}`)

	if len(class.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(class.Methods))
	}
	m := class.Methods[0]
	if m.ExternSymbol != "" {
		t.Errorf("regular method should have empty ExternSymbol, got %q", m.ExternSymbol)
	}
	if m.Body == nil {
		t.Error("regular method should have a body")
	}
}
