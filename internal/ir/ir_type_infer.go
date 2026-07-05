package ir

import (
	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// resolveUserDefinedType resolves a UserDefinedType{Name: "Foo"} to ObjectType{*Foo} via
// symbol table lookup. All other types pass through unchanged.
func resolveUserDefinedType(t zeus_value.ValueType, symTable *symbol_table.SymbolTable[zeus_value.Value]) zeus_value.ValueType {
	if t == nil {
		return nil
	}
	udt := zeus_value.AsUserDefinedType(t)
	if udt == nil {
		return t
	}
	sym, ok := symTable.GetSymbol(udt.Name)
	if !ok {
		return t
	}
	if class := zeus_value.AsClass(sym); class != nil {
		return zeus_value.NewObjectType(*class)
	}
	return t
}

// inferExprType infers the type of an AST expression without emitting any IR.
// Returns nil when the type cannot be determined — callers degrade gracefully.
// localTypes holds types already inferred for variables earlier in the same scan
// (enables chaining: let a = 1; let counter = a + 1).
func inferExprType(
	expr ast.ExprNode,
	symTable *symbol_table.SymbolTable[zeus_value.Value],
	localTypes map[string]zeus_value.ValueType,
) zeus_value.ValueType {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.NumberExprNode:
		if zeus_value.IsFloat(e.Value.Value) {
			return zeus_value.FloatType{Size: zeus_value.F64}
		}
		// Unsigned to match VisitNumber (ir.go) which emits Signed: false for all int literals.
		return zeus_value.IntType{Signed: false, Size: zeus_value.GetSignedIntSize(e.Value.Value)}

	case *ast.BooleanExprNode:
		return zeus_value.BoolType{}

	case *ast.NullExprNode:
		return zeus_value.NullType{}

	case *ast.CharExprNode:
		return zeus_value.IntType{Signed: false, Size: zeus_value.I8}

	case *ast.StringConstantExprNode:
		sym, ok := symTable.GetSymbol(zeus_value.ZEUS_PRIMORDIAL_STRING)
		if !ok {
			return nil
		}
		if class := zeus_value.AsClass(sym); class != nil {
			return zeus_value.NewObjectType(*class)
		}
		return nil

	case *ast.GroupingExprNode:
		return inferExprType(e.Expr, symTable, localTypes)

	case *ast.IdentifierExprNode:
		// Local types first (accumulated during body scan).
		if t, ok := localTypes[e.Name.Value]; ok && t != nil {
			return resolveUserDefinedType(t, symTable)
		}
		sym, ok := symTable.GetSymbol(e.Name.Value)
		if !ok {
			return nil
		}
		switch v := sym.(type) {
		case *zeus_value.Var:
			return resolveUserDefinedType(v.ValueType, symTable)
		case *zeus_value.RefCellVar:
			return resolveUserDefinedType(v.ValueType, symTable)
		case *zeus_value.Function:
			return zeus_value.ToFunctionType(*v)
		case *zeus_value.Class:
			return zeus_value.NewClassType(*v)
		case *zeus_value.Constant:
			return resolveUserDefinedType(v.ValueType, symTable)
		}
		return nil

	case *ast.UnaryExprNode:
		if e.Operator.Type == token.TokenTypeBang {
			return zeus_value.BoolType{}
		}
		return inferExprType(e.Expr, symTable, localTypes)

	case *ast.PostfixExprNode:
		return inferExprType(e.Expr, symTable, localTypes)

	case *ast.BinaryExprNode:
		switch e.Operator.Type {
		// Comparison and logical operators always return bool.
		case token.TokenTypeEqualEqual, token.TokenTypeBangEqual,
			token.TokenTypeLessThan, token.TokenTypeLessThanEqual,
			token.TokenTypeGreaterThan, token.TokenTypeGreaterThanEqual,
			token.TokenTypeAmpAmp, token.TokenTypePipePipe:
			return zeus_value.BoolType{}
		// Assignment operators: the result is not a useful type for var initializers.
		case token.TokenTypeEqual,
			token.TokenTypePlusEqual, token.TokenTypeMinusEqual,
			token.TokenTypeStarEqual, token.TokenTypeSlashEqual,
			token.TokenTypePercentEqual, token.TokenTypeDoubleStarEqual,
			token.TokenTypeBitwiseAndEqual, token.TokenTypeBitwiseOrEqual,
			token.TokenTypeBitwiseXorEqual,
			token.TokenTypeLeftShiftEqual, token.TokenTypeRightShiftEqual:
			return nil
		// Arithmetic and bitwise: result is the "bigger" of the two operand types.
		default:
			left := inferExprType(e.Left, symTable, localTypes)
			right := inferExprType(e.Right, symTable, localTypes)
			if left == nil || right == nil {
				return nil
			}
			if !zeus_value.IsNumberType(left) || !zeus_value.IsNumberType(right) {
				return nil
			}
			return zeus_value.GetBiggerType(left, right)
		}

	case *ast.TernaryExprNode:
		if t := inferExprType(e.Then, symTable, localTypes); t != nil {
			return t
		}
		return inferExprType(e.Else, symTable, localTypes)

	case *ast.FunctionCallExprNode:
		// Method call: callee is obj.method(...)
		if propAccess, ok := e.Callee.(*ast.ObjectPropertyAccessExprNode); ok {
			objType := inferExprType(propAccess.Object, symTable, localTypes)
			objType = resolveUserDefinedType(objType, symTable)
			ot := zeus_value.AsObjectType(objType)
			if ot == nil {
				return nil
			}
			for _, m := range ot.Class.Methods {
				if m.Method.OriginalName == propAccess.Property.Name.Value ||
					m.Method.Name == propAccess.Property.Name.Value {
					return resolveUserDefinedType(m.Method.ReturnType, symTable)
				}
			}
			return nil
		}
		// Direct or indirect call.
		calleeType := inferExprType(e.Callee, symTable, localTypes)
		if calleeType == nil {
			return nil
		}
		if ft := zeus_value.AsFunctionType(calleeType); ft != nil {
			return resolveUserDefinedType(ft.ReturnType, symTable)
		}
		if ot := zeus_value.AsObjectType(calleeType); ot != nil {
			if callMethod := zeus_value.GetFunctorCallMethod(&ot.Class); callMethod != nil {
				return resolveUserDefinedType(callMethod.ReturnType, symTable)
			}
		}
		return nil

	case *ast.IndexingExprNode:
		arrayType := inferExprType(e.Array, symTable, localTypes)
		arrayType = resolveUserDefinedType(arrayType, symTable)
		ot := zeus_value.AsObjectType(arrayType)
		if ot == nil {
			return nil
		}
		return resolveUserDefinedType(ot.Class.ArrayElementType, symTable)

	case *ast.ObjectPropertyAccessExprNode:
		objType := inferExprType(e.Object, symTable, localTypes)
		objType = resolveUserDefinedType(objType, symTable)
		ot := zeus_value.AsObjectType(objType)
		if ot == nil {
			return nil
		}
		for _, p := range ot.Class.Properties {
			if p.Property.Name == e.Property.Name.Value || p.Property.OriginalName == e.Property.Name.Value {
				return resolveUserDefinedType(p.Property.ValueType, symTable)
			}
		}
		return nil

	case *ast.NewExprNode:
		if ident, ok := e.Callee.(*ast.IdentifierExprNode); ok {
			sym, ok := symTable.GetSymbol(ident.Name.Value)
			if !ok {
				return nil
			}
			if class := zeus_value.AsClass(sym); class != nil {
				return zeus_value.NewObjectType(*class)
			}
		}
		return nil

	// FunctionDeclExprNode: the Priority-3 fallback in VisitVarDeclStmt handles this
	// correctly (emitFunctorClass sets the result's ValueType to ObjectType of the functor).
	// ClassDeclExprNode: handled by a fast-path at the top of VisitVarDeclStmt before
	// the ref-cell block, so it never reaches us.
	// StringConstantExprNode: depends on the string class object existing; not worth
	// the complexity here — fall through to nil.
	default:
		return nil
	}
}

