package ir_test

import (
	"testing"

	"strings"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/module"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// generateHIR parses source code and generates HIR without type checking or lowering.
func generateHIR(t *testing.T, source string) (*ir.IRBuilder, []*ir.Instr) {
	t.Helper()
	l := lexer.NewLexer(source)
	tokens, errs := l.Lex()
	if len(errs) > 0 {
		t.Fatalf("lexer errors: %v", errs)
	}
	p := parser.NewParser(tokens)
	program, errs := p.ParseProgram()
	if len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	builder := ir.NewIRBuilder()
	mod := ir.NewIRModule(builder, "test.zs", false, nil)
	irErrors := mod.Generate(program)
	if len(irErrors) > 0 {
		t.Fatalf("IR generation errors: %v", irErrors)
	}
	return builder, builder.GetInstrs()
}

// filterUserInstrs removes primordial class and function declarations that are always
// emitted at the head of every IRBuilder. It also transparently inlines the synthetic
// per-module init function ($module_init$...) that now wraps every module's top-level
// statements: the DECL_FUNC wrapper is dropped and its body's instructions are spliced in,
// so callers see top-level user code as a flat list (as before the init-function refactor).
func filterUserInstrs(instrs []*ir.Instr) []*ir.Instr {
	var result []*ir.Instr
	for _, instr := range instrs {
		if instr.Type == ir.InstrTypeDeclClass {
			class := ir.AsDeclClassInstrInput(instr.Input).Class
			if class.PrimordialName != "" {
				continue
			}
		}
		if instr.Type == ir.InstrTypeDeclPrimordialFunc {
			continue
		}
		if instr.Type == ir.InstrTypeDeclFunc {
			input := ir.AsDeclFuncInstrInput(instr.Input)
			if strings.HasPrefix(input.Function.Name, module.ModuleInitFuncPrefix) {
				// Splice the module-init body in place of its synthetic wrapper, dropping
				// the trailing void RETURN the wrapper always ends with.
				for _, bodyInstr := range allBlockInstrs(input.Body) {
					if bodyInstr.Type == ir.InstrTypeReturn {
						continue
					}
					result = append(result, bodyInstr)
				}
				continue
			}
		}
		result = append(result, instr)
	}
	return result
}

// findInstrs returns all instructions of the given type from a flat list.
func findInstrs(instrs []*ir.Instr, instrType ir.InstrType) []*ir.Instr {
	var result []*ir.Instr
	for _, i := range instrs {
		if i.Type == instrType {
			result = append(result, i)
		}
	}
	return result
}

// getFuncEntryBlock returns the entry BasicBlock of the named top-level function.
func getFuncEntryBlock(builder *ir.IRBuilder, name string) *ir.BasicBlock {
	for _, instr := range builder.GetInstrs() {
		if instr.Type == ir.InstrTypeDeclFunc {
			input := ir.AsDeclFuncInstrInput(instr.Input)
			if input.Function.Name == name {
				return input.Body
			}
		}
	}
	return nil
}

// allBlockInstrs collects all instructions from a block and all reachable successors
// using BFS. Use this to inspect all instructions inside a function body.
func allBlockInstrs(block *ir.BasicBlock) []*ir.Instr {
	if block == nil {
		return nil
	}
	visited := map[int]bool{}
	var result []*ir.Instr
	queue := []*ir.BasicBlock{block}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur.Id] {
			continue
		}
		visited[cur.Id] = true
		result = append(result, cur.Instrs...)
		queue = append(queue, cur.Successors...)
	}
	return result
}

// findPropertyAccess finds OBJECT_PROPERTY_ACCESS instructions for the given property name.
func findPropertyAccess(instrs []*ir.Instr, propertyName string) []*ir.Instr {
	var result []*ir.Instr
	for _, instr := range instrs {
		if instr.Type == ir.InstrTypeObjectPropertyAccess {
			input := ir.AsObjectPropertyAccessInstrInput(instr.Input)
			if input.Property == propertyName {
				result = append(result, instr)
			}
		}
	}
	return result
}

// findMethodCalls finds CALL_METHOD instructions for the given method name.
func findMethodCalls(instrs []*ir.Instr, methodName string) []*ir.Instr {
	var result []*ir.Instr
	for _, instr := range instrs {
		if instr.Type == ir.InstrTypeMethodCall {
			input := ir.AsMethodCallInstrInput(instr.Input)
			if input.MethodName == methodName {
				result = append(result, instr)
			}
		}
	}
	return result
}

// mustConstant asserts that v is a Constant with the given value string.
func mustConstant(t *testing.T, v zeus_value.Value, want string) {
	t.Helper()
	c := zeus_value.AsConstant(v)
	if c == nil {
		t.Fatalf("expected constant '%s', got %T(%v)", want, v, v)
	}
	if c.Value != want {
		t.Errorf("expected constant '%s', got '%s'", want, c.Value)
	}
}

// ---- Variable Declaration Tests ----

