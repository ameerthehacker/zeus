package ir_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/ir"
)

// funcBoxCount type-checks source and returns the number of BOX instructions in the named
// function's body (before lowering).
func funcBoxCount(t *testing.T, source, funcName string) int {
	t.Helper()
	builder := runTC(t, source)
	block := getFuncEntryBlock(builder, funcName)
	if block == nil {
		t.Fatalf("function '%s' not found", funcName)
	}
	return countInstrType(allBlockInstrs(block), ir.InstrTypeBox)
}

// TestAutoboxMethodCallEmitsBox: calling a method on a primitive receiver autoboxes it, so a BOX
// appears at the call site.
func TestAutoboxMethodCallEmitsBox(t *testing.T) {
	if got := funcBoxCount(t, `
function f(): void {
    let s: string = (5).toString();
}`, "f"); got != 1 {
		t.Errorf("expected 1 BOX for a primitive method-call receiver, got %d", got)
	}
}

// TestAutoboxNumberAnnotationEmitsBox: assigning a scalar into a Number binding autoboxes it.
func TestAutoboxNumberAnnotationEmitsBox(t *testing.T) {
	if got := funcBoxCount(t, `
function g(): void {
    let n: Number = 5;
}`, "g"); got != 1 {
		t.Errorf("expected 1 BOX for a Number annotation, got %d", got)
	}
}

// TestNoAutoboxInPureArithmetic: the hybrid guarantee — scalar arithmetic never autoboxes, so the
// numeric fast path is untouched.
func TestNoAutoboxInPureArithmetic(t *testing.T) {
	if got := funcBoxCount(t, `
function calc(): i32 {
    let a: i32 = 3;
    let b: i32 = 4;
    return a + b * 2;
}`, "calc"); got != 0 {
		t.Errorf("expected 0 BOX in pure arithmetic, got %d", got)
	}
}

// TestBoxLoweredToAllocAndStore: lowering expands every BOX into ALLOC_OBJ (+ a field store), so no
// BOX survives to codegen.
func TestBoxLoweredToAllocAndStore(t *testing.T) {
	builder := runTC(t, `
function g(): void {
    let n: Number = 5;
}`)
	block := getFuncEntryBlock(builder, "g")
	if got := countInstrType(allBlockInstrs(block), ir.InstrTypeBox); got != 1 {
		t.Fatalf("expected 1 BOX before lowering, got %d", got)
	}

	ir.NewLowerer(builder).Lower()

	body := allBlockInstrs(getFuncEntryBlock(builder, "g"))
	if got := countInstrType(body, ir.InstrTypeBox); got != 0 {
		t.Errorf("expected 0 BOX after lowering, got %d", got)
	}
	if got := countInstrType(body, ir.InstrTypeAllocObj); got < 1 {
		t.Errorf("expected >= 1 ALLOC_OBJ after lowering, got %d", got)
	}
}
