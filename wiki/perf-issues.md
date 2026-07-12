# Performance Issues

Known performance problems in the code Zeus generates today, discovered with the
[`bench/`](../bench) suite (Zeus vs Go vs Node.js). Each entry gives the symptom,
the **root cause with evidence**, and a fix direction. These are optimization
opportunities, not correctness bugs — see [compiler-bugs.md](compiler-bugs.md)
for the latter.

The headline finding: **Zeus is competitive with (or faster than) Go on scalar,
register-only code, but 7–90× slower on anything that touches an array.** The
entire gap comes from how array element access is lowered.

## Benchmark snapshot

Whole-process wall-clock via `hyperfine` (release Zeus / `go build` / Node with
typed arrays). Measured on Apple M1 Pro, macOS 26, at commit
`v0.0.21-alpha-16-g3b9a642`. `Zeus vs Go` = Zeus time ÷ Go time (lower is better).

| Benchmark | What it does | Zeus | Go | Node | Zeus vs Go |
|-----------|--------------|------|----|------|-----------|
| Recursive Fibonacci | `fib(40)`, pure calls + int add | 306 ms | 342 ms | 947 ms | **0.89×** |
| Loop Sum | `Σ (i & 1023)`, `1..1e9`, i64 | 215 ms | 468 ms | 968 ms | **0.46×** |
| Prime Sieve | Sieve of Eratosthenes to 3e7 | 621 ms | 85 ms | 149 ms | 7.3× |
| Insertion Sort | O(n²) sort, N=20000 | 3.25 s | 36 ms | 109 ms | **89.8×** |
| Matrix Multiply | naive 512×512 int matmul | 3.36 s | 97 ms | 268 ms | 34.6× |

The two scalar benchmarks (fib, loop-sum) beat Go. The three array benchmarks
(sieve, sort, matmul) are the outliers — and they scale with **how many array
accesses per unit of work** the algorithm does.

> **Update — FIXED (reads and writes).** Primitive array reads and writes now lower to
> a direct `ELEM_LOAD` / `ELEM_STORE` (data-pointer GEP + load/store) instead of a
> `get()`/`set()` method call. The numbers above collapsed toward Go and now **beat
> Node on every benchmark**: **matmul 3.36 s → 0.094 s (ties Go)**, **sort 3.25 s →
> 0.069 s (was 90× Go, now 1.9×)**, **sieve 0.62 s → 0.097 s (ties Go)**. See the
> Status note in issue #1.

---

## 1. Array indexing compiles to virtual (vtable) method calls

This is the dominant issue and explains all three array benchmarks.

> **Status: FIXED.** `IndexLoweringPass` now lowers single-index primitive `arr[i]` to
> `ELEM_LOAD` and `arr[i] = v` to `ELEM_STORE` (codegen: GEP + load/store), bypassing the
> vtable, the method call and the runtime round-trip. Because bracket-writes auto-extend
> the array (a documented feature), the write fast path is guarded — `if 0 <= i < length`
> does the direct store, else it falls back to the runtime `set()` which grows the array.
> Two footguns handled: the store must use the **element** width, not the value
> expression's width (a narrow literal into an `i32[]` would otherwise do a 1-byte store);
> and multiple writes per basic block are handled by locating each write's *current* block
> before splitting. Remaining array gap vs Go (insertion sort) is now auto-vectorization,
> not dispatch — see issue #2.

### Symptom

Array-dense loops run 7–90× slower than Go. The slowdown tracks access density:
insertion sort (3 accesses per inner iteration) is worst at 90×; matmul (2 loads
per multiply-accumulate) is 35×; the sieve (one sparse store per mark) is "only"
7×.

### Root cause

`arr[i]` and `arr[i] = v` are **not** lowered to a load/store. `IndexLoweringPass`
rewrites `GET_INDEX`/`SET_INDEX` into `array.get(i)` / `array.set(i, v)` **method
calls** (`internal/ir/lowering.go:453`, `BuildMethodCall(dataArray,
ARRAY_METHOD_GET, …)` at `:519`). Codegen then dispatches every method through the
class vtable (`internal/codegen/codegen.go`, `LLVMVTableMethods` /
`getLLVMVTablePtr`), so each access becomes an **indirect call**.

Here is the actual optimized (`--release`) inner loop of insertion sort
(`while (j >= 0 && arr[j] > key) { arr[j+1] = arr[j]; j-- }`), disassembled:

