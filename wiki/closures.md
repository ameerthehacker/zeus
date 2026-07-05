# Closures

This document covers Zeus's closure implementation end-to-end: the functor model, capture-by-value, capture-by-reference (ref cells), all bugs fixed during implementation, and the AST-level type pre-inference pass that makes ref cell promotion correct for complex initializers.

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
    IsRefCell    bool                 // true when Source is a ref cell object
}
```

**`freeVarCollector`** (`internal/ir/closure.go`): an AST visitor that, given a function's params and body, walks the body while tracking locally-declared names. Any identifier that resolves to a non-global symbol in the enclosing scope's symbol table becomes a `CapturedVar`.

- Pushes a new local scope when walking into a nested function (to avoid treating inner params/locals as captures).
- Continues walking into nested function bodies (to propagate transitive capture requirements upward).
- Captures `*zeus_value.Var` sources for locals and parameters.
- Captures `*zeus_value.Object` sources for `this` (class method closures).
- Captures `*zeus_value.RefCellVar` cell objects for escaped mutable vars (sets `IsRefCell=true`).
- Returns non-`this` captures first, `this` last (ordering constraint in `__call__` preamble).

### `emitFunctorClass` (`internal/ir/ir.go`)

Called from `VisitFunctionDeclExpr` and `VisitFunctionDeclStmt` for any nested/anonymous function.

1. Run `collectFreeVars(fnParams, fnBody)` while the enclosing symbol table is still live.
2. Build the functor class with one extra `public` property per captured var (`__cap_x__`).
3. Emit `DECL_CLASS`, constructor, and `__call__` (via `emitFunction`).
4. Emit `NEW_OBJ(functorClass)` to create the instance. Immediately set `resultVar.ValueType = ObjectType{functorClass}` so any outer `collectFreeVars` that captures this functor object sees a non-nil type.
5. For each captured var, store its current value into the corresponding property:
   - Ref cell (`IsRefCell=true`): store the cell object pointer directly — no load needed, the closure shares the heap cell.
   - `srcVar.IsPtr == true` (alloca local): emit `LOAD srcVar` → store result
   - `srcVar.IsPtr == false` (SSA parameter): use the var directly (no load needed)
   - `*zeus_value.Object` (`this`): store the object reference directly

### `emitFunction` preamble (`internal/ir/ir.go`)

Inside `__call__`, before the function body is visited:

**Pass 1 — non-`this` captures** (uses the functor's `this`):

For ref cell captures:
```
cellPtr = OBJECT_PROPERTY_ACCESS this.__cap_x__  // gets the cell object
loadedCell = LOAD cellPtr
symbolTable.declare("x", RefCellVar{cell: loadedCell, ValueType: T})
// reads/writes to "x" in the body automatically route through cell.value
```

For value captures:
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

The `this` capture must be last because pass 1 still needs to read from the functor's `this` to load the other properties.

---

## Part 2 — Capture-by-reference (implemented)

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
incr();  // always returns 1, not 1, 2, 3, 4
```

With capture-by-value, `count`'s value (0) is copied into `__cap_count__` at closure creation time. Each `__call__` invocation:
1. Loads 0 from `this.__cap_count__`
2. Increments the local shadow → 1
3. Returns 1
4. The shadow is discarded; `this.__cap_count__` remains 0

The deeper reason this can't be fixed by "write back on return": `count` is on `makeCounter`'s stack frame which is gone by the time `incr()` is called. The variable must be moved to the heap.

### Design: ref cells

A **ref cell** is a small GC-managed object with a single `value` property. For each type `T`, a ref cell class `__ref_cell_T__` is generated on demand:

```
class __ref_cell_i32__ {
    public value: i32;
}
```

When a variable is declared in a scope where it will be captured by a nested closure, it is allocated as a ref cell instead of a stack alloca:

```
// Before (stack alloca):
%count = alloca i32
store i32 0, ptr %count

// After (heap ref cell):
%count_cell = call zeus_gc_alloc(sizeof __ref_cell_i32__)
GEP(count_cell, value_field) ← store 0
```

The closure captures the **cell pointer** (not the value). Both the outer function and the closure operate on the same heap cell:

```
makeCounter scope:         Closure (__call__):
  count_cell → [value: 0]   __cap_count__ → same cell
       ↑                           ↑
   both reference the same GC object
```

### `RefCellVar`: transparent access in the symbol table

```go
type RefCellVar struct {
    OriginalName string
    ValueType    zeus_value.ValueType  // type of .value, not of the cell
    Cell         zeus_value.Value      // the GC ref cell object
    Span         *token.Span
}
```

`RefCellVar` implements the `Value` interface. The symbol table maps `"count"` to a `RefCellVar` instead of a `*Var`.

**Reads** (`VisitIdentifier`): detects `RefCellVar`, emits `OBJECT_PROPERTY_ACCESS cell.value` + `LOAD` → returns the loaded scalar. Identical to what callers expect.

**Writes** (`VisitAssignExpr` and compound assignments): detects `RefCellVar` target, emits `OBJECT_PROPERTY_ACCESS cell.value` + `STORE`. The alloca-based path is bypassed.

### Escape analysis pre-pass

Ref cells must be created at variable *declaration* time (in `VisitVarDeclStmt`), because by the time `VisitFunctionDeclExpr` encounters the nested closure the variable is already in the symbol table. A pure-AST pre-pass `collectEscapedVarNames` runs in `emitFunction` before `fnBody.Accept(g)`:

```go
// Returns names of vars/params in this scope that escape into nested closures.
func collectEscapedVarNames(params []*ast.VarDeclNode, body *ast.BlockStmtNode) map[string]bool
```

The walker:
1. Maintains the set of names declared at the current (top) level.
2. When it encounters a nested `FunctionDeclExprNode`, collects all identifiers referenced in its body (recursively).
3. Any name in the current-level set that is also referenced in a nested body is marked escaped.

### Ref cell type resolution: three-priority cascade

At `VisitVarDeclStmt` time, the type of an escaped variable's initializer may not yet be known (e.g., `let count = a + b` — type is resolved later by `TypeCheckingPass`). Three priority levels determine the ref cell's inner type `T`:

**Priority 1 — explicit annotation**: `let count: i32 = ...` → `T = i32` directly.

**Priority 2 — AST pre-inferred type**: `inferFunctionLocalTypes` runs before the body is emitted and pre-infers types from AST structure alone (no IR emitted). Handles: binary ops, ternary, direct/indirect calls, method calls, property access, array indexing, string literals, `new ClassName()`.

**Priority 3 — eager IR result type**: after `decl.Initializer.Accept(g)` runs, `GetValueType(initializer)` captures the type the IR gen set on the result (used for functor expressions, which set `resultVar.ValueType = ObjectType{functorClass}` eagerly).

```go
if g.escapedVarNames[varName] {
    refCellType := valueType  // Priority 1: explicit annotation
    if zeus_value.IsUndefinedType(refCellType) {
        if preInferred, ok := g.escapedVarInferredTypes[varName]; ok && ... {
            refCellType = preInferred  // Priority 2: AST pre-inferred
        }
    }
    if zeus_value.IsUndefinedType(refCellType) && initializer != nil {
        if inferred := zeus_value.GetValueType(initializer); ... {
            refCellType = inferred  // Priority 3: eager IR result
        }
    }
    if refCellType != nil && !zeus_value.IsUndefinedType(refCellType) {
        cellObj := g.allocRefCell(refCellType, initializer, decl.GetSpan())
        ...
    }
}
```

### AST pre-inference pass (`internal/ir/ir_type_infer.go`)

`inferFunctionLocalTypes` scans the function body AST before any IR is emitted. It must run after params and the captured-var preamble are registered in the symbol table (so identifier lookups succeed).

```go
func (g *IRModule) inferFunctionLocalTypes(
    escapedVarNames map[string]bool,
    body            *ast.BlockStmtNode,
) map[string]zeus_value.ValueType
```

Internal `inferExprType` handles:

