package ir

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/module"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/util"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// TryContext holds information about the current try block being processed
type TryContext struct {
	HandlerBlock *BasicBlock // The exception handler block
	ClassIds     []int       // Class IDs of catch clauses
}

type IRModule struct {
	irBuilder       *IRBuilder
	isLValueExpr    bool
	errors          []*zeus_error.ZeusError
	modulePath      string
	exportedSymbols map[string]zeus_value.Value
	getModule       func(modulePath string) *IRModule
	tryContextStack []*TryContext // Stack of try contexts for nested try blocks
}

func NewIRModule(ir_builder *IRBuilder, modulePath string, getIRModule func(modulePath string) *IRModule) *IRModule {
	return &IRModule{
		irBuilder:       ir_builder,
		isLValueExpr:    false,
		modulePath:      modulePath,
		exportedSymbols: map[string]zeus_value.Value{},
		getModule:       getIRModule,
	}
}

// symbolTable returns the IRBuilder's symbol table (single source of truth)
func (g *IRModule) symbolTable() *symbol_table.SymbolTable[zeus_value.Value] {
	return g.irBuilder.symbolTable
}

func (g *IRModule) pushError(err *zeus_error.ZeusError) {
	g.errors = append(g.errors, err)
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
	// Note: IRBuilder already has a global scope created in NewIRBuilder()
	// and primordial classes are already registered via initializePrimordials()

	// Emit DECL_PRIMORDIAL_FUNC instructions for primordial functions
	// (they were registered in symbol table during NewIRBuilder, but we still need IR instructions)
	for _, fn := range zeus_value.Registry.GetAllFunctions() {
		g.irBuilder.BuildDeclPrimordialFunc(fn, fn.Span)
	}

	for _, stmt := range program.Statements {
		stmt.Accept(g)
	}
	g.irBuilder.Optimize()

	return g.errors
}

func (g *IRModule) isSymbolDeclared(name string, span *token.Span) bool {
	if _, ok := g.symbolTable().GetSymbolInCurrentScope(name); ok {
		g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("cannot redeclare identifier '%s' in the same scope", name), span))
		return true
	}

	return false
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
		if g.isSymbolDeclared(decl.Identifier.Name.Value, decl.Identifier.Name.Span) {
			continue
		}

		var initializer zeus_value.Value
		isConst := false

		if decl.Initializer != nil {
			initializer = decl.Initializer.Accept(g)
		}

		if decl.DeclType == ast.VarDeclTypeConst {
			isConst = true
		}

		variable := g.irBuilder.BuildVarDecl(NewVarDecl(
			decl.Identifier.Name.Value,
			decl.ValueType.ValueType,
			isConst,
			initializer,
			decl.GetSpan(),
		))

		g.symbolTable().DeclareSymbol(decl.Identifier.Name.Value, variable)
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
	stmt.Body.Accept(g)
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
	stmt.Body.Accept(g)
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