func TestVarDeclLet(t *testing.T) {
	// A top-level `let`/`const` is a module-scoped variable, so it lowers to DECLARE_GLOBAL_VAR.
	_, instrs := generateHIR(t, `let x: i32 = 5`)
	decls := findInstrs(filterUserInstrs(instrs), ir.InstrTypeDeclGlobalVar)
	if len(decls) == 0 {
		t.Fatal("expected DECLARE_GLOBAL_VAR instruction")
	}
	input := ir.AsDeclGlobalVarInstrInput(decls[0].Input)
	if input.Variable.Name != "x" {
		t.Errorf("expected variable name 'x', got '%s'", input.Variable.Name)
	}
	if input.IsConst {
		t.Error("expected IsConst=false for 'let' declaration")
	}
	mustConstant(t, input.Initializer, "5")
}

func TestVarDeclConst(t *testing.T) {
	_, instrs := generateHIR(t, `const y: i32 = 10`)
	decls := findInstrs(filterUserInstrs(instrs), ir.InstrTypeDeclGlobalVar)
	if len(decls) == 0 {
		t.Fatal("expected DECLARE_GLOBAL_VAR instruction")
	}
	input := ir.AsDeclGlobalVarInstrInput(decls[0].Input)
	if input.Variable.Name != "y" {
		t.Errorf("expected variable name 'y', got '%s'", input.Variable.Name)
	}
	if !input.IsConst {
		t.Error("expected IsConst=true for 'const' declaration")
	}
}

func TestVarDeclWithoutInit(t *testing.T) {
	_, instrs := generateHIR(t, `let z: i32;`)
	decls := findInstrs(filterUserInstrs(instrs), ir.InstrTypeDeclGlobalVar)
	if len(decls) == 0 {
		t.Fatal("expected DECLARE_GLOBAL_VAR instruction")
	}
	input := ir.AsDeclGlobalVarInstrInput(decls[0].Input)
	if input.Variable.Name != "z" {
		t.Errorf("expected variable name 'z', got '%s'", input.Variable.Name)
	}
	if input.Initializer != nil {
		t.Errorf("expected nil initializer, got %v", input.Initializer)
	}
}

func TestVarDeclNoTypeNoInitializerIsError(t *testing.T) {
	l := lexer.NewLexer(`
function f(): void {
    let x;
}`)
	tokens, _ := l.Lex()
	program, _ := parser.NewParser(tokens).ParseProgram()
	builder := ir.NewIRBuilder()
	mod := ir.NewIRModule(builder, "test.zs", false, nil)
	irErrors := mod.Generate(program)
	for _, err := range irErrors {
		if err.Severity == zeus_error.ErrorSeverityError && err.Message != "" {
			return
		}
	}
	t.Error("expected an IR gen error for 'let x;' with no type annotation and no initializer")
}

// ---- Type Inference Tests ----

// runTC generates HIR, runs all TC passes (without lowering), and returns the builder.
func runTC(t *testing.T, source string) *ir.IRBuilder {
	t.Helper()
	builder, _ := generateHIR(t, source)
	tc := ir.NewTypeChecker(builder, false)
	tcErrors := tc.TypeCheck()
	for _, err := range tcErrors {
		if err.Severity == zeus_error.ErrorSeverityError {
			t.Fatalf("type check error: %v", err)
		}
	}
	return builder
}

// findDeclVar returns the DECLARE_VAR instruction for the named variable inside a function body.
func findDeclVar(t *testing.T, builder *ir.IRBuilder, funcName, varName string) *ir.Instr {
	t.Helper()
	block := getFuncEntryBlock(builder, funcName)
	if block == nil {
		t.Fatalf("function '%s' not found", funcName)
	}
	for _, instr := range allBlockInstrs(block) {
		if instr.Type == ir.InstrTypeDeclVar {
			if ir.AsDeclVarInstrInput(instr.Input).Variable.Name == varName {
				return instr
			}
		}
	}
	t.Fatalf("DECLARE_VAR for '%s' not found in '%s'", varName, funcName)
	return nil
}

func TestVarDeclInferredTypeFromInt(t *testing.T) {
	builder := runTC(t, `
function f(): void {
    let x = 5
}`)
	instr := findDeclVar(t, builder, "f", "x")
	varDecl := ir.AsDeclVarInstrInput(instr.Input)
	intType, ok := varDecl.Variable.ValueType.(zeus_value.IntType)
	if !ok {
		t.Fatalf("expected IntType after inference, got %T", varDecl.Variable.ValueType)
	}
	// Context-less integer literals default to a signed int, floored at i32.
	if !intType.Signed {
		t.Error("expected signed int inferred from literal 5")
	}
	if intType.Size != zeus_value.I32 {
		t.Errorf("expected i32 default for literal 5, got size %v", intType.Size)
	}
}

func TestVarDeclExplicitTypeRespected(t *testing.T) {
	builder := runTC(t, `
function f(): void {
    let x: i64 = 5
}`)
	instr := findDeclVar(t, builder, "f", "x")
	varDecl := ir.AsDeclVarInstrInput(instr.Input)
	intType, ok := varDecl.Variable.ValueType.(zeus_value.IntType)
	if !ok {
		t.Fatalf("expected IntType, got %T", varDecl.Variable.ValueType)
	}
	if intType.Size != zeus_value.I64 || !intType.Signed {
		t.Errorf("expected i64, got size=%v signed=%v", intType.Size, intType.Signed)
	}
}