| Expression | Inferred type |
|---|---|
| `NumberExprNode` (float) | `FloatType{F64}` |
| `NumberExprNode` (int) | `IntType{Signed: false, Size: GetSignedIntSize(v)}` |
| `BooleanExprNode` | `BoolType{}` |
| `NullExprNode` | `NullType{}` |
| `CharExprNode` | `IntType{Signed: false, Size: I8}` |
| `StringConstantExprNode` | `ObjectType{stringClass}` (via `ZEUS_PRIMORDIAL_STRING` lookup) |
| `GroupingExprNode` | recurse into inner |
| `IdentifierExprNode` | check `localTypes[name]` first; then symbol table `*Var`, `*RefCellVar`, `*Function`, `*Class`, `*Constant`; apply `resolveUserDefinedType` |
| `UnaryExprNode` with `!` | `BoolType{}` |
| `UnaryExprNode` arithmetic | recurse into operand |
| `BinaryExprNode` comparison/logical | `BoolType{}` |
| `BinaryExprNode` assignment | `nil` |
| `BinaryExprNode` arithmetic/bitwise | `GetBiggerType(left, right)` if both numeric |
| `TernaryExprNode` | try then-branch; fall back to else-branch |
| `FunctionCallExprNode` method call | resolve object type → find method by name → return type |
| `FunctionCallExprNode` direct/indirect | callee type → `FunctionType.ReturnType` or functor `__call__.ReturnType` |
| `IndexingExprNode` | resolve array object → `ArrayElementType` |
| `ObjectPropertyAccessExprNode` | resolve object → find property → property type |
| `NewExprNode` | look up class by identifier → `ObjectType{class}` |
| `FunctionDeclExprNode` | `nil` (Priority 3 handles this correctly) |

`localTypes` accumulates ALL variable types seen during the scan (not just escaped ones), enabling chained inference: `let a = 1; let counter = a + 1`.

The scan recurses into `if`/`while`/`for`/`try` bodies but never into nested `FunctionDeclExprNode` bodies — escaped vars are only those declared in the CURRENT function's scope.

### Escaped parameters

Parameters of the outer function that are also escaped into a closure are promoted to ref cells immediately after their `*Var` is created in `emitFunction`:

```go
cellObj := g.allocRefCell(paramType, paramVar, span)
refCell := &zeus_value.RefCellVar{...}
g.symbolTable().DeclareSymbol(paramName, refCell)
```

### Semantic changes vs capture-by-value

Capture-by-reference changes observable semantics:

**Outer mutation after closure creation** (now visible inside closure):
```zeus
let x: i32 = 10;
function getX(): i32 { return x; }
x = 99;
return getX();  // 99 (was 10 with capture-by-value)
```

**Inner mutation propagates to outer scope**:
```zeus
let x: i32 = 5;
function set99() { x = 99; }
set99();
// x is 99 in outer scope
```

**Multiple closures share one ref cell**:
```zeus
let i: i32 = 1;
let f1 = (): i32 => i;
i = 2;
let f2 = (): i32 => i;
i = 3;
// f1() == 3, f2() == 3 (all see the same cell's final value)
```

---

## Part 3 — Bugs fixed during implementation

### Bug 1 — `load i32, i32 %param` (SIGSEGV at runtime)

**Symptom**: Closures that capture function parameters crash at runtime.

**Root cause**: `emitFunctorClass` unconditionally called `BuildLoad(srcVar)` for every captured `*Var` source. For alloca locals (`IsPtr=true`) this is correct. For SSA parameters (`IsPtr=false`) this emitted `load i32, i32 %0` — interpreting an integer as a memory address → SIGSEGV.

**Fix**: Check `srcVar.IsPtr` before emitting the load.

### Bug 2a — nil property type panic in `resolveClass`

**Symptom**: Compiling a closure that captures another closure (functor object) panics in `ToKnownTypesPass.resolveClass`.

**Root cause**: `BuildNewObj` uses `createTempVariable` which creates a var with `ValueType = nil`. When an outer closure captured an inner functor variable, it captured `ValueType = nil`. `resolveClass` called `nil.GetSpan()` → SIGSEGV.

**Fix 1**: After `BuildNewObj` in `emitFunctorClass`, immediately set the result var's type:
```go
if resultVar := zeus_value.AsVar(functorObject); resultVar != nil {
    resultVar.ValueType = zeus_value.NewObjectType(*functorClass)
}
```