func (g *IRModule) isCompoundAssignment(tokenType token.TokenType) bool {
	return tokenType == token.TokenTypePlusEqual ||
		tokenType == token.TokenTypeMinusEqual ||
		tokenType == token.TokenTypeStarEqual ||
		tokenType == token.TokenTypeSlashEqual ||
		tokenType == token.TokenTypePercentEqual
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
			// For array[i] += value, we need to:
			// 1. Get current value: temp = array.get(i)
			// 2. Compute new value: temp = temp op value
			// 3. Store back: array.set(i, temp)
			currentValue := g.irBuilder.BuildMethodCall(arrayRef.ArrayObject, zeus_value.ARRAY_METHOD_GET,
				[]zeus_value.Value{arrayRef.Index},
				nil, // type will be inferred
				[]zeus_value.ValueType{zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: expr.GetSpan()}},
				expr.GetSpan())

			op := g.getCompoundAssignmentOp(expr.Operator.Type)
			newValue := g.irBuilder.BuildBinaryOp(currentValue, right, op, expr.GetSpan())

			i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: expr.GetSpan()}
			valueType := zeus_value.GetValueType(newValue)
			g.irBuilder.BuildMethodCall(arrayRef.ArrayObject, zeus_value.ARRAY_METHOD_SET,
				[]zeus_value.Value{arrayRef.Index, newValue},
				zeus_value.VoidType{Span: expr.GetSpan()},
				[]zeus_value.ValueType{i32Type, valueType},
				expr.GetSpan())

			return newValue
		}

		// Regular variable compound assignment
		addr := zeus_value.AsVar(left)
		if addr == nil {
			panic(fmt.Sprintf("invalid lvalue for compound assignment: %s", left))
		}

		// Load current value, apply operation, store back
		currentValue := g.irBuilder.BuildLoad(addr, expr.GetSpan())
		op := g.getCompoundAssignmentOp(expr.Operator.Type)
		newValue := g.irBuilder.BuildBinaryOp(currentValue, right, op, expr.GetSpan())
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
		// Power operation requires f64 operands in LLVM
		// Insert CAST instructions if operands are not f64
		f64Type := zeus_value.FloatType{Size: zeus_value.F64, Span: expr.GetSpan()}
		leftType := zeus_value.GetValueType(left)
		rightType := zeus_value.GetValueType(right)

		if !zeus_value.IsFloatType(leftType) || zeus_value.AsFloatType(leftType).Size != zeus_value.F64 {
			left = g.irBuilder.BuildCast(left, f64Type, expr.GetSpan())
		}
		if !zeus_value.IsFloatType(rightType) || zeus_value.AsFloatType(rightType).Size != zeus_value.F64 {
			right = g.irBuilder.BuildCast(right, f64Type, expr.GetSpan())
		}

		return g.irBuilder.BuildBinaryOp(left, right, InstrTypePower, expr.GetSpan())
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
	case token.TokenTypeEqual:
		// Check if this is an array element assignment (array[0][1] = expr)
		if arrayRef := zeus_value.AsArrayElementRef(left); arrayRef != nil {
			// Generate: arrayObject.set(index, value)
			// Type checking will catch invalid assignments (e.g., to strings)
			i32Type := zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: expr.GetSpan()}
			valueType := zeus_value.GetValueType(right)
			g.irBuilder.BuildMethodCall(arrayRef.ArrayObject, zeus_value.ARRAY_METHOD_SET,
				[]zeus_value.Value{arrayRef.Index, right},
				zeus_value.VoidType{Span: expr.GetSpan()},
				[]zeus_value.ValueType{i32Type, valueType},
				expr.GetSpan())

			// Return the value that was set
			return right
		}

		// Regular variable assignment
		addr := zeus_value.AsVar(left)
		if addr == nil {
			panic(fmt.Sprintf("invalid lvalue: %s", left))
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

func (g *IRModule) VisitFunctionCallExpr(expr *ast.FunctionCallExprNode) zeus_value.Value {
	callee := expr.Callee.Accept(g)
	params := []zeus_value.Value{}
	for _, arg := range expr.Params {
		params = append(params, arg.Accept(g))
	}

	if zeus_value.IsFunction(callee) {
		return g.irBuilder.BuildCallFunc(zeus_value.AsFunction(callee), params, expr.GetSpan())
	} else if zeus_value.IsVar(callee) {
		addr := zeus_value.AsVar(callee)
		return g.irBuilder.BuildIndirectFuncCall(addr, params, expr.GetSpan())
	}

	g.pushError(&zeus_error.ZeusError{
		Message: fmt.Sprintf("%s is not callable", expr.Callee.PrettyString()),
		Span:    expr.GetSpan(),
	})

	return nil
}

func (g *IRModule) emitFunction(name string, fnParams []*ast.VarDeclNode, returnType zeus_value.ValueType, fnBody *ast.BlockStmtNode, class *zeus_value.Class, span *token.Span) zeus_value.Value {
	params := []*VarDecl{}

	for _, param := range fnParams {
		params = append(params, NewVarDecl(
			param.Identifier.Name.Value,
			param.ValueType.ValueType,
			true,
			nil,
			param.Identifier.Name.Span,
		))
	}

	current_block := g.irBuilder.GetInsertionBlock()
	// functions are global
	g.irBuilder.SetInsertionBlock(nil)
	body := g.irBuilder.BuildBasicBlock()
	fn := g.irBuilder.BuildFuncDecl(name, params, body, returnType, class, span)
	g.symbolTable().DeclareGlobalSymbol(name, fn)
	g.symbolTable().EnterScope()

	for index, param := range fnParams {
		if _, ok := g.symbolTable().GetSymbolInCurrentScope(param.Identifier.Name.Value); ok {
			g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("cannot redeclare parameter '%s' in the same scope", param.Identifier.Name.Value), param.Identifier.Name.Span))
			return nil
		}
		g.symbolTable().DeclareSymbol(param.Identifier.Name.Value, fn.Params[index])
	}

	if class != nil {
		valueType := zeus_value.NewObjectType(*class)
		object := zeus_value.NewObject(token.THIS_KEYWORD, valueType, span)
		g.symbolTable().DeclareSymbol(token.THIS_KEYWORD, &object)
	}

	g.irBuilder.SetInsertionBlock(body)
	fnBody.Accept(g)
	g.irBuilder.SetInsertionBlock(current_block)

	g.symbolTable().ExitScope()

	return fn
}

