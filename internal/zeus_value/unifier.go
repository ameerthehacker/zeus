package zeus_value

import "fmt"

// Unifier implements Robinson's unification algorithm for Zeus types.
// It resolves TypeVars to concrete types by collecting substitutions.
// This is the foundation for future generics and overload resolution.
type Unifier struct {
	subst map[int]ValueType // TypeVar.ID → concrete type
}

func NewUnifier() *Unifier {
	return &Unifier{subst: make(map[int]ValueType)}
}

// Apply substitutes all resolved TypeVars in t with their concrete types.
func (u *Unifier) Apply(t ValueType) ValueType {
	if t == nil {
		return nil
	}
	tv := AsTypeVar(t)
	if tv == nil {
		return t
	}
	resolved, ok := u.subst[tv.ID]
	if !ok {
		return t
	}
	// Follow the substitution chain.
	return u.Apply(resolved)
}

// Unify attempts to make a and b the same type, updating substitutions.
// Returns an error on conflict (e.g. int vs string).
func (u *Unifier) Unify(a, b ValueType) error {
	a = u.Apply(a)
	b = u.Apply(b)

	if a == nil || b == nil {
		return nil
	}

	// Both TypeVars: make one point to the other.
	tvA := AsTypeVar(a)
	tvB := AsTypeVar(b)

	if tvA != nil && tvB != nil {
		if tvA.ID != tvB.ID {
			u.subst[tvA.ID] = b
		}
		return nil
	}

	// Left is TypeVar: bind it to the concrete right side.
	if tvA != nil {
		if u.occursIn(tvA.ID, b) {
			return fmt.Errorf("infinite type: T%d occurs in %s", tvA.ID, b)
		}
		u.subst[tvA.ID] = b
		return nil
	}

	// Right is TypeVar: symmetric.
	if tvB != nil {
		if u.occursIn(tvB.ID, a) {
			return fmt.Errorf("infinite type: T%d occurs in %s", tvB.ID, a)
		}
		u.subst[tvB.ID] = a
		return nil
	}

	// Both concrete: must match structurally.
	switch ca := a.(type) {
	case FunctionType:
		cb, ok := b.(FunctionType)
		if !ok {
			return fmt.Errorf("cannot unify %s with %s", a, b)
		}
		if len(ca.ParamTypes) != len(cb.ParamTypes) {
			return fmt.Errorf("function arity mismatch: %d vs %d", len(ca.ParamTypes), len(cb.ParamTypes))
		}
		for i := range ca.ParamTypes {
			if err := u.Unify(ca.ParamTypes[i], cb.ParamTypes[i]); err != nil {
				return err
			}
		}
		return u.Unify(ca.ReturnType, cb.ReturnType)
	default:
		if !CmpValueType(a, b) {
			return fmt.Errorf("type mismatch: %s vs %s", a, b)
		}
		return nil
	}
}

// occursIn returns true if TypeVar with the given ID appears anywhere in t.
// This prevents creating infinite types like T0 = List<T0>.
func (u *Unifier) occursIn(id int, t ValueType) bool {
	t = u.Apply(t)
	if t == nil {
		return false
	}
	if tv := AsTypeVar(t); tv != nil {
		return tv.ID == id
	}
	if ft, ok := t.(FunctionType); ok {
		if u.occursIn(id, ft.ReturnType) {
			return true
		}
		for _, p := range ft.ParamTypes {
			if u.occursIn(id, p) {
				return true
			}
		}
	}
	return false
}
