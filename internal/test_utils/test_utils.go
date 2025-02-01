package test_utils

import (
	"fmt"
	"testing"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

func CompareZeusErrors(t *testing.T, errors, expected []*zeus_error.ZeusError) {
	if len(errors) > 0 && len(expected) == 0 {
		t.Errorf("expected errors %v but got none", errors)
	} else if len(errors) == 0 && len(expected) > 0 {
		t.Errorf("unexpected errors: %v", expected)
	} else if len(errors) != len(expected) {
		t.Errorf("expected %d errors, got %d", len(expected), len(errors))
	} else {
		for i, error := range errors {
			expectedError := expected[i]
			if !error.Equal(expectedError) {
				t.Errorf("expected error %s, got %s", expectedError, error)
			}
		}
	}
}

// CompareExprNodes compares two expression nodes for equality using downcasting
func CompareExprNodes(t *testing.T, expr, expected ast.ExprNode) {
	logExprNotEqualError := func(expr, expectation ast.ExprNode) {
		t.Errorf("expected %s, got %s", expectation.PrettyString(), expr.PrettyString())
	}

	if expr == nil && expected == nil {
		return
	} else if expr == nil && expected != nil {
		t.Errorf("expected %s, got nil", expected.PrettyString())
	} else if expr != nil && expected == nil {
		t.Errorf("expected nil, got %s", expr.PrettyString())
	}

	switch aNode := expr.(type) {
	case *ast.NumberExprNode:
		bNode, ok := expected.(*ast.NumberExprNode)
		if !ok {
			logExprNotEqualError(aNode, expected)
			return
		}
		if aNode.Value.Value != bNode.Value.Value {
			logExprNotEqualError(aNode, expected)
		}
	case *ast.IdentifierExprNode:
		bNode, ok := expected.(*ast.IdentifierExprNode)
		if !ok {
			logExprNotEqualError(aNode, expected)
			return
		}
		if aNode.Name.Value != bNode.Name.Value {
			logExprNotEqualError(aNode, expected)
		}
	case *ast.UnaryExprNode:
		bNode, ok := expected.(*ast.UnaryExprNode)
		if !ok {
			logExprNotEqualError(aNode, expected)
			return
		}
		CompareExprNodes(t, aNode.Expr, bNode.Expr)
	case *ast.BinaryExprNode:
		bNode, ok := expected.(*ast.BinaryExprNode)
		if !ok {
			logExprNotEqualError(aNode, expected)
			return
		}
		CompareExprNodes(t, aNode.Left, bNode.Left)
		CompareExprNodes(t, aNode.Right, bNode.Right)
		if aNode.Operator.Type != bNode.Operator.Type {
			logExprNotEqualError(aNode, bNode)
		}
	case *ast.GroupingExprNode:
		bNode, ok := expected.(*ast.GroupingExprNode)
		if !ok {
			logExprNotEqualError(aNode, expected)
			return
		}
		CompareExprNodes(t, aNode.Expr, bNode.Expr)
	case *ast.FunctionCallExprNode:
		bNode, ok := expected.(*ast.FunctionCallExprNode)
		if !ok {
			logExprNotEqualError(aNode, expected)
			return
		}
		CompareExprNodes(t, aNode.Callee, bNode.Callee)
		// compare the params
		if len(aNode.Params) != len(bNode.Params) {
			logExprNotEqualError(aNode, expected)
		}
		for i := range aNode.Params {
			CompareExprNodes(t, aNode.Params[i], bNode.Params[i])
		}
	case *ast.FunctionDeclExprNode:
		bNode, ok := expected.(*ast.FunctionDeclExprNode)
		if !ok {
			logExprNotEqualError(aNode, expected)
			return
		}
		if len(aNode.Params) != len(bNode.Params) {
			t.Errorf("expected %d parameters, got %d", len(bNode.Params), len(aNode.Params))
			return
		}
		if aNode.ReturnType.Type != bNode.ReturnType.Type {
			t.Errorf("expected return type %s, got %s", bNode.ReturnType.Type, aNode.ReturnType.Type)
			return
		}
		for i := range aNode.Params {
			CompareVarDecl(t, *aNode.Params[i], *bNode.Params[i])
		}
		CompareStmtNodes(t, aNode.Body, bNode.Body)
	default:
		panic(fmt.Sprintf("unsupported node type: %T", aNode))
	}

	// at last compare the expression spans
	if !expr.GetSpan().Equal(expected.GetSpan()) {
		t.Errorf("expected expressions %s , %s spans to be equal, expected: %s got: %s", expr.PrettyString(), expected.PrettyString(), expr.GetSpan(), expected.GetSpan())
		return
	}
}

func CompareVarDecl(t *testing.T, decl ast.VarDeclNode, expected ast.VarDeclNode) {
	CompareExprNodes(t, decl.Identifier, expected.Identifier)
	if decl.DeclType != expected.DeclType {
		t.Errorf("expected %s declaration type, got %s", expected.DeclType, decl.DeclType)
	}
	CompareExprNodes(t, decl.Initializer, expected.Initializer)
}

func CompareStmtNodes(t *testing.T, stmt ast.StmtNode, expected ast.StmtNode) {
	logStmtNotEqualError := func(stmt, expected ast.StmtNode) {
		t.Errorf("expected %s, got %s", expected.PrettyString(), stmt.PrettyString())
	}

	if stmt == nil && expected == nil {
		return
	} else if stmt == nil && expected != nil {
		logStmtNotEqualError(stmt, expected)
	} else if stmt != nil && expected == nil {
		logStmtNotEqualError(stmt, expected)
	}


	switch expectedStmt := expected.(type) {
	case *ast.VarDeclStmtNode:
		varDeclStmt, ok := stmt.(*ast.VarDeclStmtNode)
		if !ok {
			logStmtNotEqualError(stmt, expected)
			return
		}
		if len(varDeclStmt.Decls) != len(expectedStmt.Decls) {
			t.Errorf("expected %d declarations, got %d", len(expectedStmt.Decls), len(varDeclStmt.Decls))
			return
		}
		for i := range varDeclStmt.Decls {
			CompareVarDecl(t, varDeclStmt.Decls[i], expectedStmt.Decls[i])
		}
	case *ast.BlockStmtNode:
		blockStmt, ok := stmt.(*ast.BlockStmtNode)
		if !ok {
			logStmtNotEqualError(stmt, expected)
			return
		}
		CompareStmts(t, blockStmt.Statements, expectedStmt.Statements)
	case *ast.ReturnStmtNode:
		returnStmt, ok := stmt.(*ast.ReturnStmtNode)
		if !ok {
			logStmtNotEqualError(stmt, expected)
			return
		}
		CompareExprNodes(t, returnStmt.Expr, expectedStmt.Expr)
	case *ast.IfStmtNode:
		ifStmt, ok := stmt.(*ast.IfStmtNode)
		if !ok {
			logStmtNotEqualError(stmt, expected)
			return
		}
		CompareExprNodes(t, ifStmt.Condition, expectedStmt.Condition)
		CompareStmtNodes(t, ifStmt.ThenStmt, expectedStmt.ThenStmt)
		if expectedStmt.ElseStmt != nil {
			CompareStmtNodes(t, ifStmt.ElseStmt, expectedStmt.ElseStmt)
		}
	default:
		t.Errorf("unknown statement type: %s", stmt.PrettyString())
	}

	if !stmt.GetSpan().Equal(expected.GetSpan()) {
		t.Errorf("expected statements %s , %s spans to be equal, expected: %s got: %s", stmt.PrettyString(), expected.PrettyString(), stmt.GetSpan(), expected.GetSpan())
		return
	}
}

func CompareStmts(t *testing.T, stmts, expectedStmts []ast.StmtNode) {
	if len(stmts) != len(expectedStmts) {
		t.Errorf("expected %d statements, got %d", len(expectedStmts), len(stmts))
		return
	}
	for i := range stmts {
		CompareStmtNodes(t, stmts[i], expectedStmts[i])
	}
}

func AssertEqual[T comparable](t *testing.T, a, b T) {
	if a != b {
		t.Errorf("expected %v, got %v", b, a)
	}
}