func (g *IRModule) VisitFunctionDeclExpr(expr *ast.FunctionDeclExprNode) zeus_value.Value {
	return g.emitFunction(expr.Name.Name.Value, expr.Params, expr.ReturnType.ValueType, expr.Body, nil, expr.Name.Name.Span)
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

		addr := zeus_value.AsVar(target)
		if addr == nil {
			panic(fmt.Sprintf("invalid target for prefix %s: %s", expr.Operator.Type, target))
		}

		// Load current value
		currentValue := g.irBuilder.BuildLoad(addr, expr.Operator.Span)

		// Use u8 for the constant so it is implicitly promoted to whatever numeric type
		// the operand has (int or float). addr.ValueType is nil for class field pointers
		// at IR generation time (resolved later by tcObjectPropertyAccess).
		one := zeus_value.NewConstant("1", zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: expr.Operator.Span}, expr.Operator.Span)

		// Apply increment/decrement
		var newValue zeus_value.Value
		if expr.Operator.Type == token.TokenTypePlusPlus {
			newValue = g.irBuilder.BuildBinaryOp(currentValue, one, InstrTypeAdd, expr.GetSpan())
		} else {
			newValue = g.irBuilder.BuildBinaryOp(currentValue, one, InstrTypeSub, expr.GetSpan())
		}

		// Store new value
		g.irBuilder.BuildStore(addr, newValue, expr.GetSpan())

		// Prefix returns the new value
		return g.irBuilder.BuildLoad(addr, expr.GetSpan())
	}

	value := expr.Expr.Accept(g)

	switch expr.Operator.Type {
	case token.TokenTypeMinus:
		return g.irBuilder.BuildUnaryOp(value, InstrTypeNeg, expr.Operator.Span)
	case token.TokenTypeBang:
		return g.irBuilder.BuildUnaryOp(value, InstrTypeNot, expr.Operator.Span)
	default:
		panic(fmt.Sprintf("unknown unary operator: %s", expr.Operator.Type))
	}
}

