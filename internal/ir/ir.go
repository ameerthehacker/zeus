package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/module"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/util"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

type IRModule struct {
	irBuilder *IRBuilder
	isLValueExpr bool
	symbolTable *symbol_table.SymbolTable[zeus_value.Value]
	errors []*zeus_error.ZeusError
	modulePath string
	exportedSymbols map[string]zeus_value.Value
	getModule func(modulePath string) *IRModule
}

func NewIRModule(ir_builder *IRBuilder, modulePath string, getIRModule func(modulePath string) *IRModule) *IRModule {
	return &IRModule{
		irBuilder: ir_builder,
		isLValueExpr: false,
		symbolTable: symbol_table.NewSymbolTable[zeus_value.Value](),
		modulePath: modulePath,
		exportedSymbols: map[string]zeus_value.Value{},
		getModule: getIRModule,
	}
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

func (g *IRModule) Generate(program *ast.ProgramNode) []*zeus_error.ZeusError {
	g.symbolTable.EnterScope()
	for _, stmt := range program.Statements {
		stmt.Accept(g)
	}
	g.symbolTable.ExitScope()
	g.irBuilder.Optimize()

	return g.errors
}

func (g *IRModule) VisitBlockStmt(stmt *ast.BlockStmtNode) { 
	g.symbolTable.EnterScope()
	for _, stmt := range stmt.Statements {
		stmt.Accept(g)
	}
	g.symbolTable.ExitScope()
}

func (g *IRModule) VisitVarDeclStmt(stmt *ast.VarDeclStmtNode) {
	for _, decl := range stmt.Decls {
		if _, ok := g.symbolTable.GetSymbolInCurrentScope(decl.Identifier.Name.Value); ok {
			g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("cannot redeclare identifier '%s' in the same scope", decl.Identifier.Name.Value), decl.Identifier.Name.Span))
			return
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
			zeus_value.ToValueType(decl.DataType),
			isConst,
			initializer,
			decl.Identifier.Name.Span,
		))

		g.symbolTable.DeclareSymbol(decl.Identifier.Name.Value, variable)
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
	g.irBuilder.BuildCondJmp(then_block, else_block, condition, stmt.Condition.GetSpan())

	// generate the then block
	g.irBuilder.SetInsertionBlock(then_block)
	stmt.ThenStmt.Accept(g)
	// jump to the merge block
	g.irBuilder.BuildJmp(merge_block, nil)
	
	// generate the else block
	if stmt.ElseStmt != nil {
		g.irBuilder.SetInsertionBlock(else_block)
		stmt.ElseStmt.Accept(g)
	}

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

func (g *IRModule) VisitBinaryExpr(expr *ast.BinaryExprNode) zeus_value.Value {
	g.isLValueExpr = expr.Operator.Type == token.TokenTypeEqual
	left := expr.Left.Accept(g)
	g.isLValueExpr = false
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
		Span: expr.GetSpan(),
	})

	return nil
}

func (g *IRModule) emitFunction(name string, fnParams []*ast.VarDeclNode, returnType zeus_value.ValueType, fnBody *ast.BlockStmtNode, class *zeus_value.Class, span *token.Span) zeus_value.Value {
	params := []*VarDecl{}

	for _, param := range fnParams {
		params = append(params, NewVarDecl(
			param.Identifier.Name.Value,
			zeus_value.ToValueType(param.DataType),
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
	g.symbolTable.DeclareGlobalSymbol(name, fn)
	g.symbolTable.EnterScope()

	for index, param := range fnParams {
		if _, ok := g.symbolTable.GetSymbolInCurrentScope(param.Identifier.Name.Value); ok {
			g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("cannot redeclare parameter '%s' in the same scope", param.Identifier.Name.Value), param.Identifier.Name.Span))
			return nil
		}
		g.symbolTable.DeclareSymbol(param.Identifier.Name.Value, fn.Params[index])
	}

	if class != nil {
		classType := zeus_value.NewClassType(*class)
		object := zeus_value.NewObject(token.THIS_KEYWORD, classType, span)
		g.symbolTable.DeclareSymbol(token.THIS_KEYWORD, &object)
	}

	g.irBuilder.SetInsertionBlock(body)
	fnBody.Accept(g)
	g.irBuilder.SetInsertionBlock(current_block)

	g.symbolTable.ExitScope()

	return fn
}

func (g *IRModule) VisitFunctionDeclExpr(expr *ast.FunctionDeclExprNode) zeus_value.Value {
	return g.emitFunction(expr.Name.Name.Value, expr.Params, zeus_value.ToValueType(expr.ReturnType), expr.Body, nil, expr.Name.Name.Span)
}

func (g *IRModule) VisitIdentifier(expr *ast.IdentifierExprNode) zeus_value.Value {
	variable, ok := g.symbolTable.GetSymbol(expr.Name.Value)

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

	panic(fmt.Sprintf("unknown identifier type: %s", expr.Name.Value))
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
				Size: zeus_value.GetSignedIntSize(expr.Value.Value),
			},
			expr.Value.Span,
		)
	}
}

