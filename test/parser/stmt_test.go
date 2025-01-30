package parser_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/error"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/test_utils"
	"github.com/ameerthehacker/zeus/internal/token"
)

func TestParseVarDeclStmt(t *testing.T) {
	tests := []struct {
		input    string
		expected *ast.VarDeclStmtNode
		errors   []*error.ZeusError
	}{
		{
			input: "let x: i8 = 5, y: f32 = 1.5;",
			expected: &ast.VarDeclStmtNode{
				Decls: []ast.VarDeclNode{
					{
						Identifier: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 5},
							End:   token.Position{Line: 1, Column: 5},
						}}},
						DataType:  &token.Token{Type: token.TokenTypeInt8, Value: "i8", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 8},
							End:   token.Position{Line: 1, Column: 9},
						}},
						DeclType:  ast.VarDeclTypeLet,
						Initializer: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "5", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 13},
							End:   token.Position{Line: 1, Column: 13},
						}}},
					},
					{
						Identifier: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "y", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 16},
							End:   token.Position{Line: 1, Column: 16},
						}}},
						DataType:  &token.Token{Type: token.TokenTypeFloat32, Value: "f32", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 19},
							End:   token.Position{Line: 1, Column: 22},
						}},
						DeclType:  ast.VarDeclTypeLet,
						Initializer: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "1.5", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 25},
							End:   token.Position{Line: 1, Column: 27},
						}}},
					},
				},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 27},
				},
			},
			errors: []*error.ZeusError{},
		},
		{
			input: "const x: i8 = 5;",
			errors: []*error.ZeusError{},
			expected: &ast.VarDeclStmtNode{
				Decls: []ast.VarDeclNode{
					{
						Identifier: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 7},
							End:   token.Position{Line: 1, Column: 7},
						}}},
						DataType:  &token.Token{Type: token.TokenTypeInt8, Value: "i8", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 10},
							End:   token.Position{Line: 1, Column: 11},
						}},
						DeclType:  ast.VarDeclTypeConst,
						Initializer: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "5", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 15},
							End:   token.Position{Line: 1, Column: 15},
						}}},
					},
				},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 15},
				},
			},
		},
		{
			input: "const x: i8;",
			errors: []*error.ZeusError{},
			expected: &ast.VarDeclStmtNode{
				Decls: []ast.VarDeclNode{
					{
						Identifier: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 7},
							End:   token.Position{Line: 1, Column: 7},
						}}},
						DataType:  &token.Token{Type: token.TokenTypeInt8, Value: "i8", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 10},
							End:   token.Position{Line: 1, Column: 11},
						}},
						DeclType:  ast.VarDeclTypeConst,
					},
				},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 11},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			l := lexer.NewLexer(test.input)
			tokens, _ := l.Lex()
			p := parser.NewParser(tokens)
			program, errors := p.ParseProgram()

			test_utils.CompareZeusErrors(t, errors, test.errors)
			test_utils.CompareStmts(t, program.Statements, []ast.StmtNode{test.expected})
		})
	}
}
