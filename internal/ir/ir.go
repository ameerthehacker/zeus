package ir

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/module"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

const anonymousName = "anonymous"

// TryContext holds information about the current try block being processed
type TryContext struct {
	HandlerBlock *BasicBlock // The exception handler block
	ClassIds     []int       // Class IDs of catch clauses
}

// LoopContext holds the break/continue targets for the current loop
type LoopContext struct {
	BreakTarget    *BasicBlock // merge block — where break jumps
	ContinueTarget *BasicBlock // condition block (while) or update block (for)
}

type IRModule struct {
	irBuilder        *IRBuilder
	isLValueExpr     bool
	errors           []*zeus_error.ZeusError
	modulePath       string
	exportedSymbols  map[string]zeus_value.Value
	getModule        func(modulePath string) *IRModule
	tryContextStack  []*TryContext  // Stack of try contexts for nested try blocks
	loopContextStack []*LoopContext // Stack of loop contexts for nested loops
	// usedIRNames tracks every IR-level name ever emitted in this module (functions,
	// class methods, locals) so generateUniqueName can guarantee global uniqueness.
	usedIRNames map[string]bool
	// escapedVarNames is set by emitFunction before visiting the body. It holds the
	// names of vars/params in the current function scope that escape into nested
	// closures and therefore need to be promoted to heap ref cells.
	escapedVarNames map[string]bool
	// funcTypeEnv is set by inferFunctionEnv in emitFunction and is the primary source
	// of truth for local variable types. It covers all vars (escaped and non-escaped)
	// whose type can be determined from the AST before any IR is emitted.
	// Saved/restored around nested emitFunction calls like escapedVarNames.
	funcTypeEnv *FunctionTypeEnv
	// isEntryPoint is true when this module is the program entry point.
	isEntryPoint bool
	// isInModuleScope is true while IREmitPass visits top-level statements inside
	// the synthetic #_zeus_main body. VisitVarDeclStmt uses BuildGlobalVarDecl when
	// this flag is set. emitFunction saves/restores it so nested function bodies use
	// ordinary local vars.
	isInModuleScope bool
}

func NewIRModule(ir_builder *IRBuilder, modulePath string, isEntryPoint bool, getIRModule func(modulePath string) *IRModule) *IRModule {
	return &IRModule{
		irBuilder:       ir_builder,
		isLValueExpr:    false,
		modulePath:      modulePath,
		exportedSymbols: map[string]zeus_value.Value{},
		getModule:       getIRModule,
		usedIRNames:     map[string]bool{},
		isEntryPoint:    isEntryPoint,
	}
}

// generateUniqueName returns a name not already used as an IR symbol in this module.
// It checks both the IRBuilder global registry (primordials, top-level functions/classes)
// and usedIRNames (class methods, local vars). The returned name is marked as used.
func (g *IRModule) generateUniqueName(name string) string {
	unique := name
	for count := 1; ; count++ {
		_, inGlobal := g.irBuilder.symbolTable.GetSymbol(unique)
		if !inGlobal && !g.usedIRNames[unique] {
			break
		}
		unique = name + strconv.Itoa(count)
	}
	g.usedIRNames[unique] = true
	return unique
}

// symbolTable returns the IRBuilder's symbol table (single source of truth)
func (g *IRModule) symbolTable() *symbol_table.SymbolTable[zeus_value.Value] {
	return g.irBuilder.symbolTable
}

func (g *IRModule) pushError(err *zeus_error.ZeusError) {
	g.errors = append(g.errors, err)
}

func (g *IRModule) pushLoopContext(brk, cont *BasicBlock) {
	g.loopContextStack = append(g.loopContextStack, &LoopContext{BreakTarget: brk, ContinueTarget: cont})
}

func (g *IRModule) popLoopContext() {
	g.loopContextStack = g.loopContextStack[:len(g.loopContextStack)-1]
}

func (g *IRModule) currentLoopContext() *LoopContext {
	if len(g.loopContextStack) == 0 {
		return nil
	}
	return g.loopContextStack[len(g.loopContextStack)-1]
}

func (g *IRModule) GetExportedSymbol(symbolName string) (zeus_value.Value, bool) {
	value, ok := g.exportedSymbols[symbolName]

	if !ok {
		return nil, false
	}

	return value, true
}

// GetAllSymbols returns all symbols from the symbol table for code completion
func (g *IRModule) GetAllSymbols() map[string]zeus_value.Value {
	symbols := make(map[string]zeus_value.Value)

	// Use the builder's symbol table to get all symbols
	g.irBuilder.symbolTable.Walk(func(name string, value zeus_value.Value) {
		symbols[name] = value
	})

	return symbols
}

func (g *IRModule) Generate(program *ast.ProgramNode) []*zeus_error.ZeusError {
	passes := []IRGenPass{
		NewDeclCheckPass(),
		NewIREmitPass(),
	}
	for _, p := range passes {
		if errs := p.Run(g, program); len(errs) > 0 {
			return errs
		}
	}
	return g.errors
}

func (g *IRModule) VisitBlockStmt(stmt *ast.BlockStmtNode) {
	g.symbolTable().EnterScope()
	for _, stmt := range stmt.Statements {
		stmt.Accept(g)
	}
	g.symbolTable().ExitScope()
}

func (g *IRModule) VisitVarDeclStmt(stmt *ast.VarDeclStmtNode) {
	for _, decl := range stmt.Decls {
		// const X = class {} / const X = class MyClass {} → never emit a var decl
		if classExpr, ok := decl.Initializer.(*ast.ClassDeclExprNode); ok {
			class := classExpr.Accept(g)
			// Named class (const X = class MyClass {}): also declare X as an alias
			if classExpr.Name.Name.Value != decl.Identifier.Name.Value {
				g.symbolTable().DeclareSymbol(decl.Identifier.Name.Value, class)
			}
			continue
		}

		varName := decl.Identifier.Name.Value

		var initializer zeus_value.Value
		isConst := false

		// Escaped variable: promote to a heap ref cell so nested closures can share
		// the same mutable binding.
		//
		// Type resolution priority:
		//   1. Explicit annotation
		//   2. AST pre-inferred type from inferFunctionEnv
		//   3. Eager IR result type (only available after the initializer is evaluated)
		//
		// When the type is known from sources 1 or 2, the ref-cell is pre-declared
		// before the initializer is evaluated so that closures in the RHS can capture
		// the variable by name:
		//   let timer = setInterval(() => { clearInterval(timer) }, 1000)
		if g.escapedVarNames[varName] {
			refCellType := zeus_value.ValueType(zeus_value.UndefinedType{Span: decl.GetSpan()})
			if decl.ValueType != nil {
				refCellType = g.resolveTypeForIRGen(decl.ValueType.ValueType, false)
			}
			if zeus_value.IsUndefinedType(refCellType) {
				if g.funcTypeEnv != nil {
					if preInferred, ok := g.funcTypeEnv.VarTypes[varName]; ok &&
						preInferred != nil && !zeus_value.IsUndefinedType(preInferred) {
						refCellType = preInferred
					}
				}
			}

			// Pre-declare when type is known from sources 1 or 2.
			var cellObj zeus_value.Value
			if !zeus_value.IsUndefinedType(refCellType) {
				cellObj = g.allocRefCell(refCellType, nil, decl.GetSpan())
				g.symbolTable().DeclareSymbol(varName, &zeus_value.RefCellVar{
					OriginalName: varName,
					ValueType:    refCellType,
					Cell:         cellObj,
					Span:         decl.GetSpan(),
				})
			}

			// Evaluate the initializer (closures compiled here can see varName if pre-declared).
			if decl.Initializer != nil {
				initializer = decl.Initializer.Accept(g)
			}

			// Source 3: infer from IR result when sources 1 and 2 both failed.
			if zeus_value.IsUndefinedType(refCellType) && initializer != nil {
				if inferred := zeus_value.GetValueType(initializer); inferred != nil && !zeus_value.IsUndefinedType(inferred) {
					refCellType = inferred
				}
			}

			if refCellType != nil && !zeus_value.IsUndefinedType(refCellType) {
				if cellObj == nil {
					// Type resolved only after init: create cell now with the initializer value.
					cellObj = g.allocRefCell(refCellType, initializer, decl.GetSpan())
					g.symbolTable().DeclareSymbol(varName, &zeus_value.RefCellVar{
						OriginalName: varName,
						ValueType:    refCellType,
						Cell:         cellObj,
						Span:         decl.GetSpan(),
					})
				} else if initializer != nil {
					// Cell was pre-declared: store the initializer value into cell.value now.
					propPtr := g.irBuilder.BuildObjectPropertyAccess(cellObj, zeus_value.ZEUS_REF_CELL_VALUE_PROPERTY, true, decl.GetSpan())
					zeus_value.AsVar(propPtr).ValueType = refCellType
					g.irBuilder.BuildStore(zeus_value.AsVar(propPtr), initializer, decl.GetSpan())
				}
				continue
			}
		}

		if decl.Initializer != nil {
			initializer = decl.Initializer.Accept(g)
		}

		if decl.DeclType == ast.VarDeclTypeConst {
			isConst = true
		}

		var valueType zeus_value.ValueType

		if decl.ValueType != nil {
			valueType = g.resolveTypeForIRGen(decl.ValueType.ValueType, false)
		} else if g.funcTypeEnv != nil {
			if preInferred, ok := g.funcTypeEnv.VarTypes[varName]; ok &&
				preInferred != nil && !zeus_value.IsUndefinedType(preInferred) {
				valueType = preInferred
			}
		}
		if valueType == nil || zeus_value.IsUndefinedType(valueType) {
			valueType = zeus_value.UndefinedType{Span: decl.GetSpan()}
		}

		// When initializer is a *Function (global function reference):
		// - No type annotation: declare as alias so calls go through CALL_FUNC (no functor)
		// - FunctionType annotation: emit CAST so CastLoweringPass wraps it in a functor
		if asFunc := zeus_value.AsFunction(initializer); asFunc != nil {
			if decl.ValueType == nil {
				g.symbolTable().DeclareSymbol(decl.Identifier.Name.Value, asFunc)
				continue
			}
			if zeus_value.IsFunctionType(valueType) {
				initializer = g.irBuilder.BuildCast(initializer, valueType, decl.GetSpan())
			}
		}

		// When assigning a functor Object to an explicitly-typed FunctionType variable, coerce
		// the variable's type to the actual ObjectType so FunctorCallLoweringPass handles calls
		// on it naturally without needing a fat-pointer or wrapper (MLIR unrealized_conversion_cast).
		if initializer != nil && zeus_value.IsFunctionType(valueType) {
			if initType := zeus_value.GetValueType(initializer); zeus_value.IsObjectType(initType) {
				initializer = g.irBuilder.BuildCoerce(initializer, valueType, decl.GetSpan())
				valueType = initType
			}
		}

		// Enforce: every variable declaration must have a type annotation or an initializer.
		if decl.ValueType == nil && decl.Initializer == nil {
			g.pushError(zeus_error.NewZeusError(
				zeus_error.ErrorSeverityError,
				fmt.Sprintf("variable '%s' must have a type annotation or an initializer", varName),
				decl.GetSpan(),
			))
		}

		varDecl := NewVarDecl(varName, valueType, isConst, initializer, decl.GetSpan())
		var variable *zeus_value.Var
		if g.isInModuleScope {
			variable = g.irBuilder.BuildGlobalVarDecl(varDecl)
		} else {
			variable = g.irBuilder.BuildVarDecl(varDecl)
		}

		g.symbolTable().DeclareSymbol(varName, variable)
	}
}

func (g *IRModule) VisitExprStmt(stmt *ast.ExprStmtNode) {
	stmt.Expr.Accept(g)
}

func (g *IRModule) VisitReturnStmt(stmt *ast.ReturnStmtNode) {
	if stmt.Expr == nil {
		g.irBuilder.BuildReturn(nil, stmt.GetSpan())
	} else {
		g.irBuilder.BuildReturn(stmt.Expr.Accept(g), stmt.GetSpan())
	}
}

