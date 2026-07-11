package ir_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// classBySourceName finds the DECL_CLASS instruction for a user class by its source name and
// returns the finalized *zeus_value.Class, so ClassLayout can be asserted without LLVM.
func classBySourceName(t *testing.T, instrs []*ir.Instr, name string) *zeus_value.Class {
	t.Helper()
	for _, instr := range instrs {
		if instr.Type != ir.InstrTypeDeclClass {
			continue
		}
		class := ir.AsDeclClassInstrInput(instr.Input).Class
		if class.SourceName() == name {
			return class
		}
	}
	t.Fatalf("class %q not found in IR", name)
	return nil
}

// fieldNames returns the source names of a layout's instance fields in physical order.
func fieldNames(layout *zeus_value.ClassLayout) []string {
	names := make([]string, len(layout.Fields))
	for i, f := range layout.Fields {
		names[i] = f.Property.Name
	}
	return names
}

func assertStrings(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", what, got, want)
		}
	}
}

// TestLayoutFieldsBaseFirst: instance fields are laid out base-first, then the derived class's own.
func TestLayoutFieldsBaseFirst(t *testing.T) {
	_, instrs := generateHIR(t, `
class Base {
    public a: i32;
    public b: i32;
}
class Derived extends Base {
    public c: i32;
}`)
	instrs = filterUserInstrs(instrs)
	base := classBySourceName(t, instrs, "Base")
	derived := classBySourceName(t, instrs, "Derived")

	assertStrings(t, "Base.Fields", fieldNames(base.Layout()), []string{"a", "b"})
	assertStrings(t, "Derived.Fields", fieldNames(derived.Layout()), []string{"a", "b", "c"})
}

// TestLayoutVTableSlotsAndOverride: an override reuses the base method's slot in place while a new
// method is appended, and each slot records the class whose method fills it (DefiningClass).
func TestLayoutVTableSlotsAndOverride(t *testing.T) {
	_, instrs := generateHIR(t, `
class Base {
    public foo(): i32 { return 1; }
    public bar(): i32 { return 2; }
}
class Derived extends Base {
    public foo(): i32 { return 3; }
    public baz(): i32 { return 4; }
}`)
	instrs = filterUserInstrs(instrs)
	base := classBySourceName(t, instrs, "Base")
	derived := classBySourceName(t, instrs, "Derived")

	// Base: foo@0, bar@1 — both defined by Base.
	baseVT := base.Layout().VTable
	if len(baseVT) != 2 {
		t.Fatalf("Base.VTable: expected 2 slots, got %d", len(baseVT))
	}

	// Derived: foo@0 (override, defined by Derived), bar@1 (inherited from Base), baz@2 (new).
	vt := derived.Layout().VTable
	if len(vt) != 3 {
		t.Fatalf("Derived.VTable: expected 3 slots, got %d", len(vt))
	}
	wantName := []string{"foo", "bar", "baz"}
	wantDefiner := []string{"Derived", "Base", "Derived"}
	for slot, entry := range vt {
		if got := entry.Method.Method.SourceName(); got != wantName[slot] {
			t.Errorf("Derived.VTable[%d]: method %q, want %q", slot, got, wantName[slot])
		}
		if got := entry.DefiningClass.SourceName(); got != wantDefiner[slot] {
			t.Errorf("Derived.VTable[%d] (%s): DefiningClass %q, want %q", slot, wantName[slot], got, wantDefiner[slot])
		}
	}

	// The override must occupy the same slot index the base assigned to foo (dynamic dispatch).
	if base.Layout().VTable[0].Method.Method.SourceName() != "foo" {
		t.Errorf("Base.VTable[0]: want foo, got %q", base.Layout().VTable[0].Method.Method.SourceName())
	}
}

// TestLayoutEffectiveConstructorOwn: a class with its own constructor is its own ConstructorClass.
func TestLayoutEffectiveConstructorOwn(t *testing.T) {
	_, instrs := generateHIR(t, `
class Point {
    public x: i32;
    constructor(x: i32) { this.x = x; }
}`)
	point := classBySourceName(t, filterUserInstrs(instrs), "Point")
	layout := point.Layout()
	if layout.Constructor == nil {
		t.Fatal("Point.Constructor: expected non-nil")
	}
	if layout.ConstructorClass == nil || layout.ConstructorClass.SourceName() != "Point" {
		t.Errorf("Point.ConstructorClass: want Point, got %v", layout.ConstructorClass)
	}
}

// TestLayoutEffectiveConstructorInherited: a derived class without its own constructor inherits the
// nearest ancestor's constructor as its effective constructor.
func TestLayoutEffectiveConstructorInherited(t *testing.T) {
	_, instrs := generateHIR(t, `
class Base {
    public a: i32;
    constructor(a: i32) { this.a = a; }
}
class Derived extends Base {
    public foo(): i32 { return 1; }
}`)
	derived := classBySourceName(t, filterUserInstrs(instrs), "Derived")
	layout := derived.Layout()
	if layout.Constructor == nil {
		t.Fatal("Derived.Constructor: expected the inherited constructor, got nil")
	}
	if layout.ConstructorClass == nil || layout.ConstructorClass.SourceName() != "Base" {
		t.Errorf("Derived.ConstructorClass: want Base, got %v", layout.ConstructorClass)
	}
}

// TestLayoutNoConstructor: a class with no constructor anywhere in its chain has a nil constructor.
func TestLayoutNoConstructor(t *testing.T) {
	_, instrs := generateHIR(t, `
class Plain {
    public x: i32;
    public foo(): i32 { return 1; }
}`)
	plain := classBySourceName(t, filterUserInstrs(instrs), "Plain")
	layout := plain.Layout()
	if layout.Constructor != nil || layout.ConstructorClass != nil {
		t.Errorf("Plain: expected no effective constructor, got Constructor=%v ConstructorClass=%v",
			layout.Constructor, layout.ConstructorClass)
	}
}