**Fix 2** (safety net): `resolveClass` skips properties with nil `ValueType` rather than panicking.

### Bug 2b — `getLLVMStructType` panic for cross-ordered class references

**Symptom**: Compiling a closure that captures `this` inside a class method panics with "llvm struct type Box not found".

**Root cause**: Functor classes are inserted at position 0 via `EmitClassDeclAtStart`. A functor class whose properties reference a user-defined class (e.g. `Box` for a `this` capture) is processed before `Box`'s own `DECL_CLASS`. `createClassStructTypes` called `getLLVMStructType("Box")` which hadn't been registered yet.

**Fix**: In `toLLVMType` for `ObjectType`, return `ptr addrspace(1)` directly (LLVM opaque-pointer mode — element type is irrelevant). No struct lookup needed at this point.

### Bug 3 — `FunctionType` inner types not resolved by `ToKnownTypesPass`

**Symptom**: A function with return type `(): string` (or any other function type annotation containing a user-defined type) fails with `type '() => string' is not assignable to type '() => string'`.

**Root cause**: `ToKnownTypesPass.resolveValueType` handled `UserDefinedType`, `ArrayType`, `NullType`, `VoidType` but NOT `FunctionType`. So `FunctionType{ReturnType: UserDefinedType{"string"}}` passed through unresolved. `CmpValueType` has no case for `UserDefinedType`, causing all comparisons involving it to return `false`.

**Fix** (`tc_known_types.go`): Added a `FunctionType` case to `resolveValueType` that recursively resolves `ReturnType` and `ParamTypes`:
```go
} else if ft := zeus_value.AsFunctionType(valueType); ft != nil {
    if ft.ReturnType != nil {
        ft.ReturnType = p.resolveValueType(tc, ft.ReturnType, true)
    }
    for i := range ft.ParamTypes {
        if ft.ParamTypes[i] != nil {
            ft.ParamTypes[i] = p.resolveValueType(tc, ft.ParamTypes[i], false)
        }
    }
    return *ft
}
```

### Bug 4 — False "unused" warnings

Three distinct false-positive warning cases were fixed in `UnusedWarningPass`:

**4a — Function passed as argument**: When `apply(double, 5)` is called, `tryImplicitCast` wraps `double` in a `CAST` instruction to coerce `*Function → FunctionType`. The `CALL_FUNC` args reference the cast result, not `double` directly. `UnusedWarningPass` handled `InstrTypeCoerce` but not `InstrTypeCast`, so the original function looked unused.

**Fix**: Added `InstrTypeCast` case to `HandleInstruction` to mark the cast's source value as used.

**4b — Exported class methods**: Each module is type-checked independently. `UnusedWarningPass.handleExport` marked the exported class as used but NOT its methods. Methods called only by importers appeared unused.