func (g *IRModule) VisitIfStmt(stmt *ast.IfStmtNode) {
	// create the condition
	condition := stmt.Condition.Accept(g)
	// create the required blocks
	then_block := g.irBuilder.BuildSuccessorBlock()
	else_block := g.irBuilder.BuildSuccessorBlock()
	merge_block := g.irBuilder.BuildSuccessorBlock()

	// build jump to if block
	g.irBuilder.BuildCondJmp(then_block, else_block, condition, stmt.GetSpan())

	// generate the then block
	g.irBuilder.SetInsertionBlock(then_block)
	stmt.ThenStmt.Accept(g)
	// jump to the merge block
	g.irBuilder.BuildJmp(merge_block, nil)

	g.irBuilder.SetInsertionBlock(else_block)
	// generate the else block
	if stmt.ElseStmt != nil {
		stmt.ElseStmt.Accept(g)
	}
	// jump to the merge block
	g.irBuilder.BuildJmp(merge_block, nil)

	g.irBuilder.SetInsertionBlock(merge_block)
}

func (g *IRModule) VisitWhileStmt(stmt *ast.WhileStmtNode) {

	// create the required blocks
	condition_block := g.irBuilder.BuildSuccessorBlock()
	body_block := g.irBuilder.BuildSuccessorBlock()
	merge_block := g.irBuilder.BuildSuccessorBlock()
	g.irBuilder.BuildJmp(condition_block, nil)

	// build condition block
	g.irBuilder.SetInsertionBlock(condition_block)
	// create the condition
	condition := stmt.Condition.Accept(g)
	g.irBuilder.BuildCondJmp(body_block, merge_block, condition, stmt.Condition.GetSpan())

	// generate the body block
	g.irBuilder.SetInsertionBlock(body_block)
	g.pushLoopContext(merge_block, condition_block)
	stmt.Body.Accept(g)
	g.popLoopContext()
	g.irBuilder.BuildJmp(condition_block, nil)

	// generate the merge block
	g.irBuilder.SetInsertionBlock(merge_block)
}

func (g *IRModule) VisitForStmt(stmt *ast.ForStmtNode) {
	// For loops create their own scope for the init variable
	g.symbolTable().EnterScope()

	// Execute init statement if present
	if stmt.Init != nil {
		stmt.Init.Accept(g)
	}

	// Create the required blocks
	condition_block := g.irBuilder.BuildSuccessorBlock()
	body_block := g.irBuilder.BuildSuccessorBlock()
	update_block := g.irBuilder.BuildSuccessorBlock()
	merge_block := g.irBuilder.BuildSuccessorBlock()
	g.irBuilder.BuildJmp(condition_block, nil)

	// Build condition block
	g.irBuilder.SetInsertionBlock(condition_block)
	if stmt.Condition != nil {
		condition := stmt.Condition.Accept(g)
		g.irBuilder.BuildCondJmp(body_block, merge_block, condition, stmt.Condition.GetSpan())
	} else {
		// No condition = infinite loop (always jump to body)
		g.irBuilder.BuildJmp(body_block, nil)
	}

	// Generate the body block
	g.irBuilder.SetInsertionBlock(body_block)
	g.pushLoopContext(merge_block, update_block)
	stmt.Body.Accept(g)
	g.popLoopContext()
	g.irBuilder.BuildJmp(update_block, nil)

	// Generate the update block
	g.irBuilder.SetInsertionBlock(update_block)
	if stmt.Update != nil {
		stmt.Update.Accept(g)
	}
	g.irBuilder.BuildJmp(condition_block, nil)

	// Generate the merge block
	g.irBuilder.SetInsertionBlock(merge_block)

	g.symbolTable().ExitScope()
}

func (g *IRModule) VisitBreakStmt(stmt *ast.BreakStmtNode) {
	ctx := g.currentLoopContext()
	if ctx == nil {
		g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "break statement outside of loop", stmt.Span))
		return
	}
	g.irBuilder.BuildJmp(ctx.BreakTarget, stmt.Span)
}

func (g *IRModule) VisitContinueStmt(stmt *ast.ContinueStmtNode) {
	ctx := g.currentLoopContext()
	if ctx == nil {
		g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "continue statement outside of loop", stmt.Span))
		return
	}
	g.irBuilder.BuildJmp(ctx.ContinueTarget, stmt.Span)
}

// buildPowerOp casts both operands to f64, computes ** via InstrTypePower, and casts
// the result back to originalType so the caller can store it in the original variable.
func (g *IRModule) buildPowerOp(left, right zeus_value.Value, originalType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	f64Type := zeus_value.FloatType{Size: zeus_value.F64, Span: span}
	if !zeus_value.IsFloatType(zeus_value.GetValueType(left)) || zeus_value.AsFloatType(zeus_value.GetValueType(left)).Size != zeus_value.F64 {
		left = g.irBuilder.BuildCast(left, f64Type, span)
	}
	if !zeus_value.IsFloatType(zeus_value.GetValueType(right)) || zeus_value.AsFloatType(zeus_value.GetValueType(right)).Size != zeus_value.F64 {
		right = g.irBuilder.BuildCast(right, f64Type, span)
	}
	result := g.irBuilder.BuildBinaryOp(left, right, InstrTypePower, span)
	if originalType != nil && (!zeus_value.IsFloatType(originalType) || zeus_value.AsFloatType(originalType).Size != zeus_value.F64) {
		result = g.irBuilder.BuildCast(result, originalType, span)
	}
	return result
}

func (g *IRModule) isCompoundAssignment(tokenType token.TokenType) bool {
	return tokenType == token.TokenTypePlusEqual ||
		tokenType == token.TokenTypeMinusEqual ||
		tokenType == token.TokenTypeStarEqual ||
		tokenType == token.TokenTypeSlashEqual ||
		tokenType == token.TokenTypePercentEqual ||
		tokenType == token.TokenTypeDoubleStarEqual ||
		tokenType == token.TokenTypeBitwiseAndEqual ||
		tokenType == token.TokenTypeBitwiseOrEqual ||
		tokenType == token.TokenTypeBitwiseXorEqual ||
		tokenType == token.TokenTypeLeftShiftEqual ||
		tokenType == token.TokenTypeRightShiftEqual
}

func (g *IRModule) getCompoundAssignmentOp(tokenType token.TokenType) InstrType {
	switch tokenType {
	case token.TokenTypePlusEqual:
		return InstrTypeAdd
	case token.TokenTypeMinusEqual:
		return InstrTypeSub
	case token.TokenTypeStarEqual:
		return InstrTypeMul
	case token.TokenTypeSlashEqual:
		return InstrTypeDiv
	case token.TokenTypePercentEqual:
		return InstrTypeMod
	case token.TokenTypeDoubleStarEqual:
		return InstrTypePower
	case token.TokenTypeBitwiseAndEqual:
		return InstrTypeBitAnd
	case token.TokenTypeBitwiseOrEqual:
		return InstrTypeBitOr
	case token.TokenTypeBitwiseXorEqual:
		return InstrTypeBitXor
	case token.TokenTypeLeftShiftEqual:
		return InstrTypeShl
	case token.TokenTypeRightShiftEqual:
		return InstrTypeShr
	default:
		panic(fmt.Sprintf("unknown compound assignment operator: %s", tokenType))
	}
}

func (g *IRModule) emitShortCircuitAnd(left zeus_value.Value, expr *ast.BinaryExprNode) zeus_value.Value {
	span := expr.GetSpan()
	resultVar := g.irBuilder.BuildVarDecl(NewVarDecl("sc_and", zeus_value.BoolType{Span: span}, false, nil, span))
	g.irBuilder.BuildStore(resultVar, left, span)

	evalRight := g.irBuilder.BuildSuccessorBlock()
	merge := g.irBuilder.BuildSuccessorBlock()
	g.irBuilder.BuildCondJmp(evalRight, merge, left, span)

	g.irBuilder.SetInsertionBlock(evalRight)
	right := expr.Right.Accept(g)
	g.irBuilder.BuildStore(resultVar, right, span)
	g.irBuilder.BuildJmp(merge, nil)

	g.irBuilder.SetInsertionBlock(merge)
	return g.irBuilder.BuildLoad(resultVar, span)
}

func (g *IRModule) emitShortCircuitOr(left zeus_value.Value, expr *ast.BinaryExprNode) zeus_value.Value {
	span := expr.GetSpan()
	resultVar := g.irBuilder.BuildVarDecl(NewVarDecl("sc_or", zeus_value.BoolType{Span: span}, false, nil, span))
	g.irBuilder.BuildStore(resultVar, left, span)

	evalRight := g.irBuilder.BuildSuccessorBlock()
	merge := g.irBuilder.BuildSuccessorBlock()
	g.irBuilder.BuildCondJmp(merge, evalRight, left, span)

	g.irBuilder.SetInsertionBlock(evalRight)
	right := expr.Right.Accept(g)
	g.irBuilder.BuildStore(resultVar, right, span)
	g.irBuilder.BuildJmp(merge, nil)

	g.irBuilder.SetInsertionBlock(merge)
	return g.irBuilder.BuildLoad(resultVar, span)
}