func TestVarDeclInferredTypeFromFloat(t *testing.T) {
	builder := runTC(t, `
function f(): void {
    let x = 3.14
}`)
	instr := findDeclVar(t, builder, "f", "x")
	varDecl := ir.AsDeclVarInstrInput(instr.Input)
	floatType, ok := varDecl.Variable.ValueType.(zeus_value.FloatType)
	if !ok {
		t.Fatalf("expected FloatType after inference, got %T", varDecl.Variable.ValueType)
	}
	if floatType.Size != zeus_value.F64 {
		t.Errorf("expected f64, got %v", floatType.Size)
	}
}

func TestVarDeclInferredTypeFromBool(t *testing.T) {
	builder := runTC(t, `
function f(): void {
    let x = true
}`)
	instr := findDeclVar(t, builder, "f", "x")
	varDecl := ir.AsDeclVarInstrInput(instr.Input)
	if _, ok := varDecl.Variable.ValueType.(zeus_value.BoolType); !ok {
		t.Fatalf("expected BoolType after inference, got %T", varDecl.Variable.ValueType)
	}
}

// ---- Arithmetic Operator Tests ----

func TestArithmeticOperators(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		instrType ir.InstrType
		left      string
		right     string
	}{
		{"add", `let r: i32 = 1 + 2`, ir.InstrTypeAdd, "1", "2"},
		{"sub", `let r: i32 = 5 - 3`, ir.InstrTypeSub, "5", "3"},
		{"mul", `let r: i32 = 2 * 4`, ir.InstrTypeMul, "2", "4"},
		{"div", `let r: i32 = 10 / 2`, ir.InstrTypeDiv, "10", "2"},
		{"mod", `let r: i32 = 7 % 3`, ir.InstrTypeMod, "7", "3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, instrs := generateHIR(t, tt.source)
			ops := findInstrs(filterUserInstrs(instrs), tt.instrType)
			if len(ops) != 1 {
				t.Fatalf("expected 1 %s instruction, got %d", tt.instrType, len(ops))
			}
			input := ir.AsBinaryOpInstrInput(ops[0].Input)
			mustConstant(t, input.Left, tt.left)
			mustConstant(t, input.Right, tt.right)
		})
	}
}

func TestArithmeticPower(t *testing.T) {
	// Use float literals to avoid implicit CAST promotion
	_, instrs := generateHIR(t, `let r: f64 = 2.0 ** 3.0`)
	powers := findInstrs(filterUserInstrs(instrs), ir.InstrTypePower)
	if len(powers) != 1 {
		t.Fatalf("expected 1 POWER instruction, got %d", len(powers))
	}
	input := ir.AsBinaryOpInstrInput(powers[0].Input)
	mustConstant(t, input.Left, "2.0")
	mustConstant(t, input.Right, "3.0")
}

// ---- Comparison Operator Tests ----

func TestComparisonOperators(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		instrType ir.InstrType
		left      string
		right     string
	}{
		{"eq", `let r: boolean = 1 == 2`, ir.InstrTypeEqEq, "1", "2"},
		{"neq", `let r: boolean = 1 != 2`, ir.InstrTypeNotEq, "1", "2"},
		{"lt", `let r: boolean = 1 < 2`, ir.InstrTypeLessThan, "1", "2"},
		{"lte", `let r: boolean = 1 <= 2`, ir.InstrTypeLessThanEq, "1", "2"},
		{"gt", `let r: boolean = 2 > 1`, ir.InstrTypeGreaterThan, "2", "1"},
		{"gte", `let r: boolean = 2 >= 1`, ir.InstrTypeGreaterThanEq, "2", "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, instrs := generateHIR(t, tt.source)
			ops := findInstrs(filterUserInstrs(instrs), tt.instrType)
			if len(ops) != 1 {
				t.Fatalf("expected 1 %s instruction, got %d", tt.instrType, len(ops))
			}
			input := ir.AsBinaryOpInstrInput(ops[0].Input)
			mustConstant(t, input.Left, tt.left)
			mustConstant(t, input.Right, tt.right)
		})
	}
}

// ---- Unary and Logical Operator Tests ----

func TestUnaryNeg(t *testing.T) {
	_, instrs := generateHIR(t, `let r: i32 = -5`)
	negs := findInstrs(filterUserInstrs(instrs), ir.InstrTypeNeg)
	if len(negs) != 1 {
		t.Fatalf("expected 1 NEG instruction, got %d", len(negs))
	}
	input := ir.AsUnaryOpInstrInput(negs[0].Input)
	mustConstant(t, input.Value, "5")
}

func TestUnaryNot(t *testing.T) {
	_, instrs := generateHIR(t, `let r: boolean = !true`)
	nots := findInstrs(filterUserInstrs(instrs), ir.InstrTypeNot)
	if len(nots) != 1 {
		t.Fatalf("expected 1 NOT instruction, got %d", len(nots))
	}
	input := ir.AsUnaryOpInstrInput(nots[0].Input)
	mustConstant(t, input.Value, "true")
}

