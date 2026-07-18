package ir_test

import (
	"testing"
)

// funcToStringCount returns how many `toString` method calls appear in the named function's body.
func funcToStringCount(t *testing.T, source, funcName string) int {
	t.Helper()
	builder := runTC(t, source)
	block := getFuncEntryBlock(builder, funcName)
	if block == nil {
		t.Fatalf("function '%s' not found", funcName)
	}
	return len(findMethodCalls(allBlockInstrs(block), "toString"))
}

// TestConsoleLogPrimitiveEmitsToString: passing a primitive where a string is expected autoboxes it
// and calls toString() (the Stringify -> string implicit conversion).
func TestConsoleLogPrimitiveEmitsToString(t *testing.T) {
	src := `
function f(): void {
    let s: string = 5;
}`
	if got := funcBoxCount(t, src, "f"); got != 1 {
		t.Errorf("expected 1 BOX (i32 -> I32) before toString, got %d", got)
	}
	if got := funcToStringCount(t, src, "f"); got != 1 {
		t.Errorf("expected 1 toString() call for i32 -> string, got %d", got)
	}
}

// TestStringConcatCoercesOperand: `string + <non-string>` converts the non-string operand via
// toString so it concatenates.
func TestStringConcatCoercesOperand(t *testing.T) {
	if got := funcToStringCount(t, `
function f(): void {
    let s: string = "count: " + 42;
}`, "f"); got != 1 {
		t.Errorf("expected 1 toString() call for the i32 concat operand, got %d", got)
	}
}

// TestUserObjectWithToStringConvertsToString: an object with toString() implements Stringify and
// converts to string.
func TestUserObjectWithToStringConvertsToString(t *testing.T) {
	if got := funcToStringCount(t, `
class P { public toString(): string { return "p"; } }
function f(p: P): void {
    let s: string = p;
}`, "f"); got != 1 {
		t.Errorf("expected 1 toString() call for a Stringify object, got %d", got)
	}
}

// TestNonStringifyToStringFails: a value whose type has no toString() cannot convert to string.
func TestNonStringifyToStringFails(t *testing.T) {
	runTCExpectError(t, `
class Bare { public x: i32; }
function f(b: Bare): void {
    let s: string = b;
}`, "assignable")
}

// TestPureNumericAddNotCoerced: numeric addition of two non-strings is untouched by the concat path.
func TestPureNumericAddNotCoerced(t *testing.T) {
	if got := funcToStringCount(t, `
function f(): void {
    let x: i32 = 3 + 4;
}`, "f"); got != 0 {
		t.Errorf("expected 0 toString() calls for pure numeric add, got %d", got)
	}
}
