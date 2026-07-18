package ir_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/ir"
)

// stringTemplateOutputType returns the output type-string of the StringTemplate instr in the named
// function's body (before lowering), or "" if none.
func stringTemplateOutputType(t *testing.T, source, funcName string) string {
	t.Helper()
	builder := runTC(t, source)
	block := getFuncEntryBlock(builder, funcName)
	if block == nil {
		t.Fatalf("function '%s' not found", funcName)
	}
	for _, instr := range allBlockInstrs(block) {
		if instr.Type == ir.InstrTypeStringTemplate && instr.Output != nil && instr.Output.ValueType != nil {
			return instr.Output.ValueType.String()
		}
	}
	return ""
}

// TestTemplateResultIsString: a template literal (even a single `${n}`) is a STRING_TEMPLATE node
// typed as `string` — fixing the old "single interpolation returns the raw value" edge.
func TestTemplateResultIsString(t *testing.T) {
	if got := stringTemplateOutputType(t, `
function f(): void {
    let s = `+"`${5}`"+`;
}`, "f"); got != "string" {
		t.Errorf("expected template result type 'string', got %q", got)
	}
}

// TestTemplateInterpolationErrorIsTemplateSpecific: interpolating a value without toString yields a
// template-specific error (NOT the generic "binary operation" message).
func TestTemplateInterpolationErrorIsTemplateSpecific(t *testing.T) {
	runTCExpectError(t, `
class Bare { public x: i32; }
function f(o: Bare): void {
    let s: string = `+"`val ${o} ff`"+`;
}`, "cannot interpolate")
}

// TestEmptyAndStaticTemplatesLower: the empty template (0-part / acc==nil branch) and a static-only
// template (single-part, no concat) both lower away cleanly to no surviving STRING_TEMPLATE.
func TestEmptyAndStaticTemplatesLower(t *testing.T) {
	// Backticks are literal inside a Go double-quoted string.
	builder := runTC(t, "function f(): void {\n\tlet a: string = ``;\n\tlet b: string = `hi`;\n}")
	body := allBlockInstrs(getFuncEntryBlock(builder, "f"))
	if got := countInstrType(body, ir.InstrTypeStringTemplate); got != 2 {
		t.Fatalf("expected 2 STRING_TEMPLATE before lowering, got %d", got)
	}
	ir.NewLowerer(builder).Lower()
	if got := countInstrType(allBlockInstrs(getFuncEntryBlock(builder, "f")), ir.InstrTypeStringTemplate); got != 0 {
		t.Errorf("expected 0 STRING_TEMPLATE after lowering, got %d", got)
	}
}

// TestTemplateLowersToConcat: StringTemplate is fully lowered away (to concat) before codegen.
func TestTemplateLowersToConcat(t *testing.T) {
	builder := runTC(t, `
function f(): void {
    let n: i32 = 5;
    let s: string = `+"`a${n}b`"+`;
}`)
	block := getFuncEntryBlock(builder, "f")
	if got := countInstrType(allBlockInstrs(block), ir.InstrTypeStringTemplate); got != 1 {
		t.Fatalf("expected 1 STRING_TEMPLATE before lowering, got %d", got)
	}

	ir.NewLowerer(builder).Lower()

	body := allBlockInstrs(getFuncEntryBlock(builder, "f"))
	if got := countInstrType(body, ir.InstrTypeStringTemplate); got != 0 {
		t.Errorf("expected 0 STRING_TEMPLATE after lowering, got %d", got)
	}
	if got := len(findMethodCalls(body, "concat")); got < 1 {
		t.Errorf("expected >= 1 concat call after lowering, got %d", got)
	}
}
