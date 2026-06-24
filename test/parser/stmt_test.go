package parser_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/test_utils"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

func TestParserStmt(t *testing.T) {
	tests := []struct {
		input    string
		expected ast.StmtNode
		errors   []*zeus_error.ZeusError
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
						ValueType: &ast.ValueTypeNode{
							ValueType: zeus_value.IntType{Signed: true, Size: zeus_value.I8},
							Span: &token.Span{
								Start: token.Position{Line: 1, Column: 8},
								End:   token.Position{Line: 1, Column: 9},
							},
						},
						DeclType: ast.VarDeclTypeLet,
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
						ValueType: &ast.ValueTypeNode{
							ValueType: zeus_value.FloatType{Size: zeus_value.F32},
							Span: &token.Span{
								Start: token.Position{Line: 1, Column: 19},
								End:   token.Position{Line: 1, Column: 22},
							},
						},
						DeclType: ast.VarDeclTypeLet,
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
			errors: []*zeus_error.ZeusError{},
		},
		{
			input:  "const x: i8 = 5;",
			errors: []*zeus_error.ZeusError{},
			expected: &ast.VarDeclStmtNode{
				Decls: []ast.VarDeclNode{
					{
						Identifier: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 7},
							End:   token.Position{Line: 1, Column: 7},
						}}},
						ValueType: &ast.ValueTypeNode{
							ValueType: zeus_value.IntType{Signed: true, Size: zeus_value.I8},
							Span: &token.Span{
								Start: token.Position{Line: 1, Column: 10},
								End:   token.Position{Line: 1, Column: 11},
							},
						},
						DeclType: ast.VarDeclTypeConst,
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
			input:  "const x: i8;",
			errors: []*zeus_error.ZeusError{},
			expected: &ast.VarDeclStmtNode{
				Decls: []ast.VarDeclNode{
					{
						Identifier: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 7},
							End:   token.Position{Line: 1, Column: 7},
						}}},
						ValueType: &ast.ValueTypeNode{
							ValueType: zeus_value.IntType{Signed: true, Size: zeus_value.I8},
							Span: &token.Span{
								Start: token.Position{Line: 1, Column: 10},
								End:   token.Position{Line: 1, Column: 11},
							},
						},
						DeclType: ast.VarDeclTypeConst,
					},
				},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 11},
				},
			},
		},
		{
			input: "let;",
			errors: []*zeus_error.ZeusError{
				{
					Message: "expected at least one variable declaration",
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 4},
						End:   token.Position{Line: 1, Column: 4},
					},
					Severity: zeus_error.ErrorSeverityError,
				},
			},
		},
		{
			input: "let x = 1;",
			errors: []*zeus_error.ZeusError{
				{
					Message: "expected : after identifier in variable declaration, but found =",
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 7},
						End:   token.Position{Line: 1, Column: 7},
					},
					Severity: zeus_error.ErrorSeverityError,
				},
			},
		},
		{
			input: "let x: = 1;",
			errors: []*zeus_error.ZeusError{
				{
					Message: "expected data type in variable declaration, but found =",
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 8},
						End:   token.Position{Line: 1, Column: 8},
					},
					Severity: zeus_error.ErrorSeverityError,
				},
			},
		},
		{
			input: "let x:i8 =;",
			errors: []*zeus_error.ZeusError{
				{
					Message: "expected expression for variable initializer, but found ;",
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 11},
						End:   token.Position{Line: 1, Column: 11},
					},
					Severity: zeus_error.ErrorSeverityError,
				},
			},
		},
		{
			input: "return;",
			expected: &ast.ReturnStmtNode{
				Expr: nil,
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 6},
				},
			},
		},
		{
			input: "if (x == 1) { return 1; } else { return 2; }",
			expected: &ast.IfStmtNode{
				Condition: &ast.BinaryExprNode{
					Left: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 5},
						End:   token.Position{Line: 1, Column: 5},
					}}},
					Right: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "1", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 10},
						End:   token.Position{Line: 1, Column: 10},
					}}},
					Operator: &token.Token{Type: token.TokenTypeEqualEqual, Value: "==", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 7},
						End:   token.Position{Line: 1, Column: 8},
					}},
				},
				ThenStmt: &ast.BlockStmtNode{
					Statements: []ast.StmtNode{
						&ast.ReturnStmtNode{
							Expr: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "1", Span: &token.Span{
								Start: token.Position{Line: 1, Column: 22},
								End:   token.Position{Line: 1, Column: 22},
							}}},
							Span: &token.Span{
								Start: token.Position{Line: 1, Column: 15},
								End:   token.Position{Line: 1, Column: 22},
							},
						},
					},
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 13},
						End:   token.Position{Line: 1, Column: 25},
					},
				},
				ElseStmt: &ast.BlockStmtNode{
					Statements: []ast.StmtNode{
						&ast.ReturnStmtNode{
							Expr: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "2", Span: &token.Span{
								Start: token.Position{Line: 1, Column: 41},
								End:   token.Position{Line: 1, Column: 41},
							}}},
							Span: &token.Span{
								Start: token.Position{Line: 1, Column: 34},
								End:   token.Position{Line: 1, Column: 41},
							},
						},
					},
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 32},
						End:   token.Position{Line: 1, Column: 44},
					},
				},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 44},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "if (x == 1) { return 1; }",
			expected: &ast.IfStmtNode{
				Condition: &ast.BinaryExprNode{
					Left: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 5},
						End:   token.Position{Line: 1, Column: 5},
					}}},
					Right: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "1", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 10},
						End:   token.Position{Line: 1, Column: 10},
					}}},
					Operator: &token.Token{Type: token.TokenTypeEqualEqual, Value: "==", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 7},
						End:   token.Position{Line: 1, Column: 8},
					}},
				},
				ThenStmt: &ast.BlockStmtNode{
					Statements: []ast.StmtNode{
						&ast.ReturnStmtNode{
							Expr: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "1", Span: &token.Span{
								Start: token.Position{Line: 1, Column: 22},
								End:   token.Position{Line: 1, Column: 22},
							}}},
							Span: &token.Span{
								Start: token.Position{Line: 1, Column: 15},
								End:   token.Position{Line: 1, Column: 22},
							},
						},
					},
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 13},
						End:   token.Position{Line: 1, Column: 25},
					},
				},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 25},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "while x > 0 { x = x - 1; }",
			expected: &ast.WhileStmtNode{
				Condition: &ast.BinaryExprNode{
					Left: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 7},
						End:   token.Position{Line: 1, Column: 7},
					}}},
					Right: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "0", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 11},
						End:   token.Position{Line: 1, Column: 11},
					}}},
					Operator: &token.Token{Type: token.TokenTypeGreaterThan, Value: ">", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 9},
						End:   token.Position{Line: 1, Column: 9},
					}},
				},
				Body: &ast.BlockStmtNode{
					Statements: []ast.StmtNode{
						&ast.ExprStmtNode{
							Expr: &ast.BinaryExprNode{
								Left: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
									Start: token.Position{Line: 1, Column: 15},
									End:   token.Position{Line: 1, Column: 15},
								}}},
								Right: &ast.BinaryExprNode{
									Left: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
										Start: token.Position{Line: 1, Column: 19},
										End:   token.Position{Line: 1, Column: 19},
									}}},
									Right: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "1", Span: &token.Span{
										Start: token.Position{Line: 1, Column: 23},
										End:   token.Position{Line: 1, Column: 23},
									}}},
									Operator: &token.Token{Type: token.TokenTypeMinus, Value: "-", Span: &token.Span{
										Start: token.Position{Line: 1, Column: 21},
										End:   token.Position{Line: 1, Column: 21},
									}},
								},
								Operator: &token.Token{Type: token.TokenTypeEqual, Value: "=", Span: &token.Span{
									Start: token.Position{Line: 1, Column: 17},
									End:   token.Position{Line: 1, Column: 17},
								}},
							},
						},
					},
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 13},
						End:   token.Position{Line: 1, Column: 26},
					},
				},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 26},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "while true return 42;",
			expected: &ast.WhileStmtNode{
				Condition: &ast.BooleanExprNode{Value: &token.Token{Type: token.TokenTypeTrue, Span: &token.Span{
					Start: token.Position{Line: 1, Column: 7},
					End:   token.Position{Line: 1, Column: 10},
				}}},
				Body: &ast.ReturnStmtNode{
					Expr: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "42", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 19},
						End:   token.Position{Line: 1, Column: 20},
					}}},
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 12},
						End:   token.Position{Line: 1, Column: 20},
					},
				},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 20},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "import { add } from \"math\";",
			expected: &ast.ImportStmtNode{
				Imports: []*ast.IdentifierExprNode{
					{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "add", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 10},
						End:   token.Position{Line: 1, Column: 12},
					}}},
				},
				Source: &token.Token{Type: token.TokenTypeString, Value: "math", Span: &token.Span{
					Start: token.Position{Line: 1, Column: 21},
					End:   token.Position{Line: 1, Column: 26},
				}},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 26},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "import { add, subtract, multiply } from \"math\";",
			expected: &ast.ImportStmtNode{
				Imports: []*ast.IdentifierExprNode{
					{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "add", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 10},
						End:   token.Position{Line: 1, Column: 12},
					}}},
					{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "subtract", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 15},
						End:   token.Position{Line: 1, Column: 22},
					}}},
					{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "multiply", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 25},
						End:   token.Position{Line: 1, Column: 32},
					}}},
				},
				Source: &token.Token{Type: token.TokenTypeString, Value: "math", Span: &token.Span{
					Start: token.Position{Line: 1, Column: 41},
					End:   token.Position{Line: 1, Column: 46},
				}},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 46},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "import { } from \"math\";",
			expected: &ast.ImportStmtNode{
				Imports: []*ast.IdentifierExprNode{},
				Source: &token.Token{Type: token.TokenTypeString, Value: "math", Span: &token.Span{
					Start: token.Position{Line: 1, Column: 17},
					End:   token.Position{Line: 1, Column: 22},
				}},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 22},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "export function add(x: i32, y: i32): i32 { return x + y; }",
			expected: &ast.ExportStmtNode{
				Expr: &ast.FunctionDeclExprNode{
					Name: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "add", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 17},
						End:   token.Position{Line: 1, Column: 19},
					}}},
					Params: []*ast.VarDeclNode{
						{
							Identifier: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
								Start: token.Position{Line: 1, Column: 21},
								End:   token.Position{Line: 1, Column: 21},
							}}},
							ValueType: &ast.ValueTypeNode{
								ValueType: zeus_value.IntType{Signed: true, Size: zeus_value.I32},
								Span: &token.Span{
									Start: token.Position{Line: 1, Column: 24},
									End:   token.Position{Line: 1, Column: 26},
								},
							},
							DeclType: ast.VarDeclTypeLet,
						},
						{
							Identifier: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "y", Span: &token.Span{
								Start: token.Position{Line: 1, Column: 29},
								End:   token.Position{Line: 1, Column: 29},
							}}},
							ValueType: &ast.ValueTypeNode{
								ValueType: zeus_value.IntType{Signed: true, Size: zeus_value.I32},
								Span: &token.Span{
									Start: token.Position{Line: 1, Column: 32},
									End:   token.Position{Line: 1, Column: 34},
								},
							},
							DeclType: ast.VarDeclTypeLet,
						},
					},
					ReturnType: &ast.ValueTypeNode{
						ValueType: zeus_value.IntType{Signed: true, Size: zeus_value.I32},
						Span: &token.Span{
							Start: token.Position{Line: 1, Column: 38},
							End:   token.Position{Line: 1, Column: 40},
						},
					},
					Body: &ast.BlockStmtNode{
						Statements: []ast.StmtNode{
							&ast.ReturnStmtNode{
								Expr: &ast.BinaryExprNode{
									Left: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
										Start: token.Position{Line: 1, Column: 51},
										End:   token.Position{Line: 1, Column: 51},
									}}},
									Right: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "y", Span: &token.Span{
										Start: token.Position{Line: 1, Column: 55},
										End:   token.Position{Line: 1, Column: 55},
									}}},
									Operator: &token.Token{Type: token.TokenTypePlus, Value: "+", Span: &token.Span{
										Start: token.Position{Line: 1, Column: 53},
										End:   token.Position{Line: 1, Column: 53},
									}},
								},
								Span: &token.Span{
									Start: token.Position{Line: 1, Column: 44},
									End:   token.Position{Line: 1, Column: 55},
								},
							},
						},
						Span: &token.Span{
							Start: token.Position{Line: 1, Column: 42},
							End:   token.Position{Line: 1, Column: 58},
						},
					},
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 8},
						End:   token.Position{Line: 1, Column: 58},
					},
				},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 58},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "export class Point { x: i32; y: i32; }",
			expected: &ast.ExportStmtNode{
				Expr: &ast.ClassDeclExprNode{
					Name: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "Point", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 14},
						End:   token.Position{Line: 1, Column: 18},
					}}},
					Properties: []*ast.ClassProperty{
						{
							Name: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
								Start: token.Position{Line: 1, Column: 22},
								End:   token.Position{Line: 1, Column: 22},
							}}},
							ValueType: &ast.ValueTypeNode{
								ValueType: zeus_value.IntType{Signed: true, Size: zeus_value.I32},
								Span: &token.Span{
									Start: token.Position{Line: 1, Column: 25},
									End:   token.Position{Line: 1, Column: 27},
								},
							},
							Span: &token.Span{
								Start: token.Position{Line: 1, Column: 22},
								End:   token.Position{Line: 1, Column: 22},
							},
						},
						{
							Name: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "y", Span: &token.Span{
								Start: token.Position{Line: 1, Column: 30},
								End:   token.Position{Line: 1, Column: 30},
							}}},
							ValueType: &ast.ValueTypeNode{
								ValueType: zeus_value.IntType{Signed: true, Size: zeus_value.I32},
								Span: &token.Span{
									Start: token.Position{Line: 1, Column: 33},
									End:   token.Position{Line: 1, Column: 35},
								},
							},
							Span: &token.Span{
								Start: token.Position{Line: 1, Column: 30},
								End:   token.Position{Line: 1, Column: 30},
							},
						},
					},
					Methods: []*ast.ClassMethod{},
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 8},
						End:   token.Position{Line: 1, Column: 38},
					},
				},
				Span: &token.Span{
					Start: token.Position{Line: 1, Column: 1},
					End:   token.Position{Line: 1, Column: 38},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "add(1, 2);",
			expected: &ast.ExprStmtNode{
				Expr: &ast.FunctionCallExprNode{
					Callee: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "add", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 1},
						End:   token.Position{Line: 1, Column: 3},
					}}},
					Params: []ast.ExprNode{
						&ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "1", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 5},
							End:   token.Position{Line: 1, Column: 5},
						}}},
						&ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "2", Span: &token.Span{
							Start: token.Position{Line: 1, Column: 8},
							End:   token.Position{Line: 1, Column: 8},
						}}},
					},
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 1},
						End:   token.Position{Line: 1, Column: 9},
					},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "x = 5;",
			expected: &ast.ExprStmtNode{
				Expr: &ast.BinaryExprNode{
					Left: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "x", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 1},
						End:   token.Position{Line: 1, Column: 1},
					}}},
					Right: &ast.NumberExprNode{Value: &token.Token{Type: token.TokenTypeNumber, Value: "5", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 5},
						End:   token.Position{Line: 1, Column: 5},
					}}},
					Operator: &token.Token{Type: token.TokenTypeEqual, Value: "=", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 3},
						End:   token.Position{Line: 1, Column: 3},
					}},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "function helper(): void { }",
			expected: &ast.ExprStmtNode{
				Expr: &ast.FunctionDeclExprNode{
					Name: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "helper", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 10},
						End:   token.Position{Line: 1, Column: 15},
					}}},
					Params: []*ast.VarDeclNode{},
					ReturnType: &ast.ValueTypeNode{
						ValueType: zeus_value.VoidType{},
						Span: &token.Span{
							Start: token.Position{Line: 1, Column: 20},
							End:   token.Position{Line: 1, Column: 23},
						},
					},
					Body: &ast.BlockStmtNode{
						Statements: []ast.StmtNode{},
						Span: &token.Span{
							Start: token.Position{Line: 1, Column: 25},
							End:   token.Position{Line: 1, Column: 27},
						},
					},
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 1},
						End:   token.Position{Line: 1, Column: 27},
					},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		{
			input: "class Helper { }",
			expected: &ast.ExprStmtNode{
				Expr: &ast.ClassDeclExprNode{
					Name: &ast.IdentifierExprNode{Name: &token.Token{Type: token.TokenTypeIdentifier, Value: "Helper", Span: &token.Span{
						Start: token.Position{Line: 1, Column: 7},
						End:   token.Position{Line: 1, Column: 12},
					}}},
					Properties: []*ast.ClassProperty{},
					Methods:    []*ast.ClassMethod{},
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 1},
						End:   token.Position{Line: 1, Column: 16},
					},
				},
			},
			errors: []*zeus_error.ZeusError{},
		},
		// Error cases
		{
			input: "import { add } from;",
			errors: []*zeus_error.ZeusError{
				{
					Message: "expected string after from, but found ;",
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 20},
						End:   token.Position{Line: 1, Column: 20},
					},
					Severity: zeus_error.ErrorSeverityError,
				},
			},
		},
		{
			input: "import { add } \"math\";",
			errors: []*zeus_error.ZeusError{
				{
					Message: "expected from after imports, but found string",
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 16},
						End:   token.Position{Line: 1, Column: 21},
					},
					Severity: zeus_error.ErrorSeverityError,
				},
			},
		},
		{
			input: "import add } from \"math\";",
			errors: []*zeus_error.ZeusError{
				{
					Message: "expected { after import, but found identifier",
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 8},
						End:   token.Position{Line: 1, Column: 10},
					},
					Severity: zeus_error.ErrorSeverityError,
				},
			},
		},
		{
			input: "import { add from \"math\";",
			errors: []*zeus_error.ZeusError{
				{
					Message: "expected identifier import, but found from",
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 14},
						End:   token.Position{Line: 1, Column: 17},
					},
					Severity: zeus_error.ErrorSeverityError,
				},
			},
		},
		{
			input: "export x;",
			errors: []*zeus_error.ZeusError{
				{
					Message: "export can only be used with function declaration",
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 8},
						End:   token.Position{Line: 1, Column: 8},
					},
					Severity: zeus_error.ErrorSeverityError,
				},
			},
		},
		{
			input: "export 5;",
			errors: []*zeus_error.ZeusError{
				{
					Message: "export can only be used with function declaration",
					Span: &token.Span{
						Start: token.Position{Line: 1, Column: 8},
						End:   token.Position{Line: 1, Column: 8},
					},
					Severity: zeus_error.ErrorSeverityError,
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
			if test.expected != nil {
				test_utils.CompareStmts(t, program.Statements, []ast.StmtNode{test.expected})
			}
		})
	}
}
