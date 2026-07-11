package ir_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/util"
)

// lowerAndFindFactoryBody type-checks + lowers the source, then returns the body instructions of
// the synthesized zeus_new_<Class> factory function for the named user class.
func lowerAndFindFactoryBody(t *testing.T, source, className string) []*ir.Instr {
	t.Helper()
	builder := runTC(t, source)
	ir.NewLowerer(builder).Lower()

	instrs := builder.GetInstrs()
	class := classBySourceName(t, instrs, className)
	factoryName := util.GetFactoryFunctionName(class.Name)

	for _, instr := range instrs {
		if instr.Type != ir.InstrTypeDeclFunc {
			continue
		}
		if fn := ir.AsDeclFuncInstrInput(instr.Input).Function; fn.Name == factoryName {
			return allBlockInstrs(ir.AsDeclFuncInstrInput(instr.Input).Body)
		}
	}
	t.Fatalf("factory %q not found after lowering", factoryName)
	return nil
}

func countInstrType(instrs []*ir.Instr, typ ir.InstrType) int {
	n := 0
	for _, instr := range instrs {
		if instr.Type == typ {
			n++
		}
	}
	return n
}

// TestFactoryLoweringWithConstructor: a user class with a constructor lowers to a factory of
// ALLOC_OBJ + constructor call (reusing SUPER_CONSTRUCTOR_CALL) + RETURN — and no field-init
// stores (the GC allocator zeroes memory).
func TestFactoryLoweringWithConstructor(t *testing.T) {
	body := lowerAndFindFactoryBody(t, `
class Point {
    public x: i32;
    constructor(x: i32) { this.x = x; }
}
function build(): Point { return new Point(5); }`, "Point")

	if got := countInstrType(body, ir.InstrTypeAllocObj); got != 1 {
		t.Errorf("expected 1 ALLOC_OBJ, got %d", got)
	}
	if got := countInstrType(body, ir.InstrTypeSuperConstructorCall); got != 1 {
		t.Errorf("expected 1 constructor call (SUPER_CONSTRUCTOR_CALL), got %d", got)
	}
	if got := countInstrType(body, ir.InstrTypeReturn); got != 1 {
		t.Errorf("expected 1 RETURN, got %d", got)
	}
}

// TestFactoryLoweringWithoutConstructor: a user class with no constructor lowers to a factory of
// just ALLOC_OBJ + RETURN (no constructor call).
func TestFactoryLoweringWithoutConstructor(t *testing.T) {
	body := lowerAndFindFactoryBody(t, `
class Bag {
    public n: i32;
}
function build(): Bag { return new Bag(); }`, "Bag")

	if got := countInstrType(body, ir.InstrTypeAllocObj); got != 1 {
		t.Errorf("expected 1 ALLOC_OBJ, got %d", got)
	}
	if got := countInstrType(body, ir.InstrTypeSuperConstructorCall); got != 0 {
		t.Errorf("expected no constructor call for a ctor-less class, got %d", got)
	}
	if got := countInstrType(body, ir.InstrTypeReturn); got != 1 {
		t.Errorf("expected 1 RETURN, got %d", got)
	}
}
