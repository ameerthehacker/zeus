# Functors and Function Types

Zeus represents first-class functions as **functors** — classes with a public `__call__` method. This document describes the design, the runtime representation, and how the compiler lowers function-type values end-to-end.

---

## Overview

A **functor** is any class that has a public `__call__` method. When you write a nested function or an anonymous function, the compiler generates a functor class for it automatically.

```zeus
fn outer() {
    fn inner(x: i32) => i32 {
        return x + 1;
    }
    // `inner` is a functor of type __inner_functor__ with a public __call__(x: i32) => i32
}
```

A **function type** (`(i32) => i32`) is the Zeus type that describes a callable with a particular signature. It is used for:
- Variable annotations: `let f: (i32) => i32 = ...`
- Parameter types: `fn apply(f: (i32) => i32, x: i32) => i32 { ... }`
- Return types: `fn makeAdder() => (i32) => i32 { ... }`

---

## The Universal Functor Protocol

**All `FunctionType` values at runtime are functor objects.**

This is the central design invariant. At the LLVM level, a `FunctionType` variable holds a `ptr addrspace(1)` — a pointer to a Zeus heap object — not a raw function pointer. The object follows the standard Zeus object layout and always has a `__call__` method at vtable slot 0.

```
toLLVMType(FunctionType) = ptr addrspace(1)   // heap object pointer
toLLVMType(ObjectType)   = ptr addrspace(1)   // same — they are the same thing at runtime
```

This uniformity means:
- Calling a `FunctionType` variable is always vtable dispatch at slot 0 — no special cases.
- Passing a functor where a `FunctionType` is expected requires no conversion (both are `ptr addrspace(1)`).
- Passing a global function where a `FunctionType` is expected requires wrapping it in a thin functor class (see below).

---

## Calling through a FunctionType variable

When the compiler sees `f(args...)` where `f` has type `(T1, T2) => R`, it emits `INDIRECT_FUNC_CALL`. Codegen lowers this via the `loadVTableMethodPtr` helper:

```
functorObj = load f                           // ptr addrspace(1)
headerPtr  = GEP(functorObj, field 0)         // field 0 of every Zeus object is the header ptr
             load headerPtr
vtablePtr  = GEP(header, field 0)             // field 0 of every header is the vtable ptr
             load vtablePtr
methodPtr  = GEP(vtable, slot 0)              // __call__ is always at slot 0
             load methodPtr
call methodPtr(args..., functorObj)           // self appended as last argument
```

The class is not known at this point — the dispatch is fully generic. The `loadVTableMethodPtr` helper in codegen handles both known-class (typed GEPs) and generic (opaque-pointer GEPs) paths:

```go
// objType != nil → typed GEPs using the specific class struct/vtable types
// objType == nil → opaque-pointer GEPs for unknown-class (FunctionType) dispatch
func (c *CodegenModule) loadVTableMethodPtr(obj llvm.Value, objType *zeus_value.ObjectType, slotIndex int, name string) llvm.Value
```

This same helper is also used by `genMethodCall` and `genObjectPropertyAccess` for known-class method dispatch, keeping all vtable navigation in one place.

---

## Using a functor as a FunctionType

If `f` is a functor (ObjectType with a `__call__`), it can be used wherever a matching `FunctionType` is expected. This is a zero-cost operation — both are `ptr addrspace(1)`. The type checker emits a `COERCE` instruction which is an identity at runtime:

```
COERCE f (ObjectType → FunctionType)   // type annotation only, no code emitted
```

`genCast` in codegen handles the `ObjectType → FunctionType` case as an identity:

```go
if zeus_value.IsObjectType(valueType) {
    if _, ok := input.CastType.(zeus_value.FunctionType); ok {
        c.llvmValues[output.Name] = c.toLLVMValue(input.Value)
        return
    }
}
```

---

## Using a global function as a FunctionType

A global (non-nested) function is a compile-time `*Function` value. It is not a heap object — it is a raw function pointer. It cannot be stored directly into a `FunctionType` variable (which expects `ptr addrspace(1)`).

The compiler handles two cases differently depending on whether a type annotation is present.

### No annotation — alias, no functor

```zeus
let f = globalFn          // no type annotation
```

`f` becomes a compile-time alias for `globalFn`. No `DECL_VAR` is emitted, no functor is created. Any call `f(args...)` lowers to a direct `CALL_FUNC(globalFn, args...)`. The alias is resolved in the symbol table during IR generation (`VisitVarDeclStmt` in `ir.go`).

### FunctionType annotation — wrapper functor

```zeus
let f: (i32) => i32 = globalFn     // explicit FunctionType annotation
fn apply(cb: (i32) => i32, x: i32) => i32 { return cb(x); }
apply(globalFn, 42)                 // implicit coercion at call site
```

When a `*Function` must become a `FunctionType` value, the type checker (`tryImplicitCast` in `tc.go`) emits a `CAST` instruction:

```
CAST globalFn → (i32) => i32
```

`CastLoweringPass` intercepts every `CAST` where the source is a `*Function` and the target is a `FunctionType`. In its `Finalize` phase it:

1. **Creates a wrapper functor class** (once per unique function, cached by name):
   - Class name: `__fnwrap_<funcname>__`
   - No-op constructor
   - `__call__` method that forwards all arguments directly to the original function

2. **Replaces the `CAST` in-place** with `NEW_OBJ(__fnwrap_<funcname>__)`:

```
CAST globalFn → (i32) => i32
    ↓  (CastLoweringPass)
NEW_OBJ __fnwrap_globalFn__()
```

The resulting object is a valid `ptr addrspace(1)` functor with `__call__` at vtable slot 0.

---

## Wrapper class structure

For a global function `add(a: i32, b: i32) => i32`, `CastLoweringPass` generates:

```
DECLARE_CLASS __fnwrap_add__
  methods:
    constructor (no-op)
    __call__(p0: i32, p1: i32) => i32:
      result = CALL_FUNC add(p0, p1)
      RETURN result
```

The class is emitted at the top of the instruction list (via `EmitClassDeclAtStart`) so it is declared before any use.

### Critical placement constraint

The `DECLARE_CLASS_METHOD` instructions for the wrapper class **must be top-level instructions** in `builder.instrs`, not inside any basic block. `Walk` in the IR builder only looks for nested function/method bodies when it encounters `DECLARE_CLASS_METHOD` at the top level — if it lands inside a block, the body is never walked during codegen and the LLVM function remains declared but undefined (linker error).

This means in `createFuncWrapperClass`, the builder state must be reset to top-level (`currentBlock = nil`) before calling `BuildFuncDecl` for each method, even if we just finished emitting instructions into a basic block:

```go
// After emitting constructor body — currentBlock is now constructorBlock.
// Reset to top-level before emitting the __call__ decl, otherwise
// BuildFuncDecl pushes DECLARE_CLASS_METHOD into constructorBlock.Instrs
// instead of builder.instrs, and Walk never sees its body.
builder.currentBlock = nil
builder.insertionIndex = len(builder.instrs)
callBlock := builder.BuildBasicBlock()
callFn := builder.BuildFuncDecl(className+"___call__", ...)
```

---

## Pass pipeline summary

| Pass | What happens to function-type values |
|---|---|
| **IR generation** (`ir.go`) | `let f = globalFn` with no annotation → symbol table alias, no instruction. `let f: FnType = globalFn` → emits `CAST`. |
| **TypeCheckingPass** (`tc.go`) | `tryImplicitCast` catches `*Function → FunctionType` before equality check, emits `CAST`. `tcCast` validates and annotates the output type. Functor-to-FunctionType uses `COERCE` (zero cost). |
| **CastLoweringPass** (`lowering.go`) | Collects `CAST(*Function → FunctionType)` instructions. In `Finalize`: creates wrapper class, replaces each `CAST` in-place with `NEW_OBJ`. |
| **FunctorCallLoweringPass** (`lowering.go`) | Converts `CALL_METHOD(obj, "__call__", args)` into `INDIRECT_FUNC_CALL`. Only fires on ObjectType values, not FunctionType (already handled generically). |
| **Codegen** (`codegen.go`) | `INDIRECT_FUNC_CALL` on a `FunctionType` value → generic vtable dispatch via `loadVTableMethodPtr(obj, nil, 0, "__call__")`. `NEW_OBJ(__fnwrap_...)` → allocates and initializes wrapper functor. |

---

## Type system rules

| Source type | Target type | Mechanism | Cost |
|---|---|---|---|
| `ObjectType` with `__call__` | matching `FunctionType` | `COERCE` | zero — identity at runtime |
| `*Function` (global) | `FunctionType` | `CAST` → lowered to `NEW_OBJ` | allocates wrapper functor once |
| `*Function` (global) | no annotation (`let f = g`) | symbol alias | zero — no instruction emitted |
| `FunctionType` var | call site | `INDIRECT_FUNC_CALL` → vtable slot 0 | one indirect call |

---

## Relevant source locations

| File | What it handles |
|---|---|
| `internal/ir/ir.go` — `VisitVarDeclStmt` | `*Function` alias vs CAST decision |
| `internal/ir/tc.go` — `tryImplicitCast`, `tcCast` | CAST insertion and validation |
| `internal/ir/lowering.go` — `CastLoweringPass` | Wrapper class generation, CAST → NEW_OBJ |
| `internal/codegen/codegen.go` — `genIndirectFuncCall` | Generic vtable dispatch for FunctionType calls |
| `internal/codegen/codegen.go` — `loadVTableMethodPtr` | Shared vtable navigation helper (used by method calls, property access, and indirect calls) |
| `internal/codegen/codegen.go` — `toLLVMType(FunctionType)` | Returns `ptr addrspace(1)` |
