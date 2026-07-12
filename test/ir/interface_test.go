package ir_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// fieldType returns the type of a class's instance field by name (via its layout).
func fieldType(t *testing.T, class *zeus_value.Class, name string) zeus_value.ValueType {
	t.Helper()
	for _, f := range class.Layout().Fields {
		if f.Property.Name == name {
			return f.Property.ValueType
		}
	}
	t.Fatalf("field %q not found on class %q", name, class.SourceName())
	return nil
}

// methodReturnType returns the return type of a class method by source name.
func methodReturnType(t *testing.T, class *zeus_value.Class, name string) zeus_value.ValueType {
	t.Helper()
	m := zeus_value.LookupMethod(class, name)
	if m == nil {
		t.Fatalf("method %q not found on class %q", name, class.SourceName())
	}
	return m.Method.ReturnType
}

// vtableSlot returns the vtable slot index of a method by source name in a class's layout.
func vtableSlot(t *testing.T, class *zeus_value.Class, name string) int {
	t.Helper()
	for slot, entry := range class.Layout().VTable {
		if entry.Method.Method.SourceName() == name {
			return slot
		}
	}
	t.Fatalf("method %q has no vtable slot on class %q", name, class.SourceName())
	return -1
}

// fieldIndex returns the 0-based index of a field in a class's instance layout.
func fieldIndex(t *testing.T, class *zeus_value.Class, name string) int {
	t.Helper()
	for i, f := range class.Layout().Fields {
		if f.Property.Name == name {
			return i
		}
	}
	t.Fatalf("field %q not found on class %q", name, class.SourceName())
	return -1
}

// shapeInterfaceFor builds `interface Shape { radius: i32; area(): i32; }` using the concrete
// member types of the given class, so structural conformance compares matching types without
// hard-coding a specific i32 representation.
func shapeInterfaceFor(t *testing.T, class *zeus_value.Class) *zeus_value.Interface {
	t.Helper()
	radiusVar := zeus_value.NewVar("radius", fieldType(t, class, "radius"), false, nil)
	prop := zeus_value.NewClassProperty(radiusVar, nil, false, false, nil)

	area := zeus_value.NewFunction("area", []*zeus_value.Var{}, methodReturnType(t, class, "area"), nil)
	area.OriginalName = "area"

	return zeus_value.NewInterface("Shape", nil, []*zeus_value.ClassProperty{prop}, []*zeus_value.Function{area}, nil)
}

// TestInterfaceConformance: a class conforms iff it structurally has every interface member.
func TestInterfaceConformance(t *testing.T) {
	_, instrs := generateHIR(t, `
class Circle {
  public radius: i32;
  constructor(r: i32) { this.radius = r; }
  public area(): i32 { return this.radius; }
}
class Square {
  public side: i32;
  constructor(s: i32) { this.side = s; }
}`)
	instrs = filterUserInstrs(instrs)
	circle := classBySourceName(t, instrs, "Circle")
	square := classBySourceName(t, instrs, "Square")

	shape := shapeInterfaceFor(t, circle)

	if !zeus_value.ClassConformsToInterface(circle, shape) {
		t.Fatalf("Circle should conform to Shape")
	}
	// Square has neither `radius` nor `area()`, so it must not conform.
	if zeus_value.ClassConformsToInterface(square, shape) {
		t.Fatalf("Square should NOT conform to Shape")
	}
}

// TestInterfaceDispatchLayout: BuildInterfaceDispatchLayout materializes, as pure data, one row
// per conforming class with the correct vtable slot per method and field index per property.
func TestInterfaceDispatchLayout(t *testing.T) {
	_, instrs := generateHIR(t, `
class Circle {
  public radius: i32;
  constructor(r: i32) { this.radius = r; }
  public area(): i32 { return this.radius; }
}
class Square {
  public side: i32;
  constructor(s: i32) { this.side = s; }
}`)
	instrs = filterUserInstrs(instrs)
	circle := classBySourceName(t, instrs, "Circle")
	square := classBySourceName(t, instrs, "Square")

	shape := shapeInterfaceFor(t, circle)
	layout := zeus_value.BuildInterfaceDispatchLayout(shape, []*zeus_value.Class{circle, square})

	// Only Circle conforms, so exactly one row keyed by Circle's class id.
	if len(layout.Rows) != 1 {
		t.Fatalf("expected 1 dispatch row, got %d", len(layout.Rows))
	}
	row := layout.Rows[0]
	if row.ClassId != circle.Id {
		t.Fatalf("row class id: got %d, want %d", row.ClassId, circle.Id)
	}
	if layout.MaxClassId != circle.Id {
		t.Fatalf("max class id: got %d, want %d", layout.MaxClassId, circle.Id)
	}

	// Method slot must equal Circle's own vtable slot for area().
	wantSlot := vtableSlot(t, circle, "area")
	if len(row.MethodSlots) != 1 || row.MethodSlots[0] != wantSlot {
		t.Fatalf("method slots: got %v, want [%d]", row.MethodSlots, wantSlot)
	}

	// Property backing must be the field radius at its 0-based index in Circle's fields.
	wantIndex := fieldIndex(t, circle, "radius")
	if len(row.PropertyBackings) != 1 ||
		row.PropertyBackings[0].Kind != zeus_value.PropertyBackingField ||
		row.PropertyBackings[0].FieldIndex != wantIndex {
		t.Fatalf("property backings: got %+v, want field @%d", row.PropertyBackings, wantIndex)
	}
}

// TestInterfacePropertyBackingAccessor: a class that provides a property via get/set accessors
// resolves to an Accessor PropertyBacking pointing at the getter/setter vtable slots.
func TestInterfacePropertyBackingAccessor(t *testing.T) {
	_, instrs := generateHIR(t, `
class Widget {
  private v: i32;
  constructor(w: i32) { this.v = w; }
  get value(): i32 { return this.v; }
  set value(x: i32) { this.v = x; }
}`)
	instrs = filterUserInstrs(instrs)
	widget := classBySourceName(t, instrs, "Widget")

	acc := zeus_value.LookupAccessor(widget, "value")
	if acc == nil || acc.Getter == nil || acc.Setter == nil {
		t.Fatalf("Widget should have a get/set accessor for value")
	}
	// A writable interface property `value: T` backed by Widget's accessor.
	prop := zeus_value.NewClassProperty(
		zeus_value.NewVar("value", acc.Getter.ReturnType, false, nil), nil, false, false, nil)

	backing, ok := zeus_value.ResolveInterfacePropertyBacking(widget, prop, true /*writable*/)
	if !ok {
		t.Fatalf("Widget should back a writable `value` via its accessor")
	}
	if backing.Kind != zeus_value.PropertyBackingAccessor {
		t.Fatalf("backing kind: got %+v, want accessor", backing)
	}
	wantGet := vtableSlot(t, widget, acc.Getter.SourceName())
	wantSet := vtableSlot(t, widget, acc.Setter.SourceName())
	if backing.GetterSlot != wantGet || backing.SetterSlot != wantSet {
		t.Fatalf("accessor slots: got get=%d set=%d, want get=%d set=%d",
			backing.GetterSlot, backing.SetterSlot, wantGet, wantSet)
	}

	// Readonly (getter-only requirement) also resolves.
	if _, ok := zeus_value.ResolveInterfacePropertyBacking(widget, prop, false /*readonly*/); !ok {
		t.Fatalf("Widget should back a readonly `value` via its getter")
	}
}
