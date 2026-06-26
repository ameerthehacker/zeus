# Release Compilation

Zeus supports two build modes: **debug** (the default) and **release**. Release builds apply whole-program optimizations that produce smaller, faster binaries at the cost of longer compile times and no incremental caching.

---

## Usage

```bash
# Debug build (default)
zeus build main.zs

# Release build
zeus build main.zs --release
```

All other flags (`-o`, `--target-dir`) work identically in both modes.

---

## What Changes in Release Mode

| | Debug | Release |
|---|---|---|
| LLVM modules | One per source file | All merged into one |
| Optimization | None | O3 (full pipeline) |
| Incremental cache | Yes (hash-based `.o` files) | No (always recompiles) |
| Output directory | `target/debug/` | `target/release/` |

---

## Directory Layout

```
target/
  debug/
    obj/   ← per-file object files (<hash>.o)
    bin/   ← executable
  release/
    obj/   ← single merged object file (program.o)
    bin/   ← executable
```

---

## How Release Compilation Works

### 1. Shared front-end (same as debug)

Lexing → parsing → Zeus IR generation → type checking → IR lowering → LLVM IR generation runs identically for both modes. Each source file still gets its own `CodegenModule` at this stage.

### 2. Module merging

All per-file LLVM modules are linked together into a single `zeus_program` module using `llvm.LinkModules`. This enables whole-program analysis and cross-file inlining that per-file compilation cannot achieve.

**Primordial factory function pinning** (see [below](#primordial-factory-function-pinning)) runs on each source module immediately before it is merged.

### 3. O3 optimization

The merged module is run through LLVM's full `default<O3>` pass pipeline, which includes inlining, dead code elimination, loop transformations, vectorization, and more.

### 4. Single object file emission

The optimized module is emitted as a single `target/release/obj/program.o`.

### 5. Linking

The single object file is linked with the Zeus runtime objects and the BDW-GC library, producing `target/release/bin/<name>`.

---

## No Incremental Cache in Release

Debug builds cache each source file's object file by SHA256 hash (see [incremental-compilation.md](incremental-compilation.md)). Release builds merge all modules before optimization, so there is no meaningful per-file unit to cache. Every `zeus build --release` is a full rebuild.

---

## Primordial Factory Function Pinning

### Background

Every class in Zeus has a **factory function** named `zeus_new_<ClassName>` (e.g. `zeus_new_string`, `zeus_new_u8_array`). The naming prefix is the exported constant `codegen.FactoryFunctionPrefix`. For primordial (built-in) classes these functions are emitted into **every** compilation unit with `LinkOnceODRLinkage`, which tells the LLVM IR linker "all copies are identical — keep exactly one."

The Zig runtime calls these factory functions by name (e.g. `zeus_string_concat` calls `zeus_new_u8_array`). Because those call sites live in a separate Zig-compiled object file and are never visible to the LLVM IR linker, the factory functions appear to have **no callers** inside the LLVM module.

### The problem

`llvm.LinkModules` silently drops `LinkOnceODR` functions that have no intra-module callers. This means `zeus_new_string` and `zeus_new_u8_array` disappear from the merged module before O3 even runs, causing the system linker to fail with:

```
undefined symbols:
  _zeus_new_string, referenced from: zeus-runtime.o
  _zeus_new_u8_array, referenced from: zeus-runtime.o
```

### The fix — `pinPrimordialFactoryFunctions`

Before each source module is passed to `llvm.LinkModules`, `pinPrimordialFactoryFunctions` runs and does two things:

1. **Promotes to `ExternalLinkage`** — but only for the *first* module that defines a given factory function (tracked via a `pinned` name map). `ExternalLinkage` is a strong symbol; `LinkModules` cannot drop it. Subsequent modules that also define the same factory function keep `LinkOnceODRLinkage`; `LinkModules` resolves them against the already-present `External` definition and discards the duplicates without a "multiply defined" error.

2. **Adds to `@llvm.used`** — every copy (promoted or not) is listed in the module's `@llvm.used` metadata array. This tells LLVM optimization passes not to eliminate the function even if it has no visible callers.

```
source module A  →  pin (promote to External, add to @llvm.used)  →  LinkModules
source module B  →  pin (already seen, keep LinkOnceODR, add to @llvm.used)  →  LinkModules
...
merged module  →  O3  →  emit program.o  →  system linker
```

### Why not promote all copies to External?

Promoting every copy to `ExternalLinkage` before merging causes `LinkModules` to see multiple strong definitions of the same symbol and fail with "symbol multiply defined". The `pinned` map ensures only one strong definition enters the merge.

### Detection

A function is identified as a primordial factory function when all three hold:

- Name starts with `codegen.FactoryFunctionPrefix` (`"zeus_new_"`)
- Linkage is `LinkOnceODRLinkage` (user-defined class factories use `ExternalLinkage` and are excluded)
- It is a definition, not a declaration

---

## LLVM IR Dump (ZEUS_DEBUG)

When the `ZEUS_DEBUG` environment variable is set, the release compiler writes the **merged, optimized** LLVM IR alongside the object file:

```
target/release/obj/program.o    ← object file
target/release/obj/program.ll   ← merged optimized LLVM IR
```

This is useful for inspecting what the optimizer produced.

```bash
ZEUS_DEBUG=1 zeus build main.zs --release
cat target/release/obj/program.ll
```

---

## Implementation Reference

| Location | Responsibility |
|---|---|
| `cmd/build.go` | `--release` flag; derives `modeDir` (`debug`/`release`); sets `objDir` and `outputPath` |
| `internal/codegen/codegen.go` → `FactoryFunctionPrefix` | Canonical prefix for all primordial factory function names |
| `internal/zeus_compiler/compiler.go` → `NewCompiler` | Uses `CodeGenLevelAggressive` for release, `CodeGenLevelDefault` for debug |
| `internal/zeus_compiler/compiler.go` → `isPrimordialFactoryFunction` | Detects factory functions by name prefix + `LinkOnceODRLinkage` |
| `internal/zeus_compiler/compiler.go` → `pinPrimordialFactoryFunctions` | Promotes first definition to `External`, adds all to `@llvm.used` |
| `internal/zeus_compiler/compiler.go` → `mergeModules` | Links all per-file LLVM modules into one via `llvm.LinkModules` |
| `internal/zeus_compiler/compiler.go` → `runReleasePasses` | Runs `default<O3>` on the merged module |
| `internal/zeus_compiler/compiler.go` → `compileRelease` | Orchestrates merge → optimize → emit → link |
| `internal/zeus_compiler/compiler.go` → `compileDebug` | Existing per-file obj cache path (no optimization passes) |
| `internal/codegen/codegen.go` → `NewMergeTarget` | Creates the bare destination LLVM module for linking |
