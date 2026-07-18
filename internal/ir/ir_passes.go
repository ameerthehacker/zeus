package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/module"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// IRGenPass is a pass that runs over the AST before IR emission.
// Passes run in order; if a pass returns errors, subsequent passes are skipped.
type IRGenPass interface {
	Name() string
	Run(g *IRModule, program *ast.ProgramNode) []*zeus_error.ZeusError
}

// IREmitPass visits every AST statement and emits IR instructions.
// It runs after DeclCheckPass, so it can assume declarations are unique and
// that top-level stubs are already registered in the global registry.
type IREmitPass struct{}

func NewIREmitPass() *IREmitPass { return &IREmitPass{} }

func (p *IREmitPass) Name() string { return "IREmitPass" }

func (p *IREmitPass) Run(g *IRModule, program *ast.ProgramNode) []*zeus_error.ZeusError {
	defaultSpan := token.NewSpan(*token.NewPosition(1, 1), *token.NewPosition(1, 1))

	// Emit primordial functions
	for _, fn := range zeus_value.Registry.GetAllFunctions() {
		g.irBuilder.BuildDeclPrimordialFunc(fn, fn.Span)
	}

	// Every module (entry and non-entry) wraps its top-level *initializer* statements into a
	// per-module init function ($module_init$...). Function/class/interface declarations still
	// emit globally — their visitors save/restore the insertion point, so they land at module
	// scope rather than inside this body. The entry point's `main` calls each module's init in
	// dependency order at startup (see emitFunction / the synthesized main below).
	initBody := g.irBuilder.BuildBasicBlock()
	voidType := zeus_value.ValueType(zeus_value.VoidType{Span: defaultSpan})
	g.irBuilder.BuildFuncDecl(module.ModuleInitFuncName(g.modulePath), []*VarDecl{}, initBody, voidType, nil, defaultSpan)
	g.irBuilder.SetInsertionBlock(initBody)
	g.isInModuleScope = true

	// Visit user top-level statements; module-level var decls become globals, function/class
	// decls are emitted to b.instrs via emitFunction's save/restore.
	for _, stmt := range program.Statements {
		stmt.Accept(g)
	}
	g.isInModuleScope = false
	g.irBuilder.BuildReturn(nil, defaultSpan) // module init returns void
	g.irBuilder.SetInsertionBlock(nil)

	if g.isEntryPoint {
		// `main` is the program's OS entry point. A user-defined `main` was already emitted with the
		// module-init dispatch injected as its first statements and marked IsOSEntry (see
		// emitFunction). If the user did not define one, synthesize a `main` that runs the dispatch
		// and returns 0.
		userMainDefined := false
		for _, instr := range g.irBuilder.GetInstrs() {
			if IsFunctionDeclInstr(instr.Type) && AsDeclFuncInstrInput(instr.Input).Function.Name == token.MAIN_FUNCTION_NAME {
				userMainDefined = true
				break
			}
		}

		if !userMainDefined {
			i32Type := zeus_value.ValueType(zeus_value.IntType{Size: zeus_value.I32, Signed: true, Span: defaultSpan})
			mainBody := g.irBuilder.BuildBasicBlock()
			mainFn := g.irBuilder.BuildFuncDecl(token.MAIN_FUNCTION_NAME, []*VarDecl{}, mainBody, i32Type, nil, defaultSpan)
			mainFn.IsOSEntry = true
			g.irBuilder.SetInsertionBlock(mainBody)
			g.emitModuleInitDispatch(defaultSpan)
			g.irBuilder.BuildReturn(zeus_value.NewConstant("0", i32Type, defaultSpan), defaultSpan)
			g.irBuilder.SetInsertionBlock(nil)
		}
	}

	g.irBuilder.Optimize()
	return g.errors
}

