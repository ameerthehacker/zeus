# Zeus Type Inference Architecture

Type inference in Zeus is a multi-layer system. Each layer has a precise responsibility; no layer repairs what an earlier one should have done correctly.

---

## Pipeline overview

```
Source AST
    │
    ▼
DeclCheckPass          — pre-registers all class/function names as stubs
    │
    ▼
IRModule.Generate()    — walks AST, emits HIR
    ├─ resolveTypeForIRGen()    — inline: converts type annotations → ObjectType
    ├─ inferFunctionEnv()       — inline: pre-scans function bodies before IR emission
    └─ stub back-fill           — after each class is registered, resolves stub *Var types
    │
    ▼
TypeCheckingPass       — validates types, inserts implicit casts, fallback inference
UnusedWarningPass      — warns on unused variables
UndefinedTypeCheckPass — final check: no undefined types remain
    │
    ▼
Lowering → Codegen
```

---

## Layer 1: DeclCheckPass

**File:** `internal/ir/ir_passes.go`

Before any IR is emitted, `DeclCheckPass` walks the entire AST and pre-registers every top-level class and function in the symbol table as a *stub*. Stubs contain the right names and shape (properties, method signatures) but property/parameter types are stored as raw `UserDefinedType{Name: "Foo"}` values — not yet resolved to concrete `ObjectType` pointers.

**Purpose:** enable forward references. Without stubs, `class A { b: B }` would fail to resolve `B` if `B` is declared after `A`.

**Stubs share `*Var` pointers** — this is load-bearing. Each property is a `*ClassProperty` containing a `*Var`. When `resolveTypeForIRGen` copies the stub class (Go struct copy of `Class{Properties: []*ClassProperty{...}}`), the copy shares the same `*ClassProperty` and `*Var` pointers as the stub. Updating `var.ValueType` through the pointer mutates the stub, the copy, and every other copy simultaneously.

---

## Layer 2: `resolveTypeForIRGen` — inline type resolution

**File:** `internal/ir/ir.go`, function `resolveTypeForIRGen`

Called at the moment each explicit type annotation appears in the source:

- Variable declarations: `let x: Point = …`
- Function/method parameters and return types
- Class property types and method signatures
- `new Point[]` base element type
- `catch (e: Error)` clause types

It converts raw AST types to concrete IR types:

| Input type | Output | Notes |
|---|---|---|
| `UserDefinedType{"Foo"}` | `ObjectType{*FooClass}` | Looks up `Foo` in symbol table; finds stub or full class |
| `ArrayType{ElementType: T}` | `ObjectType{*PointArrayClass}` | Recursively resolves element type first, then calls `getOrCreateArrayClass` |
| `FunctionType{Params, Return}` | `FunctionType{resolved params, resolved return}` | Recurses into params and return type |
| Primitive types | unchanged | `i32`, `f64`, `bool`, etc. pass through |

`resolveTypeForIRGen` **does not defer or repair** — it resolves immediately or pushes an error. This is the primary mechanism replacing the old `ToKnownTypesPass` repair sweep.

---

## Layer 3: Stub back-fill after class registration

**File:** `internal/ir/ir.go`, in `VisitClassDeclExpr` after `DeclareSymbol`

The first resolution pass (layer 2) runs before the class being declared is in the symbol table. For self-referential properties like `Node.next: Node`, `resolveTypeForIRGen` finds the DeclCheckPass stub (not the full class), so the property type becomes `ObjectType{*stub_copy}`. The stub_copy carries unresolved `UserDefinedType{"Node"}` values inside its own properties.

After `VisitClassDeclExpr` registers the full class, a second pass iterates the **DeclCheckPass stub's `*Var` objects** and re-resolves any remaining `UserDefinedType` values:

```
VisitClassDeclExpr(Node):
  1. resolve Node.next type → ObjectType{*Node_stub_copy}  (stub still in symbol table)
  2. create Node_full, register in symbol table             (overwrites stub)
  3. iterate stub.Properties:
       stub.Properties[next].Property.ValueType = UserDefinedType{"Node"}
       resolveTypeForIRGen("Node") → now finds Node_full → ObjectType{*Node_full}
       mutate *Var: stub.Properties[next].Property.ValueType = ObjectType{*Node_full}
```

Because `ObjectType{stub_copy}` shares the stub's `*Var` pointers, this mutation propagates automatically. Any class holding a property of type `ObjectType{*stub_copy}` now sees `ObjectType{*Node_full}` when it reads the property type through the pointer.

This handles:
- **Self-referential classes** (`Node.next: Node`)
- **Forward references where another class was resolved against the stub** (`class A { b: B }` declared before `class B`)

---

## Layer 4: `inferFunctionEnv` — AST pre-scan for function bodies