func (g *IRModule) VisitBinaryExpr(expr *ast.BinaryExprNode) zeus_value.Value {
	// Handle compound assignments (+=, -=, etc.)
	if g.isCompoundAssignment(expr.Operator.Type) {
		// Get the LHS address
		g.isLValueExpr = true
		left := expr.Left.Accept(g)
		g.isLValueExpr = false
		right := expr.Right.Accept(g)

		// Check if this is an array element compound assignment
		if arrayRef := zeus_value.AsArrayElementRef(left); arrayRef != nil {
			// For array[i] += value:
			// 1. GET_INDEX to read current value
			// 2. Compute new value
			// 3. SET_INDEX to write back
			currentValue := g.irBuilder.BuildGetIndex(arrayRef.ArrayObject, []zeus_value.Value{arrayRef.Index}, expr.GetSpan())

			op := g.getCompoundAssignmentOp(expr.Operator.Type)
			var newValue zeus_value.Value
			if op == InstrTypePower {
				newValue = g.buildPowerOp(currentValue, right, nil, expr.GetSpan())
			} else {
				newValue = g.irBuilder.BuildBinaryOp(currentValue, right, op, expr.GetSpan())
			}

			g.irBuilder.BuildSetIndex(arrayRef.ArrayObject, arrayRef.Index, newValue, expr.GetSpan())
			return newValue
		}

		// Check if this is an accessor compound assignment (obj.prop += value)
		if accLVal := zeus_value.AsAccessorLValue(left); accLVal != nil {
			currentValue := g.irBuilder.BuildGetAccessor(accLVal.Object, accLVal.AccessorName, expr.GetSpan())
			op := g.getCompoundAssignmentOp(expr.Operator.Type)
			var newValue zeus_value.Value
			if op == InstrTypePower {
				newValue = g.buildPowerOp(currentValue, right, nil, expr.GetSpan())
			} else {
				newValue = g.irBuilder.BuildBinaryOp(currentValue, right, op, expr.GetSpan())
			}
			return g.irBuilder.BuildSetAccessor(accLVal.Object, accLVal.AccessorName, newValue, expr.GetSpan())
		}

		// Regular variable compound assignment
		addr := zeus_value.AsVar(left)
		if addr == nil {
			g.pushError(&zeus_error.ZeusError{
				Message: "invalid lvalue in compound assignment",
				Span:    expr.GetSpan(),
			})
			return nil
		}

		// Load current value, apply operation, store back
		currentValue := g.irBuilder.BuildLoad(addr, expr.GetSpan())
		op := g.getCompoundAssignmentOp(expr.Operator.Type)
		var newValue zeus_value.Value
		if op == InstrTypePower {
			newValue = g.buildPowerOp(currentValue, right, addr.ValueType, expr.GetSpan())
		} else {
			newValue = g.irBuilder.BuildBinaryOp(currentValue, right, op, expr.GetSpan())
		}
		g.irBuilder.BuildStore(addr, newValue, expr.GetSpan())
		return g.irBuilder.BuildLoad(addr, expr.GetSpan())
	}

	g.isLValueExpr = expr.Operator.Type == token.TokenTypeEqual
	left := expr.Left.Accept(g)
	g.isLValueExpr = false

	switch expr.Operator.Type {
	case token.TokenTypeAmpAmp:
		return g.emitShortCircuitAnd(left, expr)
	case token.TokenTypePipePipe:
		return g.emitShortCircuitOr(left, expr)
	}

	right := expr.Right.Accept(g)

	switch expr.Operator.Type {
	case token.TokenTypePlus:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeAdd, expr.GetSpan())
	case token.TokenTypeMinus:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeSub, expr.GetSpan())
	case token.TokenTypeStar:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeMul, expr.GetSpan())
	case token.TokenTypeSlash:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeDiv, expr.GetSpan())
	case token.TokenTypePercent:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeMod, expr.GetSpan())
	case token.TokenTypeDoubleStar:
		return g.buildPowerOp(left, right, nil, expr.GetSpan())
	case token.TokenTypeBangEqual:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeNotEq, expr.GetSpan())
	case token.TokenTypeEqualEqual:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeEqEq, expr.GetSpan())
	case token.TokenTypeLessThan:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeLessThan, expr.GetSpan())
	case token.TokenTypeLessThanEqual:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeLessThanEq, expr.GetSpan())
	case token.TokenTypeGreaterThan:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeGreaterThan, expr.GetSpan())
	case token.TokenTypeGreaterThanEqual:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeGreaterThanEq, expr.GetSpan())
	case token.TokenTypeBitwiseAnd:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeBitAnd, expr.GetSpan())
	case token.TokenTypeBitwiseOr:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeBitOr, expr.GetSpan())
	case token.TokenTypeBitwiseXor:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeBitXor, expr.GetSpan())
	case token.TokenTypeLeftShift:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeShl, expr.GetSpan())
	case token.TokenTypeRightShift:
		return g.irBuilder.BuildBinaryOp(left, right, InstrTypeShr, expr.GetSpan())
	case token.TokenTypeEqual:
		// Check if this is an array element assignment (array[0][1] = expr)
		if arrayRef := zeus_value.AsArrayElementRef(left); arrayRef != nil {
			// Emit SET_INDEX HIR instruction; lowering pass converts to .set() method call
			g.irBuilder.BuildSetIndex(arrayRef.ArrayObject, arrayRef.Index, right, expr.GetSpan())
			return right
		}

		// Check if this is a setter accessor assignment (obj.prop = value)
		if accLVal := zeus_value.AsAccessorLValue(left); accLVal != nil {
			return g.irBuilder.BuildSetAccessor(accLVal.Object, accLVal.AccessorName, right, expr.GetSpan())
		}

		// Regular variable assignment
		addr := zeus_value.AsVar(left)
		if addr == nil {
			g.pushError(&zeus_error.ZeusError{
				Message: "invalid lvalue in assignment",
				Span:    expr.GetSpan(),
			})
			return nil
		}

		g.irBuilder.BuildStore(addr, right, expr.GetSpan())
		return g.irBuilder.BuildLoad(addr, expr.GetSpan())

	default:
		panic(fmt.Sprintf("unknown binary operator: %s", expr.Operator.Type))
	}
}

func (g *IRModule) VisitGroupingExpr(expr *ast.GroupingExprNode) zeus_value.Value {
	return expr.Expr.Accept(g)
}

func (g *IRModule) VisitTernaryExpr(expr *ast.TernaryExprNode) zeus_value.Value {
	span := expr.GetSpan()

	// Evaluate the condition in the current (parent) block.
	condition := expr.Condition.Accept(g)

	// Create then and else blocks as successors of the parent block.
	parentBlock := g.irBuilder.GetInsertionBlock()
	thenBlock := g.irBuilder.BuildSuccessorBlock()
	elseBlock := g.irBuilder.BuildSuccessorBlock()

	// Evaluate the then-branch first (in thenBlock) to discover the result type.
	// The type checker will later resolve any unresolved types from the stores.
	g.irBuilder.SetInsertionBlock(thenBlock)
	thenValue := expr.Then.Accept(g)

	// Return to parentBlock to declare the result variable and emit the conditional jump.
	g.irBuilder.SetInsertionBlock(parentBlock)
	resultVar := g.irBuilder.BuildVarDecl(NewVarDecl("ternary", zeus_value.UndefinedType{Span: span}, false, nil, span))
	g.irBuilder.BuildCondJmp(thenBlock, elseBlock, condition, span)

	// Finish the then-block: store the then-value and jump to merge.
	g.irBuilder.SetInsertionBlock(thenBlock)
	mergeBlock := g.irBuilder.BuildSuccessorBlock()
	g.irBuilder.BuildStore(resultVar, thenValue, span)
	g.irBuilder.BuildJmp(mergeBlock, span)

	// Else-block: evaluate else-branch, store, and jump to merge.
	g.irBuilder.SetInsertionBlock(elseBlock)
	elseValue := expr.Else.Accept(g)
	g.irBuilder.BuildStore(resultVar, elseValue, span)
	g.irBuilder.BuildJmp(mergeBlock, span)

	g.irBuilder.SetInsertionBlock(mergeBlock)
	return g.irBuilder.BuildLoad(resultVar, span)
}

func (g *IRModule) VisitFunctionCallExpr(expr *ast.FunctionCallExprNode) zeus_value.Value {
	// Detect method call: callee is obj.method — emit a single CALL_METHOD instead of the
	// OBJECT_PROPERTY_ACCESS + LOAD + CALL_INDIRECT_FUNC chain.
	if propAccess, ok := expr.Callee.(*ast.ObjectPropertyAccessExprNode); ok {
		object := propAccess.Object.Accept(g)
		if asVar := zeus_value.AsVar(object); asVar != nil && asVar.IsPtr {
			object = g.irBuilder.BuildLoad(asVar, propAccess.GetSpan())
		}
		if !g.isThisExpression(propAccess.Object) {
			g.emitNullCheck(object, propAccess.Property.Name.Value, propAccess.GetSpan())
		}
		args := []zeus_value.Value{}
		for _, param := range expr.Params {
			args = append(args, param.Accept(g))
		}
		return g.irBuilder.BuildMethodCall(object, propAccess.Property.Name.Value, args, nil, nil, expr.GetSpan())
	}

	callee := expr.Callee.Accept(g)
	params := []zeus_value.Value{}
	for _, arg := range expr.Params {
		params = append(params, arg.Accept(g))
	}

	if zeus_value.IsFunction(callee) {
		fn := zeus_value.AsFunction(callee)
		return g.irBuilder.BuildCallFunc(fn, params, expr.GetSpan())
	} else if zeus_value.IsVar(callee) {
		addr := zeus_value.AsVar(callee)
		return g.irBuilder.BuildIndirectFuncCall(addr, params, expr.GetSpan())
	} else if zeus_value.IsObject(callee) {
		object := zeus_value.AsObject(callee)
		return g.irBuilder.BuildIndirectFuncCall(object, params, expr.GetSpan())
	}

	g.pushError(&zeus_error.ZeusError{
		Message: fmt.Sprintf("%s is not callable", expr.Callee.PrettyString()),
		Span:    expr.GetSpan(),
	})

	return nil
}

// checkVariadicParamType reports an error when a rest parameter's resolved type is not
// an array. Rest parameters are collected into a Zeus array during lowering, so the
// declared type must be an array (e.g. `...args: i32[]`).
func (g *IRModule) checkVariadicParamType(param *ast.VarDeclNode, paramType zeus_value.ValueType) {
	objType := zeus_value.AsObjectType(paramType)
	if objType == nil || objType.Class.ArrayElementType == nil {
		g.pushError(zeus_error.NewZeusError(
			zeus_error.ErrorSeverityError,
			fmt.Sprintf("rest parameter '%s' must have an array type", param.Identifier.Name.Value),
			param.Identifier.Name.Span,
		))
	}
}

func (g *IRModule) emitFunction(name string, fnParams []*ast.VarDeclNode, returnType zeus_value.ValueType, fnBody *ast.BlockStmtNode, class *zeus_value.Class, capturedVars []*CapturedVar, span *token.Span) zeus_value.Value {
	params := []*VarDecl{}

	for _, param := range fnParams {
		paramType := g.resolveTypeForIRGen(param.ValueType.ValueType, false)
		varDecl := NewVarDecl(
			param.Identifier.Name.Value,
			paramType,
			true,
			nil,
			param.Identifier.Name.Span,
		)
		if param.IsVariadic {
			g.checkVariadicParamType(param, paramType)
			varDecl.IsVariadic = true
		}
		params = append(params, varDecl)
	}

	// Escape analysis: determine which params/vars in this scope are referenced by
	// nested closures and therefore need heap ref cells. Only meaningful when there
	// is an actual body (fnBody != nil, i.e. not a stub).
	escapedNames := map[string]bool{}
	if fnBody != nil {
		escapedNames = collectEscapedVarNames(fnParams, fnBody)
	}

	current_block := g.irBuilder.GetInsertionBlock()
	// functions are global
	g.irBuilder.SetInsertionBlock(nil)
	body := g.irBuilder.BuildBasicBlock()
	// BuildFuncDecl carries the rest-parameter flag from the VarDecls onto the built
	// Vars and the Function, so both direct and forward references observe variadic-ness
	// by the time type checking runs.
	fn := g.irBuilder.BuildFuncDecl(name, params, body, returnType, class, span)
	g.symbolTable().EnterScope()

	// Declare params; for escaped params, immediately promote to a ref cell.
	for index, param := range fnParams {
		paramName := param.Identifier.Name.Value
		paramVar := fn.Params[index]
		g.symbolTable().DeclareSymbol(paramName, paramVar)
	}

	if class != nil {
		valueType := zeus_value.NewObjectType(class)
		object := zeus_value.NewObject(token.THIS_KEYWORD, valueType, span)
		g.symbolTable().DeclareSymbol(token.THIS_KEYWORD, &object)
	}

	g.irBuilder.SetInsertionBlock(body)

	// Promote escaped params to ref cells now that we're in the body block.
	for index, param := range fnParams {
		paramName := param.Identifier.Name.Value
		if !escapedNames[paramName] {
			continue
		}
		paramVar := fn.Params[index]
		paramType := param.ValueType.ValueType
		if zeus_value.IsUndefinedType(paramType) {
			continue // can't box an unknown type; leave as alloca
		}
		cellObj := g.allocRefCell(paramType, paramVar, param.Identifier.Name.Span)
		refCell := &zeus_value.RefCellVar{
			OriginalName: paramName,
			ValueType:    paramType,
			Cell:         cellObj,
			Span:         param.Identifier.Name.Span,
		}
		g.symbolTable().DeclareSymbol(paramName, refCell)
	}

	// Emit captured variable preamble: load each captured value from the
	// functor's own properties and shadow it as a local variable so the body
	// can reference the outer name transparently.
	//
	// Pass 1: non-this captures — must happen before "this" is shadowed so we
	// can still reach functor properties through the functor's own "this".
	for _, cap := range capturedVars {
		if cap.OriginalName == token.THIS_KEYWORD {
			continue
		}
		functorThis, _ := g.symbolTable().GetSymbol(token.THIS_KEYWORD)
		propPtr := g.irBuilder.BuildObjectPropertyAccess(functorThis, cap.PropertyName, false, span)
		propPtrVar := zeus_value.AsVar(propPtr)
		propPtrVar.ValueType = cap.ValueType
		loadedVal := g.irBuilder.BuildLoad(propPtrVar, span)
		zeus_value.AsVar(loadedVal).ValueType = cap.ValueType

		if cap.IsRefCell {
			// Ref cell capture: register RefCellVar so reads/writes go through cell.value.
			cellObjType := zeus_value.AsObjectType(cap.ValueType)
			innerType := getRefCellValueType(cellObjType.Class)
			refCell := &zeus_value.RefCellVar{
				OriginalName: cap.OriginalName,
				ValueType:    innerType,
				Cell:         loadedVal,
				Span:         span,
			}
			g.symbolTable().DeclareSymbol(cap.OriginalName, refCell)
		} else {
			// Plain value capture: shadow with a local alloca copy.
			localVar := g.irBuilder.BuildVarDecl(NewVarDecl(cap.OriginalName, cap.ValueType, false, nil, span))
			g.irBuilder.BuildStore(localVar, loadedVal, span)
			// Register under the original source name so body can look it up —
			// same pattern as VisitVarDeclStmt (ir.go).
			g.symbolTable().DeclareSymbol(cap.OriginalName, localVar)
		}
	}

	// Pass 2: "this" capture — shadow the functor's "this" with the outer class
	// instance. Done last so pass 1 property loads still use the functor's this.
	for _, cap := range capturedVars {
		if cap.OriginalName != token.THIS_KEYWORD {
			continue
		}
		functorThis, _ := g.symbolTable().GetSymbol(token.THIS_KEYWORD)
		propPtr := g.irBuilder.BuildObjectPropertyAccess(functorThis, cap.PropertyName, false, span)
		propPtrVar := zeus_value.AsVar(propPtr)
		propPtrVar.ValueType = cap.ValueType
		loadedVal := g.irBuilder.BuildLoad(propPtrVar, span)
		loadedVar := zeus_value.AsVar(loadedVal)
		loadedVar.ValueType = cap.ValueType
		// IsPtr=false: the loaded value is already a heap pointer (not a
		// pointer-to-pointer), so VisitObjectPropertyAccessExpr must not
		// dereference it again.
		loadedVar.IsPtr = false
		g.symbolTable().DeclareSymbol(token.THIS_KEYWORD, loadedVar)
	}

	// Pre-infer types for all local vars before emitting IR for the function body.
	// Must run after params and captured-var preamble are in the symbol table so
	// identifier lookups in inferExprType succeed.
	var funcTypeEnv *FunctionTypeEnv
	if fnBody != nil {
		funcTypeEnv = g.inferFunctionEnv(fnBody)
	}

	// Save/restore all per-function fields so nested emitFunction calls don't clobber them.
	// Also reset isInModuleScope so vars declared inside a function body are local.
	prevEscapedVarNames := g.escapedVarNames
	prevFuncTypeEnv := g.funcTypeEnv
	prevIsInModuleScope := g.isInModuleScope
	g.escapedVarNames = escapedNames
	g.funcTypeEnv = funcTypeEnv
	g.isInModuleScope = false
	fnBody.Accept(g)
	g.escapedVarNames = prevEscapedVarNames
	g.funcTypeEnv = prevFuncTypeEnv
	g.isInModuleScope = prevIsInModuleScope

	g.irBuilder.SetInsertionBlock(current_block)

	g.symbolTable().ExitScope()

	return fn
}