// PrescanAmbientGlobals registers a module's top-level `global` declarations up front — before any
// module is IR-generated — so a `global` referenced in a module compiled before its definer still
// resolves, and so cross-module duplicate definitions are reported once. Types come from the
// annotation when present; unannotated globals register as undefined and are refined to their
// concrete type when the defining module is IR-generated (see RefineAmbientGlobalType).
func PrescanAmbientGlobals(program *ast.ProgramNode, modulePath string) []*zeus_error.ZeusError {
	var errors []*zeus_error.ZeusError
	for _, stmt := range program.Statements {
		varDeclStmt, ok := stmt.(*ast.VarDeclStmtNode)
		if !ok {
			continue
		}
		for _, decl := range varDeclStmt.Decls {
			if decl.DeclType != ast.VarDeclTypeGlobal {
				continue
			}
			name := decl.Identifier.Name.Value
			var valueType zeus_value.ValueType = zeus_value.UndefinedType{Span: decl.Identifier.Name.Span}
			if decl.ValueType != nil {
				valueType = decl.ValueType.ValueType
			}
			if err := zeus_value.RegisterAmbientGlobalDef(name, modulePath, valueType, decl.Identifier.Name.Span); err != nil {
				errors = append(errors, zeus_error.NewZeusError(zeus_error.ErrorSeverityError, err.Error(), decl.Identifier.Name.Span))
			}
		}
	}
	return errors
}

// DeclCheckPass walks the full AST, validates declaration uniqueness at every
// scope level, and pre-registers top-level function/class stubs in the IRBuilder
// global registry so forward references resolve during IR emission.
//
// It uses the existing SymbolTable to track scopes — the value stored is the
// declaration span, used only for error reporting.
type DeclCheckPass struct {
	st     *symbol_table.SymbolTable[*token.Span]
	errors []*zeus_error.ZeusError
}

func NewDeclCheckPass() *DeclCheckPass { return &DeclCheckPass{} }

func (p *DeclCheckPass) Name() string { return "DeclCheckPass" }

func (p *DeclCheckPass) Run(g *IRModule, program *ast.ProgramNode) []*zeus_error.ZeusError {
	p.st = symbol_table.NewSymbolTable[*token.Span]()
	p.errors = nil
	p.st.EnterScope()
	for _, stmt := range program.Statements {
		p.walkStmt(g, stmt, true)
	}
	p.st.ExitScope()
	return p.errors
}

// checkAndDeclare registers name in the current scope.
// Reports an error and returns false if the name is already declared there.
func (p *DeclCheckPass) checkAndDeclare(name string, span *token.Span) bool {
	if _, exists := p.st.GetSymbolInCurrentScope(name); exists {
		p.errors = append(p.errors, zeus_error.NewZeusError(
			zeus_error.ErrorSeverityError,
			fmt.Sprintf("cannot redeclare identifier '%s' in the same scope", name),
			span,
		))
		return false
	}
	p.st.DeclareSymbol(name, span)
	return true
}