// inferFunctionLocalTypes scans the function body AST (before any IR is emitted) and
// returns the pre-inferred type for each escaped var whose type can be determined
// from its initializer expression. The scan processes statements in source order so
// that chained declarations (let a = 1; let counter = a + 1) resolve correctly.
//
// escapedVarNames must be the result of collectEscapedVarNames for this function.
// symTable must already contain the function's params and captured-var preamble aliases.
func (g *IRModule) inferFunctionLocalTypes(
	escapedVarNames map[string]bool,
	body *ast.BlockStmtNode,
) map[string]zeus_value.ValueType {
	result := make(map[string]zeus_value.ValueType)
	// localTypes accumulates ALL variable types seen during the scan (not just escaped
	// ones) so that chained declarations resolve correctly.
	localTypes := make(map[string]zeus_value.ValueType)
	symTable := g.symbolTable()

	var scanStmt func(stmt ast.StmtNode)
	scanStmt = func(stmt ast.StmtNode) {
		if stmt == nil {
			return
		}
		switch s := stmt.(type) {
		case *ast.VarDeclStmtNode:
			for _, decl := range s.Decls {
				name := decl.Identifier.Name.Value
				var varType zeus_value.ValueType
				if decl.ValueType != nil {
					varType = resolveUserDefinedType(decl.ValueType.ValueType, symTable)
				} else if decl.Initializer != nil {
					varType = inferExprType(decl.Initializer, symTable, localTypes)
				}
				if varType != nil && !zeus_value.IsUndefinedType(varType) {
					localTypes[name] = varType
					if escapedVarNames[name] {
						result[name] = varType
					}
				}
			}

		case *ast.BlockStmtNode:
			for _, inner := range s.Statements {
				scanStmt(inner)
			}

		case *ast.IfStmtNode:
			scanStmt(s.ThenStmt)
			if s.ElseStmt != nil {
				scanStmt(s.ElseStmt)
			}

		case *ast.WhileStmtNode:
			scanStmt(s.Body)

		case *ast.ForStmtNode:
			scanStmt(s.Init)
			scanStmt(s.Body)

		case *ast.TryCatchStmtNode:
			scanStmt(s.TryBody)
			for _, clause := range s.CatchClauses {
				if clause.ErrorVar != nil && clause.ErrorType != nil {
					name := clause.ErrorVar.Name.Value
					varType := resolveUserDefinedType(clause.ErrorType.ValueType, symTable)
					if varType != nil {
						localTypes[name] = varType
						if escapedVarNames[name] {
							result[name] = varType
						}
					}
				}
				scanStmt(clause.Body)
			}
			// ExprStmt, ReturnStmt, ThrowStmt, BreakStmt, ContinueStmt, ImportStmt,
			// ExportStmt: no variable declarations — skip.
			// FunctionDeclExprNode bodies are never recursed into because function
			// expressions only appear inside ExprStmtNode or as VarDecl initializers,
			// and neither path recurses into the function body here.
		}
	}

	for _, stmt := range body.Statements {
		scanStmt(stmt)
	}
	return result
}