// getOrCreateRefCellClass returns (creating if necessary) the class used to box a
// variable of the given valueType on the heap for capture-by-reference closures.
func (g *IRModule) getOrCreateRefCellClass(valueType zeus_value.ValueType, span *token.Span) *zeus_value.Class {
	className := zeus_value.RefCellClassName(valueType)
	if existing, ok := g.symbolTable().GetSymbol(className); ok {
		if cls, ok := existing.(*zeus_value.Class); ok {
			return cls
		}
	}
	refCellClass := zeus_value.GetRefCellClassDefinition(valueType, span)
	g.irBuilder.EmitClassDeclAtStart(refCellClass)
	g.symbolTable().DeclareSymbol(className, refCellClass)
	return refCellClass
}

// getRefCellValueType extracts the inner scalar type from a ref cell class's `value` property.
func getRefCellValueType(class *zeus_value.Class) zeus_value.ValueType {
	for _, prop := range class.Properties {
		if prop.Property.Name == zeus_value.ZEUS_REF_CELL_VALUE_PROPERTY {
			return prop.Property.ValueType
		}
	}
	return nil
}

// allocRefCell creates a new ref cell object for valueType, stores initializer into
// cell.value (if non-nil), and returns the cell object *Var.
func (g *IRModule) allocRefCell(valueType zeus_value.ValueType, initializer zeus_value.Value, span *token.Span) zeus_value.Value {
	refCellClass := g.getOrCreateRefCellClass(valueType, span)
	cellObj := g.irBuilder.BuildNewObj(refCellClass, []zeus_value.Value{}, span)
	if cellVar := zeus_value.AsVar(cellObj); cellVar != nil {
		cellVar.ValueType = zeus_value.NewObjectType(refCellClass)
	}
	if initializer != nil {
		propPtr := g.irBuilder.BuildObjectPropertyAccess(cellObj, zeus_value.ZEUS_REF_CELL_VALUE_PROPERTY, true, span)
		zeus_value.AsVar(propPtr).ValueType = valueType
		g.irBuilder.BuildStore(zeus_value.AsVar(propPtr), initializer, span)
	}
	return cellObj
}

func (g *IRModule) emitFunctorClass(fnName string, fnParams []*ast.VarDeclNode, returnType zeus_value.ValueType, fnBody *ast.BlockStmtNode, span *token.Span) zeus_value.Value {
	functorClassName := g.generateUniqueName(fmt.Sprintf("%sFunctor", fnName))

	// Collect free variables before any IR is emitted: the symbol table still
	// reflects the enclosing scope at this point, so GetSymbol finds outer vars.
	capturedVars := g.collectFreeVars(fnParams, fnBody)

	// Build the captured-var class properties (public fields on the functor struct).
	capturedProps := make([]*zeus_value.ClassProperty, 0, len(capturedVars))
	for _, cap := range capturedVars {
		capturedProps = append(capturedProps, zeus_value.NewClassProperty(
			zeus_value.NewVar(cap.PropertyName, cap.ValueType, false, span),
			&token.Token{Type: token.TokenTypePublic, Span: span},
			false,
		))
	}

	// Create stubs with actual param types; IR names (Name) are updated below after
	// generateUniqueName produces class-unique names, matching buildClass's pattern.
	methodParams := []*zeus_value.Var{}
	for _, param := range fnParams {
		methodParams = append(methodParams, zeus_value.NewVar(param.Identifier.Name.Value, param.ValueType.ValueType, false, param.Identifier.Name.Span))
	}
	constructorStub := zeus_value.NewFunction(token.CONSTRUCTOR_METHOD_NAME, []*zeus_value.Var{}, zeus_value.VoidType{Span: span}, span)
	callStub := zeus_value.NewFunction(token.FUNCTOR_CALL_METHOD_NAME, methodParams, returnType, span)
	// These stubs are placeholders: once BuildFuncDecl/emitFunction produce the real
	// constructor and __call__ Functions below, callMethod/constructorMethod are repointed
	// to share those pointers, so the emitted Function is the single source of truth for
	// signature, return type and variadic-ness (no per-field copying to keep in sync).
	constructorMethod := zeus_value.NewClassMethod(constructorStub, &token.Token{Type: token.TokenTypePublic, Span: span})
	callMethod := zeus_value.NewClassMethod(callStub, &token.Token{Type: token.TokenTypePublic, Span: span})
	functorClass := zeus_value.NewClass(functorClassName, capturedProps,
		[]*zeus_value.ClassMethod{constructorMethod, callMethod}, nil, "", nil, span)
	g.irBuilder.EmitClassDeclAtStart(functorClass)

	// Constructor: no AST body, just a return. Use BuildFuncDecl with a unique IR name
	// (same approach as buildClass uses for regular class methods via generateUniqueName).
	constructorIRName := g.generateUniqueName(token.CONSTRUCTOR_METHOD_NAME)
	savedBlock := g.irBuilder.GetInsertionBlock()
	g.irBuilder.SetInsertionBlock(nil)
	constructorBlock := g.irBuilder.BuildBasicBlock()
	constructorFn := g.irBuilder.BuildFuncDecl(constructorIRName, []*VarDecl{}, constructorBlock, zeus_value.VoidType{Span: span}, functorClass, span)
	constructorFn.OriginalName = token.CONSTRUCTOR_METHOD_NAME
	constructorMethod.Method = constructorFn
	g.irBuilder.SetInsertionBlock(constructorBlock)
	g.irBuilder.BuildReturn(nil, span)
	g.irBuilder.SetInsertionBlock(savedBlock)

	// Make fnName resolve to 'this' in the parent scope before processing the body.
	// This lets named function expressions call themselves recursively:
	//   fibonacci(n-1) → INDIRECT_FUNC_CALL(this) → FunctorCallLoweringPass → CALL_METHOD(this, __call__, [n-1])
	// Anonymous functors use a generated unique name so this declaration is harmless.
	selfRef := zeus_value.NewObject(token.THIS_KEYWORD, zeus_value.NewObjectType(functorClass), span)
	g.symbolTable().DeclareSymbol(fnName, &selfRef)

	// __call__: emitFunction handles the DECL_CLASS_METHOD instruction, unique IR naming,
	// param scoping, and body generation — same as buildClass does for user-defined methods.
	// Pass capturedVars so emitFunction emits the preamble that loads captured values.
	callIRName := g.generateUniqueName(token.FUNCTOR_CALL_METHOD_NAME)
	callFn := zeus_value.AsFunction(g.emitFunction(callIRName, fnParams, returnType, fnBody, functorClass, capturedVars, span))
	if callFn != nil {
		callFn.OriginalName = token.FUNCTOR_CALL_METHOD_NAME
		callMethod.Method = callFn
	}

	// Overwrite the selfRef entry with the actual functor object so the name is usable
	// in the enclosing scope after the declaration (e.g. `return fibonacci(8)`).
	functorObject := g.irBuilder.BuildNewObj(functorClass, []zeus_value.Value{}, span)
	// Set the type immediately so any outer collectFreeVars sees the correct ObjectType.
	if resultVar := zeus_value.AsVar(functorObject); resultVar != nil {
		resultVar.ValueType = zeus_value.NewObjectType(functorClass)
	}
	g.symbolTable().DeclareSymbol(fnName, functorObject)

	// Initialize captured-var properties on the new functor object.
	// Each property gets the current value of the corresponding outer variable.
	for _, cap := range capturedVars {
		var currentVal zeus_value.Value
		if srcVar := zeus_value.AsVar(cap.Source); srcVar != nil {
			if cap.IsRefCell {
				// Ref cell: Source is the cell *Var (IsPtr=false) — store the cell
				// pointer itself so the closure shares the same heap cell.
				currentVal = srcVar
			} else if srcVar.IsPtr {
				// alloca (local variable) — load current value from its stack slot
				currentVal = g.irBuilder.BuildLoad(srcVar, span)
			} else {
				// SSA parameter — the var IS the value, no load needed
				currentVal = srcVar
			}
		} else {
			// *zeus_value.Object (e.g. "this"): the object reference IS the value.
			currentVal = cap.Source
		}
		propPtr := g.irBuilder.BuildObjectPropertyAccess(functorObject, cap.PropertyName, true, span)
		g.irBuilder.BuildStore(zeus_value.AsVar(propPtr), currentVal, span)
	}

	return functorObject
}

func (g *IRModule) VisitFunctionDeclExpr(expr *ast.FunctionDeclExprNode) zeus_value.Value {
	var returnType zeus_value.ValueType
	returnType = zeus_value.VoidType{Span: expr.GetSpan()}

	if expr.ReturnType != nil {
		returnType = g.resolveTypeForIRGen(expr.ReturnType.ValueType, true)
	}

	fnName := ""
	var fnSpan *token.Span
	if expr.Name == nil {
		fnName = g.generateUniqueName(anonymousName)
		fnSpan = expr.GetSpan()
	} else {
		fnName = expr.Name.Name.Value
		fnSpan = expr.Name.GetSpan()
	}

	// When inside a function body that is NOT the module-level scope, create a closure.
	// When at module level (isInModuleScope) or truly outside any block, emit as global function.
	if g.irBuilder.currentBlock != nil && !g.isInModuleScope {
		return g.emitFunctorClass(fnName, expr.Params, returnType, expr.Body, fnSpan)
	}

	return g.emitFunction(fnName, expr.Params, returnType, expr.Body, nil, nil, fnSpan)
}

