# Class Layout Materialization (`ClassLayout`)

**Shipped 2026-07-11.** An architecture note — the code is the source of truth.

## What & why

Class *physical layout* — base-first field order, the vtable slot→method map (with override
resolution), and the effective constructor — used to be re-derived inside `internal/codegen` via the
`Flattened*` helpers, and the vtable was filled by a two-phase mechanism (`genClassMethod` writing
name-resolved slots + `finalizeInheritedVTables` copying inherited base functions). That put Zeus
*layout policy* in the LLVM backend, against the "dumb codegen" rule that `internal/codegen` should
be a mechanical translator.

Now the layout is computed once as data by `func (c *Class) Layout() *ClassLayout` in
`internal/zeus_value/value.go` (`ClassLayout` / `VTableEntry`); codegen, `internal/util`, and
`internal/ir/tc_type_check.go` read it. The `Flattened*` helpers survive only as the builder's
internals — except `FlattenedMethods`, a *name* resolver still used by the type checker. What's left
in codegen is pure LLVM mechanism.

## Non-obvious mechanics

- **`flattenedVTableEntries`** is `FlattenedVTableMethods` adapted to also record
  `VTableEntry.DefiningClass` — the class whose method list contributed the slot (base for an
  inherited slot, derived for an override). Needed to name extern (primordial) methods, which are
  compiled under a class-scoped symbol.
- **`fillVTables`** (replaced `finalizeInheritedVTables`) sets every vtable in one pass after all
  bodies are emitted, resolving each slot by name — `entry.Method.Method.Name`, or
  `util.GetClassMethodName(entry.DefiningClass.Name, …)` when `IsExtern` — via `module.NamedFunction`
  (the same pattern `genStaticMethodCall` uses for `super`). It iterates every non-imported class in
  `zeusClassLLVMStructMap` because it subsumes *both* the old per-method slot writes
  (primordials/externs) *and* the inherited-slot copy. Resolving by name also removed the
  base-before-derived ordering dependency the old fill relied on.
- **Memoization is per-copy.** Codegen passes `zeus_value.Class` by value, so the memoized `layout`
  field doesn't share across copies — harmless (the builder is idempotent and cheap), just don't
  expect cross-call cache hits. `Layout()` is only ever built post-IR-gen (IR-gen uses the
  name-resolution `Lookup*` helpers, never `Layout`), so it always builds against a finalized class.

## Factory body → synthesized IR (user classes done)

The object factory used to be synthesized entirely in codegen from `NEW_OBJ`. For **user classes**
it is now a real IR function: `FactoryLoweringPass` (last pass in `internal/ir/lowering.go`)
synthesizes `zeus_new_<Class>` as `ALLOC_OBJ <Class>` + (if an effective constructor is compiled)
`SUPER_CONSTRUCTOR_CALL` + `RETURN`. The one new instruction, **`ALLOC_OBJ`**, is the pure-mechanism
core (`emitAllocAndHeader`: `zeus_gc_alloc` + header store); there are no field-init stores because
`zeus_gc_alloc` (Boehm `GC_malloc`) returns zeroed memory and every field default is a zero-value.
The constructor call reuses `SUPER_CONSTRUCTOR_CALL`, and the factory symbol is external so
cross-module `new` links (`genDeclFunc`, keyed on `util.FactoryFunctionPrefix`).

Still in codegen: `genFactoryFunctionBody`/`declareFactoryFunction` now run for **primordials and
arrays only** (`string`, `Error`, `u8[]`, `Object[]`, …). Moving those to synthesized IR too — then
deleting `genFactoryFunctionBody` entirely — is the remaining follow-up; it needs care for the
runtime-called factories (`zeus_new_string`) and array internals (length/capacity/data).