func (g *IRModule) VisitPostfixExpr(expr *ast.PostfixExprNode) zeus_value.Value {
	// Get the address of the variable
	g.isLValueExpr = true
	target := expr.Expr.Accept(g)
	g.isLValueExpr = false

	addr := zeus_value.AsVar(target)
	if addr == nil {
		panic(fmt.Sprintf("invalid target for postfix %s: %s", expr.Operator.Type, target))
	}

	// Load current value (this is what we'll return)
	currentValue := g.irBuilder.BuildLoad(addr, expr.Operator.Span)

	// Use u8 for the constant so it is implicitly promoted to whatever numeric type
	// the operand has (int or float). addr.ValueType is nil for class field pointers
	// at IR generation time (resolved later by tcObjectPropertyAccess).
	one := zeus_value.NewConstant("1", zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: expr.Operator.Span}, expr.Operator.Span)

	// Apply increment/decrement
	var newValue zeus_value.Value
	if expr.Operator.Type == token.TokenTypePlusPlus {
		newValue = g.irBuilder.BuildBinaryOp(currentValue, one, InstrTypeAdd, expr.GetSpan())
	} else {
		newValue = g.irBuilder.BuildBinaryOp(currentValue, one, InstrTypeSub, expr.GetSpan())
	}

	// Store new value
	g.irBuilder.BuildStore(addr, newValue, expr.GetSpan())

	// Postfix returns the OLD value (before increment/decrement)
	return currentValue
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

	// Register in symbol table
	g.symbolTable().DeclareSymbol(arrayClassName, arrayClass)

	// Emit DECL_CLASS at the beginning of the instruction list
	g.irBuilder.EmitClassDeclAtStart(arrayClass)

	return arrayClass
}

// buildClass builds the IR for a class declaration and registers it in the symbol table
// For user-defined classes, pass methodASTs to emit method bodies
// For primordial classes, pass nil for methodASTs (methods are implemented in runtime)
func (g *IRModule) buildClass(class *zeus_value.Class, methodASTs []*ast.ClassMethod) string {
	irClassName := g.irBuilder.BuildClassDecl(class, class.GetSpan())
	g.symbolTable().DeclareSymbol(class.Name, class)

	// Emit method bodies if AST nodes are provided (user-defined classes)
	for _, method := range methodASTs {
		g.emitFunction(util.GetClassMethodName(irClassName, method.Name.Name.Value), method.Params, method.ReturnType.ValueType, method.Body, class, method.Name.Name.Span)
	}

	return irClassName
}

func (g *IRModule) VisitIndexingExpression(expr *ast.IndexingExprNode) zeus_value.Value {
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
			// Handle user-defined types (e.g., Point, MyClass)
			baseElementType = zeus_value.UserDefinedType{
				Name: identifierNode.Name.Value,
				Span: identifierNode.Name.Span,
			}
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
	if g.isSymbolDeclared(expr.Name.Name.Value, expr.Name.GetSpan()) {
		return nil
	}

	g.symbolTable().EnterScope()
	properties := []*zeus_value.ClassProperty{}
	for _, property := range expr.Properties {
		if g.isSymbolDeclared(property.Name.Name.Value, property.Name.GetSpan()) {
			continue
		}
		g.symbolTable().DeclareSymbol(property.Name.Name.Value, zeus_value.NewVar(property.Name.Name.Value, property.ValueType.ValueType, false, property.Name.GetSpan()))

		properties = append(properties, zeus_value.NewClassProperty(zeus_value.NewVar(property.Name.Name.Value, property.ValueType.ValueType, false, property.Name.GetSpan()), property.AccessModifier))
	}

	methods := []*zeus_value.ClassMethod{}
	seenMethods := map[string]bool{}
	for _, method := range expr.Methods {
		if seenMethods[method.Name.Name.Value] {
			g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("cannot redeclare identifier '%s' in the same scope", method.Name.Name.Value), method.Name.GetSpan()))
			continue
		}
		seenMethods[method.Name.Name.Value] = true

		if method.Name.Name.Value == token.CONSTRUCTOR_METHOD_NAME {
			if !zeus_value.IsVoidType(method.ReturnType.ValueType) {
				g.pushError(&zeus_error.ZeusError{
					Message: "constructor return type must be void",
					Span:    method.ReturnType.Span,
				})
				fmt.Println(method.ReturnType.Span)
			}
		}
		params := []*zeus_value.Var{}
		for _, param := range method.Params {
			params = append(params, zeus_value.NewVar(param.Identifier.Name.Value, param.ValueType.ValueType, false, param.Identifier.Name.Span))
		}

		function := zeus_value.NewFunction(
			method.Name.Name.Value,
			params,
			method.ReturnType.ValueType,
			method.Span,
		)
		methods = append(methods, zeus_value.NewClassMethod(
			function,
			method.AccessModifier,
		))
	}

	class := zeus_value.NewClass(expr.Name.Name.Value, properties, methods, "", nil, expr.GetSpan())
	g.buildClass(class, expr.Methods)
	g.symbolTable().ExitScope()

	// Re-declare the class in the outer scope after exiting the class members scope
	g.symbolTable().DeclareSymbol(expr.Name.Name.Value, class)
	return class
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

	propertyPtr := g.irBuilder.BuildObjectPropertyAccess(object, property, g.isLValueExpr, expr.GetSpan())

	if g.isLValueExpr {
		return propertyPtr
	} else {
		return g.irBuilder.BuildLoad(zeus_value.AsVar(propertyPtr), expr.GetSpan())
	}
}

