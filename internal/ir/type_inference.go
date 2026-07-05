package ir

import "github.com/ameerthehacker/zeus/internal/zeus_value"

// undefinedOrNil returns true when a type carries no useful information yet.
func undefinedOrNil(t zeus_value.ValueType) bool {
	return t == nil || zeus_value.IsUndefinedType(t)
}

// TypeInferencePass propagates types through the IR before the main type-checker
// runs. It is registered as the very first TCPass so that downstream passes see
// concrete types everywhere possible.
//
// Rules applied per instruction:
//
//	NEW_OBJ(cls)           → output.ValueType = ObjectType{cls}
//	DECL_VAR(v, init)      → if v.ValueType is unknown and init has a type, set v.ValueType
//	LOAD(addr)             → output.ValueType = addr.ValueType
//	STORE(addr, val)       → if addr.ValueType is unknown, set it from val's type
//	OBJECT_PROPERTY_ACCESS → bidirectionally sync output var ↔ class property definition
//
// The pass calls tc.runPass(itself) from Finalize when any type was updated,
// repeating until a fixpoint is reached.
type TypeInferencePass struct {
	changed bool
}

func NewTypeInferencePass() *TypeInferencePass {
	return &TypeInferencePass{}
}

func (p *TypeInferencePass) GetName() string { return "TypeInferencePass" }

func (p *TypeInferencePass) HandleInstruction(tc *TypeChecker, instr *Instr) {
	switch instr.Type {

	// NEW_OBJ: the result always has the type of the instantiated class.
	case InstrTypeNewObj:
		if instr.Output == nil || !undefinedOrNil(instr.Output.ValueType) {
			return
		}
		input := AsNewObjInstrInput(instr.Input)
		if cls := zeus_value.AsClass(input.Callee); cls != nil {
			instr.Output.ValueType = zeus_value.NewObjectType(*cls)
			p.changed = true
		}

	// DECL_VAR: when the variable has no declared type, infer from its initializer.
	case InstrTypeDeclVar:
		input := AsDeclVarInstrInput(instr.Input)
		if !undefinedOrNil(input.Variable.ValueType) || input.Initializer == nil {
			return
		}
		if vt := zeus_value.GetValueType(input.Initializer); !undefinedOrNil(vt) {
			input.Variable.ValueType = vt
			p.changed = true
		}

	// LOAD: the loaded value has the same type as the source alloca.
	case InstrTypeLoad:
		if instr.Output == nil || !undefinedOrNil(instr.Output.ValueType) {
			return
		}
		input := AsLoadInstrInput(instr.Input)
		if !undefinedOrNil(input.Addr.ValueType) {
			instr.Output.ValueType = input.Addr.ValueType
			p.changed = true
		}

	// STORE: if the target has no type yet, infer it from what is being stored.
	case InstrTypeStore:
		input := AsStoreInstrInput(instr.Input)
		if !undefinedOrNil(input.Addr.ValueType) {
			return
		}
		if vt := zeus_value.GetValueType(input.Value); !undefinedOrNil(vt) {
			input.Addr.ValueType = vt
			p.changed = true
		}

	// OBJECT_PROPERTY_ACCESS: bidirectionally sync the output var with the class
	// property definition.
	//
	//   forward  – class prop has a type, output does not → copy to output.
	//   backward – output has a type (set by prior STORE propagation), class prop
	//              does not → write back to class property so that later reads from
	//              the same property (e.g. inside a functor __call__) resolve too.
	//
	// Class.Properties is []*ClassProperty, so mutations through the pointer are
	// reflected in the original class definition shared across all references.
	case InstrTypeObjectPropertyAccess:
		if instr.Output == nil {
			return
		}
		input := AsObjectPropertyAccessInstrInput(instr.Input)
		objType := zeus_value.GetValueType(input.Object)
		if !zeus_value.IsObjectType(objType) {
			return
		}
		cls := zeus_value.AsObjectType(objType).Class
		for _, prop := range cls.Properties {
			if prop.Property.Name != input.Property {
				continue
			}
			// Forward: class property → output var
			if undefinedOrNil(instr.Output.ValueType) && !undefinedOrNil(prop.Property.ValueType) {
				instr.Output.ValueType = prop.Property.ValueType
				p.changed = true
			}
			// Backward: output var → class property
			if !undefinedOrNil(instr.Output.ValueType) && undefinedOrNil(prop.Property.ValueType) {
				prop.Property.ValueType = instr.Output.ValueType
				p.changed = true
			}
			break
		}
	}
}

// Finalize re-runs the pass when any type was updated so that multi-step chains
// fully resolve (e.g. NEW_OBJ → DECL_VAR → LOAD → STORE → OBJECT_PROPERTY_ACCESS
// backward → another OBJECT_PROPERTY_ACCESS in __call__).
func (p *TypeInferencePass) Finalize(tc *TypeChecker) {
	if p.changed {
		p.changed = false
		tc.runPass(p)
	}
}