func (g *IRModule) VisitUnaryExpr(expr *ast.UnaryExprNode) zeus_value.Value {
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
	callee := expr.Callee.Accept(g)
	args := []zeus_value.Value{}
	for _, arg := range expr.Args {
		args = append(args, arg.Accept(g))
	}

	return g.irBuilder.BuildNewObj(callee, args, expr.GetSpan())
}

func (g *IRModule) VisitExportStmt(stmt *ast.ExportStmtNode) {
	exportedValue := stmt.Expr.Accept(g)

	switch exportedValue := exportedValue.(type) {
	// track the exported values from the module
	// make the exported function module scoped to avoid conflicts between modules
	case *zeus_value.Function:
		exportedValueName := exportedValue.Name
		moduleScopedName := module.GetModuleScopedName(g.modulePath, exportedValueName)
		g.exportedSymbols[moduleScopedName] = exportedValue
		exportedValue.Name = moduleScopedName
	case *zeus_value.Class:
		exportedValueName := exportedValue.Name
		moduleScopedName := module.GetModuleScopedName(g.modulePath, exportedValueName)
		g.exportedSymbols[moduleScopedName] = exportedValue
		exportedValue.Name = moduleScopedName
	default:
		g.pushError(&zeus_error.ZeusError{
			Message: "cannot export non-function expression",
			Span: stmt.GetSpan(),
		})
	}

	g.irBuilder.BuildExport(exportedValue, stmt.GetSpan())
}

func (g *IRModule) VisitClassDeclExpr(expr *ast.ClassDeclExprNode) zeus_value.Value {
	properties := []*zeus_value.ClassProperty{}
	for _, property := range expr.Properties {
		properties = append(properties, zeus_value.NewClassProperty(zeus_value.NewVar(property.Name.Name.Value, zeus_value.ToValueType(property.ValueType), false, property.Name.GetSpan()), property.AccessModifier))
	}

	methods := []*zeus_value.ClassMethod{}
	for _, method := range expr.Methods {
		if method.Name.Name.Value == token.CONSTRUCTOR_METHOD_NAME {
			if method.ReturnType.Type != token.TokenTypeVoid {
				g.pushError(&zeus_error.ZeusError{
					Message: "constructor return type must be void",
					Span:    method.ReturnType.Span,
				})
				fmt.Println(method.ReturnType.Span)
			}
		}
		params := []*zeus_value.Var{}
		for _, param := range method.Params {
			params = append(params, zeus_value.NewVar(param.Identifier.Name.Value, zeus_value.ToValueType(param.DataType), false, param.Identifier.Name.Span))
		}

		function := zeus_value.NewFunction(
			method.Name.Name.Value,
			params,
			zeus_value.ToValueType(method.ReturnType),
			method.Span,
		)
		methods = append(methods, zeus_value.NewClassMethod(
			function,
			method.AccessModifier,
		))
	}

	class := zeus_value.NewClass(expr.Name.Name.Value, properties, methods, expr.GetSpan())
	irClassName := g.irBuilder.BuildClassDecl(class, expr.GetSpan())
	g.symbolTable.DeclareSymbol(expr.Name.Name.Value, class)

	for _, method := range expr.Methods {
		// emit global class methods
		g.emitFunction(util.GetClassMethodName(irClassName, method.Name.Name.Value), method.Params, zeus_value.ToValueType(method.ReturnType), method.Body, class, method.Name.Name.Span)
	}

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
	propertyPtr := g.irBuilder.BuildObjectPropertyAccess(object, property, expr.GetSpan())

	if g.isLValueExpr {
		return propertyPtr
	} else {
		return g.irBuilder.BuildLoad(zeus_value.AsVar(propertyPtr), expr.GetSpan())
	}
}


func (g *IRModule) VisitImportStmt(stmt *ast.ImportStmtNode) {
	absoluteModulePath := module.ResolveFilePath(g.modulePath, stmt.Source.Value)
	irModule := g.getModule(absoluteModulePath)

	zeus_error.Assert(irModule != nil, fmt.Sprintf("IR module %s not found", absoluteModulePath))

	for _, _import := range stmt.Imports {
		symbolName := module.GetModuleScopedName(absoluteModulePath, _import.Name.Value)
		importedValue, ok := irModule.GetExportedSymbol(symbolName)

		if !ok {
			g.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("module '%s' does not export '%s'", stmt.Source.Value, _import.Name.Value), _import.Name.Span))
			return
		}

		g.irBuilder.BuildImport(_import.Name.Value, importedValue, _import.Name.Span)
		g.symbolTable.DeclareSymbol(_import.Name.Value, importedValue)
	}
}
