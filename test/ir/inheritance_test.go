package ir_test

import (
	"strings"
	"testing"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

// generateHIRErrors runs IR generation and returns any errors instead of failing the test,
// so error paths (missing/misplaced super) can be asserted. Lexer/parser errors still fail.
func generateHIRErrors(t *testing.T, source string) []*zeus_error.ZeusError {
	t.Helper()
	l := lexer.NewLexer(source)
	tokens, errs := l.Lex()
	if len(errs) > 0 {
		t.Fatalf("lexer errors: %v", errs)
	}
	p := parser.NewParser(tokens)
	program, perrs := p.ParseProgram()
	if len(perrs) > 0 {
		t.Fatalf("parser errors: %v", perrs)
	}
	builder := ir.NewIRBuilder()
	mod := ir.NewIRModule(builder, "test.zs", false, nil)
	return mod.Generate(program)
}

func hasErrorContaining(errs []*zeus_error.ZeusError, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Message, substr) {
			return true
		}
	}
	return false
}

// allSuperConstructorCalls collects every CALL_SUPER_CONSTRUCTOR instruction across all
// class-method bodies in the module.
func allSuperConstructorCalls(instrs []*ir.Instr) []*ir.Instr {
	var result []*ir.Instr
	for _, instr := range instrs {
		if instr.Type == ir.InstrTypeDeclClassMethod {
			body := ir.AsDeclClassMethodInstrInput(instr.Input).Body
			result = append(result, findInstrs(allBlockInstrs(body), ir.InstrTypeSuperConstructorCall)...)
		}
	}
	return result
}

func TestSuperConstructorCallEmitted(t *testing.T) {
	_, instrs := generateHIR(t, `
class Base {
    public a: i32;
    constructor(a: i32) { this.a = a; }
}
class Derived extends Base {
    public b: i32;
    constructor(a: i32, b: i32) {
        super(a);
        this.b = b;
    }
}`)
	userInstrs := filterUserInstrs(instrs)
	supers := allSuperConstructorCalls(userInstrs)
	if len(supers) != 1 {
		t.Fatalf("expected exactly 1 CALL_SUPER_CONSTRUCTOR, got %d", len(supers))
	}
	input := ir.AsSuperConstructorCallInstrInput(supers[0].Input)
	if input.ParentClass == nil || input.ParentClass.Name != "Base" {
		t.Errorf("expected super to target base class 'Base', got %v", input.ParentClass)
	}
	if len(input.Args) != 1 {
		t.Errorf("expected super(...) to carry 1 argument, got %d", len(input.Args))
	}
	if input.ThisObject == nil {
		t.Error("expected super(...) to carry the `this` object")
	}
}

func TestSuperTargetsNearestConstructorInChain(t *testing.T) {
	// Middle class B has no constructor of its own; super() in C targets B's nearest
	// ancestor-with-a-constructor, which is A.
	_, instrs := generateHIR(t, `
class A {
    public a: i32;
    constructor(a: i32) { this.a = a; }
}
class B extends A { }
class C extends B {
    public c: i32;
    constructor(a: i32, c: i32) {
        super(a);
        this.c = c;
    }
}`)
	supers := allSuperConstructorCalls(filterUserInstrs(instrs))
	if len(supers) != 1 {
		t.Fatalf("expected 1 CALL_SUPER_CONSTRUCTOR, got %d", len(supers))
	}
	input := ir.AsSuperConstructorCallInstrInput(supers[0].Input)
	if input.ParentClass == nil || input.ParentClass.Name != "A" {
		t.Errorf("expected super to target nearest constructor class 'A', got %v", input.ParentClass)
	}
}

func TestNoSuperWhenBaseHasNoConstructor(t *testing.T) {
	// Base has no constructor, so a derived constructor neither needs nor emits super().
	_, instrs := generateHIR(t, `
class Shape {
    public area(): i32 { return 0; }
}
class Square extends Shape {
    public s: i32;
    constructor(s: i32) { this.s = s; }
}`)
	if supers := allSuperConstructorCalls(filterUserInstrs(instrs)); len(supers) != 0 {
		t.Fatalf("expected no CALL_SUPER_CONSTRUCTOR, got %d", len(supers))
	}
}

func TestMissingSuperIsError(t *testing.T) {
	errs := generateHIRErrors(t, `
class Base {
    public a: i32;
    constructor(a: i32) { this.a = a; }
}
class Derived extends Base {
    public b: i32;
    constructor(b: i32) { let x: i32 = b + 1; }
}`)
	if !hasErrorContaining(errs, "must call super") {
		t.Errorf("expected a 'must call super' error, got: %v", errs)
	}
}

func TestThisBeforeSuperIsError(t *testing.T) {
	errs := generateHIRErrors(t, `
class Base {
    public a: i32;
    constructor(a: i32) { this.a = a; }
}
class Derived extends Base {
    public b: i32;
    constructor(a: i32, b: i32) {
        this.b = b;
        super(a);
    }
}`)
	if !hasErrorContaining(errs, "before calling super") {
		t.Errorf("expected a 'this before super' error, got: %v", errs)
	}
}

func TestSuperOutsideConstructorIsError(t *testing.T) {
	errs := generateHIRErrors(t, `
class Base {
    constructor() { }
}
class Derived extends Base {
    constructor() { super(); }
    public oops(): void { super(); }
}`)
	if !hasErrorContaining(errs, "can only be called inside a constructor") {
		t.Errorf("expected a 'super outside constructor' error, got: %v", errs)
	}
}