func (p *DeclCheckPass) walkStmt(g *IRModule, stmt ast.StmtNode, isTopLevel bool) {
	switch s := stmt.(type) {
	case *ast.ExprStmtNode:
		p.walkExpr(g, s.Expr, isTopLevel)

	case *ast.VarDeclStmtNode:
		for _, decl := range s.Decls {
			classExpr, isClass := decl.Initializer.(*ast.ClassDeclExprNode)
			if isClass {
				if decl.DeclType != ast.VarDeclTypeConst {
					p.errors = append(p.errors, zeus_error.NewZeusError(
						zeus_error.ErrorSeverityError,
						"class declarations must use const",
						decl.Identifier.Name.Span,
					))
				}
				if classExpr.Name == nil {
					// const X = class {} → anonymous: promote, use var name as class name
					classExpr.Name = decl.Identifier
					p.walkExpr(g, classExpr, isTopLevel)
				} else {
					// const X = class MyClass {} → both X and MyClass become class names
					p.checkAndDeclare(decl.Identifier.Name.Value, decl.Identifier.Name.Span)
					p.walkExpr(g, classExpr, isTopLevel)
				}
				continue
			}
			p.checkAndDeclare(decl.Identifier.Name.Value, decl.Identifier.Name.Span)
			if decl.Initializer != nil {
				p.walkExpr(g, decl.Initializer, false)
			}
		}

	case *ast.BlockStmtNode:
		p.st.EnterScope()
		for _, inner := range s.Statements {
			p.walkStmt(g, inner, false)
		}
		p.st.ExitScope()

	case *ast.IfStmtNode:
		if s.Condition != nil {
			p.walkExpr(g, s.Condition, false)
		}
		p.walkStmt(g, s.ThenStmt, false)
		if s.ElseStmt != nil {
			p.walkStmt(g, s.ElseStmt, false)
		}

	case *ast.WhileStmtNode:
		if s.Condition != nil {
			p.walkExpr(g, s.Condition, false)
		}
		p.walkStmt(g, s.Body, false)

	case *ast.ForStmtNode:
		p.st.EnterScope()
		if s.Init != nil {
			p.walkStmt(g, s.Init, false)
		}
		if s.Condition != nil {
			p.walkExpr(g, s.Condition, false)
		}
		if s.Update != nil {
			p.walkExpr(g, s.Update, false)
		}
		p.walkStmt(g, s.Body, false)
		p.st.ExitScope()

	case *ast.ReturnStmtNode:
		if s.Expr != nil {
			p.walkExpr(g, s.Expr, false)
		}

	case *ast.ExportStmtNode:
		p.walkExpr(g, s.Expr, isTopLevel)

	case *ast.TryCatchStmtNode:
		p.walkStmt(g, s.TryBody, false)
		for _, clause := range s.CatchClauses {
			p.st.EnterScope()
			p.checkAndDeclare(clause.ErrorVar.Name.Value, clause.ErrorVar.GetSpan())
			p.walkStmt(g, clause.Body, false)
			p.st.ExitScope()
		}

	case *ast.ThrowStmtNode:
		if s.Expr != nil {
			p.walkExpr(g, s.Expr, false)
		}

	case *ast.ImportStmtNode, *ast.BreakStmtNode, *ast.ContinueStmtNode:
		// no declarations to check
	}
}

