package parser_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
)

// TestClassPropertyInitializerIsParsed verifies the parser captures a class property's `= <expr>`
// default (previously discarded) onto ClassProperty.Initializer, and leaves it nil when absent.
func TestClassPropertyInitializerIsParsed(t *testing.T) {
	input := "class C { public x: i32 = 41 + 1; public static readonly N: i32 = 42; public y: i32; }"

	tokens, lexErrs := lexer.NewLexer(input).Lex()
	if len(lexErrs) > 0 {
		t.Fatalf("unexpected lex errors: %v", lexErrs)
	}
	program, parseErrs := parser.NewParser(tokens).ParseProgram()
	if len(parseErrs) > 0 {
		t.Fatalf("unexpected parse errors: %v", parseErrs)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}

	exprStmt, ok := program.Statements[0].(*ast.ExprStmtNode)
	if !ok {
		t.Fatalf("expected ExprStmtNode, got %T", program.Statements[0])
	}
	classDecl, ok := exprStmt.Expr.(*ast.ClassDeclExprNode)
	if !ok {
		t.Fatalf("expected ClassDeclExprNode, got %T", exprStmt.Expr)
	}
	if len(classDecl.Properties) != 3 {
		t.Fatalf("expected 3 properties, got %d", len(classDecl.Properties))
	}

	// x: i32 = 41 + 1  -> initializer is a binary expression
	if classDecl.Properties[0].Initializer == nil {
		t.Fatalf("property 'x' expected an initializer, got nil")
	}
	if _, ok := classDecl.Properties[0].Initializer.(*ast.BinaryExprNode); !ok {
		t.Errorf("property 'x' initializer: expected *ast.BinaryExprNode, got %T", classDecl.Properties[0].Initializer)
	}

	// static readonly N: i32 = 42 -> initializer is a number literal
	if classDecl.Properties[1].Initializer == nil {
		t.Fatalf("property 'N' expected an initializer, got nil")
	}
	if num, ok := classDecl.Properties[1].Initializer.(*ast.NumberExprNode); !ok {
		t.Errorf("property 'N' initializer: expected *ast.NumberExprNode, got %T", classDecl.Properties[1].Initializer)
	} else if num.Value.Value != "42" {
		t.Errorf("property 'N' initializer: expected value 42, got %q", num.Value.Value)
	}

	// y: i32 (no initializer)
	if classDecl.Properties[2].Initializer != nil {
		t.Errorf("property 'y' expected no initializer, got %v", classDecl.Properties[2].Initializer)
	}
}
