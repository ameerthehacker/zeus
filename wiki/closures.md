# Closures

This document covers Zeus's closure implementation end-to-end: the capture-by-value system that is live today, the bugs that were fixed during implementation, and the design for the upcoming capture-by-reference (ref cell) rewrite.

---

## Background: functors and the closure problem

Zeus represents every nested or anonymous function as a **functor** — a class with a `__call__` method (see `wiki/functors.md`). This means the compiler already had the machinery to create heap-allocated callable objects. What it lacked was variable capture: if a nested function references a variable from its enclosing scope, that variable is only on the enclosing function's stack frame, which is gone by the time the closure is called.

```zeus
function makeAdder(n: i32): (x: i32) => i32 {
    function add(x: i32): i32 {
        return x + n;   // n lives in makeAdder's stack frame — dangling at call time
    }
    return add;
}
```

The solution is to lift free variables out of the stack and into the functor object itself.

---

## Part 1 — Capture-by-value (implemented)

### Design

At the point where a functor object is created, copy the current value of each captured variable into a named property on the functor. Inside `__call__`, load those properties back into local shadow variables before the function body runs.

```
makeAdder(3):
  n = 3  (SSA parameter, IsPtr=false)
  new addFunctor() → obj
  obj.__cap_n__ = 3        // copy at creation time

addFunctor.__call__(obj, 4):
  let n: i32 = obj.__cap_n__  // shadow var, value = 3
  return 4 + n                // returns 7
```

Mutations to the shadow variable do NOT propagate back to `obj.__cap_n__` or to the outer scope's variable. This is snapshot / move semantics, analogous to Rust's `move ||` closures or C++ `[=]` captures.

### Key structures

**`CapturedVar`** (`internal/ir/closure.go`):
```go
type CapturedVar struct {
    OriginalName string               // e.g. "n", "x", "this"
    PropertyName string               // e.g. "__cap_n__", "__cap_this__"
    ValueType    zeus_value.ValueType // type of the captured value
    Source       zeus_value.Value     // *Var (local/param) or *Object (for "this")
}
```

**`freeVarCollector`** (`internal/ir/closure.go`): an AST visitor that, given a function's params and body, walks the body while tracking locally-declared names. Any identifier that resolves to a non-global symbol in the enclosing scope's symbol table becomes a `CapturedVar`.

- Pushes a new local scope when walking into a nested function (to avoid treating inner params/locals as captures).
- Continues walking into nested function bodies (to propagate transitive capture requirements upward).
- Captures `*zeus_value.Var` sources for locals and parameters.
- Captures `*zeus_value.Object` sources for `this` (class method closures).
- Returns non-`this` captures first, `this` last (ordering constraint in `__call__` preamble).

### `emitFunctorClass` (`internal/ir/ir.go`)

Called from `VisitFunctionDeclExpr` and `VisitFunctionDeclStmt` for any nested/anonymous function.

1. Run `collectFreeVars(fnParams, fnBody)` while the enclosing symbol table is still live.
2. Build the functor class with one extra `public` property per captured var (`__cap_x__`).
3. Emit `DECL_CLASS`, constructor, and `__call__` (via `emitFunction`).
4. Emit `NEW_OBJ(functorClass)` to create the instance. Immediately set `resultVar.ValueType = ObjectType{functorClass}` so any outer `collectFreeVars` that captures this functor object sees a non-nil type.
5. For each captured var, store its current value into the corresponding property:
   - `srcVar.IsPtr == true` (alloca local): emit `LOAD srcVar` → store result
   - `srcVar.IsPtr == false` (SSA parameter): use the var directly (no load needed)
   - `*zeus_value.Object` (`this`): store the object reference directly

### `emitFunction` preamble (`internal/ir/ir.go`)

Inside `__call__`, before the function body is visited:

**Pass 1 — non-`this` captures** (uses the functor's `this`):
```
propPtr = OBJECT_PROPERTY_ACCESS this.__cap_x__
loadedVal = LOAD propPtr
localVar = DECL_VAR x: T
STORE localVar = loadedVal
symbolTable.declare("x", localVar)
```

**Pass 2 — `this` capture** (must come last, overrides the functor's `this`):
```
propPtr = OBJECT_PROPERTY_ACCESS this.__cap_this__
loadedOuterThis = LOAD propPtr
loadedOuterThis.IsPtr = false  // already a loaded pointer, not pointer-to-pointer
symbolTable.declare("this", loadedOuterThis)
```

The `this` capture must be last because pass 1 still needs to read from the functor's `this` to load the other properties. Once `this` is re-declared as the outer object, reads of `this` in the body correctly see the enclosing class instance.

### Eager type inference for untyped variables

Variables declared without an explicit type annotation (`let count = 0`) get `UndefinedType{}` at IR-gen time, with the real type resolved later by `TypeCheckingPass.tcDeclVar`. When such a variable is captured by a closure, its `UndefinedType{}` propagates into the functor class property, causing `UndefinedTypeCheckPass` to error after TC.

**Fix** (`VisitVarDeclStmt`, `internal/ir/ir.go`): eagerly infer the type from the initializer value using `zeus_value.GetValueType(initializer)`. If the initializer already carries a concrete type (common for literals, function-call results, etc.) use it immediately; otherwise fall back to `UndefinedType{}` for TC to resolve later.

```go
if decl.ValueType != nil {
    valueType = decl.ValueType.ValueType
} else if initializer != nil {
    inferred := zeus_value.GetValueType(initializer)
    if inferred != nil && !zeus_value.IsUndefinedType(inferred) {
        valueType = inferred   // e.g. IntType{I8} for literal 0
    } else {
        valueType = zeus_value.UndefinedType{Span: decl.GetSpan()}
    }
} else {
    valueType = zeus_value.UndefinedType{Span: decl.GetSpan()}
}
```

The nil guard (`inferred != nil`) is critical: `new Foo()` returns a temp var with nil ValueType before TC's `tcNewObj` runs, so `GetValueType` would return nil. Without the guard, nil would be used as the variable's type, breaking TC for `let x = new Foo()` patterns.

---

## Part 2 — Bugs fixed during implementation

### Bug 1 — `load i32, i32 %param` (SIGSEGV at runtime)

**Symptom**: Closures that capture function parameters crash at runtime.

**Root cause**: `emitFunctorClass` unconditionally called `BuildLoad(srcVar)` for every captured `*Var` source. For alloca locals (`IsPtr=true`) this is correct. For SSA parameters (`IsPtr=false`) this emitted `load i32, i32 %0` — attempting to interpret an integer value as a memory address → SIGSEGV.

**Fix**: Check `srcVar.IsPtr` before emitting the load:
```go
if srcVar.IsPtr {
    currentVal = g.irBuilder.BuildLoad(srcVar, span)  // alloca
} else {
    currentVal = srcVar                                // SSA param — already the value
}
```

### Bug 2a — nil property type panic in `resolveClass`

**Symptom**: Compiling a closure that captures another closure (functor object) panics in `ToKnownTypesPass.resolveClass` with a nil pointer dereference.

**Root cause**: `BuildNewObj` uses `createTempVariable` which creates a var with `ValueType = nil`. When `collectFreeVars` in an outer closure captured an inner functor variable, it captured `ValueType = nil`. This nil propagated into the outer functor's class property, and `resolveClass` called `nil.GetSpan()` → SIGSEGV.

**Fix 1**: After `BuildNewObj` in `emitFunctorClass`, immediately set the result var's type:
```go
if resultVar := zeus_value.AsVar(functorObject); resultVar != nil {
    resultVar.ValueType = zeus_value.NewObjectType(*functorClass)
}
```
This ensures any outer `collectFreeVars` sees a concrete `ObjectType` instead of nil.

**Fix 2** (safety net): `resolveClass` in `tc.go` skips properties with nil ValueType rather than panicking. These are resolved later via the first `STORE` into the property.

### Bug 2b — `getLLVMStructType` panic for cross-ordered class references

**Symptom**: Compiling a closure that captures `this` inside a class method panics with "llvm struct type Box not found".

**Root cause**: Functor classes are inserted at position 0 in the instruction list (via `EmitClassDeclAtStart`). A functor class whose properties reference a user-defined class (e.g., `Box` for a `this` capture) is processed in Phase 1 (DECL_CLASS processing) before `Box`'s own `DECL_CLASS`. `createClassStructTypes` called `getLLVMStructType("Box")` which hadn't been registered yet.

**Fix**: In `toLLVMType` for `ObjectType`, stop calling `getLLVMStructType`. In LLVM's opaque-pointer mode (LLVM 15+), all GC object pointers are `ptr addrspace(1)` regardless of the nominal element type. The element type passed to `PointerType` is silently discarded by LLVM — no information is lost and no optimization is affected.

```go
case zeus_value.ObjectType:
    // In LLVM opaque-pointer mode the element type is irrelevant.
    return llvm.PointerType(c.cxt.VoidType(), 1)
```

Note: `toLLVMStructType` (used for GEPs) is a separate function and still performs the lookup correctly, but by that point all classes have been registered in Phase 1.

---

## Part 3 — Capture-by-reference (upcoming)

### Why capture-by-value cannot support mutable closures

```zeus
function makeCounter(): () => i32 {
    let count: i32 = 0;
    return (): i32 => {
        count += 1;
        return count;
    };
}
let incr = makeCounter();
incr();  // returns 1, not 1, 2, 3, 4
```

With capture-by-value, `count`'s value (0) is copied into `__cap_count__` when the closure is created. Each `__call__` invocation:
1. Loads 0 from `this.__cap_count__`
2. Increments the local shadow → 1
3. Returns 1
4. The shadow is discarded; `this.__cap_count__` remains 0

The local shadow variable is never written back to `this.__cap_count__`. Each call starts fresh at 0.

The deeper reason this can't be fixed by "just write back on return": `count` is an alloca on `makeCounter`'s stack frame. That frame is gone by the time `incr()` is called. Any pointer to the alloca would be dangling. **The variable must be moved to the heap.**

This is what JavaScript, Go, Python, and most languages with closures do internally: variables that escape into closures are heap-allocated ("escaping" in compiler terminology). The closure holds a pointer to the heap cell, not a copy of the value.

### Design: ref cells

A **ref cell** is a small GC-managed object with a single `value` property. For each type `T`, a ref cell class `__ref_cell_T__` is generated on demand:

```
class __ref_cell_i32__ {
    public value: i32;
}
```

When a variable is declared in a scope where it will be captured, it is allocated as a ref cell instead of a stack alloca:

```
// Before (capture-by-value, stack alloca):
%count = alloca i32
store i32 0, ptr %count

// After (capture-by-reference, heap ref cell):
%count_cell = call zeus_gc_alloc(sizeof __ref_cell_i32__)
GEP(count_cell, value_field) ← store 0
```

All reads and writes to `count` in the outer scope go through the cell:
- Read `count` → `load GEP(count_cell, value_field)`
- Write `count = v` → `store v, GEP(count_cell, value_field)`

The closure captures the **cell pointer** (not the value). Both the outer function and the closure operate on the same heap cell:

```
makeCounter scope:        Closure (__call__):
  count_cell → [value: 0]   __cap_count__ → same cell
       ↑                           ↑
   both reference the same GC object
```

Inside `__call__`:
- `count += 1` → load from cell, add 1, store back to cell
- Returns the cell's value → 1 on first call, 2 on second, etc.

### RefCellVar: transparent access in the symbol table

To avoid changing every visitor that reads or writes a variable, a new value type `RefCellVar` is added to `zeus_value/`. It wraps the underlying cell object and acts as a transparent proxy:

```go
type RefCellVar struct {
    OriginalName string
    ValueType    zeus_value.ValueType  // type of .value, not of the cell
    Cell         zeus_value.Value      // the GC ref cell object
}
```

`RefCellVar` implements the `Value` interface. The symbol table maps `"count"` to a `RefCellVar` instead of a `*Var`.

**Reads** (`VisitIdentifier`): detects `RefCellVar`, emits `OBJECT_PROPERTY_ACCESS cell.value` + `LOAD` → returns the loaded scalar value. Identical to what the callers expect.

**Writes** (`VisitAssignExpr` and compound assignments): detects `RefCellVar` target, emits `OBJECT_PROPERTY_ACCESS cell.value` + `STORE`. The alloca-based `BuildStore(var, val)` path is bypassed.

### Escape analysis pre-pass

Ref cells must be created at variable *declaration* time (in `VisitVarDeclStmt`), because by the time `VisitFunctionDeclExpr` encounters the nested closure the variable is already in the symbol table. We need to know *before* generating the function body which variables will be captured.

A new pure-AST pre-pass `collectEscapedVarNames` runs in `emitFunction` before `fnBody.Accept(g)`. It walks the function body's AST (no IR emitted) and finds: variables or parameters declared in this scope that are referenced in any nested function body.

```go
// Returns names of vars/params in this scope that escape into nested closures.
func collectEscapedVarNames(params []*ast.VarDeclNode, body *ast.BlockStmtNode) map[string]bool
```

The walker:
1. Maintains a set of names declared at the current (top) level.
2. When it encounters a nested `FunctionDeclExprNode` or `FunctionDeclStmtNode`, collects all identifiers referenced in its body (recursively, including deeper nested closures).
3. Any name in the current-level set that is also referenced in a nested body is marked escaped.

### Implementation changes (component by component)

**`internal/zeus_value/`** — add `RefCellVar` type implementing `Value`.

**`internal/ir/closure.go`** — add `collectEscapedVarNames` AST pre-pass.

**`internal/ir/ir.go` — `emitFunction`**:
- Call `collectEscapedVarNames(params, fnBody)` before `fnBody.Accept(g)`.
- For escaped params: immediately after the param `*Var` is created, allocate a ref cell, store the param's SSA value into it, and re-register the symbol as `RefCellVar`.
- Pass the escaped-names set into the body generation context.

**`internal/ir/ir.go` — `VisitVarDeclStmt`**:
- If the declared name is in the escaped set: allocate `new __ref_cell_T__()`, store the initializer into `cell.value`, register `RefCellVar` in symbol table.
- Otherwise: existing alloca path unchanged.

**`internal/ir/ir.go` — `VisitIdentifier`**:
- Add a `case *zeus_value.RefCellVar` branch: emit `OBJECT_PROPERTY_ACCESS cell.value` + `LOAD`.

**`internal/ir/ir.go` — `VisitAssignExpr` (and `+=`, `-=`, etc.)**:
- Add a `case *zeus_value.RefCellVar` branch for the assignment target: emit `OBJECT_PROPERTY_ACCESS cell.value` + `STORE`.

**`internal/ir/closure.go` — `collectFreeVars`**:
- For `RefCellVar` symbols, capture the underlying cell object as the `Source` with `ValueType = ObjectType{__ref_cell_T__}`.

**`internal/ir/ir.go` — `emitFunctorClass` capture initialization**:
- For ref cell captures (source is a ref cell object): store the cell pointer directly into the property. No load needed — the cell itself is the shared reference.

**`internal/ir/ir.go` — `emitFunction` `__call__` preamble**:
- For ref cell captures: load `this.__cap_x__` → get the cell object → register `RefCellVar{cell: loaded_cell}` in the local symbol table under the original name. The body's reads/writes automatically route through the cell.

**`internal/ir/tc.go` — TypeCheckingPass**:
- Ref cell properties have a concrete `ObjectType{__ref_cell_T__}` type (no nil/UndefinedType issues).
- `tcObjectPropertyAccess` for `.value` on a ref cell class resolves to type `T` normally.

**`internal/codegen/codegen.go`**:
- No changes needed for ref cell access paths — `OBJECT_PROPERTY_ACCESS` + `LOAD`/`STORE` already work for any class. The ref cell classes are ordinary Zeus classes.
- Ref cell class generation follows the same pattern as array primordial classes.

### Ref cell class generation

Ref cell classes are generated on demand by a new pass (or extended `PrimordialClassGenPass`). For each unique value type `T` that needs boxing, a class `__ref_cell_T__` is declared:

```
DECL_CLASS __ref_cell_i32__
  properties:
    public value: i32
  methods:
    constructor() { /* no-op */ }
```

The class name is derived from the type string: `__ref_cell_` + `T.String()` + `__`. For nested generics (e.g., an `i32[]` captured), the class name contains the array type string.

Classes are emitted via `EmitClassDeclAtStart` so they precede any use, and cached to avoid duplicates.

### Semantic changes vs capture-by-value

Switching to capture-by-reference changes the observable semantics of several patterns:

**Pattern 1 — outer mutation after closure creation** (now visible):
```zeus
let x: i32 = 10;
function getX(): i32 { return x; }
x = 99;
return getX();  // capture-by-value: 10 | capture-by-reference: 99
```

**Pattern 2 — inner mutation** (now propagates to outer scope):
```zeus
let x: i32 = 5;
function set99() { x = 99; }
set99();
// x is now 99 in outer scope too
```

**Pattern 3 — shared variable across multiple closures** (all see the same cell):
```zeus
let i: i32 = 1;
let f1 = (): i32 => i;
i = 2;
let f2 = (): i32 => i;
i = 3;
// f1() == 3, f2() == 3 (all reference the same cell, see final value)
```

To get snapshot semantics explicitly, the programmer copies before capturing:
```zeus
let i: i32 = 1;
let snap1: i32 = i;   // snap1 has its own ref cell, value = 1
let f1 = (): i32 => snap1;
i = 2;
// f1() == 1 (snap1's cell was never mutated)
```

### Tests that change with the switch

These existing closure tests were written to document value semantics and need to be updated:

| Test file | Current expected exit | Why it changes |
|---|---|---|
| `value_semantics_outer_mutation.zs` | 10 | getX() will see x=99 (latest value) |
| `value_semantics_inner_mutation.zs` | 0 (x==5 in outer) | x is shared; outer x becomes 99 too |
| `closure_in_loop.zs` | 0 (f1=1, f2=2, f3=3) | all three closures share i; all return 3 |

New test to add: `mutable_counter.zs` — the canonical capture-by-reference test:
```zeus
function makeCounter(): () => i32 {
    let count: i32 = 0;
    return (): i32 => {
        count += 1;
        return count;
    };
}

function main(): i32 {
    let incr: () => i32 = makeCounter();
    incr();
    incr();
    incr();
    if (incr() == 4) {
        return 0;
    }
    return 1;
}
```

---

## Pass pipeline summary (after ref cell)

| Phase | What happens |
|---|---|
| **Escape analysis** (pre-IR, AST walk) | Identifies which vars/params in each function scope will be captured by nested closures |
| **IR gen — `VisitVarDeclStmt`** | Escaped vars → allocate ref cell (`NEW_OBJ __ref_cell_T__`), initialize `cell.value`, register `RefCellVar` in symbol table |
| **IR gen — `VisitIdentifier`** | `RefCellVar` → emit `PROP_ACCESS cell.value` + `LOAD` |
| **IR gen — `VisitAssignExpr`** | `RefCellVar` target → emit `PROP_ACCESS cell.value` + `STORE` |
| **IR gen — `collectFreeVars`** | Captures ref cell objects (not values); property type = `ObjectType{__ref_cell_T__}` |
| **IR gen — `emitFunctorClass`** | Stores cell pointer into `__cap_x__` property (no load) |
| **IR gen — `__call__` preamble** | Loads cell from `this.__cap_x__`, registers `RefCellVar` in local symbol table |
| **TypeCheckingPass** | `.value` property access resolves to type `T` normally; ref cell classes are ordinary classes |
| **Codegen** | `PROP_ACCESS` + `LOAD`/`STORE` on ref cell objects — no new codegen cases needed |

---

## Relevant source locations

| File | Relevance |
|---|---|
| `internal/ir/closure.go` | `CapturedVar`, `freeVarCollector`, `collectFreeVars`, `collectEscapedVarNames` (upcoming) |
| `internal/ir/ir.go` — `emitFunctorClass` | Functor class creation, capture property initialization, type-setting after `BuildNewObj` |
| `internal/ir/ir.go` — `emitFunction` | `__call__` preamble: shadow var setup (by-value) / ref cell setup (by-reference) |
| `internal/ir/ir.go` — `VisitVarDeclStmt` | Escaped var → ref cell allocation (upcoming); eager type inference from initializer |
| `internal/ir/tc.go` — `resolveClass` | Skips nil property types (safety guard for delayed type resolution) |
| `internal/codegen/codegen.go` — `toLLVMType(ObjectType)` | Returns `ptr addrspace(1)` without struct lookup (opaque-pointer safe) |
| `internal/zeus_value/` | `RefCellVar` type (upcoming) |
| `test/e2e/specs/closures/` | 21 closure tests (capture-by-value); will grow and some will change for by-reference |