func TestLogicalAnd(t *testing.T) {
	// && is short-circuit: emits COND_JMP on the left operand, right is only evaluated in the true branch
	_, instrs := generateHIR(t, `let r: boolean = true && false`)
	condJmps := findInstrs(filterUserInstrs(instrs), ir.InstrTypeCondJmp)
	if len(condJmps) != 1 {
		t.Fatalf("expected 1 COND_JMP for short-circuit &&, got %d", len(condJmps))
	}
	input := ir.AsCondJmpInstrInput(condJmps[0].Input)
	mustConstant(t, input.Condition, "true") // condition is the left operand
}

func TestLogicalOr(t *testing.T) {
	// || is short-circuit: emits COND_JMP on the left operand, right is only evaluated in the false branch
	_, instrs := generateHIR(t, `let r: boolean = true || false`)
	condJmps := findInstrs(filterUserInstrs(instrs), ir.InstrTypeCondJmp)
	if len(condJmps) != 1 {
		t.Fatalf("expected 1 COND_JMP for short-circuit ||, got %d", len(condJmps))
	}
	input := ir.AsCondJmpInstrInput(condJmps[0].Input)
	mustConstant(t, input.Condition, "true") // condition is the left operand
}

// ---- Load and Store Tests ----

func TestLoad(t *testing.T) {
	// Reading a let variable generates a LOAD instruction
	_, instrs := generateHIR(t, `let x: i32 = 5
let y: i32 = x`)
	loads := findInstrs(filterUserInstrs(instrs), ir.InstrTypeLoad)
	if len(loads) == 0 {
		t.Fatal("expected at least one LOAD instruction when reading a variable")
	}
	input := ir.AsLoadInstrInput(loads[0].Input)
	if input.Addr.Name != "x" {
		t.Errorf("expected LOAD from 'x', got '%s'", input.Addr.Name)
	}
}

func TestStore(t *testing.T) {
	// Assignment to a variable emits a STORE instruction
	_, instrs := generateHIR(t, `let x: i32 = 0
x = 42`)
	stores := findInstrs(filterUserInstrs(instrs), ir.InstrTypeStore)
	if len(stores) == 0 {
		t.Fatal("expected at least one STORE instruction for assignment")
	}
	input := ir.AsStoreInstrInput(stores[0].Input)
	if input.Addr.Name != "x" {
		t.Errorf("expected STORE to 'x', got '%s'", input.Addr.Name)
	}
	mustConstant(t, input.Value, "42")
}

// ---- Control Flow Tests (inside functions) ----