```asm
0x10c0  ldr w9, [x19, #0xc]     ; load array length  (bounds info)
0x10c8  cmp w22, w9
0x10cc  b.gt <throw>            ; bounds check #1
0x10d0  ldr x9, [x19]           ; reload array object ptr from stack slot
0x10e0  ldr x9, [x9]            ; load vtable ptr
0x10e4  ldr x9, [x9, #0x10]     ; load .get fn ptr from vtable slot 0x10
0x10e8  blr x9                  ; INDIRECT CALL  ->  arr.get(j)
0x10ec  cmp x0, x21            ; compare to key
...
0x1110  ldr x8, [x8, #0x10]     ; .get again
0x1114  blr x8                  ; INDIRECT CALL  ->  arr.get(j)
0x112c  ldr x8, [x8, #0x18]     ; .set fn ptr, vtable slot 0x18
0x1130  blr x8                  ; INDIRECT CALL  ->  arr.set(j+1, …)
0x113c  b.gt 0x10c0            ; loop
```

Static call counts in the user `main`: **207 `blr` (indirect calls) vs 27 `bl`
(direct)**. Go compiles `arr[j]` to a single `ldr` off a register-held slice base.
So per element Zeus does ~3 indirect *function calls* where Go does ~2 register
*loads*.

Four things compound, all visible above:

1. **Indirect dispatch → no inlining.** `blr` on a vtable-loaded function pointer
   can't be inlined by LLVM and defeats the branch predictor. The `get`/`set`
   bodies (a bounds check + one load/store) stay behind a full call-ABI boundary
   (arg marshalling, spill/restore) instead of collapsing to one instruction.
2. **Base pointer never hoisted / register-promoted.** `ldr x9, [x19]` reloads the
   array object pointer from its stack slot *on every access*, then chases the
   vtable (`ldr [x9]; ldr [x9, #0x10]`). None of this is loop-invariant-hoisted,
   because the value crosses an opaque call and lives in memory.
3. **Bounds check on every access, no elimination.** Each site does `ldr length;
   cmp; b.gt →throw`. There's no bounds-check elimination for provably-in-range
   indices (e.g. `j` in `0..len`).
4. **Dispatch is virtual even though the type is statically known.** The element
   type (`i32[]`, `i64[]`, `u8[]`) is fixed at compile time, so `get`/`set` could
   be a direct or inlined operation, but codegen uses uniform vtable dispatch for
   all methods.

### Why the scalar benchmarks are fine

fib and loop-sum never construct or index an array — they're pure register
arithmetic, where Zeus/LLVM is on par with or beats Go. This is strong evidence
that the array-access lowering is the *sole* cause of the array-heavy gap, not
codegen quality in general.

### Fix direction

Add a fast path in lowering/codegen for **primitive-typed arrays**: lower
`GET_INDEX`/`SET_INDEX` on `iN[]`/`uN[]`/`fN[]` directly to `getelementptr +
load/store` against a raw data pointer, instead of a `get`/`set` method call. This
matches the "keep codegen a dumb, mechanical translator" principle — the flat
data-pointer + element layout is materialized in IR/lowering and codegen just
emits the GEP. Concretely:

- Keep the array's raw data pointer in the object at a fixed offset; emit a direct
  `GEP(base, i)` + `load`/`store` so LLVM can hoist `base` into a register and
  keep the whole access in-loop.
- Emit the bounds check inline as a compare + `cond-branch` to a shared trap block
  (so LLVM can prove and delete redundant checks), and add basic bounds-check
  elimination for monotonic loop indices.
- Fall back to the `get`/`set` method path only for genuinely dynamic receivers
  (interface/`Object`-typed collections) once those exist.

Touch points: `internal/ir/lowering.go` (`IndexLoweringPass.lowerGetIndex` /
`lowerSetIndex`), the array layout in `internal/zeus_value` primordials, and the
method-call codegen in `internal/codegen/codegen.go`.

---

## 2. Numeric inner loops are not auto-vectorized

### Symptom

Matrix multiply (512×512, the most vectorizable kernel) is 35× slower than Go even
though its inner loop is dense arithmetic. Once issue #1 is fixed the remaining
gap here will be dominated by vectorization.

### Root cause

