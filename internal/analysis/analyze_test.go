package analysis

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

func errorCount(diags []*zeus_error.ZeusError) int {
	n := 0
	for _, d := range diags {
		if d.Severity == zeus_error.ErrorSeverityError {
			n++
		}
	}
	return n
}

func classID(t *testing.T, res *Result, name string) int {
	t.Helper()
	if res.Module == nil {
		t.Fatalf("expected a module, got nil (diagnostics: %v)", res.Diagnostics)
	}
	sym, ok := res.Module.GetAllSymbols()[name]
	if !ok {
		t.Fatalf("class %q not found in module symbols", name)
	}
	class := zeus_value.AsClass(sym)
	if class == nil {
		t.Fatalf("symbol %q is not a class", name)
	}
	return class.Id
}

// Analyzing the same source repeatedly (as the language server does on every keystroke) must be
// deterministic: user-class ids are rewound to the primordial watermark each run, so they do not
// drift or grow without bound.
func TestAnalyzeIsStableAcrossRuns(t *testing.T) {
	src := `const Point = class {
  x: i32
  y: i32
}
let p = new Point()`

	r1 := Analyze("mem.zs", src, nil)
	r2 := Analyze("mem.zs", src, nil)

	if n := errorCount(r1.Diagnostics); n != 0 {
		t.Fatalf("unexpected error-severity diagnostics on valid source: %v", r1.Diagnostics)
	}
	if r1.AST == nil {
		t.Fatalf("expected a parsed AST for valid source")
	}

	if id1, id2 := classID(t, r1, "Point"), classID(t, r2, "Point"); id1 != id2 {
		t.Fatalf("class id drifted across runs: %d then %d", id1, id2)
	}
}

// Incomplete/broken source must produce diagnostics and return normally — never panic or exit.
func TestAnalyzeToleratesBrokenSource(t *testing.T) {
	res := Analyze("mem.zs", "let x = ", nil)
	if len(res.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics for incomplete source")
	}
	// Reaching this point (no panic, no os.Exit) is the assertion.
}
