package ir_test

import (
	"testing"
)

// TestPreludesLoadedViaNewIRBuilder confirms the relocated prelude bootstrap makes the
// prelude-defined primordials (string, Error, Console, and the timer functions) available to a
// plain NewIRBuilder-driven IR-gen — i.e. without going through the compiler's NewCompiler. It also
// checks that string/Error/Console arrive with extern methods, i.e. sourced from prelude/*.zs.
func TestPreludesLoadedViaNewIRBuilder(t *testing.T) {
	// This compiles only if string.concat, new Error / .message, and setTimeout all resolve — which
	// requires the preludes to have loaded for this NewIRBuilder (generateHIR uses one). The
	// `console` *global object* is only created for entry modules, so it is not referenced here; the
	// Console *class* is still emitted into every module by initializePrimordials and asserted below.
	_, instrs := generateHIR(t, `
function main(): i32 {
    let s: string = "hi".concat("!");
    let e: Error = new Error("boom");
    let m: string = e.message;
    let id: i32 = setTimeout(() => { }, 1);
    return 0;
}`)

	for _, name := range []string{"string", "Error", "Console"} {
		class := classBySourceName(t, instrs, name)
		hasExtern := false
		for _, m := range class.Methods {
			if m.IsExtern {
				hasExtern = true
				break
			}
		}
		if !hasExtern {
			t.Errorf("%s: expected extern (prelude-sourced) methods, found none", name)
		}
	}
}
