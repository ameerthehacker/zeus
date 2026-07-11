# Class Inheritance

Zeus supports single class inheritance with the TypeScript-style `extends` keyword. A derived
class inherits its base class's fields, methods, and accessors; it can add its own members and
**override** inherited methods. Overrides are dispatched dynamically — calling a method through a
base-typed reference to a derived object runs the derived implementation.

```zeus
class Animal {
    public name: string;
    constructor(name: string) { this.name = name; }
    public sound(): i32 { return 0; }
}

class Dog extends Animal {
    public breed: string;
    constructor(name: string, breed: string) {
        this.name = name;   // inherited field
        this.breed = breed; // own field
    }
    public sound(): i32 { return 42; }   // override
}

let a: Animal = new Dog("Rex", "Lab");   // upcast — a derived object as a base reference
a.sound();                               // 42 — Dog.sound via dynamic dispatch
```

---

## What is supported

| Feature | Status |
|---------|--------|
| Inherited data fields (read/write) | ✅ |
| Inherited instance methods | ✅ |
| Method overriding | ✅ |
| Dynamic dispatch through a base-typed reference | ✅ |
| Multi-level chains (`C extends B extends A`) | ✅ |
| Polymorphism through a base-typed parameter/return | ✅ |
| Upcast (derived → base) assignment | ✅ |
| Inherited getter/setter accessors | ✅ |
| Inherited static members | ✅ (pre-existing) |
| `super(...)` constructor chaining | ✅ (see [super](#super-constructor-chaining)) |
| `super.method()` (non-virtual base call) | ✅ (see [super](#super-constructor-chaining)) |

`extends` accepts a single base class. There are no interfaces here (those are a separate
feature) and no multiple inheritance.

---

## Mental model: one flattened member list

The entire feature rests on a single idea: **treat a class as the base-first concatenation of its
ancestors' members plus its own.** Field slots, vtable slots, and every "which member is this?"
lookup are derived from that flattened view, so no subsystem has to special-case inheritance — it
just reads a longer list.

Three helpers in `internal/zeus_value/value.go` produce the flattened views, and every consumer
routes through them:

| Helper | Returns | Used for |
|--------|---------|----------|
| `FlattenedInstanceProperties(class)` | instance fields, **base-first then own** | object struct layout, `GetPropertyIndex` |
| `FlattenedVTableMethods(class)` | vtable methods, base slots first, **an override reusing the base's slot**, new methods appended | vtable layout, `GetMethodIndex`, `GetVTableSlot` |
| `FlattenedMethods(class)` | all methods (any kind), base-first then own | type-checker member resolution |
| `LookupMethod(class, name)` | nearest method by source name (derived shadows base) | method-call resolution |
| `IsSubclassOf(class, ancestor)` | `bool` | upcast assignability |

`FlattenedInstanceProperties` and `FlattenedVTableMethods` also define the **physical memory
layout**, so the compiler front-end and codegen agree on member offsets by construction.

---

## Object memory layout

Instance fields are laid out **base-first**. The object struct is the object header pointer
followed by every instance field, ancestors before descendants:

```
class Base    { public a: i32; }              →  { header*, a }
class Derived extends Base { public b: i32; } →  { header*, a, b }
```

Because a derived object *begins* with its base's fields in the same order, **a `Derived*` is
already a valid `Base*`** — no field ever moves. `util.GetPropertyIndex` walks the flattened list
so `a` resolves to slot 1 in both `Base` and `Derived`, and `b` to slot 2 in `Derived` (slot 0 is
the header).

```go
// util.GetPropertyIndex
for instanceIndex, property := range zeus_value.FlattenedInstanceProperties(class) {
    if property.Property.Name == propertyName {
        return instanceIndex + 1 // +1 skips the object header
    }
}
```

Static properties live in dedicated globals, not the instance struct, so
`FlattenedInstanceProperties` excludes them.

---

## VTable layout and dynamic dispatch

Every class has a vtable — an array of function pointers, one slot per instance method. Inheritance
requires that **a method occupies the same slot index in every class of the hierarchy**, so that a
call site indexing with the *static* type still lands on the correct *dynamic* implementation.

`FlattenedVTableMethods` produces exactly that ordering:

1. Start with the base's flattened vtable methods (recursively).
2. For each of the class's own methods: if a same-named method already exists in the list it is an
   **override** → replace that slot in place (keeping its index); otherwise it is **new** → append.

```
class Animal { sound(); eat(); }               vtable: [0:sound,      1:eat]
class Dog extends Animal { sound(); bark(); }   vtable: [0:Dog.sound,  1:Animal.eat,  2:Dog.bark]
                                                          ^ override in     ^ inherited    ^ new
                                                            the base's slot
```

`Animal.sound` and `Dog.sound` are both slot 0. So for `a.sound()` where `a` is statically
`Animal` but dynamically `Dog`:

- the call site computes `GetMethodIndex(Animal, "sound") == 0`;
- codegen loads the vtable pointer **from the object header** (which points at `Dog`'s vtable, set
  when the object was constructed);
- it reads slot 0 → `Dog.sound`.

That is dynamic dispatch, and it falls out of the layout for free.

### Populating a derived vtable

A class only emits function bodies for the methods it *declares*. A derived class's inherited
(non-overridden) slots therefore have no body to write and must be filled from the base. This
happens in two steps in `internal/codegen/codegen.go`:

1. **During body generation** (`genClassMethod`) each declared method is written into its own
   slot, resolved by name via `util.GetVTableSlot`. An override lands in the base's slot; a new
   method lands after the inherited ones.

   ```go
   slot := util.GetVTableSlot(&class, &method)   // name-resolved, not an incrementing counter
   if slot >= 0 {
       structInfo.LLVMVTableMethods[slot] = function
   }
   ```

2. **After all bodies exist** (`finalizeInheritedVTables`) each derived class copies its base's
   compiled function pointers into the inherited slots it did not override. Classes are visited
   **base-before-derived** (declaration order — a base is always declared before its derived
   classes), so a base vtable is complete before a derived class reads from it, and multi-level
   chains fill correctly grandparent → parent → child.

   ```go
   parentFlat := zeus_value.FlattenedVTableMethods(class.ParentClass)
   childFlat  := zeus_value.FlattenedVTableMethods(class)
   for i := range parentFlat {
       // Same *ClassMethod on both sides ⇒ not overridden ⇒ inherit the base's compiled fn.
       // A different *ClassMethod (override) keeps what genClassMethod already wrote.
       if i < len(childFlat) && childFlat[i] == parentFlat[i] {
           childInfo.LLVMVTableMethods[i] = parentInfo.LLVMVTableMethods[i]
       }
   }
   ```

`createClassStructTypes` sizes the vtable struct and `genClass` sizes the method array with
`len(FlattenedVTableMethods(class))`, so both agree with the slot indices above.

---

## Upcasting

Assigning or passing a derived value where a base type is expected is a no-op at runtime — the
pointer is already layout-compatible (fields are base-first, and LLVM 19 opaque pointers mean no
bitcast is needed). The type checker allows it in `tryImplicitCast`
(`internal/ir/tc_type_check.go`):

```go
case zeus_value.ObjectType:
    if zeus_value.IsSubclassOf(valueType.Class, targetType.Class) {
        return value, true   // retype only; dispatch still uses the object's own vtable
    }
```

The value keeps its original identity; only its static type changes. Method calls on the
base-typed reference dispatch through the object's vtable pointer, so overrides are still reached
(that is the whole point of the shared slot indices).

---

## Member resolution in the type checker

Instance-member resolution walks the inheritance chain via the flattened helpers:

| Site (`tc_type_check.go`) | Change |
|---------------------------|--------|
| `tcObjectPropertyAccess` | field/method lookup uses `FlattenedInstanceProperties` / `FlattenedMethods` (a derived member shadows a same-named base member) |
| `tcMethodCall` | method resolved with `LookupMethod`; the function-typed-property fallback uses `LookupInstanceProperty` |
| `tryImplicitCast` | upcast via `IsSubclassOf` (above) |

Codegen's `genMethodCall` resolves the call's signature with `LookupMethod` as well (the concrete
dispatch still goes through the vtable slot, which for an override holds the derived function).

---

## The `GetVTableSlot` name skew

Writing a compiled method into its vtable slot cannot simply use `GetMethodIndex(sourceName)`,
because the *name a method is known by* differs between its descriptor and its compiled function in
two cases:

| Method kind | Descriptor name (`ClassMethod`) | Compiled function |
|-------------|--------------------------------|-------------------|
| Regular method | IR name == compiled name | same |
| **Accessor** | mangled `#get_x` / `#set_x` | `SourceName()` is the property name `x` (its `OriginalName` is set) |
| **Extern/primordial** | unprefixed (`charAt`) | compiled under a class-scoped IR name (`string.charAt`) |

So `util.GetVTableSlot` matches by **IR name first, then source name**:

```go
func GetVTableSlot(class *zeus_value.Class, method *zeus_value.Function) int {
    flat := zeus_value.FlattenedVTableMethods(class)
    for i, m := range flat { if m.Method.Name == method.Name { return i } }          // regular, accessor
    for i, m := range flat { if m.Method.SourceName() == method.SourceName() { return i } } // extern
    return -1
}
```

`GetMethodIndex` (used by **call sites**, which know a source name) stays source-name based. A
naive slot-by-source-name write left accessor slots null → a runtime segfault; this two-key lookup
is what keeps accessors and primordials working under the new flattened vtable.

---

## Pipeline summary

```
extends (token TokenTypeExtends)
  → Parser: ClassDeclExprNode.ParentClass
    → IR gen (VisitClassDeclExpr): NewClassWithParent → Class.ParentClass
      → Type checker: flattened member resolution + upcast (IsSubclassOf)
        → Codegen:
             createClassStructTypes  — base-first fields, flattened vtable slots
             genClass                — vtable sized to FlattenedVTableMethods
             genClassMethod          — write own methods to name-resolved slots
             finalizeInheritedVTables— fill inherited slots from the base
```

### Files touched

| File | Role |
|------|------|
| `internal/zeus_value/value.go` | flattened-member helpers (`FlattenedInstanceProperties`, `FlattenedVTableMethods`, `FlattenedMethods`, `LookupMethod`, `IsSubclassOf`) |
| `internal/util/util.go` | `GetPropertyIndex` / `GetMethodIndex` via flattened views; new `GetVTableSlot` |
| `internal/codegen/codegen.go` | base-first struct + flattened vtable; `finalizeInheritedVTables`; slot-by-name writes; inheritance-aware `genMethodCall` |
| `internal/ir/tc_type_check.go` | flattened member resolution + upcast |

`extends` parsing (`parser.go`), the `TokenTypeExtends` token, `ClassDeclExprNode.ParentClass`
(`ast/expr.go`), and `Class.ParentClass` / `NewClassWithParent` (`zeus_value/value.go`) already
existed before this work — they were the unused scaffolding this feature wired up.

---

## `super` constructor chaining

A derived constructor invokes its base constructor with `super(...)`, so base initialization runs
before the derived constructor continues:

```zeus
class Animal {
    public legs: i32;
    constructor(legs: i32) { this.legs = legs; }
}
class Dog extends Animal {
    public tail: i32;
    constructor(legs: i32, tail: i32) {
        super(legs);      // runs Animal's constructor with `this`
        this.tail = tail;
    }
}
```

### Rules (JavaScript semantics)

| Rule | Where enforced |
|------|----------------|
| A derived constructor **must** call `super(...)` when the base chain has a constructor | IR gen (`buildClass`) |
| `super(...)` must run **before** any use of `this` (non-`this` statements may precede it) | IR gen (`VisitIdentifier`, source-order traversal) |
| `super(...)` arguments must match the base constructor's parameters | Type checker (`tcSuperConstructorCall`) |
| `super(...)` is only valid inside a constructor of a class with a base constructor | IR gen (`emitSuperConstructorCall`) |
| A derived class with **no** constructor forwards `new Derived(args)` to the base constructor | Codegen (`effectiveConstructor`) |

`super` is not a reserved token — like `this`, it is an identifier recognized at IR gen. The
"before `this`" rule is enforced by riding IR generation's source-order traversal: a
`currentConstructorClass`/`superCalledInCtor` flag pair errors if `this` resolves before the
`super(...)` call is emitted, so no separate flow-analysis pass is needed.

### How it lowers

`super(a)` becomes a `CALL_SUPER_CONSTRUCTOR` HIR instruction carrying the target base class, the
current `this`, and the arguments. Codegen (`genSuperConstructorCall`) emits a **direct** call to
the base constructor's LLVM function (via `getLLVMConstructorMethod`) with `(args…, this)` — the
constructor is not in the vtable, so this is a non-virtual call. The target is the *nearest*
ancestor that declares a constructor (`zeus_value.LookupConstructorClass`), matching JS's behavior
when an intermediate class omits its constructor.

The object factory (`genFactoryFunctionBody`) default-initializes all fields base-first
(`FlattenedInstanceProperties`) and calls the **effective constructor** — the class's own, or the
nearest inherited one so a derived class without a constructor forwards to the base.

### `super.method()` — non-virtual base calls

`super.method(...)` inside an instance method calls the **base** implementation directly, even if
the object overrides it — so an override can extend the base behavior:

```zeus
class Animal {
    public sound(): i32 { return 10; }
}
class Dog extends Animal {
    public sound(): i32 { return super.sound() + 32; } // 10 + 32
}
```

This is the counterpart to normal method dispatch, which is *virtual* (through the object's
vtable). A `super.method()` call carries a `StaticClass` on the `CALL_METHOD` instruction; codegen
(`genStaticMethodCall`) resolves the method up the base chain and calls it **by symbol**, never
through the vtable. Passing the object through a base-typed reference still dispatches virtually to
the override, and the override's `super.sound()` reaches the base without recursing — the two
mechanisms compose. `super` is bound lexically to the *enclosing method's* class, not the
receiver's dynamic type. Reading `super.property` (non-call) is not supported.

---

## Limitations

- **`super.property` read.** Only `super.method(...)` calls are supported, not `super.field` /
  `super.getter` reads (rejected with a targeted error).
- **Field shadowing.** A derived field with the same name as a base field is not diagnosed; the
  field lookup and the struct layout can disagree on which slot it means. Avoid shadowing names.
- **Override signature compatibility** (parameter/return variance) is not checked; an override may
  differ from the base signature without an error.
- **`super(...)` placement.** It must be a top-level statement of the constructor (not nested in an
  `if`/loop); a nested `super(...)` reads as "absent" and triggers the must-call-super error. Full
  all-paths flow analysis is out of scope.

---

## Testing

End-to-end specs live in `test/e2e/specs/inheritance/`: inherited + own fields, inherited methods,
overrides dispatched dynamically, multi-level chains, polymorphism through a base-typed parameter,
inherited accessors, a base method mutating an inherited field, `super(...)` chaining (single and
multi-level), a derived class forwarding to the base constructor, `super.method()` (extending the
base, non-virtual under dynamic dispatch, and resolving up the chain), and compile-error cases
(missing `super`, `this` before `super`, wrong `super` arg count, `super.method()` with no base).
HIR-level unit tests for `super` live in `test/ir/inheritance_test.go`.

> **Runtime note.** Validate with the e2e harness
> (`ZEUS_HOME=/path/to/zeus go test -tags llvm19 ./test/e2e/... -count=1`), not by directly running
> a compiled binary with `ZEUS_HOME` set — the harness links differently, and direct invocation can
> segfault on cases (e.g. `accessor/getter.zs`) that the harness runs correctly. This predates
> inheritance and is a runtime/linking artifact, not a compiler bug.
