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
		return zeus_value.NewObjectType(class)
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
			return zeus_value.NewObjectType(class)
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
			return zeus_value.NewClassType(v)
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
			if callMethod := zeus_value.GetFunctorCallMethod(ot.Class); callMethod != nil {
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
		// Simple object instantiation: new Foo(...)
		if ident, ok := e.Callee.(*ast.IdentifierExprNode); ok {
			sym, ok := symTable.GetSymbol(ident.Name.Value)
			if !ok {
				return nil
			}
			if class := zeus_value.AsClass(sym); class != nil {
				return zeus_value.NewObjectType(class)
			}
		}
		// Array creation: new Foo[] or new u8[n][]
		if indexingExpr := ast.AsIndexingExpr(e.Callee); indexingExpr != nil {
			var baseElemType zeus_value.ValueType
			if vtNode, ok := indexingExpr.Array.(*ast.ValueTypeNode); ok {
				baseElemType = vtNode.ValueType
			} else if identNode, ok := indexingExpr.Array.(*ast.IdentifierExprNode); ok {
				baseElemType = resolveUserDefinedType(
					zeus_value.UserDefinedType{Name: identNode.Name.Value}, symTable,
				)
			}
			if baseElemType == nil || zeus_value.IsUndefinedType(baseElemType) {
				return nil
			}
			numDims := len(indexingExpr.IndexingMeta.IndexingExprs)
			arrayType := baseElemType
			for i := 0; i < numDims; i++ {
				arrayType = zeus_value.NewArrayType(arrayType, indexingExpr.GetSpan())
			}
			if at, ok := arrayType.(zeus_value.ArrayType); ok {
				if sym, ok := symTable.GetSymbol(at.String()); ok {
					if class := zeus_value.AsClass(sym); class != nil {
						return zeus_value.NewObjectType(class)
					}
				}
			}
		}
		return nil

	case *ast.ArrayLiteralExprNode:
		// Array literal: [1, 2, 3] -> i32[], [[1], []] -> i32[][]. Infer the widest element
		// type, then return the array primordial class if it is already registered (mirrors the
		// `new Foo[]` branch above); otherwise nil (tcDeclVar still recovers the var type from
		// the initializer's NEW_OBJ output).
		elemType := inferArrayLiteralElementType(e, symTable, localTypes)
		if elemType == nil || zeus_value.IsUndefinedType(elemType) {
			return nil
		}
		arrayType := zeus_value.NewArrayType(elemType, e.GetSpan())
		if sym, ok := symTable.GetSymbol(arrayType.String()); ok {
			if class := zeus_value.AsClass(sym); class != nil {
				return zeus_value.NewObjectType(class)
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

// inferArrayLiteralElementType statically determines an array literal's element type by widening
// the types of its non-empty elements. Nested array-literal elements are inferred recursively;
// empty elements (and elements whose type is unknown) contribute nothing. Numeric leaves widen to
// a common type (e.g. int + float -> float); non-numeric element types keep the first seen.
// Returns nil when no element type can be determined (e.g. a fully-empty literal with no context).
func inferArrayLiteralElementType(
	expr *ast.ArrayLiteralExprNode,
	symTable *symbol_table.SymbolTable[zeus_value.Value],
	localTypes map[string]zeus_value.ValueType,
) zeus_value.ValueType {
	var elementType zeus_value.ValueType
	for _, el := range expr.Elements {
		var elType zeus_value.ValueType
		if nested, ok := el.(*ast.ArrayLiteralExprNode); ok {
			inner := inferArrayLiteralElementType(nested, symTable, localTypes)
			if inner == nil {
				continue
			}
			elType = zeus_value.NewArrayType(inner, nested.GetSpan())
		} else {
			elType = inferExprType(el, symTable, localTypes)
		}
		if elType == nil || zeus_value.IsUndefinedType(elType) {
			continue
		}
		if elementType == nil {
			elementType = elType
			continue
		}
		// Widen numeric leaves to a common type; otherwise keep the first element type.
		if zeus_value.IsNumberType(elementType) && zeus_value.IsNumberType(elType) {
			elementType = zeus_value.GetBiggerType(elementType, elType)
		}
	}
	return elementType
}

// FunctionTypeEnv holds the types inferred for all local variables in a function
// body by the AST pre-scan (inferFunctionEnv). It is the primary source of truth for
// local variable types; the post-IR TypeInferencePass acts only as a fallback.
type FunctionTypeEnv struct {
	VarTypes map[string]zeus_value.ValueType
}

// inferFunctionEnv scans the function body AST (before any IR is emitted) and returns
// the pre-inferred type for every local variable whose type can be determined from its
// declaration. The scan processes statements in source order so that chained declarations
// (let a = 1; let counter = a + 1) resolve correctly.
//
// symTable must already contain the function's params and captured-var preamble aliases.
func (g *IRModule) inferFunctionEnv(body *ast.BlockStmtNode) *FunctionTypeEnv {
	env := &FunctionTypeEnv{VarTypes: make(map[string]zeus_value.ValueType)}
	// localTypes accumulates ALL variable types seen during the scan so that chained
	// declarations resolve correctly (e.g. let a = 1; let b = a + 1).
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
					env.VarTypes[name] = varType
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
						env.VarTypes[name] = varType
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
	return env
}