// walkExpr handles the two expression types that introduce declarations.
// All other expression types are leaves for declaration-checking purposes.
func (p *DeclCheckPass) walkExpr(g *IRModule, expr ast.ExprNode, isTopLevel bool) {
	switch e := expr.(type) {
	case *ast.FunctionDeclExprNode:
		fnName := e.Name

		if fnName != nil {
			ok := p.checkAndDeclare(fnName.Name.Value, e.Name.GetSpan())
			if ok && isTopLevel {
				p.registerFunctionStub(g, e)
			}
		} else if isTopLevel {
			// we cannot have anonymous functions on the top level
			// since they can never be assiged to a variable they never can be called
			p.errors = append(p.errors, zeus_error.NewZeusError(
				zeus_error.ErrorSeverityError,
				"cannot declare anonymous functions in the global scope",
				expr.GetSpan(),
			))
		}

		p.st.EnterScope()
		for _, param := range e.Params {
			p.checkAndDeclare(param.Identifier.Name.Value, param.Identifier.Name.Span)
		}
		// Extern functions (prelude/primordial) have no body.
		if e.Body != nil {
			p.walkStmt(g, e.Body, false)
		}
		p.st.ExitScope()

	case *ast.ClassDeclExprNode:
		if e.Name != nil {
			ok := p.checkAndDeclare(e.Name.Name.Value, e.Name.GetSpan())
			if ok && isTopLevel {
				p.registerClassStub(g, e)
			}
		} else if isTopLevel {
			p.errors = append(p.errors, zeus_error.NewZeusError(
				zeus_error.ErrorSeverityError,
				"cannot declare anonymous classes in the global scope",
				expr.GetSpan(),
			))
		}
		p.st.EnterScope()
		for _, prop := range e.Properties {
			p.checkAndDeclare(prop.Name.Name.Value, prop.Name.GetSpan())
		}
		for _, method := range e.Methods {
			// Use mangled names for accessors so getter+setter pair for the same name
			// can coexist without triggering the redeclaration check.
			declName := method.Name.Name.Value
			switch method.Accessor {
			case ast.AccessorKindGetter:
				declName = "#get_" + declName
			case ast.AccessorKindSetter:
				declName = "#set_" + declName
			}
			p.checkAndDeclare(declName, method.Name.GetSpan())
			p.st.EnterScope()
			for _, param := range method.Params {
				p.checkAndDeclare(param.Identifier.Name.Value, param.Identifier.Name.Span)
			}
			// Extern methods have no body (they forward to a runtime symbol).
			if method.Body != nil {
				p.walkStmt(g, method.Body, false)
			}
			p.st.ExitScope()
		}
		p.st.ExitScope()

	case *ast.InterfaceDeclExprNode:
		if e.Name != nil {
			ok := p.checkAndDeclare(e.Name.Name.Value, e.Name.GetSpan())
			if ok && isTopLevel {
				p.registerInterfaceStub(g, e)
			}
		} else if isTopLevel {
			p.errors = append(p.errors, zeus_error.NewZeusError(
				zeus_error.ErrorSeverityError,
				"cannot declare anonymous interfaces in the global scope",
				expr.GetSpan(),
			))
		}
		p.st.EnterScope()
		for _, prop := range e.Properties {
			p.checkAndDeclare(prop.Name.Name.Value, prop.Name.GetSpan())
		}
		for _, method := range e.Methods {
			p.checkAndDeclare(method.Name.Name.Value, method.Name.GetSpan())
		}
		p.st.ExitScope()
	}
}

func (p *DeclCheckPass) registerFunctionStub(g *IRModule, expr *ast.FunctionDeclExprNode) {
	name := expr.Name.Name.Value
	params := make([]*zeus_value.Var, 0, len(expr.Params))
	for _, param := range expr.Params {
		params = append(params, zeus_value.NewVar(
			param.Identifier.Name.Value,
			param.ValueType.ValueType,
			false,
			param.Identifier.Name.Span,
		))
	}
	var returnType zeus_value.ValueType = zeus_value.VoidType{Span: expr.GetSpan()}
	if expr.ReturnType != nil {
		returnType = expr.ReturnType.ValueType
	}
	fn := zeus_value.NewFunction(name, params, returnType, expr.Name.Name.Span)
	fn.OriginalName = name

	// A user extern("C",...) is a body-less, module-level function whose wrapper must be emitted at
	// module scope (currentBlock == nil). IREmitPass emits every registered primordial function at
	// the top of the module, BEFORE visiting statements — so the C-extern must be in the Registry by
	// then. Registering it only when its statement is visited (VisitFunctionDeclExpr) is too late and
	// mis-nests the wrapper into #_zeus_main. Pre-register it here, in DeclCheckPass (a pre-pass).
	if expr.IsCExtern {
		fn.OriginalName = ""
		fn.ExternRuntimeName = expr.ExternSymbol
		fn.IsCExtern = true
		zeus_value.Registry.RegisterFunction(fn)
	}

	g.irBuilder.symbolTable.DeclareGlobalSymbol(name, fn)
}