// isThisExpression checks if an expression is a 'this' reference
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

// emitBoundsCheck generates IR to check if an array index is within bounds and throw if not
func (g *IRModule) emitBoundsCheck(array zeus_value.Value, index zeus_value.Value, span *token.Span) {
	// Create blocks for bounds check
	throwBlock := g.irBuilder.BuildSuccessorBlock()
	continueBlock := g.irBuilder.BuildSuccessorBlock()

	// Get array length
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
		setMethodPtr := g.irBuilder.BuildObjectPropertyAccess(u8Array, zeus_value.ARRAY_METHOD_SET, false, span)
		setMethod := g.irBuilder.BuildLoad(zeus_value.AsVar(setMethodPtr), span)
		idx := zeus_value.NewConstant(fmt.Sprintf("%d", i), i32Type, span)
		byteVal := zeus_value.NewConstant(fmt.Sprintf("%d", b), zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: span}, span)
		g.irBuilder.BuildIndirectFuncCall(setMethod, []zeus_value.Value{idx, byteVal}, span)
	}

	// Create string object from u8[] array
	return g.irBuilder.BuildNewObj(stringClass, []zeus_value.Value{u8Array}, span)
}

func (g *IRModule) VisitImportStmt(stmt *ast.ImportStmtNode) {
	absoluteModulePath := module.ResolveFilePath(g.modulePath, stmt.Source.Value)
	irModule := g.getModule(absoluteModulePath)

	zeus_error.Assert(irModule != nil, fmt.Sprintf("IR module %s not found", absoluteModulePath))

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
		// Get the .set() method from the array object
		setMethodPtr := g.irBuilder.BuildObjectPropertyAccess(u8Array, zeus_value.ARRAY_METHOD_SET, false, expr.GetSpan())
		setMethod := g.irBuilder.BuildLoad(zeus_value.AsVar(setMethodPtr), expr.GetSpan())

		// Create index and byte constants
		index := zeus_value.NewConstant(fmt.Sprintf("%d", i), zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: expr.GetSpan()}, expr.GetSpan())
		byteVal := zeus_value.NewConstant(fmt.Sprintf("%d", b), zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: expr.GetSpan()}, expr.GetSpan())

		// Call array.set(index, byte)
		g.irBuilder.BuildIndirectFuncCall(setMethod, []zeus_value.Value{index, byteVal}, expr.GetSpan())
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

// getCurrentTryContext returns the current try context or nil if not in a try block
func (g *IRModule) getCurrentTryContext() *TryContext {
	if len(g.tryContextStack) > 0 {
		return g.tryContextStack[len(g.tryContextStack)-1]
	}
	return nil
}

// getClassIdFromType extracts the class ID from a type
func (g *IRModule) getClassIdFromType(valueType zeus_value.ValueType) int {
	switch t := valueType.(type) {
	case zeus_value.ObjectType:
		return t.Class.Id
	case *zeus_value.ObjectType:
		return t.Class.Id
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
		exception := g.irBuilder.BuildGetException(clause.ErrorType.ValueType, clause.GetSpan())

		// Declare the catch variable (e.g., 'e' in catch (e: Error))
		errorVar := zeus_value.NewVar(
			clause.ErrorVar.Name.Value,
			clause.ErrorType.ValueType,
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