func (g *IRModule) VisitIdentifier(expr *ast.IdentifierExprNode) zeus_value.Value {
	variable, ok := g.symbolTable().GetSymbol(expr.Name.Value)

	if !ok {
		g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("undefined identifier '%s'", expr.Name.Value), expr.Name.Span))
		return nil
	}

	asFn := zeus_value.AsFunction(variable)

	if asFn != nil {
		return asFn
	}

	// RefCellVar: reads and writes go through cell.value transparently.
	if refCell := zeus_value.AsRefCellVar(variable); refCell != nil {
		propPtr := g.irBuilder.BuildObjectPropertyAccess(refCell.Cell, zeus_value.ZEUS_REF_CELL_VALUE_PROPERTY, g.isLValueExpr, expr.Name.Span)
		propPtrVar := zeus_value.AsVar(propPtr)
		propPtrVar.ValueType = refCell.ValueType
		if g.isLValueExpr {
			return propPtrVar
		}
		loadedVal := g.irBuilder.BuildLoad(propPtrVar, expr.Name.Span)
		zeus_value.AsVar(loadedVal).ValueType = refCell.ValueType
		return loadedVal
	}

	if g.isLValueExpr {
		return variable
	}

	asVar := zeus_value.AsVar(variable)
	asClass := zeus_value.AsClass(variable)
	asObject := zeus_value.AsObject(variable)

	if asVar != nil {
		if asVar.IsPtr {
			return g.irBuilder.BuildLoad(asVar, expr.Name.Span)
		} else {
			return asVar
		}
	}

	if asClass != nil {
		return asClass
	}

	if asObject != nil {
		return asObject
	}

	panic(fmt.Sprintf("unknown identifier type: %T", variable))
}

func (g *IRModule) VisitNumber(expr *ast.NumberExprNode) zeus_value.Value {
	if zeus_value.IsFloat(expr.Value.Value) {
		return zeus_value.NewConstant(
			expr.Value.Value,
			zeus_value.FloatType{
				Size: zeus_value.F64,
			},
			expr.Value.Span,
		)
	} else {
		return zeus_value.NewConstant(
			expr.Value.Value,
			zeus_value.IntType{
				Signed: false,
				Size:   zeus_value.GetSignedIntSize(expr.Value.Value),
			},
			expr.Value.Span,
		)
	}
}

func (g *IRModule) VisitNull(expr *ast.NullExprNode) zeus_value.Value {
	return zeus_value.NewConstant(
		zeus_value.NULL_CONSTANT_VALUE,
		zeus_value.NullType{},
		expr.GetSpan(),
	)
}

func (g *IRModule) VisitUnaryExpr(expr *ast.UnaryExprNode) zeus_value.Value {
	// Handle prefix ++/-- (increment then return new value)
	if expr.Operator.Type == token.TokenTypePlusPlus || expr.Operator.Type == token.TokenTypeMinusMinus {
		// Get the address of the variable
		g.isLValueExpr = true
		target := expr.Expr.Accept(g)
		g.isLValueExpr = false

		one := zeus_value.NewConstant("1", zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: expr.Operator.Span}, expr.Operator.Span)

		// Check accessor lvalue first
		if accLVal := zeus_value.AsAccessorLValue(target); accLVal != nil {
			currentValue := g.irBuilder.BuildGetAccessor(accLVal.Object, accLVal.AccessorName, expr.GetSpan())
			newValue := g.buildIncrDecrOp(expr.Operator.Type, currentValue, one, expr.GetSpan())
			return g.irBuilder.BuildSetAccessor(accLVal.Object, accLVal.AccessorName, newValue, expr.GetSpan())
		}

		addr := zeus_value.AsVar(target)
		if addr == nil {
			g.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("invalid operand for prefix '%s': expression is not assignable", expr.Operator.Type),
				Span:    expr.Operator.Span,
			})
			return nil
		}

		// Load current value
		currentValue := g.irBuilder.BuildLoad(addr, expr.Operator.Span)

		// Use u8 for the constant so it is implicitly promoted to whatever numeric type
		// the operand has (int or float). addr.ValueType is nil for class field pointers
		// at IR generation time (resolved later by tcObjectPropertyAccess).

		newValue := g.buildIncrDecrOp(expr.Operator.Type, currentValue, one, expr.GetSpan())
		g.irBuilder.BuildStore(addr, newValue, expr.GetSpan())
		return g.irBuilder.BuildLoad(addr, expr.GetSpan())
	}

	value := expr.Expr.Accept(g)

	switch expr.Operator.Type {
	case token.TokenTypeMinus:
		return g.irBuilder.BuildUnaryOp(value, InstrTypeNeg, expr.Operator.Span)
	case token.TokenTypeBang:
		return g.irBuilder.BuildUnaryOp(value, InstrTypeNot, expr.Operator.Span)
	case token.TokenTypeBitwiseNot:
		return g.irBuilder.BuildUnaryOp(value, InstrTypeBitNot, expr.Operator.Span)
	default:
		panic(fmt.Sprintf("unknown unary operator: %s", expr.Operator.Type))
	}
}

func (g *IRModule) VisitPostfixExpr(expr *ast.PostfixExprNode) zeus_value.Value {
	// Get the address of the variable
	g.isLValueExpr = true
	target := expr.Expr.Accept(g)
	g.isLValueExpr = false

	one := zeus_value.NewConstant("1", zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: expr.Operator.Span}, expr.Operator.Span)

	// Handle accessor lvalue (obj.prop++)
	if accLVal := zeus_value.AsAccessorLValue(target); accLVal != nil {
		currentValue := g.irBuilder.BuildGetAccessor(accLVal.Object, accLVal.AccessorName, expr.GetSpan())
		newValue := g.buildIncrDecrOp(expr.Operator.Type, currentValue, one, expr.GetSpan())
		g.irBuilder.BuildSetAccessor(accLVal.Object, accLVal.AccessorName, newValue, expr.GetSpan())
		return currentValue // postfix returns old value
	}

	addr := zeus_value.AsVar(target)
	if addr == nil {
		g.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("invalid operand for postfix '%s': expression is not assignable", expr.Operator.Type),
			Span:    expr.Operator.Span,
		})
		return nil
	}

	// Load current value (this is what we'll return)
	currentValue := g.irBuilder.BuildLoad(addr, expr.Operator.Span)

	// Use u8 for the constant so it is implicitly promoted to whatever numeric type
	// the operand has (int or float). addr.ValueType is nil for class field pointers
	// at IR generation time (resolved later by tcObjectPropertyAccess).

	newValue := g.buildIncrDecrOp(expr.Operator.Type, currentValue, one, expr.GetSpan())
	g.irBuilder.BuildStore(addr, newValue, expr.GetSpan())
	return currentValue // postfix returns old value
}

// buildIncrDecrOp returns current + 1 or current - 1 depending on the operator.
func (g *IRModule) buildIncrDecrOp(opType token.TokenType, current, one zeus_value.Value, span *token.Span) zeus_value.Value {
	if opType == token.TokenTypePlusPlus {
		return g.irBuilder.BuildBinaryOp(current, one, InstrTypeAdd, span)
	}
	return g.irBuilder.BuildBinaryOp(current, one, InstrTypeSub, span)
}

// getOrCreateArrayClass gets or creates an array primordial class from the registry.
// The class is automatically registered in the symbol table and a DECL_CLASS instruction
// is emitted at the beginning of the instruction list.
func (g *IRModule) getOrCreateArrayClass(arrayType zeus_value.ArrayType) *zeus_value.Class {
	arrayClassName := arrayType.String()

	// Check if already in symbol table (already processed)
	if existingClass, ok := g.symbolTable().GetSymbol(arrayClassName); ok {
		return existingClass.(*zeus_value.Class)
	}

	// Get or create from registry (handles nested array types internally)
	arrayClass := zeus_value.Registry.GetOrCreateArrayClass(arrayType)

	// Register in global scope so the type checker can find it regardless of call site
	g.symbolTable().DeclareGlobalSymbol(arrayClassName, arrayClass)

	// Emit DECL_CLASS at the beginning of the instruction list
	g.irBuilder.EmitClassDeclAtStart(arrayClass)

	return arrayClass
}

// buildClass builds the IR for a class declaration and registers it in the symbol table.
// For user-defined classes, pass methodASTs to emit method bodies.
// For primordial classes, pass nil for methodASTs (methods are implemented in runtime).
func (g *IRModule) buildClass(class *zeus_value.Class, methodASTs []*ast.ClassMethod) string {
	savedBlock := g.irBuilder.GetInsertionBlock()
	g.irBuilder.SetInsertionBlock(nil)
	irClassName := g.irBuilder.BuildClassDecl(class, class.GetSpan())
	g.irBuilder.SetInsertionBlock(savedBlock)

	// Emit method bodies if AST nodes are provided (user-defined classes).
	// generateUniqueName gives each method a globally unique IR name (e.g. "constructor",
	// "constructor1") without class-name prefixing. We write the IR name back into
	// class.Methods[i].Method so that codegen and imports can look it up via Method.Name,
	// and keep the source name in OriginalName for constructor detection and error messages.
	for i, method := range methodASTs {
		var returnType zeus_value.ValueType = zeus_value.VoidType{Span: method.Name.GetSpan()}
		if method.ReturnType != nil {
			returnType = g.resolveTypeForIRGen(method.ReturnType.ValueType, true)
		}
		irMethodName := g.generateUniqueName(method.Name.Name.Value)
		fn := zeus_value.AsFunction(g.emitFunction(irMethodName, method.Params, returnType, method.Body, class, nil, method.Name.Name.Span))
		if fn != nil {
			fn.OriginalName = method.Name.Name.Value
			// Sync the class method descriptor so downstream passes see the IR name.
			class.Methods[i].Method.Name = irMethodName
			class.Methods[i].Method.OriginalName = method.Name.Name.Value
		}
	}

	return irClassName
}

// buildAccessors emits the IR function bodies for getter/setter accessors.
// Each accessor body is emitted via emitFunction with a mangled name (__get_X / __set_X).
// The resulting IR function is written back into class.Accessors so codegen can find it.
func (g *IRModule) buildAccessors(class *zeus_value.Class, accessorASTs []*ast.ClassMethod) {
	for _, method := range accessorASTs {
		name := method.Name.Name.Value
		var returnType zeus_value.ValueType = zeus_value.VoidType{Span: method.Span}
		if method.ReturnType != nil {
			returnType = g.resolveTypeForIRGen(method.ReturnType.ValueType, true)
		}

		var mangledName string
		if method.Accessor == ast.AccessorKindGetter {
			mangledName = g.generateUniqueName("#get_" + name)
		} else {
			mangledName = g.generateUniqueName("#set_" + name)
		}

		fn := zeus_value.AsFunction(g.emitFunction(mangledName, method.Params, returnType, method.Body, class, nil, method.Name.Name.Span))
		if fn == nil {
			continue
		}
		fn.OriginalName = method.Name.Name.Value

		// Write mangled name back into the class accessor descriptor.
		for _, acc := range class.Accessors {
			if acc.Name != name {
				continue
			}
			if method.Accessor == ast.AccessorKindGetter && acc.Getter != nil {
				acc.Getter.Name = mangledName
				acc.Getter.OriginalName = name
			} else if method.Accessor == ast.AccessorKindSetter && acc.Setter != nil {
				acc.Setter.Name = mangledName
				acc.Setter.OriginalName = name
			}
			break
		}
	}
}