func (p *DeclCheckPass) registerClassStub(g *IRModule, expr *ast.ClassDeclExprNode) {
	name := expr.Name.Name.Value
	properties := make([]*zeus_value.ClassProperty, 0, len(expr.Properties))
	for _, prop := range expr.Properties {
		v := zeus_value.NewVar(prop.Name.Name.Value, prop.ValueType.ValueType, false, prop.Name.GetSpan())
		// StaticGlobalVar stays nil here — back-filled by VisitClassDeclExpr before method bodies are emitted
		cp := zeus_value.NewClassProperty(v, prop.AccessModifier, prop.IsReadonly, prop.IsStatic, nil)
		properties = append(properties, cp)
	}
	methods := make([]*zeus_value.ClassMethod, 0)
	accessors := make([]*zeus_value.ClassAccessor, 0)
	for _, method := range expr.Methods {
		if method.Accessor != ast.AccessorKindNone {
			// Build accessor stub
			accName := method.Name.Name.Value
			var acc *zeus_value.ClassAccessor
			for _, a := range accessors {
				if a.Name == accName {
					acc = a
					break
				}
			}
			if acc == nil {
				acc = zeus_value.NewClassAccessor(accName, nil, nil, method.AccessModifier)
				acc.IsStatic = method.IsStatic
				accessors = append(accessors, acc)
			}
			mParams := make([]*zeus_value.Var, 0, len(method.Params))
			for _, mp := range method.Params {
				mParams = append(mParams, zeus_value.NewVar(mp.Identifier.Name.Value, mp.ValueType.ValueType, false, mp.Identifier.Name.Span))
			}
			var returnType zeus_value.ValueType = zeus_value.VoidType{Span: method.Span}
			if method.ReturnType != nil {
				returnType = method.ReturnType.ValueType
			}
			fn := zeus_value.NewFunction("#get_"+accName, mParams, returnType, method.Name.Name.Span)
			if method.Accessor == ast.AccessorKindGetter {
				acc.Getter = fn
			} else {
				fn.Name = "#set_" + accName
				acc.Setter = fn
			}
			continue
		}
		mParams := make([]*zeus_value.Var, 0, len(method.Params))
		for _, mp := range method.Params {
			mParams = append(mParams, zeus_value.NewVar(
				mp.Identifier.Name.Value,
				mp.ValueType.ValueType,
				false,
				mp.Identifier.Name.Span,
			))
		}
		var returnType zeus_value.ValueType = zeus_value.VoidType{Span: method.Span}
		if method.ReturnType != nil {
			returnType = method.ReturnType.ValueType
		}
		fn := zeus_value.NewFunction(method.Name.Name.Value, mParams, returnType, method.Name.Name.Span)
		fn.OriginalName = method.Name.Name.Value
		cm := zeus_value.NewClassMethod(fn, method.AccessModifier)
		cm.IsStatic = method.IsStatic
		methods = append(methods, cm)
	}
	// Wire up parent class so the stub's parent chain is valid during static method body emission.
	var parentClass *zeus_value.Class
	if expr.ParentClass != nil {
		if parentVal, ok := g.irBuilder.symbolTable.GetSymbol(expr.ParentClass.Name.Value); ok {
			parentClass = zeus_value.AsClass(parentVal)
		}
	}
	var class *zeus_value.Class
	if parentClass != nil {
		class = zeus_value.NewClassWithParent(name, parentClass, properties, methods, accessors, "", nil, expr.GetSpan())
	} else {
		class = zeus_value.NewClass(name, properties, methods, accessors, "", nil, expr.GetSpan())
	}
	class.OriginalName = name
	g.irBuilder.symbolTable.DeclareGlobalSymbol(name, class)
}

// registerInterfaceStub registers an empty interface under its name so forward and
// self type references resolve to InterfaceType{stub}. The stub's members are filled
// in later by VisitInterfaceDeclExpr (which mutates this same pointer in place).
func (p *DeclCheckPass) registerInterfaceStub(g *IRModule, expr *ast.InterfaceDeclExprNode) {
	name := expr.Name.Name.Value
	iface := zeus_value.NewInterface(name, nil, nil, nil, expr.GetSpan())
	g.irBuilder.symbolTable.DeclareGlobalSymbol(name, iface)
}