**Fix**: `handleExport` now marks all methods and properties of exported classes as used (they're part of the public API).

**4c — Exported functions as arguments** (pre-existing): This is the combination of 4a and 4b — covered by the `InstrTypeCast` fix.

### Bug 5 — Global instruction implicit cast panic

**Symptom**: Compiling a module with a global variable whose initializer requires an implicit cast (e.g. integer constant → larger int type) panics with "instruction DECLARE_VAR ... not found in block instructions list".

**Root cause**: `Walk` processes function body blocks inline (via worklist) before continuing to the next global instruction. After processing a function's body, `tc.currentBlock` is left set to the last block visited. When `tryImplicitCast` is later called for a global instruction, it calls `SetBlockInsertionBefore(tc.currentBlock, instr)` — but `instr` is in the global list (`b.instrs`), not in `tc.currentBlock` → assert fails.

**Fix** (`builder.go`): Added `SetInsertionBeforeInstr(block, instr)` which tries block-scoped insertion first and falls back to global-list insertion if the instruction is not in the block.

```go
func (b *IRBuilder) SetInsertionBeforeInstr(block *BasicBlock, instr *Instr) {
    if block != nil {
        if idx := slices.Index(block.Instrs, instr); idx != -1 {
            b.SetInsertionBlock(block)
            b.blockIdInsetionIndexMap[block.Id] = idx
            return
        }
    }
    // Global instruction: insert into the top-level list.
    idx := slices.Index(b.instrs, instr)
    zeus_error.Assert(idx != -1, ...)
    b.SetInsertionBlock(nil)
    b.insertionIndex = idx
}
```

`tryImplicitCast` now calls `SetInsertionBeforeInstr` instead of `SetBlockInsertionBefore`.

---

## Pass pipeline summary

| Phase | What happens |
|---|---|
| **Escape analysis** (`collectEscapedVarNames`, AST walk) | Identifies which vars/params in each function scope escape into nested closures |
| **AST pre-inference** (`inferFunctionLocalTypes`, AST walk) | Pre-infers types of escaped vars from initializer expressions; runs after params and preamble are in symbol table |
| **IR gen — `VisitVarDeclStmt`** | Escaped vars → allocate ref cell (`NEW_OBJ __ref_cell_T__`), initialize `cell.value`, register `RefCellVar` in symbol table; three-priority cascade for type resolution |
| **IR gen — `VisitIdentifier`** | `RefCellVar` → emit `PROP_ACCESS cell.value` + `LOAD` |
| **IR gen — `VisitAssignExpr`** | `RefCellVar` target → emit `PROP_ACCESS cell.value` + `STORE` |
| **IR gen — `collectFreeVars`** | Captures ref cell objects (not values); property type = `ObjectType{__ref_cell_T__}`; `IsRefCell=true` |
| **IR gen — `emitFunctorClass`** | Stores cell pointer into `__cap_x__` property (no load for ref cells) |
| **IR gen — `__call__` preamble** | Loads cell from `this.__cap_x__`, registers `RefCellVar` in local symbol table |
| **`ToKnownTypesPass`** | Resolves `UserDefinedType` inside `FunctionType` return/param types (fixed Bug 3) |
| **`TypeCheckingPass`** | `.value` property access on ref cell resolves to type `T` normally; ref cell classes are ordinary Zeus classes |
| **Codegen** | `PROP_ACCESS` + `LOAD`/`STORE` on ref cell objects — no new codegen cases needed |

---

## Relevant source locations

| File | Relevance |
|---|---|
| `internal/ir/closure.go` | `CapturedVar`, `freeVarCollector`, `collectFreeVars`, `collectEscapedVarNames` |
| `internal/ir/ir_type_infer.go` | `resolveUserDefinedType`, `inferExprType`, `inferFunctionLocalTypes` |
| `internal/ir/ir.go` — `emitFunctorClass` | Functor class creation, capture property initialization |
| `internal/ir/ir.go` — `emitFunction` | Escape analysis, AST pre-inference, escaped param promotion, `__call__` preamble |
| `internal/ir/ir.go` — `VisitVarDeclStmt` | Three-priority ref cell promotion cascade |
| `internal/ir/ir.go` — `VisitIdentifier` | `RefCellVar` → `PROP_ACCESS` + `LOAD` |
| `internal/ir/tc_known_types.go` — `resolveValueType` | `FunctionType` inner type resolution (Bug 3 fix) |
| `internal/ir/tc_unused.go` | False warning fixes: `InstrTypeCast`, exported class members |
| `internal/ir/tc_type_check.go` — `tryImplicitCast` | Uses `SetInsertionBeforeInstr` for global-instruction safety |
| `internal/ir/builder.go` — `SetInsertionBeforeInstr` | Block-or-global insertion helper (Bug 5 fix) |
| `internal/ir/tc.go` — `resolveClass` | Skips nil property types (safety guard) |
| `internal/codegen/codegen.go` — `toLLVMType(ObjectType)` | Returns `ptr addrspace(1)` without struct lookup (Bug 2b fix) |
| `internal/zeus_value/value.go` — `RefCellVar` | Ref cell value type implementing `Value` interface |
| `internal/zeus_value/primordials.go` | `RefCellClassName`, `GetRefCellClassDefinition`, `ZEUS_REF_CELL_VALUE_PROPERTY` |
| `test/e2e/specs/closures/` | 27 closure e2e tests |
