package ast_test

import (
	"strings"
	"testing"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/token"
)

func parseProgram(t *testing.T, src string) *ast.ProgramNode {
	t.Helper()
	tokens, lexErrs := lexer.NewLexer(src).Lex()
	if len(lexErrs) > 0 {
		t.Fatalf("unexpected lex errors: %v", lexErrs)
	}
	program, parseErrs := parser.NewParser(tokens).ParseProgram()
	if len(parseErrs) > 0 {
		t.Fatalf("unexpected parse errors: %v", parseErrs)
	}
	return program
}

// posOf returns the 1-based line/column of the first character of the nth (1-based) occurrence
// of needle in src. Assumes ASCII (adequate for these fixtures).
func posOf(t *testing.T, src, needle string, occurrence int) token.Position {
	t.Helper()
	idx := -1
	for i := 0; i < occurrence; i++ {
		rel := strings.Index(src[idx+1:], needle)
		if rel < 0 {
			t.Fatalf("occurrence %d of %q not found", occurrence, needle)
		}
		idx = idx + 1 + rel
	}
	line, col := 1, 1
	for i := 0; i < idx; i++ {
		if src[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return token.Position{Line: line, Column: col}
}

func identAt(t *testing.T, program *ast.ProgramNode, pos token.Position) *ast.IdentifierExprNode {
	t.Helper()
	node := ast.InnermostAt(program, pos)
	if node == nil {
		t.Fatalf("no node at %v", pos)
	}
	id, ok := node.(*ast.IdentifierExprNode)
	if !ok {
		t.Fatalf("expected IdentifierExprNode at %v, got %T", pos, node)
	}
	return id
}

const walkFixture = `const Point = class {
  x: i32
  getX(): i32 {
    return this.x
  }
}
let p = new Point()
let v = p.getX()`

func TestNodeAtResolvesIdentifiers(t *testing.T) {
	program := parseProgram(t, walkFixture)

	// The class name in its declaration.
	if got := identAt(t, program, posOf(t, walkFixture, "Point", 1)).Name.Value; got != "Point" {
		t.Errorf("class-name position: got %q, want %q", got, "Point")
	}

	// The property `x` inside `this.x`, deep in a method body.
	xNode := identAt(t, program, posOf(t, walkFixture, "x", 2))
	if xNode.Name.Value != "x" {
		t.Errorf("member position: got %q, want %q", xNode.Name.Value, "x")
	}

	// The method name in the member access `p.getX()` — must resolve to the Property identifier,
	// which is what hover/definition need, not the whole call.
	getXNode := identAt(t, program, posOf(t, walkFixture, "getX", 2))
	if getXNode.Name.Value != "getX" {
		t.Errorf("call-member position: got %q, want %q", getXNode.Name.Value, "getX")
	}
}

func TestNodeAtPathIsInnermostFirst(t *testing.T) {
	program := parseProgram(t, walkFixture)
	path := ast.NodeAt(program, posOf(t, walkFixture, "x", 2))
	if len(path) < 2 {
		t.Fatalf("expected a multi-node path, got %d", len(path))
	}
	if _, ok := path[0].(*ast.IdentifierExprNode); !ok {
		t.Errorf("innermost node should be an identifier, got %T", path[0])
	}
	// Somewhere above the identifier there must be the member-access node.
	foundMember := false
	for _, n := range path {
		if _, ok := n.(*ast.ObjectPropertyAccessExprNode); ok {
			foundMember = true
		}
	}
	if !foundMember {
		t.Errorf("expected an ObjectPropertyAccessExprNode ancestor in path %v", path)
	}
}

func TestNodeAtOutsideAnyStatement(t *testing.T) {
	program := parseProgram(t, walkFixture)
	// A line past the end of the program is inside no statement. (A column past the end of an
	// interior line is NOT outside — a multi-line span linearly contains it — so we go past the
	// last line entirely.)
	if node := ast.InnermostAt(program, token.Position{Line: 100, Column: 1}); node != nil {
		t.Errorf("expected nil for out-of-range position, got %T", node)
	}
}