func (g *IRModule) VisitIndexingExpression(expr *ast.IndexingExprNode) zeus_value.Value {
	// Catch `i32[]` / `MyType[]` written without `new` — the parser produces an
	// IndexingExprNode whose base is a ValueTypeNode, which has no runtime value.
	if typeNode, ok := expr.Array.(*ast.ValueTypeNode); ok {
		span := token.NewSpan(typeNode.GetSpan().Start, expr.GetSpan().End)
		g.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("use 'new %s[]' to create an array", typeNode.ValueType),
			Span:    span,
		})
		return nil
	}

	// Save and clear isLValueExpr flag temporarily to properly load the base array
	// Otherwise, if array is a variable, VisitIdentifier would return it without loading
	wasLValueExpr := g.isLValueExpr
	// indices are always r value so while evaluating them consider them as rvalues
	g.isLValueExpr = false

	// Start with the base array
	currentValue := expr.Array.Accept(g)

	// Collect all indices
	indices := []zeus_value.Value{}
	for _, indexExpr := range expr.IndexingMeta.IndexingExprs {
		index := indexExpr.Accept(g)

		// Cast index to i32 if needed (array methods expect i32)
		indexType := zeus_value.GetValueType(index)
		if intType, ok := indexType.(zeus_value.IntType); ok && intType.Size != zeus_value.I32 {
			index = g.irBuilder.BuildCast(index, zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: expr.GetSpan()}, expr.GetSpan())
		}

		indices = append(indices, index)
	}

	// When this is an lvalue (left side of assignment), we need to handle it differently
	// For array[0][1] = expr, we need to emit GET_INDEX for all indices except the last,
	// then return ArrayElementRef with the last index for the assignment handling
	// Note: String immutability is enforced during type checking when .set() is called
	// Note: Bounds checking is NOT done for writes - array.set() handles extension automatically
	if wasLValueExpr {
		// Process all indices except the last one with GET_INDEX
		for i := 0; i < len(indices)-1; i++ {
			currentValue = g.irBuilder.BuildGetIndex(currentValue, []zeus_value.Value{indices[i]}, expr.GetSpan())
		}

		// Return an ArrayElementRef that contains the object and the last index
		// This will be used by VisitBinaryExpr to generate the .set() call
		lastIndex := indices[len(indices)-1]
		return zeus_value.NewArrayElementRef(currentValue, lastIndex, expr.GetSpan())
	}

	// For reading (rvalue), emit bounds check for the first index only
	// The lowering pass will transform GET_INDEX into the appropriate .get() method calls
	// Note: We only bounds check the first index here; nested array accesses will have
	// their own bounds checks when the GET_INDEX is lowered
	if len(indices) > 0 {
		g.emitBoundsCheck(currentValue, indices[0], expr.GetSpan())
	}

	// Emit GET_INDEX with all indices (lowering pass will handle the .get() calls)
	return g.irBuilder.BuildGetIndex(currentValue, indices, expr.GetSpan())
}

func (g *IRModule) VisitBoolean(expr *ast.BooleanExprNode) zeus_value.Value {
	if expr.Value.Type == token.TokenTypeTrue {
		return zeus_value.NewConstant(
			"true",
			zeus_value.BoolType{},
			expr.Value.Span,
		)
	}
	return zeus_value.NewConstant(
		"false",
		zeus_value.BoolType{},
		expr.Value.Span,
	)
}

func (g *IRModule) VisitNewExpr(expr *ast.NewExprNode) zeus_value.Value {
	callee := expr.Callee

	// Check if callee is an IndexingExprNode (array creation)
	if indexingExpr := ast.AsIndexingExpr(callee); indexingExpr != nil {
		// Array creation: new u8[10][][] or new Point[10][]

		// 1. Extract the base element type from the indexing expression
		var baseElementType zeus_value.ValueType

		// Handle primitive types (e.g., u8, i32, f32)
		if valueTypeNode, ok := indexingExpr.Array.(*ast.ValueTypeNode); ok {
			baseElementType = valueTypeNode.ValueType
		} else if identifierNode, ok := indexingExpr.Array.(*ast.IdentifierExprNode); ok {
			// Handle user-defined types (e.g., Point, MyClass) — resolve immediately so the
			// array class is created with ObjectType{*Class} element types rather than raw
			// UserDefinedType placeholders that the type checker cannot compare against
			// concrete ObjectType values.
			baseElementType = g.resolveTypeForIRGen(zeus_value.UserDefinedType{
				Name: identifierNode.Name.Value,
				Span: identifierNode.Name.Span,
			}, false)
		} else {
			g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "array base type must be a type name", indexingExpr.Array.GetSpan()))
			return nil
		}

		// 2. Build nested array type based on number of dimensions
		// e.g., u8 with 2 dimensions -> u8[][]
		numDimensions := len(indexingExpr.IndexingMeta.IndexingExprs)
		arrayType := baseElementType
		for i := 0; i < numDimensions; i++ {
			arrayType = zeus_value.NewArrayType(arrayType, indexingExpr.GetSpan())
		}

		// 3. Validate: only the first dimension can have a capacity expression
		// Capacity for first dimension is optional, but dimensions 2+ cannot have capacity
		for i := 1; i < numDimensions; i++ {
			if indexingExpr.IndexingMeta.IndexingExprs[i] != nil {
				g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "only the first dimension can specify capacity in array creation", indexingExpr.IndexingMeta.IndexingExprs[i].GetSpan()))
			}
		}

		// 4. Get or create the array primordial class for this type
		arrayTypeValue := arrayType.(zeus_value.ArrayType)
		arrayClass := g.getOrCreateArrayClass(arrayTypeValue)

		// 5. Evaluate capacity expression (only first dimension, if provided)
		var capacity zeus_value.Value
		if indexingExpr.IndexingMeta.IndexingExprs[0] != nil {
			capacity = indexingExpr.IndexingMeta.IndexingExprs[0].Accept(g)
		} else {
			// No capacity provided, pass 0 as default (runtime will use default capacity)
			capacity = zeus_value.NewConstant("0", zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: indexingExpr.GetSpan()}, indexingExpr.GetSpan())
		}
		args := []zeus_value.Value{capacity}

		// 6. Pass to BuildNewObj - it's just a class with constructor args!
		return g.irBuilder.BuildNewObj(arrayClass, args, expr.GetSpan())
	}

	// Class instantiation: new MyClass(args)
	calleeValue := callee.Accept(g)
	args := []zeus_value.Value{}
	for _, arg := range expr.Args {
		args = append(args, arg.Accept(g))
	}

	return g.irBuilder.BuildNewObj(calleeValue, args, expr.GetSpan())
}

func (g *IRModule) VisitExportStmt(stmt *ast.ExportStmtNode) {
	exportedValue := stmt.Expr.Accept(g)

	switch exportedValue := exportedValue.(type) {
	// track the exported values from the module
	// make the exported function module scoped to avoid conflicts between modules
	case *zeus_value.Function:
		g.exportedSymbols[exportedValue.Name] = exportedValue
	case *zeus_value.Class:
		g.exportedSymbols[exportedValue.Name] = exportedValue
	default:
		g.pushError(&zeus_error.ZeusError{
			Message: "cannot export non-function expression",
			Span:    stmt.GetSpan(),
		})
	}

	g.irBuilder.BuildExport(g.modulePath, exportedValue, stmt.GetSpan())
}

func (g *IRModule) VisitClassDeclExpr(expr *ast.ClassDeclExprNode) zeus_value.Value {
	// Top-level classes use their source name as the IR name (DeclCheckPass guarantees global
	// uniqueness at top level). Nested classes (inside a function body) can shadow outer names,
	// so we generate a unique IR name to avoid LLVM struct-name collisions in codegen.
	// Anonymous classes (no name) always get a generated unique name.
	var sourceName string
	if expr.Name != nil {
		sourceName = expr.Name.Name.Value
	} else {
		sourceName = anonymousName
	}
	var irClassName string
	if expr.Name != nil && (g.irBuilder.GetInsertionBlock() == nil || g.isInModuleScope) {
		irClassName = sourceName
	} else {
		irClassName = g.generateUniqueName(sourceName)
	}

	g.symbolTable().EnterScope()
	properties := []*zeus_value.ClassProperty{}
	for _, property := range expr.Properties {
		propType := g.resolveTypeForIRGen(property.ValueType.ValueType, false)
		propVar := zeus_value.NewVar(property.Name.Name.Value, propType, false, property.Name.GetSpan())
		g.symbolTable().DeclareSymbol(property.Name.Name.Value, propVar)
		properties = append(properties, zeus_value.NewClassProperty(propVar, property.AccessModifier, property.IsReadonly))
	}

	methods := []*zeus_value.ClassMethod{}
	accessors := []*zeus_value.ClassAccessor{}
	var regularMethodASTs []*ast.ClassMethod
	var accessorMethodASTs []*ast.ClassMethod

	for _, method := range expr.Methods {
		if method.Accessor != ast.AccessorKindNone {
			accessorMethodASTs = append(accessorMethodASTs, method)
			continue
		}
		if method.Name.Name.Value == token.CONSTRUCTOR_METHOD_NAME {
			if !zeus_value.IsVoidType(method.ReturnType.ValueType) {
				g.pushError(&zeus_error.ZeusError{
					Message: "constructor return type must be void",
					Span:    method.ReturnType.Span,
				})
			}
		}
		params := []*zeus_value.Var{}
		for _, param := range method.Params {
			paramType := g.resolveTypeForIRGen(param.ValueType.ValueType, false)
			if param.IsVariadic {
				g.checkVariadicParamType(param, paramType)
			}
			params = append(params, zeus_value.NewVar(param.Identifier.Name.Value, paramType, false, param.Identifier.Name.Span, param.IsVariadic))
		}

		methodReturnType := g.resolveTypeForIRGen(method.ReturnType.ValueType, true)
		function := zeus_value.NewFunction(
			method.Name.Name.Value,
			params,
			methodReturnType,
			method.Span,
		)
		function.IsVariadic = len(params) > 0 && params[len(params)-1].IsVariadic
		methods = append(methods, zeus_value.NewClassMethod(
			function,
			method.AccessModifier,
		))
		regularMethodASTs = append(regularMethodASTs, method)
	}

	// Build ClassAccessor stubs — function bodies are emitted in buildAccessors below.
	for _, method := range accessorMethodASTs {
		name := method.Name.Name.Value
		// Find or create an accessor entry for this name.
		var acc *zeus_value.ClassAccessor
		for _, a := range accessors {
			if a.Name == name {
				acc = a
				break
			}
		}
		if acc == nil {
			acc = zeus_value.NewClassAccessor(name, nil, nil, method.AccessModifier)
			accessors = append(accessors, acc)
		}

		params := []*zeus_value.Var{}
		for _, param := range method.Params {
			paramType := g.resolveTypeForIRGen(param.ValueType.ValueType, false)
			params = append(params, zeus_value.NewVar(param.Identifier.Name.Value, paramType, false, param.Identifier.Name.Span))
		}
		var returnType zeus_value.ValueType = zeus_value.VoidType{Span: method.Span}
		if method.ReturnType != nil {
			returnType = g.resolveTypeForIRGen(method.ReturnType.ValueType, true)
		}

		if method.Accessor == ast.AccessorKindGetter {
			fn := zeus_value.NewFunction("#get_"+name, params, returnType, method.Span)
			acc.Getter = fn
		} else {
			fn := zeus_value.NewFunction("#set_"+name, params, zeus_value.VoidType{Span: method.Span}, method.Span)
			acc.Setter = fn
		}
	}

	class := zeus_value.NewClass(irClassName, properties, methods, accessors, "", nil, expr.GetSpan())
	class.OriginalName = sourceName
	g.buildClass(class, regularMethodASTs)
	g.buildAccessors(class, accessorMethodASTs)
	g.symbolTable().ExitScope()

	// Register by source name so VisitIdentifier resolves correctly.
	// For top-level classes this overwrites the DeclCheckPass stub with the full class.
	// Anonymous classes register under their unique IR name so multiple anonymous classes
	// don't overwrite each other in the symbol table.
	registerName := sourceName
	if sourceName == anonymousName {
		registerName = irClassName
	}

	// Back-fill the DeclCheckPass stub's property/method types with the resolved ones.
	// The stub pointer (*stubClass) may have been embedded in ObjectType values during
	// self-referential property resolution (e.g. Node.next: Node → ObjectType{*stubClass}).
	// Those ObjectType instances remain valid since they hold the pointer, but the stub's
	// own property types are still raw UserDefinedType — update them so that accessing
	// properties through the stub returns the resolved type (e.g. current.next.next works).
	if oldVal, ok := g.symbolTable().GetSymbol(registerName); ok {
		if stubClass := zeus_value.AsClass(oldVal); stubClass != nil && stubClass != class {
			// Repoint the stub's members at the fully-resolved ones so the stub (which may
			// be embedded in self-referential ObjectType values) shares a single source of
			// truth. Sharing the pointers avoids per-field copying that silently drops
			// newly-added fields (e.g. a Function's IsVariadic, a Var's flags).
			for i, prop := range class.Properties {
				if i < len(stubClass.Properties) {
					stubClass.Properties[i].Property = prop.Property
				}
			}
			for i, method := range class.Methods {
				if i < len(stubClass.Methods) {
					stubClass.Methods[i].Method = method.Method
				}
			}
			for i, acc := range class.Accessors {
				if i < len(stubClass.Accessors) {
					stubClass.Accessors[i].Getter = acc.Getter
					stubClass.Accessors[i].Setter = acc.Setter
				}
			}
		}
	}

	g.symbolTable().DeclareSymbol(registerName, class)
	return class
}

