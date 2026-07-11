# Inheritance: Architecture Review & Refactor Backlog

> **Status: assessment / backlog, not yet implemented.** An honest critique of the inheritance
> implementation (Phase 1 fields/methods/dispatch, Phase 2 `super(...)`, Phase 3 `super.method()`),
> with prioritized cleanups. See [inheritance.md](inheritance.md) for how it currently works and
> [codegen-refactor.md](codegen-refactor.md) for the layout-materialization plan that fixes the two
> biggest items here.

Overall verdict: **the data model is well-designed; the codegen boundary and the enforcement
mechanics are not, and there are real type-safety gaps.** Grade ≈ B. The bones are right; the edges
aren't clean yet.

---

## What's well-architected

- **The layout model is the right one.** Base-first fields + a shared-slot vtable is the standard,
  proven single-inheritance design, kept as one source of truth (`FlattenedInstanceProperties` /
  `FlattenedVTableMethods`). Three properties fall out for free instead of being special-cased:
  - **Upcasting is a no-op** — a `Dog*` *is* an `Animal*` (base-first layout + LLVM opaque pointers).
  - **Dynamic dispatch is just "same slot index across the hierarchy"** — no runtime type check.
  - **Overriding is ~4 lines** ("replace the base's slot in place"), not a feature.
- **Minimal surface area.** `super` reuses the identifier machinery (like `this`, no new token);
  `super.method()` reuses `CALL_METHOD` via one `StaticClass` field instead of a new instruction.
  The IR vocabulary didn't grow for variations of things that already exist.

---

## Weaknesses (severity order)

### 1. Zeus policy lives in the LLVM backend
Codegen *re-derives* layout (calls the flattened helpers repeatedly) and owns the object factory.
This is the "dumb codegen" violation and the single biggest architectural flaw. Fix: the parked
[codegen-refactor.md](codegen-refactor.md) — materialize a `ClassLayout` that codegen consumes.

### 2. The two-phase vtable fill is fragile
`genClassMethod` writes each method into a name-resolved slot (`util.GetVTableSlot`) and
`finalizeInheritedVTables` copies the inherited ones. The split is subtle and already caused a bug:
`GetVTableSlot`'s "IR-name-then-source-name" fallback exists *only* because an accessor's descriptor
name and its compiled function's name disagree. A single layout-driven fill deletes that whole class
of bug. (Also addressed by the codegen refactor.)

### 3. `super` enforcement is stateful and leaky
"Call `super` before `this`" is enforced by mutating `IRModule` fields
(`currentConstructorClass` / `superCalledInCtor` / `superRequiredInCtor`) and riding IR generation's
source-order traversal. Problems:
- It couples a **language rule** to **emission order** — semantics shouldn't depend on how IR is
  walked.
- **Nested-closure edge:** a closure inside a constructor inherits the flag — an accepted
  correctness gap.
- The **"`super(...)` must be a top-level statement (not in an `if`/loop)"** limitation is a direct
  consequence of this shortcut (a linear scan instead of JS's all-paths analysis).
- `emitSuperMethodCall` uses `classStack[len-1]` to find the enclosing class — same
  closure/nested-class fragility.

Fix: a dedicated constructor-analysis pass over the AST instead of mutable flags on IR gen.

### 4. Duplicated call-emission in codegen
`genMethodCall`, `genStaticMethodCall`, `genSuperConstructorCall`, and the factory all repeat the
same "build `params + this` function type → resolve the fn → `CreateCall`" dance. And
`genStaticMethodCall` re-implements the base-chain walk inline *because it needs the defining class*
— a sign `LookupMethod` should also return the defining class instead of every caller re-walking.
Fix: one or two shared helpers (`emitDirectCall`, `LookupMethodWithClass`).

### 5. Real type-safety gaps (not just missing features)
| Gap | Consequence |
|-----|-------------|
| **No override-signature checking** | A derived class can override with an incompatible signature; it silently dispatches through the same slot with a mismatched type. Type-unsafe. |
| **Field shadowing is inconsistent** | `GetPropertyIndex` returns the base's slot (first match) while the type checker's last-match-wins picks the derived's type — they can disagree. Latent bug, undiagnosed. |
| **Access control across the hierarchy is under-thought** | Checks lean on `currentClass.Name == class.Name`, which doesn't model a derived method accessing an inherited private member; there is no `protected` at all. |

---

## Prioritized cleanups

1. **Layout materialization** ([codegen-refactor.md](codegen-refactor.md)) — removes weaknesses #1
   and #2 (policy-in-codegen, fragile two-phase fill). Highest leverage.
2. **Constructor-analysis pass** — move `super` presence + "`this` before `super`" out of IR-gen
   mutable state into an explicit AST pass; removes weakness #3 and its edge gaps, and unlocks
   nested/conditional `super`.
3. **Override-signature compatibility checking** — closes the type-safety hole in #5.
4. **De-duplicate codegen call emission** (#4) and make member lookup return the defining class.
5. **Field-shadowing diagnostics** and inheritance-aware access control (`protected`) (#5).

Items 1–2 remove fragility; item 3 closes the most important safety hole.