**File:** `internal/ir/ir_type_infer.go`

Called by `emitFunction` (in `ir.go`) **before** emitting IR for the function body, but **after** parameters and captured-variable preamble are in the symbol table.

It scans the function body AST (without emitting any IR) and returns a `FunctionTypeEnv`:

```go
type FunctionTypeEnv struct {
    VarTypes map[string]zeus_value.ValueType  // all local vars, keyed by name
}
```

**What it infers:**

| Case | Mechanism |
|---|---|
| `let x: T = …` (explicit annotation) | Resolves `T` via `resolveUserDefinedType` using the symbol table |
| `let x = expr` (implicit from initializer) | Calls `inferExprType(expr)` — a read-only type inference over AST expressions |

**Why pre-scan, not post-IR?** Variable types must be set on `*Var` objects *before* IR is emitted for the variable declaration, because the IR builder's `DECLARE_VAR` instruction stores a reference to the `*Var`. Trying to patch it afterwards would require a second walk.

**`inferExprType` handles:**
- Literals (number, bool, char, string, null)
- Identifiers — looks up in symbol table + `localTypes` accumulated during the scan
- Unary/binary/ternary expressions — propagates types according to operator semantics
- Function calls — infers callee type, returns the callee's return type
- Method calls (`obj.method(...)`) — looks up the method in the inferred object class
- Indexing (`arr[i]`) — returns `arr.ArrayElementType`
- Property access (`obj.prop`) — returns the property type from the class definition
- `new Foo(...)` — returns `ObjectType{*FooClass}`
- `new Foo[]` / `new u8[n]` — builds the array type, looks up the registered array class by name

When `inferExprType` returns `nil` (e.g. calling a function with no explicit return type annotation), the var type remains `UndefinedType` and `TypeCheckingPass.tcDeclVar` fills it in as a fallback.

---

## Layer 5: `TypeCheckingPass` — validation + implicit casts + fallback inference

**File:** `internal/ir/tc_type_check.go`

The primary responsibility is **validation** — reporting type errors. It also inserts implicit cast instructions where allowed (int widening, int→float).

It acts as a **fallback** for the small set of cases `inferFunctionEnv` can't resolve at AST-scan time:

- **`tcDeclVar`**: if a var's type is still `UndefinedType` when the instruction is reached (e.g. the initializer calls a function with no explicit return type), infers from the initializer value.
- **`tcLoad`**: unconditionally sets `output.ValueType = addr.ValueType` — propagates the addr type to the load result.
- **`tcStore`**: if the target var has no declared type (e.g. a ternary result var declared with `nil` type), infers from the stored value.

It handles one additional resolution case as a **safety net**: in `tcObjectPropertyAccess`, if a property's type is still `UserDefinedType` when validation runs (e.g., a pathological forward-reference chain where the stub back-fill at layer 3 didn't reach), it resolves it from the symbol table and mutates the `*Var` in place. This is defensive — under normal circumstances it never triggers.

---

## `ObjectType` stores `Class` by pointer

`ObjectType` embeds `Class` as a pointer:

```go
type ObjectType struct {
    Class *Class
}
```

This means all `ObjectType` values that refer to the same class share the same `*Class`. Mutations to the class (adding methods, resolving property types) are immediately visible through all `ObjectType` references.

**Why name-based comparison:** `CmpValueType` for `ObjectType` compares only by **class name**:

```go
case ObjectType:
    bClassType, ok := b.(ObjectType)
    if !ok { return false }
    if a.Class.Name == bClassType.Class.Name { return true }
```

This means `ObjectType{*Node_full}` and `ObjectType{*Node_stub}` compare equal as long as both have `Class.Name == "Node"`, which is correct: they represent the same type at different stages of resolution.

**Why pointer sharing in `Class.Properties` matters:** `Class.Properties` is `[]*ClassProperty`. Copying a `Class` struct (e.g., when `resolveTypeForIRGen` returns `ObjectType{*stub}`) copies the slice header — the backing array and its `*ClassProperty` pointers are shared. `ClassProperty.Property` is `*Var`. Mutating `prop.Property.ValueType` propagates through all copies that share the same pointer. The stub back-fill (layer 3) and the safety-net resolution (layer 6) both rely on this.

---

## PrimordialRegistry — array classes and base classes

**File:** `internal/zeus_value/primordial_registry.go`

The global `Registry` singleton stores:
- **Fixed primordial classes:** `string`, `Error`, `Console`
- **Parameterized array classes:** `u8[]`, `i32[]`, `Point[]`, etc. — created on demand

When a new array class is created (e.g., `Point[]`), `resolveArrayMethodTypes` is called immediately to resolve any raw `ArrayType` references in method signatures (push param type, pop/get return type, etc.) to `ObjectType{*registeredClass}`.

