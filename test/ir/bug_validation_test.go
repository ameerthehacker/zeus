package ir_test

// Validation tests for each bug documented in wiki/compiler-bugs.md.
// Run with: go test ./test/ir/... -run TestBug -v -count=1

import (
	"strings"
	"testing"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// compile parses and runs IR generation; returns (error messages, panic string).
func compile(t *testing.T, src string) (errs []string, panicMsg string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case string:
				panicMsg = "PANIC: " + v
			case error:
				panicMsg = "PANIC: " + v.Error()
			default:
				panicMsg = "PANIC: unknown"
			}
		}
	}()

	l := lexer.NewLexer(src)
	tokens, lexErrs := l.Lex()
	for _, e := range lexErrs {
		errs = append(errs, "LEX:"+e.Message)
	}
	if len(lexErrs) > 0 {
		return
	}

	p := parser.NewParser(tokens)
	program, parseErrs := p.ParseProgram()
	for _, e := range parseErrs {
		errs = append(errs, "PARSE:"+e.Message)
	}

	b := ir.NewIRBuilder()
	mod := ir.NewIRModule(b, "test.zs", nil)
	irErrs := mod.Generate(program)
	for _, e := range irErrs {
		errs = append(errs, "IR:"+e.Message)
	}
	return
}

func hasErr(errs []string, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}

// ─── Bug 1: Nested functions are hoisted to global scope ─────────────────────

// A function declared inside outer() must NOT be callable from main().
// Currently the compiler DOES NOT raise an error — this test asserts the bug exists.
func TestBug_NestedFuncHoistedToGlobal(t *testing.T) {
	src := `
function outer(): i32 {
  function inner(): i32 { return 5; }
  return inner();
}
function main(): i32 {
  return inner();
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	if hasErr(errs, "undefined") || hasErr(errs, "inner") {
		t.Log("FIXED: 'inner' correctly produces an undefined-identifier error when called outside its declaring function")
	} else {
		t.Error("REGRESSION: calling a nested function from outside its declaring function produced no error — scoping fix reverted")
	}
}

// Secondary check: inner IS accessible from within outer (baseline, must pass).
func TestBug_NestedFuncAccessibleWithin(t *testing.T) {
	src := `
function outer(): i32 {
  function inner(): i32 { return 5; }
  return inner();
}
function main(): i32 { return outer(); }
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("panic: %s", panicMsg)
	}
	if len(errs) != 0 {
		t.Errorf("nested function should be callable within its declaring function; got errors: %v", errs)
	}
}

// ─── Bug 2: Two nested functions with the same name overwrite each other ──────

// foo() and bar() each have a local "helper". At the IR level the second
// BuildFuncDecl("helper") finds the first as an existing stub and updates it
// in place — both DECL_FUNC instructions share the same Function object.
func TestBug_NestedFuncSameNameOverwrite(t *testing.T) {
	src := `
function foo(): i32 {
  function helper(): i32 { return 1; }
  return helper();
}
function bar(): i32 {
  function helper(): i32 { return 2; }
  return helper();
}
function main(): i32 { return foo() + bar(); }
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	if len(errs) != 0 {
		t.Errorf("IR-level errors (unexpected): %v", errs)
	}

	// Verify at IR level that two DECL_FUNC instructions exist for "helper".
	l := lexer.NewLexer(src)
	tokens, _ := l.Lex()
	p := parser.NewParser(tokens)
	program, _ := p.ParseProgram()
	b := ir.NewIRBuilder()
	mod := ir.NewIRModule(b, "test.zs", nil)
	mod.Generate(program)

	// Collect all DECL_FUNC instructions whose source name is "helper"
	// (matches both the original "helper" and any IR-renamed "helper1", "helper2", etc.)
	var helpers []*ir.Instr
	for _, instr := range b.GetInstrs() {
		if instr.Type == ir.InstrTypeDeclFunc {
			input := ir.AsDeclFuncInstrInput(instr.Input)
			if input.Function.SourceName() == "helper" {
				helpers = append(helpers, instr)
			}
		}
	}

	switch len(helpers) {
	case 0:
		t.Error("no DECL_FUNC found for 'helper' at all")
	case 1:
		t.Error("only one DECL_FUNC for 'helper' — second declaration was lost (overwrite bug still present)")
	case 2:
		fn1 := ir.AsDeclFuncInstrInput(helpers[0].Input).Function
		fn2 := ir.AsDeclFuncInstrInput(helpers[1].Input).Function
		if fn1 == fn2 {
			t.Error("two DECL_FUNC instructions share the same *Function object — overwrite bug still present")
		} else if fn1.Name == fn2.Name {
			t.Errorf("two DECL_FUNC instructions have the same IR name %q — LLVM collision still present", fn1.Name)
		} else {
			t.Logf("FIXED: two DECL_FUNC with distinct IR names (%q, %q), both with source name %q", fn1.Name, fn2.Name, "helper")
		}
	default:
		t.Errorf("unexpected: found %d DECL_FUNC instructions for 'helper'", len(helpers))
	}
}

// ─── Bug 3: Function type annotations not supported ───────────────────────────

func TestBug_FuncTypeAnnotationInParam(t *testing.T) {
	src := `
function apply(f: function(a: i32, b: i32): i32, x: i32, y: i32): i32 {
  return f(x, y);
}
function add(a: i32, b: i32): i32 { return a + b; }
function main(): i32 { return apply(add, 3, 4); }
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	if hasErr(errs, "data type") && hasErr(errs, "function") {
		t.Log("BUG CONFIRMED: function type annotation in parameter causes parse error:", errs)
	} else if len(errs) == 0 {
		t.Log("Bug may be fixed — no errors for function type annotation")
	} else {
		t.Logf("Unexpected error set: %v", errs)
	}
}

