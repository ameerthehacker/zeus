# Codegen Refactor: Materialize Class Layout (planned)

> **Status: planned, not yet implemented.** Captured here to revisit later.

## Motivation

Class layout — the base-first field order, the vtable slot→method map (with override resolution),
and the effective constructor — is currently **re-derived inside codegen** by repeatedly calling
the flattened helpers (`FlattenedInstanceProperties` / `FlattenedVTableMethods` /
`LookupConstructorClass`) and the `util` index functions. Worse, the vtable is filled by a subtle
**two-phase mechanism**:

1. `genClassMethod` writes each compiled method into a name-resolved slot (`util.GetVTableSlot`,
   which needs an IR-name-then-source-name fallback to survive accessor and extern name skew), and
2. `finalizeInheritedVTables` copies inherited base functions into the remaining slots.

That is Zeus *policy* living in the LLVM backend. The guiding principle (see the "dumb codegen"
rule) is that `internal/codegen` should be a **mechanical translator**: only LLVM mechanics (struct
types, GEPs, calls, global initializers, function-pointer resolution), with all Zeus-semantic
layout computed elsewhere and handed to codegen as data.

**Goal:** compute the layout once as explicit data (`ClassLayout`) in `zeus_value`, have every
consumer read it, and replace the two-phase vtable fill with a single layout-driven pass. No
behavior change; the win is separation of concerns, a single source of truth, and a layout that is
unit-testable without LLVM.

**Safe to memoize:** the physical-layout consumers (`Flattened*`, `Get*Index`, `GetVTableSlot`) run
only in `internal/codegen`, `internal/util`, and `internal/ir/tc_type_check.go` — all *after* IR
generation, when classes are final. (IR-gen only calls the name-resolution `Lookup*` helpers.) So a
`ClassLayout` built lazily on first access is always built against a finalized class.

## Design

### 1. `ClassLayout` in `internal/zeus_value/value.go`

```go
type VTableEntry struct {
    Method        *ClassMethod // the method that fills this slot (override or inherited)
    DefiningClass *Class       // class whose method fills it — needed to name extern fns
}
type ClassLayout struct {
    Fields           []*ClassProperty // base-first instance fields; struct slot = index+1
    VTable           []VTableEntry    // base-first vtable slots; slot = index
    Constructor      *Function        // effective constructor (own or nearest inherited), or nil
    ConstructorClass *Class           // class that declares Constructor
}
```

- Add a memoized `layout *ClassLayout` field on `Class`; expose `func (c *Class) Layout() *ClassLayout`
  that builds and caches on first call.
- The builder folds in the existing logic: `Fields` = base-first non-static properties;
  `VTable` = base-first vtable methods with overrides replacing the base slot in place (reuse the
  `FlattenedVTableMethods` algorithm but also record `DefiningClass` per slot); `Constructor` /
  `ConstructorClass` via the `LookupConstructorClass` walk.
- Keep the existing `Flattened*` helpers as the builder's internals (or inline them); they stop
  being called from codegen/util directly.

### 2. `internal/util/util.go` reads the layout

- `GetPropertyIndex` → index into `class.Layout().Fields` (`+1` for the header).
- `GetMethodIndex` → index into `class.Layout().VTable` by `Method.SourceName()`.
- **Delete `GetVTableSlot`** (only used by the write path being removed).

### 3. codegen consumes the layout; single vtable fill (`internal/codegen/codegen.go`)

- `createClassStructTypes`: field types from `layout.Fields`; vtable slot types from `layout.VTable`.
- `genClass`: `methodCount = len(layout.VTable)`.
- `genClassMethod`: **remove** the vtable slot write (keep body generation and the constructor-ref
  capture). `emitExternMethods` likewise stops relying on the write.
- **Replace** `finalizeInheritedVTables` with `fillVTables()` run once after all functions are
  emitted: for each class, for each `layout.VTable` entry, resolve the LLVM function by name
  (`entry.Method.Name`, or `util.GetClassMethodName(entry.DefiningClass.Name, entry.Method.Name)`
  when `entry.Method.IsExtern`) via `c.module.NamedFunction`, and set the vtable global initializer.
  This removes the name-skew and the two-phase subtlety in one pass.
- Factory (`declareFactoryFunction`, `genFactoryFunctionBody`): use `layout.Constructor` /
  `layout.ConstructorClass` and `layout.Fields` for the base-first default-init. **Delete the
  `effectiveConstructor` helper** (subsumed by the layout).

### 4. type checker (`internal/ir/tc_type_check.go`)

- `tcObjectPropertyAccess`: field resolution reads `class.Layout().Fields`. Method/accessor **name**
  resolution keeps using `LookupMethod`/`FlattenedMethods`/`LookupAccessor` (that is name lookup,
  not physical layout, and correctly stays a walk).

Everything left in codegen after this is LLVM mechanism: struct/vtable type creation, GEPs, calls,
`ConstStruct`, global initializers, function-pointer resolution.

## Files to modify

| File | Change |
|------|--------|
| `internal/zeus_value/value.go` | `ClassLayout`/`VTableEntry`, memoized `Class.Layout()`, builder |
| `internal/util/util.go` | `GetPropertyIndex`/`GetMethodIndex` via layout; delete `GetVTableSlot` |
| `internal/codegen/codegen.go` | struct/vtable/factory read layout; `fillVTables` replaces `finalizeInheritedVTables`; drop slot write + `effectiveConstructor` |
| `internal/ir/tc_type_check.go` | field resolution via `layout.Fields` |
| `test/ir/` (new) | unit tests for `ClassLayout` (field order, vtable slots + overrides, effective ctor) — no LLVM needed |

## Verification

- New unit tests asserting layout for a hierarchy: `Fields` base-first, `VTable` slot indices with
  an override reusing the base slot and a new method appended, `DefiningClass` per slot, and
  `Constructor`/`ConstructorClass` for own vs inherited constructors. These run without LLVM
  (`go test ./test/ir/...`).
- Regression — the whole point is zero behavior change, so the **full e2e suite must stay green**,
  with extra attention to **primordials** (string, arrays, Error, Console) since their extern-method
  vtables now fill through the new name-resolved `fillVTables` path.

```bash
go build -tags llvm19 ./...
go test ./test/parser/... ./test/lexer/... ./test/ir/...
ZEUS_HOME=/Users/ameerthehacker/code/zeus go test -tags llvm19 ./test/e2e/... -count=1
```

Validate via the e2e **harness**, not direct binary invocation.

## Out of scope (separate, larger follow-up)

- **Factory body → explicit IR.** The object factory (allocate + header + field init + constructor
  call) is still synthesized entirely in codegen from `NEW_OBJ`. Lowering it into a real synthesized
  constructor function (STORE/CALL instructions) is the other big "Zeus semantics in codegen"
  offender, but it is pre-existing and much larger; do it as its own pass.