Because every `a[i*n+k]` / `b[k*n+j]` is an opaque indirect call (issue #1), LLVM
cannot see the loop as a stream of loads over contiguous memory, so its
loop-vectorizer and SLP passes have nothing to work with. Go emits tight scalar
(and in places vectorizable) code over its slices.

### Fix direction

Mostly falls out of fixing #1: once accesses are plain GEP+load/store, LLVM's
existing vectorizer can engage. After that, confirm the `--release` pipeline
actually runs the loop-vectorization passes (check the opt level / pass list in
`internal/zeus_compiler/compiler.go::compileRelease`) and add `noalias`/alignment
metadata where sound.

---

## 3. (resolved) Element width in the sort benchmark

Earlier the `sort` benchmark used `i64[]` (8-byte elements) versus Go's `[]int32`,
which was blamed on a `u32` codegen crash. That was a **misdiagnosis**: `u32` compiles
fine — the original crash was the sibling-loop-variable-name-reuse bug (since fixed by
the member-model refactor, `fc5c343`). The benchmark now uses `i32[]` with a `u32` LCG,
matching Go's element width.

---

## 4. Interface dispatch sites can't cache their object files (build perf)

### Symptom

Any module that dispatches through an interface (`s.area()` where `s: Shape`) is
recompiled whenever **any** interface in the program changes shape — even an
interface it doesn't use, and even if the dispatching module's own source is
untouched. Non-dispatching modules still cache normally; only dispatch sites pay.

### Root cause

An interface method call bakes two *interface-layout constants* into the call
site's machine code (`internal/codegen/codegen.go`, `genInterfaceMethodCall` /
`genInterfacePropertyPtr`):

1. the method's **interface slot** (`InterfaceMethodIndex`), and
2. the **itable stride** (`numMethods`/`numProps`, the inner-array width used in
   the `getelementptr`).

Both are derived from the interface *definition*, which may live in a different
file than the call site. So a dispatch module's `.o` is **not** a pure function
of its own source — it also depends on the interfaces it dispatches. Zeus caches
`.o` files by per-file source hash (`EmitObjFiles`), which assumes the pure-function
property, so a stale cached dispatch `.o` would carry the wrong slot/stride when an
interface changes in another file (a silent miscompile — verified: adding a method
to an imported interface without touching the dispatch file made `areaOf` return
0 instead of 36).

**Current fix is conservative (the remaining issue is the *cost*, not correctness):**
a module that dispatches is flagged (`CodegenModule.dispatchesInterface`) and its
object-file cache key is salted with a digest of *all* interface tables
(`SourceFile.ObjCacheKeySalt` = `Codegen.interfaceTablesDigest`). Correct, but any
interface-shape change busts every dispatch module's cache, not just the ones that
use the changed interface.

### Fix direction

Stop baking the slot/stride into the dispatch site. Emit them as small **external
constants in the content-addressed itable module** (e.g. `@__zeus_iface_N_stride`
and a per-method slot constant), and have the dispatch site **load** them and
compute a flat index `classId*stride + slot` against a flat `[K x i32]` itable.
Then the dispatch site's `.o` no longer depends on any interface's shape → it
caches by per-file hash like everything else, and the `dispatchesInterface` salt
can be dropped. Cost: ~2 extra loads per interface call (both hit a small constant
table). Alternatively, dependency-track: mix only the *dispatched* interfaces'
definition hashes into the module's key instead of the whole-program digest
(tighter, but needs per-module dependency records).

Touch points: `genInterfaceMethodCall` / `genInterfacePropertyPtr` and the
`defineInterface*DispatchTable` emitters in `internal/codegen/codegen.go`.

---

## Reproducing / measuring

```bash
# Rebuild and time everything (needs: hyperfine, go, node, the zeus toolchain)
bash bench/run.sh

# Inspect what an array access compiles to:
export ZEUS_HOME=$(pwd)
./zeus build bench/cases/sort/sort.zs --release -o /tmp/sort
otool -tV /tmp/sort | awk '/^_#_zeus_main:/{f=1} f&&/^_[A-Za-z#].*:$/&&!/_#_zeus_main:/{f=0} f' \
  | grep -cE '\bblr\b'   # indirect calls in main; high == the array-dispatch problem
```

On Linux use `objdump -d /tmp/sort` instead of `otool -tV`.

See [`bench/README.md`](../bench/README.md) for the harness and the published
report at **docs → Performance → Benchmarks**.