func TestIfStatement(t *testing.T) {
	builder, _ := generateHIR(t, `
function test(flag: boolean): void {
    if (flag) {
        let x: i32 = 1
    }
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	condJmps := findInstrs(all, ir.InstrTypeCondJmp)
	if len(condJmps) != 1 {
		t.Fatalf("expected 1 COND_JMP instruction, got %d", len(condJmps))
	}
	input := ir.AsCondJmpInstrInput(condJmps[0].Input)
	if input.TrueTarget == nil {
		t.Error("expected non-nil TrueTarget block")
	}
	if input.FalseTarget == nil {
		t.Error("expected non-nil FalseTarget block")
	}
	if input.TrueTarget == input.FalseTarget {
		t.Error("expected TrueTarget and FalseTarget to be different blocks")
	}
}

func TestIfElseStatement(t *testing.T) {
	builder, _ := generateHIR(t, `
function test(flag: boolean): void {
    if (flag) {
        let x: i32 = 1
    } else {
        let x: i32 = 2
    }
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	condJmps := findInstrs(all, ir.InstrTypeCondJmp)
	if len(condJmps) != 1 {
		t.Fatalf("expected 1 COND_JMP instruction, got %d", len(condJmps))
	}
	input := ir.AsCondJmpInstrInput(condJmps[0].Input)
	// Both branches must exist and be distinct
	if input.TrueTarget == nil || input.FalseTarget == nil {
		t.Fatal("both TrueTarget and FalseTarget must be non-nil")
	}
	if input.TrueTarget == input.FalseTarget {
		t.Error("expected distinct then and else blocks")
	}
	// Each branch carries its own DECLARE_VAR
	thenDecls := findInstrs(input.TrueTarget.Instrs, ir.InstrTypeDeclVar)
	elseDecls := findInstrs(input.FalseTarget.Instrs, ir.InstrTypeDeclVar)
	if len(thenDecls) == 0 {
		t.Error("expected DECLARE_VAR in then block")
	}
	if len(elseDecls) == 0 {
		t.Error("expected DECLARE_VAR in else block")
	}
}

func TestWhileLoop(t *testing.T) {
	builder, _ := generateHIR(t, `
function test(): void {
    let i: i32 = 0
    while (i < 10) {
        i = i + 1
    }
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	// While loop generates COND_JMP for the condition and JMP instructions for the loop edges
	condJmps := findInstrs(all, ir.InstrTypeCondJmp)
	if len(condJmps) == 0 {
		t.Fatal("expected COND_JMP instruction for while condition")
	}
	// The condition block must branch to a body block and a merge block
	cj := ir.AsCondJmpInstrInput(condJmps[0].Input)
	if cj.TrueTarget == nil || cj.FalseTarget == nil {
		t.Error("COND_JMP must have non-nil TrueTarget and FalseTarget")
	}
	if cj.TrueTarget == cj.FalseTarget {
		t.Error("loop body and exit must be different blocks")
	}
}

func TestForLoop(t *testing.T) {
	builder, _ := generateHIR(t, `
function test(): void {
    for (let i: i32 = 0; i < 10; i++) {
    }
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	// Init variable is declared
	decls := findInstrs(all, ir.InstrTypeDeclVar)
	found := false
	for _, d := range decls {
		if ir.AsDeclVarInstrInput(d.Input).Variable.Name == "i" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected DECLARE_VAR for loop variable 'i'")
	}
	// Condition is a LESS_THAN comparison
	lts := findInstrs(all, ir.InstrTypeLessThan)
	if len(lts) == 0 {
		t.Error("expected LESS_THAN instruction for loop condition 'i < 10'")
	}
	// Conditional jump controls loop entry vs exit
	condJmps := findInstrs(all, ir.InstrTypeCondJmp)
	if len(condJmps) == 0 {
		t.Error("expected COND_JMP instruction in for loop")
	}
}

// ---- Function Declaration and Call Tests ----

func TestFunctionDecl(t *testing.T) {
	_, instrs := generateHIR(t, `
function add(a: i32, b: i32): i32 {
    return a + b
}`)
	userInstrs := filterUserInstrs(instrs)
	funcs := findInstrs(userInstrs, ir.InstrTypeDeclFunc)
	if len(funcs) != 1 {
		t.Fatalf("expected 1 DECLARE_FUNC instruction, got %d", len(funcs))
	}
	input := ir.AsDeclFuncInstrInput(funcs[0].Input)
	if input.Function.Name != "add" {
		t.Errorf("expected function name 'add', got '%s'", input.Function.Name)
	}
	if len(input.Function.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(input.Function.Params))
	}
	if input.Function.Params[0].Name != "a" {
		t.Errorf("expected first param 'a', got '%s'", input.Function.Params[0].Name)
	}
	if input.Function.Params[1].Name != "b" {
		t.Errorf("expected second param 'b', got '%s'", input.Function.Params[1].Name)
	}
	if input.Body == nil {
		t.Error("expected non-nil function body block")
	}
}

func TestFunctionBody(t *testing.T) {
	// Body of `return a + b` should contain ADD and RETURN
	builder, _ := generateHIR(t, `
function add(a: i32, b: i32): i32 {
    return a + b
}`)
	entry := getFuncEntryBlock(builder, "add")
	if entry == nil {
		t.Fatal("function 'add' not found")
	}
	all := allBlockInstrs(entry)
	if len(findInstrs(all, ir.InstrTypeAdd)) == 0 {
		t.Error("expected ADD instruction in function body")
	}
	returns := findInstrs(all, ir.InstrTypeReturn)
	if len(returns) == 0 {
		t.Error("expected RETURN instruction in function body")
	}
	retInput := ir.AsReturnInstrInput(returns[0].Input)
	if retInput.Value == nil {
		t.Error("expected non-nil return value")
	}
}

func TestFunctionCall(t *testing.T) {
	_, instrs := generateHIR(t, `
function add(a: i32, b: i32): i32 {
    return a + b
}
add(1, 2)`)
	userInstrs := filterUserInstrs(instrs)
	calls := findInstrs(userInstrs, ir.InstrTypeCallFunc)
	if len(calls) == 0 {
		t.Fatal("expected CALL_FUNC instruction")
	}
	input := ir.AsCallFuncInstrInput(calls[0].Input)
	fn := zeus_value.AsFunction(input.Callee)
	if fn == nil || fn.Name != "add" {
		t.Errorf("expected callee 'add', got %v", input.Callee)
	}
	if len(input.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(input.Args))
	}
	mustConstant(t, input.Args[0], "1")
	mustConstant(t, input.Args[1], "2")
}

func TestReturnWithValue(t *testing.T) {
	builder, _ := generateHIR(t, `
function getX(x: i32): i32 {
    return x
}`)
	entry := getFuncEntryBlock(builder, "getX")
	if entry == nil {
		t.Fatal("function 'getX' not found")
	}
	returns := findInstrs(allBlockInstrs(entry), ir.InstrTypeReturn)
	if len(returns) == 0 {
		t.Fatal("expected RETURN instruction")
	}
	input := ir.AsReturnInstrInput(returns[0].Input)
	if input.Value == nil {
		t.Error("expected non-nil return value")
	}
	v := zeus_value.AsVar(input.Value)
	if v == nil || v.Name != "x" {
		t.Errorf("expected return value to be parameter 'x', got %v", input.Value)
	}
}

func TestReturnVoid(t *testing.T) {
	builder, _ := generateHIR(t, `
function noop(): void {
    return;
}`)
	entry := getFuncEntryBlock(builder, "noop")
	if entry == nil {
		t.Fatal("function 'noop' not found")
	}
	returns := findInstrs(allBlockInstrs(entry), ir.InstrTypeReturn)
	if len(returns) == 0 {
		t.Fatal("expected RETURN instruction")
	}
	input := ir.AsReturnInstrInput(returns[0].Input)
	if input.Value != nil {
		t.Errorf("expected nil return value for void function, got %v", input.Value)
	}
}

// ---- Class Declaration and Object Tests ----

func TestClassDecl(t *testing.T) {
	_, instrs := generateHIR(t, `
class Point {
    public x: i32;
    public y: i32;
}`)
	userInstrs := filterUserInstrs(instrs)
	classDecls := findInstrs(userInstrs, ir.InstrTypeDeclClass)
	if len(classDecls) == 0 {
		t.Fatal("expected DECLARE_CLASS instruction for user-defined class")
	}
	input := ir.AsDeclClassInstrInput(classDecls[0].Input)
	if input.Class.Name != "Point" {
		t.Errorf("expected class name 'Point', got '%s'", input.Class.Name)
	}
	if len(input.Class.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(input.Class.Properties))
	}
	if input.Class.Properties[0].Property.Name != "x" {
		t.Errorf("expected first property 'x', got '%s'", input.Class.Properties[0].Property.Name)
	}
	if input.Class.Properties[1].Property.Name != "y" {
		t.Errorf("expected second property 'y', got '%s'", input.Class.Properties[1].Property.Name)
	}
}

func TestClassMethod(t *testing.T) {
	_, instrs := generateHIR(t, `
class Counter {
    public count: i32;
    constructor() {
        this.count = 0
    }
    public increment(): void {
        this.count = this.count + 1
    }
}`)
	userInstrs := filterUserInstrs(instrs)
	methods := findInstrs(userInstrs, ir.InstrTypeDeclClassMethod)
	if len(methods) < 2 {
		t.Fatalf("expected at least 2 DECLARE_CLASS_METHOD (constructor + increment), got %d", len(methods))
	}
	// constructor is the first method
	constructorInput := ir.AsDeclClassMethodInstrInput(methods[0].Input)
	if constructorInput.Method.Name == "" {
		t.Error("expected non-empty method name for constructor")
	}
	if constructorInput.Class == nil {
		t.Error("expected non-nil class in method declaration")
	}
	if constructorInput.Body == nil {
		t.Error("expected non-nil body block in method declaration")
	}
}

func TestNewObj(t *testing.T) {
	_, instrs := generateHIR(t, `
class Foo {
    constructor() {}
}
let obj: Foo = new Foo()`)
	userInstrs := filterUserInstrs(instrs)
	newObjs := findInstrs(userInstrs, ir.InstrTypeNewObj)
	if len(newObjs) == 0 {
		t.Fatal("expected NEW_OBJ instruction")
	}
	input := ir.AsNewObjInstrInput(newObjs[0].Input)
	cls := zeus_value.AsClass(input.Callee)
	if cls == nil || cls.Name != "Foo" {
		t.Errorf("expected class 'Foo', got %v", input.Callee)
	}
}

func TestNewObjWithArgs(t *testing.T) {
	_, instrs := generateHIR(t, `
class Point {
    public x: i32;
    public y: i32;
    constructor(x: i32, y: i32) {
        this.x = x
        this.y = y
    }
}
let p: Point = new Point(3, 7)`)
	userInstrs := filterUserInstrs(instrs)
	newObjs := findInstrs(userInstrs, ir.InstrTypeNewObj)
	if len(newObjs) == 0 {
		t.Fatal("expected NEW_OBJ instruction")
	}
	input := ir.AsNewObjInstrInput(newObjs[0].Input)
	if len(input.Args) != 2 {
		t.Errorf("expected 2 constructor args, got %d", len(input.Args))
	}
	mustConstant(t, input.Args[0], "3")
	mustConstant(t, input.Args[1], "7")
}

func TestPropertyRead(t *testing.T) {
	builder, _ := generateHIR(t, `
class Point {
    public x: i32;
    constructor(x: i32) {
        this.x = x
    }
}
function test(p: Point): i32 {
    return p.x
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	accesses := findPropertyAccess(all, "x")
	if len(accesses) == 0 {
		t.Fatal("expected OBJECT_PROPERTY_ACCESS for 'x'")
	}
	input := ir.AsObjectPropertyAccessInstrInput(accesses[0].Input)
	if input.IsLValue {
		t.Error("property read should have IsLValue=false")
	}
	// A LOAD should follow to dereference the property pointer
	loads := findInstrs(all, ir.InstrTypeLoad)
	if len(loads) == 0 {
		t.Error("expected LOAD instruction after property access")
	}
}

func TestPropertyWrite(t *testing.T) {
	builder, _ := generateHIR(t, `
class Point {
    public x: i32;
    constructor(x: i32) {
        this.x = x
    }
}
function test(p: Point): void {
    p.x = 99
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	accesses := findPropertyAccess(all, "x")
	if len(accesses) == 0 {
		t.Fatal("expected OBJECT_PROPERTY_ACCESS for 'x'")
	}
	// Find the write-mode access (IsLValue=true)
	foundWrite := false
	for _, a := range accesses {
		if ir.AsObjectPropertyAccessInstrInput(a.Input).IsLValue {
			foundWrite = true
			break
		}
	}
	if !foundWrite {
		t.Error("expected at least one OBJECT_PROPERTY_ACCESS with IsLValue=true for property write")
	}
	stores := findInstrs(all, ir.InstrTypeStore)
	if len(stores) == 0 {
		t.Error("expected STORE instruction for property write")
	}
}

func TestMethodCall(t *testing.T) {
	builder, _ := generateHIR(t, `
class Greeter {
    public greet(): void {}
}
function test(g: Greeter): void {
    g.greet()
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	// Method call is now a single CALL_METHOD instruction
	calls := findMethodCalls(all, "greet")
	if len(calls) == 0 {
		t.Error("expected CALL_METHOD instruction for 'greet' invocation")
	}
}

// ---- Array Indexing Tests ----

func TestArrayIndex(t *testing.T) {
	builder, _ := generateHIR(t, `
function test(arr: i32[]): i32 {
    return arr[0]
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	gets := findInstrs(all, ir.InstrTypeGetIndex)
	if len(gets) != 1 {
		t.Fatalf("expected 1 GET_INDEX instruction (HIR), got %d", len(gets))
	}
	input := ir.AsGetIndexInstrInput(gets[0].Input)
	// arr[0] produces 1 index operand (may be cast to i32 but is semantically index 0)
	if len(input.Indices) != 1 {
		t.Errorf("expected 1 index operand in GET_INDEX, got %d", len(input.Indices))
	}
}

func TestNestedArrayIndex(t *testing.T) {
	// arr[1][2] is parsed as a single IndexingExpression with TWO index operands.
	// HIR emits ONE GET_INDEX with Indices=[1, 2]; lowering later unrolls it into two .get() calls.
	builder, _ := generateHIR(t, `
function test(arr: i32[][]): i32 {
    return arr[1][2]
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	gets := findInstrs(all, ir.InstrTypeGetIndex)
	if len(gets) != 1 {
		t.Fatalf("expected 1 GET_INDEX for arr[1][2] at HIR stage, got %d", len(gets))
	}
	input := ir.AsGetIndexInstrInput(gets[0].Input)
	if len(input.Indices) != 2 {
		t.Errorf("expected 2 index operands for arr[1][2], got %d", len(input.Indices))
	}
}

// ---- String Operator Tests (HIR level — not yet lowered) ----

func TestStringConcatHIR(t *testing.T) {
	// At HIR stage, string + emits ADD (same instruction as numeric add).
	// Lowering converts it to a .concat() method call.
	builder, _ := generateHIR(t, `
function test(a: string, b: string): string {
    return a + b
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	adds := findInstrs(allBlockInstrs(entry), ir.InstrTypeAdd)
	if len(adds) == 0 {
		t.Error("expected ADD instruction for string + at HIR stage (before lowering)")
	}
}

func TestStringEqualityHIR(t *testing.T) {
	// At HIR stage, string == emits EQ_EQ (same as numeric equality).
	// Lowering converts it to a .equals() method call.
	builder, _ := generateHIR(t, `
function test(a: string, b: string): boolean {
    return a == b
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	eqs := findInstrs(allBlockInstrs(entry), ir.InstrTypeEqEq)
	if len(eqs) == 0 {
		t.Error("expected EQ_EQ instruction for string == at HIR stage (before lowering)")
	}
}

// ---- Exception Handling Tests ----

func TestThrow(t *testing.T) {
	builder, _ := generateHIR(t, `
function fail(): void {
    throw new Error("TestError", "test failed")
}`)
	entry := getFuncEntryBlock(builder, "fail")
	if entry == nil {
		t.Fatal("function 'fail' not found")
	}
	// bounds-check helper also emits THROW for index-out-of-bounds; find the user's throw
	throws := findInstrs(allBlockInstrs(entry), ir.InstrTypeThrow)
	if len(throws) == 0 {
		t.Fatal("expected THROW instruction")
	}
	// The user-level throw for `throw new Error(...)` must have a non-nil object pointer.
	// At HIR stage the class ID is 0 (types are resolved by the type checker, not IR gen).
	input := ir.AsThrowInstrInput(throws[0].Input)
	if input.ObjectPtr == nil {
		t.Error("expected non-nil ObjectPtr in THROW instruction")
	}
}

func TestTryCatch(t *testing.T) {
	builder, _ := generateHIR(t, `
function test(): void {
    try {
    } catch (e: Error) {
    }
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	pushes := findInstrs(all, ir.InstrTypePushHandler)
	if len(pushes) == 0 {
		t.Error("expected PUSH_HANDLER instruction at try entry")
	}
	pops := findInstrs(all, ir.InstrTypePopHandler)
	if len(pops) == 0 {
		t.Error("expected POP_HANDLER instruction at try exit")
	}
	gets := findInstrs(all, ir.InstrTypeGetException)
	if len(gets) == 0 {
		t.Error("expected GET_EXCEPTION instruction in catch block")
	}
	clears := findInstrs(all, ir.InstrTypeClearException)
	if len(clears) == 0 {
		t.Error("expected CLEAR_EXCEPTION instruction after catch")
	}
	// PUSH_HANDLER must reference the handler block and the catch class IDs
	pushInput := ir.AsPushHandlerInstrInput(pushes[0].Input)
	if pushInput.HandlerBlock == nil {
		t.Error("expected non-nil HandlerBlock in PUSH_HANDLER")
	}
	if len(pushInput.ClassIds) == 0 {
		t.Error("expected at least one ClassId in PUSH_HANDLER (Error class)")
	}
}

// ---- Export Test ----

func TestExport(t *testing.T) {
	_, instrs := generateHIR(t, `
export function greet(): void {
}`)
	userInstrs := filterUserInstrs(instrs)
	exports := findInstrs(userInstrs, ir.InstrTypeExport)
	if len(exports) == 0 {
		t.Fatal("expected EXPORT instruction")
	}
	// The exported value must be a function
	input := ir.AsExportInstrInput(exports[0].Input)
	fn := zeus_value.AsFunction(input.Value)
	if fn == nil {
		t.Errorf("expected exported value to be a function, got %T", input.Value)
	}
	if fn.Name != "greet" {
		t.Errorf("expected exported function 'greet', got '%s'", fn.Name)
	}
	// The DECLARE_FUNC must also be present
	funcs := findInstrs(userInstrs, ir.InstrTypeDeclFunc)
	if len(funcs) == 0 {
		t.Error("expected DECLARE_FUNC along with EXPORT")
	}
}

// runTCExpectError generates HIR, runs TC passes, and asserts that at least one
// error with the given substring is present.
func runTCExpectError(t *testing.T, source, wantSubstr string) {
	t.Helper()
	l := lexer.NewLexer(source)
	tokens, lexErrs := l.Lex()
	if len(lexErrs) > 0 {
		t.Fatalf("lexer errors: %v", lexErrs)
	}
	program, parseErrs := parser.NewParser(tokens).ParseProgram()
	if len(parseErrs) > 0 {
		t.Fatalf("parser errors: %v", parseErrs)
	}
	builder := ir.NewIRBuilder()
	mod := ir.NewIRModule(builder, "test.zs", false, nil)
	// Ignore IR gen errors — TC errors are what we want to assert.
	mod.Generate(program)
	tc := ir.NewTypeChecker(builder, false)
	errs := tc.TypeCheck()
	for _, err := range errs {
		if err.Severity == zeus_error.ErrorSeverityError && containsSubstr(err.Message, wantSubstr) {
			return
		}
	}
	t.Errorf("expected a TC error containing %q, got: %v", wantSubstr, errs)
}

func containsSubstr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstrHelper(s, sub)))
}

func containsSubstrHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---- Indexing type-check error tests ----

func TestIndexOnNonArrayIsError(t *testing.T) {
	runTCExpectError(t, `
function f(): void {
    let x: i32 = 1
    let y = x[0]
}`, "cannot use indexing operator []")
}

func TestIndexOnBoolIsError(t *testing.T) {
	runTCExpectError(t, `
function f(): void {
    let x: bool = true
    let y = x[0]
}`, "cannot use indexing operator []")
}

func TestSetIndexOnNonArrayIsError(t *testing.T) {
	runTCExpectError(t, `
function f(): void {
    let x: i32 = 1
    x[0] = 2
}`, "cannot use indexing operator []")
}

func TestSetIndexOnStringIsError(t *testing.T) {
	runTCExpectError(t, `
function f(): void {
    let s: string = "hello"
    s[0] = 'x'
}`, "immutable")
}

// ---- Readonly property type-check tests ----

func TestReadonlyPropertyWriteOutsideConstructorIsError(t *testing.T) {
	runTCExpectError(t, `
class Point {
    public readonly x: i32;
    constructor(x: i32) {
        this.x = x;
    }
}
function f(): void {
    let p = new Point(1);
    p.x = 5;
}`, "cannot assign to readonly property 'x'")
}

func TestReadonlyPropertyWriteInConstructorIsAllowed(t *testing.T) {
	runTC(t, `
class Point {
    public readonly x: i32;
    constructor(x: i32) {
        this.x = x;
    }
}
function f(): void {
    let p = new Point(1);
    let v: i32 = p.x;
}`)
}

func TestArrayCreationWithoutNewIsError(t *testing.T) {
	l := lexer.NewLexer(`
function f(): void {
    let a = i32[]
}`)
	tokens, _ := l.Lex()
	program, _ := parser.NewParser(tokens).ParseProgram()
	builder := ir.NewIRBuilder()
	mod := ir.NewIRModule(builder, "test.zs", false, nil)
	errs := mod.Generate(program)
	for _, err := range errs {
		if err.Severity == zeus_error.ErrorSeverityError && containsSubstr(err.Message, "new") {
			return
		}
	}
	t.Error("expected an IR gen error suggesting 'new' for 'i32[]' without new")
}