func TestBug_FuncTypeAnnotationInVarDecl(t *testing.T) {
	src := `
function main(): i32 {
  let f: function(): i32 = function myFn(): i32 { return 1; };
  return f();
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	if len(errs) > 0 {
		t.Logf("BUG CONFIRMED: function type annotation in var decl causes errors: %v", errs)
	} else {
		t.Log("Bug may be fixed — no errors")
	}
}

// ─── Bug 4: Debug fmt.Println fires on non-void constructor ──────────────────

// This is a side-effect test: we can't easily capture stdout in Go tests without
// os.Pipe tricks, so we just confirm the proper error IS raised and note the issue.
func TestBug_ConstructorNonVoidDebugPrint(t *testing.T) {
	src := `
function main(): i32 {
  class Foo {
    public x: i32;
    constructor(v: i32): i32 { this.x = v; }
    val(): i32 { return this.x; }
  }
  let f = new Foo(5);
  return f.val();
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	hasConstructorErr := hasErr(errs, "constructor") || hasErr(errs, "void")
	if hasConstructorErr {
		t.Log("Correct error raised for non-void constructor.")
		t.Log("NOTE: a raw span struct is also printed to STDOUT by fmt.Println at ir.go inside VisitClassDeclExpr — see compiler-bugs.md.")
	} else {
		t.Errorf("Expected a constructor return-type error but got: %v", errs)
	}
}

// ─── Gotcha 5: Named function expression name leaks into outer scope ──────────

func TestGotcha_FuncExprInnerNameLeaks(t *testing.T) {
	src := `
function main(): i32 {
  let f = function foo(): i32 { return 42; };
  return foo();
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	if len(errs) == 0 {
		t.Error("REGRESSION: 'foo' (function expression name) is callable from outer scope — name-leak fix reverted")
	} else {
		t.Log("FIXED: calling 'foo' from outer scope correctly produces an error — function expression name is scoped correctly")
	}
}

// Also confirm the variable 'f' holds a callable function pointer.
func TestGotcha_FuncExprVarCallable(t *testing.T) {
	src := `
function main(): i32 {
  let f = function foo(): i32 { return 42; };
  return f();
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	if len(errs) != 0 {
		t.Errorf("calling via variable 'f' should work; got errors: %v", errs)
	}
}

// ─── Gotcha 6: Semicolons required after function/class body in let/const ─────

func TestGotcha_MissingSemiAfterFuncBody(t *testing.T) {
	// Missing ';' after the function body — parser keeps consuming.
	src := `
function main(): i32 {
  let f = function foo(): i32 { return 1; }
  return f();
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	if len(errs) > 0 {
		t.Logf("GOTCHA CONFIRMED: missing ';' after function body in let causes parse errors: %v", errs)
	} else {
		t.Log("No errors — semicolon may now be optional after function bodies in declarations")
	}
}

func TestGotcha_WithSemiAfterFuncBody(t *testing.T) {
	// Correct usage with ';'.
	src := `
function main(): i32 {
  let f = function foo(): i32 { return 1; };
  return f();
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	if len(errs) != 0 {
		t.Errorf("with proper semicolon, should compile cleanly; got: %v", errs)
	}
}

func TestGotcha_MissingSemiAfterClassBody(t *testing.T) {
	src := `
function main(): i32 {
  let MyClass = class Point { public v: i32; constructor(v: i32) { this.v = v; } }
  let p = new Point(3);
  return p.v;
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	if len(errs) > 0 {
		t.Logf("GOTCHA CONFIRMED: missing ';' after class body in let causes parse errors: %v", errs)
	} else {
		t.Log("No errors — semicolon may now be optional after class bodies in declarations")
	}
}

// ─── Confirmed limitations (parser rejects these) ────────────────────────────

func TestLimitation_TrulyAnonymousFunction(t *testing.T) {
	src := `
function main(): i32 {
  let f = function(): i32 { return 1; };
  return f();
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	if hasErr(errs, "identifier") && hasErr(errs, "function name") {
		t.Log("CONFIRMED LIMITATION: anonymous functions (no name) are not supported; parse error raised.")
	} else if len(errs) == 0 {
		t.Log("Limitation lifted — anonymous functions now work")
	} else {
		t.Logf("Unexpected errors: %v", errs)
	}
}

func TestLimitation_TrulyAnonymousClass(t *testing.T) {
	src := `
function main(): i32 {
  let obj = new class { public x: i32; constructor(v: i32) { this.x = v; } }(10);
  return obj.x;
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" {
		t.Fatalf("unexpected panic: %s", panicMsg)
	}
	if hasErr(errs, "identifier") && hasErr(errs, "class name") {
		t.Log("CONFIRMED LIMITATION: anonymous classes (no name) are not supported; parse error raised.")
	} else if len(errs) == 0 {
		t.Log("Limitation lifted — anonymous classes now work")
	} else {
		t.Logf("Unexpected errors: %v", errs)
	}
}

// ─── Regression guard: known-good patterns must keep working ─────────────────

func TestRegression_NamedFuncExprCallable(t *testing.T) {
	src := `
function main(): i32 {
  let f = function add(a: i32, b: i32): i32 { return a + b; };
  return f(3, 4);
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" || len(errs) != 0 {
		t.Errorf("named function expression should compile; panic=%q errs=%v", panicMsg, errs)
	}
}

func TestRegression_NestedClass(t *testing.T) {
	src := `
function main(): i32 {
  class Point {
    public x: i32;
    public y: i32;
    constructor(x: i32, y: i32) { this.x = x; this.y = y; }
    sum(): i32 { return this.x + this.y; }
  }
  let p = new Point(3, 7);
  return p.sum();
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" || len(errs) != 0 {
		t.Errorf("nested class should compile; panic=%q errs=%v", panicMsg, errs)
	}
}

func TestRegression_IIFE(t *testing.T) {
	src := `
function main(): i32 {
  let x = (function iife(): i32 { return 99; })();
  return x;
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" || len(errs) != 0 {
		t.Errorf("IIFE should compile; panic=%q errs=%v", panicMsg, errs)
	}
}

func TestRegression_FuncPtrReassign(t *testing.T) {
	src := `
function main(): i32 {
  let fn = function first(): i32 { return 1; };
  let total = fn();
  fn = function second(): i32 { return 2; };
  total = total + fn();
  return total;
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" || len(errs) != 0 {
		t.Errorf("func ptr reassign should compile; panic=%q errs=%v", panicMsg, errs)
	}
}

func TestRegression_NestedClassSameNameDifferentOuter(t *testing.T) {
	// Classes use generateUniqueName so two nested classes with the same source
	// name in different outer functions should produce distinct IR names.
	src := `
function foo(): i32 {
  class Node { public v: i32; constructor(v: i32) { this.v = v; } }
  let n = new Node(10);
  return n.v;
}
function bar(): i32 {
  class Node { public v: i32; constructor(v: i32) { this.v = v; } }
  let n = new Node(20);
  return n.v;
}
function main(): i32 { return foo() + bar(); }
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" || len(errs) != 0 {
		t.Errorf("nested classes with same source name should compile; panic=%q errs=%v", panicMsg, errs)
	}

	// Also verify distinct IR names were generated.
	l := lexer.NewLexer(src)
	tokens, _ := l.Lex()
	p := parser.NewParser(tokens)
	program, _ := p.ParseProgram()
	b := ir.NewIRBuilder()
	mod := ir.NewIRModule(b, "test.zs", nil)
	mod.Generate(program)

	nodeClasses := map[string]bool{}
	for _, instr := range b.GetInstrs() {
		if instr.Type == ir.InstrTypeDeclClass {
			c := ir.AsDeclClassInstrInput(instr.Input).Class
			if c.PrimordialName == "" && strings.HasPrefix(c.Name, "Node") {
				nodeClasses[c.Name] = true
			}
		}
	}
	if len(nodeClasses) < 2 {
		t.Errorf("expected 2 distinct IR names for 'Node' nested classes, got: %v", nodeClasses)
	} else {
		t.Logf("Nested Node classes have distinct IR names: %v", nodeClasses)
	}
}

func TestRegression_TopLevelFuncAndClass(t *testing.T) {
	src := `
class Point {
  public x: i32;
  public y: i32;
  constructor(x: i32, y: i32) { this.x = x; this.y = y; }
  sum(): i32 { return this.x + this.y; }
}
function makePoint(x: i32, y: i32): Point {
  return new Point(x, y);
}
function main(): i32 {
  let p = makePoint(4, 6);
  return p.sum();
}
`
	errs, panicMsg := compile(t, src)
	if panicMsg != "" || len(errs) != 0 {
		t.Errorf("top-level func and class should compile; panic=%q errs=%v", panicMsg, errs)
	}
}

// compile-time check that the IR exports used above exist.
var _ ir.InstrType = ir.InstrTypeDeclClass
var _ = (*zeus_value.Class)(nil)