// findAccessorInClass searches the class and its parent chain for an accessor by name.
func findAccessorInClass(class *zeus_value.Class, name string) *zeus_value.ClassAccessor {
	for class != nil {
		for _, acc := range class.Accessors {
			if acc.Name == name {
				return acc
			}
		}
		class = class.ParentClass
	}
	return nil
}

func (g *IRModule) VisitObjectPropertyAccessExpr(expr *ast.ObjectPropertyAccessExprNode) zeus_value.Value {
	object := expr.Object.Accept(g)
	property := expr.Property.Name.Value
	asVar := zeus_value.AsVar(object)

	// if the object is stored in a pointer variable then dereference it first
	if asVar != nil && asVar.IsPtr {
		object = g.irBuilder.BuildLoad(asVar, expr.GetSpan())
	}

	// Null check: if object is null, throw NullReferenceException
	// Skip null check for 'this' expressions since 'this' is never null inside a class
	if !g.isThisExpression(expr.Object) {
		g.emitNullCheck(object, property, expr.GetSpan())
	}

	// For non-this objects, check if the property is backed by a getter/setter accessor.
	// Inside getter/setter bodies, 'this.x' bypasses the accessor to avoid infinite recursion.
	if !g.isThisExpression(expr.Object) {
		objType := zeus_value.AsObjectType(zeus_value.GetValueType(object))
		if objType != nil {
			if acc := findAccessorInClass(objType.Class, property); acc != nil {
				if g.isLValueExpr {
					return zeus_value.NewAccessorLValue(object, property, expr.GetSpan())
				}
				if acc.Getter == nil {
					g.pushError(&zeus_error.ZeusError{
						Message: fmt.Sprintf("property '%s' is write-only (no getter)", property),
						Span:    expr.GetSpan(),
					})
					return nil
				}
				return g.irBuilder.BuildGetAccessor(object, property, expr.GetSpan())
			}
		}
	}

	propertyPtr := g.irBuilder.BuildObjectPropertyAccess(object, property, g.isLValueExpr, expr.GetSpan())

	if g.isLValueExpr {
		return propertyPtr
	} else {
		return g.irBuilder.BuildLoad(zeus_value.AsVar(propertyPtr), expr.GetSpan())
	}
}

func (g *IRModule) isThisExpression(expr ast.ExprNode) bool {
	if identExpr, ok := expr.(*ast.IdentifierExprNode); ok {
		return identExpr.Name.Value == token.THIS_KEYWORD
	}
	return false
}

// emitNullCheck generates IR to check if an object is null and throw an error if it is
func (g *IRModule) emitNullCheck(object zeus_value.Value, propertyName string, span *token.Span) {
	// Create blocks for null check
	throwBlock := g.irBuilder.BuildSuccessorBlock()
	continueBlock := g.irBuilder.BuildSuccessorBlock()

	// Create null constant for comparison
	nullConst := zeus_value.NewConstant(zeus_value.NULL_CONSTANT_VALUE, zeus_value.NullType{Span: span}, span)

	// Compare object with null (object == null)
	isNull := g.irBuilder.BuildBinaryOp(object, nullConst, InstrTypeEqEq, span)

	// If null, jump to throw block; otherwise continue
	g.irBuilder.BuildCondJmp(throwBlock, continueBlock, isNull, span)

	// Generate throw block: create Error and throw
	g.irBuilder.SetInsertionBlock(throwBlock)

	// Create and throw NullReferenceException
	errorName := "NullReferenceException"
	errorMessage := fmt.Sprintf("Cannot access property '%s' on null object", propertyName)
	g.emitThrowError(errorName, errorMessage, span)

	// Continue block: normal execution continues here
	g.irBuilder.SetInsertionBlock(continueBlock)
}

// getOrCreateErrorClass returns the Error primordial class from the symbol table.
func (g *IRModule) getOrCreateErrorClass() *zeus_value.Class {
	// Error class is registered with name "Error" (not the primordial constant "error")
	errorClassName := "Error"

	// Error class should always be pre-registered in the symbol table
	if existingClass, ok := g.symbolTable().GetSymbol(errorClassName); ok {
		return existingClass.(*zeus_value.Class)
	}

	// This should never happen - Error is registered during IRBuilder init
	zeus_error.Assert(false, "Error class not found in symbol table - this is a bug")
	return nil
}

// getOrCreateConsoleClass returns the Console primordial class, registering it if needed.
func (g *IRModule) getOrCreateConsoleClass() *zeus_value.Class {
	consoleClassName := zeus_value.ZEUS_PRIMORDIAL_CONSOLE

	if existingClass, ok := g.symbolTable().GetSymbol(consoleClassName); ok {
		return existingClass.(*zeus_value.Class)
	}

	consoleClass := zeus_value.Registry.GetClass(consoleClassName)
	zeus_error.Assert(consoleClass != nil, "Console class not found in registry - this is a bug")

	g.symbolTable().DeclareGlobalSymbol(consoleClassName, consoleClass)
	g.irBuilder.EmitClassDeclAtStart(consoleClass)

	return consoleClass
}

// initPrimordialGlobals creates the `console` global object inside the entry function body.
// Must be called while the insertion block is set to the entry function's body block
// and isInModuleScope is true.
func (g *IRModule) initPrimordialGlobals(span *token.Span) {
	consoleClass := g.getOrCreateConsoleClass()
	consoleObj := g.irBuilder.BuildNewObj(consoleClass, []zeus_value.Value{}, span)

	consoleVarDecl := NewVarDecl("console", zeus_value.NewObjectType(consoleClass), true, consoleObj, span)
	consoleVar := g.irBuilder.BuildGlobalVarDecl(consoleVarDecl)
	consoleVar.IsUsed = true // primordial global — suppress unused warning

	g.symbolTable().DeclareGlobalSymbol("console", consoleVar)
}

// emitBoundsCheck generates IR to check if an array index is within bounds and throw if not
func (g *IRModule) emitBoundsCheck(array zeus_value.Value, index zeus_value.Value, span *token.Span) {
	// Create blocks for bounds check
	throwBlock := g.irBuilder.BuildSuccessorBlock()
	continueBlock := g.irBuilder.BuildSuccessorBlock()

	i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: span}
	lengthPtr := g.irBuilder.BuildObjectPropertyAccess(array, zeus_value.ARRAY_PROPERTY_LENGTH, false, span)
	length := g.irBuilder.BuildLoad(zeus_value.AsVar(lengthPtr), span)

	// Check if index < 0
	zero := zeus_value.NewConstant("0", i32Type, span)
	isNegative := g.irBuilder.BuildBinaryOp(index, zero, InstrTypeLessThan, span)

	// Check if index >= length
	isOverflow := g.irBuilder.BuildBinaryOp(index, length, InstrTypeGreaterThanEq, span)

	// Combine: outOfBounds = isNegative || isOverflow
	outOfBounds := g.irBuilder.BuildBinaryOp(isNegative, isOverflow, InstrTypeOr, span)

	// If out of bounds, jump to throw block; otherwise continue
	g.irBuilder.BuildCondJmp(throwBlock, continueBlock, outOfBounds, span)

	// Generate throw block: create Error and throw
	g.irBuilder.SetInsertionBlock(throwBlock)

	// Create and throw IndexOutOfBoundsException
	errorName := "IndexOutOfBoundsException"
	errorMessage := "Array index out of bounds"
	g.emitThrowError(errorName, errorMessage, span)

	// Continue block: normal execution continues here
	g.irBuilder.SetInsertionBlock(continueBlock)
}

// emitThrowError creates an Error object with the given name and message and throws it
func (g *IRModule) emitThrowError(errorName string, errorMessage string, span *token.Span) {
	// Get the Error class from symbol table
	errorClass := g.getOrCreateErrorClass()

	// Get the string class and u8[] array class
	stringClass := g.getOrCreateStringClass()
	u8ArrayType := zeus_value.NewArrayType(zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: span}, span)
	u8ArrayClass := g.getOrCreateArrayClass(u8ArrayType)
	i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: span}

	// Create string object for error name
	nameStringObj := g.createStringObject(errorName, stringClass, u8ArrayClass, i32Type, span)

	// Create string object for error message
	messageStringObj := g.createStringObject(errorMessage, stringClass, u8ArrayClass, i32Type, span)

	// Create Error object with name and message
	errorObj := g.irBuilder.BuildNewObj(errorClass, []zeus_value.Value{nameStringObj, messageStringObj}, span)

	// Throw the error
	classId := errorClass.Id
	g.irBuilder.BuildThrow(classId, errorObj, g.modulePath, span)
}

// createStringObject creates a string object from a Go string
func (g *IRModule) createStringObject(str string, stringClass *zeus_value.Class, u8ArrayClass *zeus_value.Class, i32Type zeus_value.IntType, span *token.Span) zeus_value.Value {
	stringBytes := []byte(str)
	u8Array := g.irBuilder.BuildNewObj(u8ArrayClass, []zeus_value.Value{
		zeus_value.NewConstant(fmt.Sprintf("%d", len(stringBytes)), i32Type, span),
	}, span)

	// Set each byte in the array
	for i, b := range stringBytes {
		idx := zeus_value.NewConstant(fmt.Sprintf("%d", i), i32Type, span)
		byteVal := zeus_value.NewConstant(fmt.Sprintf("%d", b), zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: span}, span)
		g.irBuilder.BuildMethodCall(u8Array, zeus_value.ARRAY_METHOD_SET, []zeus_value.Value{idx, byteVal}, nil, nil, span)
	}

	// Create string object from u8[] array
	return g.irBuilder.BuildNewObj(stringClass, []zeus_value.Value{u8Array}, span)
}

func (g *IRModule) VisitImportStmt(stmt *ast.ImportStmtNode) {
	absoluteModulePath := module.ResolveFilePath(g.modulePath, stmt.Source.Value)
	irModule := g.getModule(absoluteModulePath)

	if irModule == nil {
		g.pushError(zeus_error.NewZeusError(
			zeus_error.ErrorSeverityError,
			fmt.Sprintf("module '%s' not found", stmt.Source.Value),
			stmt.Source.Span,
		))
		return
	}

	for _, _import := range stmt.Imports {
		importedValue, ok := irModule.GetExportedSymbol(_import.Name.Value)

		if !ok {
			g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("module '%s' does not export '%s'", stmt.Source.Value, _import.Name.Value), _import.Name.Span))
			return
		}

		g.irBuilder.BuildImport(absoluteModulePath, _import.Name.Value, importedValue, _import.Name.Span)
		g.symbolTable().DeclareSymbol(_import.Name.Value, importedValue)
	}
}

