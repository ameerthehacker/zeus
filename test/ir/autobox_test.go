package ir_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/ir"
)

// funcInstrCount type-checks source and returns the count of the given instruction type in the
// named function's body (before lowering).
func funcInstrCount(t *testing.T, source, funcName string, typ ir.InstrType) int {
	t.Helper()
	builder := runTC(t, source)
	block := getFuncEntryBlock(builder, funcName)
	if block == nil {
		t.Fatalf("function '%s' not found", funcName)
	}
	return countInstrType(allBlockInstrs(block), typ)
}

func funcBoxCount(t *testing.T, source, funcName string) int {
	t.Helper()
	return funcInstrCount(t, source, funcName, ir.InstrTypeBox)
}

func funcReflectCount(t *testing.T, source, funcName string) int {
	t.Helper()
	return funcInstrCount(t, source, funcName, ir.InstrTypeReflectToString)
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

// funcBoxTargetNames returns the target class name of every BOX in the named function's body.
func funcBoxTargetNames(t *testing.T, source, funcName string) []string {
	t.Helper()
	builder := runTC(t, source)
	block := getFuncEntryBlock(builder, funcName)
	if block == nil {
		t.Fatalf("function '%s' not found", funcName)
	}
	var names []string
	for _, instr := range allBlockInstrs(block) {
		if instr.Type == ir.InstrTypeBox {
			names = append(names, ir.AsBoxInstrInput(instr.Input).TargetClass.Name)
		}
	}
	return names
}

// funcValueOfCount returns how many `valueOf` method calls appear in the named function's body.
func funcValueOfCount(t *testing.T, source, funcName string) int {
	t.Helper()
	builder := runTC(t, source)
	block := getFuncEntryBlock(builder, funcName)
	if block == nil {
		t.Fatalf("function '%s' not found", funcName)
	}
	return len(findMethodCalls(allBlockInstrs(block), "valueOf"))
}

// funcUnboxAndValueOfCounts returns (field-read UNBOX count, valueOf() call count) in the named
// function's body from a single type-check compile.
func funcUnboxAndValueOfCounts(t *testing.T, source, funcName string) (unbox int, valueOf int) {
	t.Helper()
	builder := runTC(t, source)
	block := getFuncEntryBlock(builder, funcName)
	if block == nil {
		t.Fatalf("function '%s' not found", funcName)
	}
	instrs := allBlockInstrs(block)
	return countInstrType(instrs, ir.InstrTypeUnbox), len(findMethodCalls(instrs, "valueOf"))
}

// TestUserClassShadowingBoxNameIsNotAutoboxed: a user class reusing a box name (I32) must NOT be
// treated as the primordial box — box detection/lookup are identity-based, so autoboxing a scalar
// into the user's I32 (a different LLVM struct) is rejected as a plain assignability error instead
// of silently type-punning (which segfaulted before the fix).
func TestUserClassShadowingBoxNameIsNotAutoboxed(t *testing.T) {
	runTCExpectError(t, `
class I32 { public tag: string; }
function f(): void {
    let n: I32 = 5;
}`, "assignable")
}

// TestBoxPicksExactType: a value boxes into its exact per-type box, not a single f64 box.
// 9007199254740993 exceeds i32, so it is an i64 literal and must box into I64.
func TestBoxPicksExactType(t *testing.T) {
	names := funcBoxTargetNames(t, `
function f(): void {
    let s: string = (9007199254740993).toString();
}`, "f")
	if len(names) != 1 || names[0] != "I64" {
		t.Errorf("expected box target I64, got %v", names)
	}
}

// TestConcreteBoxUnboxIsExact: unboxing a concrete first-class box (I32) into its primitive reads
// the exact field (UNBOX), NOT valueOf()/f64 — that is the whole point of the fine-grained boxes.
func TestConcreteBoxUnboxIsExact(t *testing.T) {
	unbox, valueOf := funcUnboxAndValueOfCounts(t, `
function f(): void {
    let n: I32 = 5;
    let x: i32 = n;
}`, "f")
	if unbox != 1 {
		t.Errorf("expected 1 field-read UNBOX for I32 -> i32, got %d", unbox)
	}
	if valueOf != 0 {
		t.Errorf("expected 0 valueOf() (no f64 collapse) for a concrete box, got %d", valueOf)
	}
}

// TestConcreteBoxArithmeticIsExact: arithmetic on concrete boxes unboxes each to its exact scalar
// (field-read UNBOX), so I32 + I32 computes on i32 — no valueOf()/f64 collapse.
func TestConcreteBoxArithmeticIsExact(t *testing.T) {
	unbox, valueOf := funcUnboxAndValueOfCounts(t, `
function f(): void {
    let a: I32 = 3;
    let b: I32 = 4;
    let s: I32 = a + b;
}`, "f")
	if unbox != 2 {
		t.Errorf("expected 2 field-read UNBOX for `a + b` on two I32, got %d", unbox)
	}
	if valueOf != 0 {
		t.Errorf("expected 0 valueOf() for concrete-box arithmetic, got %d", valueOf)
	}
}

// TestUnboxNumberViaValueOf: a Number (umbrella) assigned into an f64 slot unboxes through
// valueOf() (the concrete type is hidden behind the interface), not a field-read UNBOX.
func TestUnboxNumberViaValueOf(t *testing.T) {
	unbox, valueOf := funcUnboxAndValueOfCounts(t, `
function f(): void {
    let n: Number = 5;
    let d: f64 = n;
}`, "f")
	if valueOf != 1 {
		t.Errorf("expected 1 valueOf() call for Number -> f64, got %d", valueOf)
	}
	if unbox != 0 {
		t.Errorf("expected 0 field-read UNBOX for a Number, got %d", unbox)
	}
}

// TestArithmeticUnboxesViaValueOf: both boxed operands of an arithmetic op unbox via valueOf().
func TestArithmeticUnboxesViaValueOf(t *testing.T) {
	if got := funcValueOfCount(t, `
function f(): void {
    let a: Number = 3;
    let b: Number = 4;
    let s: Number = a + b;
}`, "f"); got != 2 {
		t.Errorf("expected 2 valueOf() calls for `a + b` on two Numbers, got %d", got)
	}
}

// TestBoolUnboxViaFieldRead: Bool is a concrete class, so unboxing it into a boolean reads its
// exact `value` field (a real UNBOX) rather than going through valueOf.
func TestBoolUnboxViaFieldRead(t *testing.T) {
	unbox, valueOf := funcUnboxAndValueOfCounts(t, `
function f(): void {
    let b: Bool = true;
    let raw: boolean = b;
}`, "f")
	if unbox != 1 {
		t.Errorf("expected 1 field-read UNBOX for Bool -> boolean, got %d", unbox)
	}
	if valueOf != 0 {
		t.Errorf("expected 0 valueOf() calls for a Bool, got %d", valueOf)
	}
}

// TestNoUnboxAgainstNull: a method call on a Number receiver keeps its compiler-generated null
// check (`receiver == null`) — the receiver must NOT be unboxed there, so no valueOf appears.
func TestNoUnboxAgainstNull(t *testing.T) {
	if got := funcValueOfCount(t, `
function f(): void {
    let n: Number = 5;
    let s: string = n.toString();
}`, "f"); got != 0 {
		t.Errorf("expected 0 valueOf() calls for a method call on a Number receiver, got %d", got)
	}
}

// TestBoolUnboxLoweredToLoad: lowering expands a Bool's field-read UNBOX into
// OBJECT_PROPERTY_ACCESS + LOAD, so no UNBOX survives to codegen.
func TestBoolUnboxLoweredToLoad(t *testing.T) {
	builder := runTC(t, `
function f(): void {
    let b: Bool = true;
    let raw: boolean = b;
}`)
	if got := countInstrType(allBlockInstrs(getFuncEntryBlock(builder, "f")), ir.InstrTypeUnbox); got != 1 {
		t.Fatalf("expected 1 UNBOX before lowering, got %d", got)
	}

	ir.NewLowerer(builder).Lower()

	if got := countInstrType(allBlockInstrs(getFuncEntryBlock(builder, "f")), ir.InstrTypeUnbox); got != 0 {
		t.Errorf("expected 0 UNBOX after lowering, got %d", got)
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