After all base classes are registered, `resolveStringRefs` is called once to replace all `UserDefinedType{"string"}` placeholders in primordial method signatures with `ObjectType{*stringClass}`. Primordial definitions use `UserDefinedType` as a placeholder to avoid forward-reference issues during package init.

**Important:** the registry's resolvers can only see types registered in the registry itself (primordials). They have no knowledge of user-defined classes. For user-defined array element types (e.g., `new Point[]`), `VisitNewExpr` first calls `resolveTypeForIRGen` on the base element type to convert `UserDefinedType{"Point"}` → `ObjectType{*PointClass}`, then passes the resolved `ArrayType` to `getOrCreateArrayClass`. This ensures the `Point[]` class is created with concrete method param/return types from the start.

---

## How self-referential classes work end-to-end

Example: `class Node { public next: Node; }`

```
DeclCheckPass:
  registers Node_stub = Class{Name: "Node", Properties: [*CP{Property: *var_next, ValueType: UserDefinedType{"Node"}}]}
             ↑ var_next is a shared *Var

VisitClassDeclExpr(Node):
  resolveTypeForIRGen(UserDefinedType{"Node"})
    → looks up "Node" → finds Node_stub
    → returns ObjectType{*Node_stub}
      (Node_stub.Properties[0] = *CP{Property: var_next} — same pointer)
  
  creates var_next_full = *Var{Name: "next", ValueType: ObjectType{*Node_stub}}
  creates Node_full = Class{..., Properties: [*CP{Property: var_next_full}]}
  
  registers Node_full in symbol table (overwrites Node_stub)
  
  stub back-fill:
    stub.Properties[0].Property = var_next  (ValueType = UserDefinedType{"Node"})
    resolveTypeForIRGen("Node") → now finds Node_full → ObjectType{*Node_full}
    var_next.ValueType = ObjectType{*Node_full}   ← mutates *Var in place

Result:
  var_next.ValueType = ObjectType{*Node_full}
  ObjectType{*Node_stub}.Class.Properties[0].Property.ValueType = ObjectType{*Node_full}
  (because Node_stub.Properties[0] points to the same *CP → same var_next)

Type checking: current.next.next
  current.ValueType = ObjectType{*Node_full}
  .next → Node_full.Properties[next].Property.ValueType = ObjectType{*Node_stub}
         instr.output.ValueType = ObjectType{*Node_stub}
  .next → Node_stub.Properties[next].Property.ValueType = var_next.ValueType
                                                        = ObjectType{*Node_full}  ✓
  store check: CmpValueType(ObjectType{Node_full}, ObjectType{Node_full})
             → "Node" == "Node" → true ✓
```

---

## Pass execution order and file map

| Order | Pass | File | When |
|---|---|---|---|
| 0 | `DeclCheckPass` | `ir_passes.go` | Before IR gen |
| — | IR generation | `ir.go` | — |
| — | `resolveTypeForIRGen` | `ir.go` | Inline during IR gen |
| — | `inferFunctionEnv` | `ir_type_infer.go` | Inline, pre-function-body |
| — | Stub back-fill | `ir.go` | After each class registered |
| 1 | `TypeCheckingPass` | `tc_type_check.go` | Post-IR |
| 2 | `UnusedWarningPass` | `tc_unused.go` | Post-IR |
| 3 | `UndefinedTypeCheckPass` | `tc_undefined.go` | Post-IR |

---

## What was removed (design history)

**`ToKnownTypesPass`** — a full-IR sweep that converted all `UserDefinedType` references to `ObjectType` after IR generation. Problem: it was a repair pass, patching types that the IR generator should have set correctly. Replaced by `resolveTypeForIRGen` (inline at declaration time) + stub back-fill.

**`PrimordialClassGenPass`** — emitted DECLARE_CLASS instructions for primordial classes after IR generation. Replaced by `getOrCreateArrayClass` (emits class declarations on demand during IR gen) and `PrimordialRegistry.GetAllClasses` (used at IR gen entry to emit fixed primordials).

**`TypeInferencePass`** — a post-IR fixpoint pass that re-examined emitted instructions to patch types. It was the same anti-pattern as `ToKnownTypesPass`: repairing after the fact instead of setting types correctly upfront. Removed entirely because every rule it contained was already handled elsewhere: `tcNewObj` and `tcObjectPropertyAccess` in `TypeCheckingPass` cover NEW_OBJ and OPA; `inferFunctionEnv` covers all local vars before IR emission; functor class properties are typed explicitly during `emitFunctorClass`.

These removals mean the type checker now only **validates** and **propagates** — it does not repair.
