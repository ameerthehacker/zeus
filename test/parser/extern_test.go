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

// TestParseExternMethod: `@extern("sym")` parses to a body-less method carrying the runtime symbol.
func TestParseExternMethod(t *testing.T) {
	class := firstClassDecl(t, `class Console {
    @extern("zeus_Console_log") public log(message: string): void;
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

// TestParseExternFunction: a top-level `@extern("sym") function ...;` parses to a body-less
// FunctionDeclExprNode carrying the runtime symbol.
func TestParseExternFunction(t *testing.T) {
	l := lexer.NewLexer(`@extern("zeus_setTimeout") function setTimeout(delay: i32): i32;`)
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

// TestParseCExternFunction: `@extern("C", "sym")` sets IsCExtern and the raw symbol; `@extern("zeus",
// "sym")` prefixes zeus_.
func TestParseCExternFunction(t *testing.T) {
	prog := parseOK(t, `@extern("C", "open") function c_open(path: cstr, flags: cint): cint;
@extern("zeus", "malloc") function cMalloc(size: csize): cptr;`)

	fns := []*ast.FunctionDeclExprNode{}
	for _, stmt := range prog.Statements {
		if es, ok := stmt.(*ast.ExprStmtNode); ok {
			if f, ok := es.Expr.(*ast.FunctionDeclExprNode); ok {
				fns = append(fns, f)
			}
		}
	}
	if len(fns) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(fns))
	}
	if !fns[0].IsCExtern || fns[0].ExternSymbol != "open" {
		t.Errorf("open: IsCExtern=%v symbol=%q, want true/open", fns[0].IsCExtern, fns[0].ExternSymbol)
	}
	if !fns[1].IsCExtern || fns[1].ExternSymbol != "zeus_malloc" {
		t.Errorf("malloc: IsCExtern=%v symbol=%q, want true/zeus_malloc", fns[1].IsCExtern, fns[1].ExternSymbol)
	}
}

// TestParseLinkDirective: a standalone `@link(...)` parses to an AnnotationStmtNode.
func TestParseLinkDirective(t *testing.T) {
	prog := parseOK(t, `@link("sqlite3");
@link("z", "/usr/local/lib");`)

	var links []*ast.Annotation
	for _, stmt := range prog.Statements {
		if a, ok := stmt.(*ast.AnnotationStmtNode); ok {
			links = append(links, a.Annotation)
		}
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 @link directives, got %d", len(links))
	}
	if links[0].Name != "link" || len(links[0].Args) != 1 || links[0].Args[0].Value != "sqlite3" {
		t.Errorf("link[0] = %+v", links[0])
	}
	if len(links[1].Args) != 2 || links[1].Args[1].Value != "/usr/local/lib" {
		t.Errorf("link[1] = %+v", links[1])
	}
}

// TestAnnotationValidationErrors: malformed annotations are rejected, not silently accepted.
func TestAnnotationValidationErrors(t *testing.T) {
	cases := map[string]string{
		"@link with no args":            `@link();`,
		"@link with too many args":      `@link("a", "b", "c");`,
		"unknown decorator with extern": `@extern("C", "abs") @foo function bar(): cint;`,
		"unknown standalone directive":  `@nope("x");`,
		"trailing comma in args":        `@extern("C",) function b(): cint;`,
		"unknown extern ABI":            `@extern("nope", "x") function b(): cint;`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			tokens, _ := lexer.NewLexer(src).Lex()
			_, errs := parser.NewParser(tokens).ParseProgram()
			if len(errs) == 0 {
				t.Errorf("expected a parse error for %q, got none", src)
			}
		})
	}
}

// parseOK lexes+parses source, failing the test on any lexer/parser error.
func parseOK(t *testing.T, source string) *ast.ProgramNode {
	t.Helper()
	tokens, lexErrs := lexer.NewLexer(source).Lex()
	if len(lexErrs) > 0 {
		t.Fatalf("lexer errors: %v", lexErrs)
	}
	program, errs := parser.NewParser(tokens).ParseProgram()
	if len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	return program
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