func (g *IRModule) VisitChar(expr *ast.CharExprNode) zeus_value.Value {
	zeus_error.Assert(utf8.RuneCount([]byte(expr.Value.Value)) == 1, "char literal must be a single character")

	firstRune, _ := utf8.DecodeRuneInString(expr.Value.Value)
	u8Value := byte(firstRune)
	u8ValueString := strconv.Itoa(int(u8Value))

	return zeus_value.NewConstant(
		u8ValueString,
		zeus_value.IntType{
			Size:   zeus_value.I8,
			Signed: false,
		},
		expr.Value.Span,
	)
}

// getOrCreateStringClass returns the string primordial class from the registry.
// The class is pre-registered during IRBuilder initialization, so this just looks it up.
func (g *IRModule) getOrCreateStringClass() *zeus_value.Class {
	stringClassName := zeus_value.ZEUS_PRIMORDIAL_STRING

	// String class should always be pre-registered in the symbol table
	if existingClass, ok := g.symbolTable().GetSymbol(stringClassName); ok {
		return existingClass.(*zeus_value.Class)
	}

	// This should never happen - string is registered during IRBuilder init
	zeus_error.Assert(false, "string class not found in symbol table - this is a bug")
	return nil
}

func (g *IRModule) VisitTemplateString(expr *ast.TemplateStringExprNode) zeus_value.Value {
	span := expr.GetSpan()

	var acc zeus_value.Value
	for _, part := range expr.Parts {
		var val zeus_value.Value
		if part.IsExpr {
			val = part.Expr.Accept(g)
		} else if part.Str != "" {
			val = g.VisitStringConstant(&ast.StringConstantExprNode{
				Value: &token.Token{Value: part.Str, Span: span},
			})
		} else {
			continue
		}
		if acc == nil {
			acc = val
		} else {
			acc = g.irBuilder.BuildBinaryOp(acc, val, InstrTypeAdd, span)
		}
	}

	if acc == nil {
		// All parts were empty static strings — return an empty string.
		return g.VisitStringConstant(&ast.StringConstantExprNode{
			Value: &token.Token{Value: "", Span: span},
		})
	}
	return acc
}

func (g *IRModule) VisitStringConstant(expr *ast.StringConstantExprNode) zeus_value.Value {
	// Get or create the string class
	stringClass := g.getOrCreateStringClass()

	// Get string bytes
	stringBytes := []byte(expr.Value.Value)
	stringLen := len(stringBytes)

	// Get or create u8[] array class
	u8ArrayType := zeus_value.NewArrayType(zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: expr.GetSpan()}, expr.GetSpan())
	u8ArrayClass := g.getOrCreateArrayClass(u8ArrayType)

	// Create u8[] array with capacity = string length
	capacity := zeus_value.NewConstant(fmt.Sprintf("%d", stringLen), zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: expr.GetSpan()}, expr.GetSpan())
	u8Array := g.irBuilder.BuildNewObj(u8ArrayClass, []zeus_value.Value{capacity}, expr.GetSpan())

	// Set each byte using array.set(index, byte)
	for i, b := range stringBytes {
		index := zeus_value.NewConstant(fmt.Sprintf("%d", i), zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: expr.GetSpan()}, expr.GetSpan())
		byteVal := zeus_value.NewConstant(fmt.Sprintf("%d", b), zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: expr.GetSpan()}, expr.GetSpan())
		g.irBuilder.BuildMethodCall(u8Array, zeus_value.ARRAY_METHOD_SET, []zeus_value.Value{index, byteVal}, nil, nil, expr.GetSpan())
	}

	// Create string object with the u8[] array
	return g.irBuilder.BuildNewObj(stringClass, []zeus_value.Value{u8Array}, expr.GetSpan())
}

func (g *IRModule) VisitValueType(expr *ast.ValueTypeNode) zeus_value.Value {
	zeus_error.Assert(false, "value type should not be emitted in the IR")
	return nil
}

// pushTryContext pushes a new try context onto the stack
func (g *IRModule) pushTryContext(handlerBlock *BasicBlock, classIds []int) {
	g.tryContextStack = append(g.tryContextStack, &TryContext{
		HandlerBlock: handlerBlock,
		ClassIds:     classIds,
	})
}

// popTryContext pops the current try context from the stack
func (g *IRModule) popTryContext() {
	if len(g.tryContextStack) > 0 {
		g.tryContextStack = g.tryContextStack[:len(g.tryContextStack)-1]
	}
}

// getClassIdFromType extracts the class ID from a type
func (g *IRModule) getClassIdFromType(valueType zeus_value.ValueType) int {
	switch t := valueType.(type) {
	case zeus_value.ObjectType:
		return t.Class.Id
	case *zeus_value.ObjectType:
		return t.Class.Id
	case zeus_value.UserDefinedType:
		// At IR generation time, catch clause types are still UserDefinedType
		// (ToKnownTypesPass hasn't run yet). Look up the class in the symbol table.
		sym, ok := g.symbolTable().GetSymbol(t.Name)
		zeus_error.Assert(ok, "getClassIdFromType: unresolved type '"+t.Name+"' not found in symbol table")
		class := zeus_value.AsClass(sym)
		zeus_error.Assert(class != nil, "getClassIdFromType: symbol '"+t.Name+"' is not a class")
		return class.Id
	default:
		return 0
	}
}

// VisitTryCatchStmt generates IR for try-catch statements
func (g *IRModule) VisitTryCatchStmt(stmt *ast.TryCatchStmtNode) {
	// 1. Create blocks for try body, handler (catch dispatch), and merge (after try-catch)
	tryBodyBlock := g.irBuilder.BuildSuccessorBlock()
	handlerBlock := g.irBuilder.BuildSuccessorBlock()
	mergeBlock := g.irBuilder.BuildSuccessorBlock()

	// 2. Collect class IDs from all catch clauses
	classIds := []int{}
	for _, clause := range stmt.CatchClauses {
		// The type checker should have validated that this is an Error class or subclass
		classId := g.getClassIdFromType(clause.ErrorType.ValueType)
		classIds = append(classIds, classId)
	}

	// 3. Push handler at try block entry - this emits setjmp and conditional branch
	// Will branch to tryBodyBlock if setjmp returns 0, handlerBlock if returns 1
	g.irBuilder.BuildPushHandler(handlerBlock, tryBodyBlock, classIds, stmt.GetSpan())

	// 4. Switch to try body block
	g.irBuilder.SetInsertionBlock(tryBodyBlock)

	// 5. Mark that we're in a try block (affects call generation)
	g.pushTryContext(handlerBlock, classIds)

	// 6. Generate try body
	stmt.TryBody.Accept(g)

	// 7. Pop handler and jump to merge block at end of try block (if no exception)
	g.irBuilder.BuildPopHandler(stmt.GetSpan())
	g.irBuilder.BuildJmp(mergeBlock, stmt.GetSpan())

	// 7. Exit try context
	g.popTryContext()

	// 8. Generate exception handler block
	g.irBuilder.SetInsertionBlock(handlerBlock)

	// Generate catch clause matching logic
	// For now, we only support a single catch clause per try block
	// Multiple catch clauses would need proper instanceof checking
	for i, clause := range stmt.CatchClauses {
		classId := classIds[i]
		_ = classId // Will be used for instanceof check in future

		// Create block for this catch clause body
		catchBodyBlock := g.irBuilder.BuildSuccessorBlock()

		// For now, jump directly to the catch body
		// TODO: Add instanceof check for multiple catch clauses
		g.irBuilder.BuildJmp(catchBodyBlock, clause.GetSpan())

		// Generate catch body
		g.irBuilder.SetInsertionBlock(catchBodyBlock)

		// Enter a new scope for the catch variable
		g.symbolTable().EnterScope()

		// Get the exception with the expected type
		catchType := g.resolveTypeForIRGen(clause.ErrorType.ValueType, false)
		exception := g.irBuilder.BuildGetException(catchType, clause.GetSpan())

		// Declare the catch variable (e.g., 'e' in catch (e: Error))
		errorVar := zeus_value.NewVar(
			clause.ErrorVar.Name.Value,
			catchType,
			true,
			clause.GetSpan(),
		)
		g.symbolTable().DeclareSymbol(clause.ErrorVar.Name.Value, errorVar)

		// Store the exception object in the catch variable
		declInput := NewDeclareVarInstrInput(errorVar, exception, false)
		g.irBuilder.pushInstr(&Instr{
			Type:  InstrTypeDeclVar,
			Input: declInput,
			Span:  clause.GetSpan(),
		})

		// Clear the exception (it's been caught)
		g.irBuilder.BuildClearException(clause.GetSpan())

		// Generate catch body statements
		clause.Body.Accept(g)

		// Exit catch variable scope
		g.symbolTable().ExitScope()

		// Jump to merge block after catch body
		g.irBuilder.BuildJmp(mergeBlock, clause.GetSpan())

		// Note: For multiple catch clauses, we'd need instanceof checks here
		// For now, we only support a single catch clause
		break
	}

	// 9. Continue at merge block
	g.irBuilder.SetInsertionBlock(mergeBlock)
}

// VisitThrowStmt generates IR for throw statements
func (g *IRModule) VisitThrowStmt(stmt *ast.ThrowStmtNode) {
	// Evaluate the exception expression (must be Error or subclass)
	exceptionValue := stmt.Expr.Accept(g)

	// Get the class ID from the evaluated value
	// The type checker should have validated this is an Error class or subclass
	classId := 0
	if exceptionVar, ok := exceptionValue.(*zeus_value.Var); ok && exceptionVar.ValueType != nil {
		classId = g.getClassIdFromType(exceptionVar.ValueType)
	} else if _, ok := exceptionValue.(zeus_value.Object); ok {
		// For new expressions, the result is an Object
		objectValue := exceptionValue.(zeus_value.Object)
		classId = objectValue.ValueType.Class.Id
	}

	// Build the THROW instruction
	g.irBuilder.BuildThrow(classId, exceptionValue, g.modulePath, stmt.GetSpan())
}

// resolveTypeForIRGen converts raw AST type annotations to concrete IR types.
// It must be called at every point where a type annotation appears in the source.
//
//   - UserDefinedType{"Foo"}  → ObjectType{*FooClass}  (symbol table lookup)
//   - ArrayType{T}            → ObjectType{*T[]Class}  (getOrCreateArrayClass)
//   - FunctionType{…}         → same shape with resolved param/return types
//   - Primitive types         → unchanged
func (g *IRModule) resolveTypeForIRGen(t zeus_value.ValueType, isReturnType bool) zeus_value.ValueType {
	if t == nil {
		return t
	}

	switch typ := t.(type) {
	case zeus_value.UserDefinedType:
		sym, ok := g.symbolTable().GetSymbol(typ.Name)
		if !ok {
			g.errors = append(g.errors, zeus_error.NewZeusError(
				zeus_error.ErrorSeverityError,
				fmt.Sprintf("undefined type '%s'", typ.Name),
				typ.Span,
			))
			return zeus_value.UndefinedType{Span: typ.Span}
		}
		cls := zeus_value.AsClass(sym)
		if cls == nil {
			g.errors = append(g.errors, zeus_error.NewZeusError(
				zeus_error.ErrorSeverityError,
				fmt.Sprintf("'%s' is not a type", typ.Name),
				typ.Span,
			))
			return zeus_value.UndefinedType{Span: typ.Span}
		}
		return zeus_value.NewObjectType(cls)

	case zeus_value.ArrayType:
		resolvedElement := g.resolveTypeForIRGen(typ.ElementType, false)
		if zeus_value.IsUndefinedType(resolvedElement) {
			return resolvedElement
		}
		arrayType := zeus_value.NewArrayType(resolvedElement, typ.Span)
		cls := g.getOrCreateArrayClass(arrayType)
		return zeus_value.NewObjectType(cls)

	case zeus_value.FunctionType:
		resolved := typ
		if typ.ReturnType != nil {
			resolved.ReturnType = g.resolveTypeForIRGen(typ.ReturnType, true)
		}
		for i, p := range typ.ParamTypes {
			if p != nil {
				resolved.ParamTypes[i] = g.resolveTypeForIRGen(p, false)
			}
		}
		return resolved

	default:
		return t
	}
}
