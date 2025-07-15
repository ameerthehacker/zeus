package test_utils

import (
	"fmt"
	"testing"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

func CompareZeusErrors(t *testing.T, errors, expected []*zeus_error.ZeusError) {
	if len(errors) > 0 && len(expected) == 0 {
		t.Errorf("expected errors %v, but found none", errors)
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
	case *ast.BooleanExprNode:
		bNode, ok := expected.(*ast.BooleanExprNode)
		if !ok {
			logExprNotEqualError(aNode, expected)
			return
		}
		if aNode.Value.Value != bNode.Value.Value {
			logExprNotEqualError(aNode, expected)
		}
	case *ast.ClassDeclExprNode:
		bNode, ok := expected.(*ast.ClassDeclExprNode)
		if !ok {
			logExprNotEqualError(aNode, expected)
			return
		}
		CompareExprNodes(t, aNode.Name, bNode.Name)
		if len(aNode.Properties) != len(bNode.Properties) {
			t.Errorf("expected %d properties, got %d", len(bNode.Properties), len(aNode.Properties))
			return
		}
		for i := range aNode.Properties {
			CompareExprNodes(t, aNode.Properties[i].Name, bNode.Properties[i].Name)
			if aNode.Properties[i].ValueType.Type != bNode.Properties[i].ValueType.Type {
				t.Errorf("expected property type %s, got %s", bNode.Properties[i].ValueType.Type, aNode.Properties[i].ValueType.Type)
			}
		}
		if len(aNode.Methods) != len(bNode.Methods) {
			t.Errorf("expected %d methods, got %d", len(bNode.Methods), len(aNode.Methods))
			return
		}
		for i := range aNode.Methods {
			CompareExprNodes(t, aNode.Methods[i].Name, bNode.Methods[i].Name)
		}
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
	case *ast.WhileStmtNode:
		whileStmt, ok := stmt.(*ast.WhileStmtNode)
		if !ok {
			logStmtNotEqualError(stmt, expected)
			return
		}
		CompareExprNodes(t, whileStmt.Condition, expectedStmt.Condition)
		CompareStmtNodes(t, whileStmt.Body, expectedStmt.Body)
	case *ast.ImportStmtNode:
		importStmt, ok := stmt.(*ast.ImportStmtNode)
		if !ok {
			logStmtNotEqualError(stmt, expected)
			return
		}
		if len(importStmt.Imports) != len(expectedStmt.Imports) {
			t.Errorf("expected %d imports, got %d", len(expectedStmt.Imports), len(importStmt.Imports))
			return
		}
		for i := range importStmt.Imports {
			CompareExprNodes(t, importStmt.Imports[i], expectedStmt.Imports[i])
		}
		if importStmt.Source.Value != expectedStmt.Source.Value {
			t.Errorf("expected source %s, got %s", expectedStmt.Source.Value, importStmt.Source.Value)
		}
	case *ast.ExportStmtNode:
		exportStmt, ok := stmt.(*ast.ExportStmtNode)
		if !ok {
			logStmtNotEqualError(stmt, expected)
			return
		}
		CompareExprNodes(t, exportStmt.Expr, expectedStmt.Expr)
	case *ast.ExprStmtNode:
		exprStmt, ok := stmt.(*ast.ExprStmtNode)
		if !ok {
			logStmtNotEqualError(stmt, expected)
			return
		}
		CompareExprNodes(t, exprStmt.Expr, expectedStmt.Expr)
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
