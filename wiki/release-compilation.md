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
| `internal/zeus_compiler/compiler.go` → `NewCompiler` | Uses `CodeGenLevelAggressive` for release, `CodeGenLevelDefault` for debug |
| `internal/zeus_compiler/compiler.go` → `mergeModules` | Links all per-file LLVM modules into one via `llvm.LinkModules` |
| `internal/zeus_compiler/compiler.go` → `runReleasePasses` | Runs `default<O3>` on the merged module |
| `internal/zeus_compiler/compiler.go` → `compileRelease` | Orchestrates merge → optimize → emit → link |
| `internal/zeus_compiler/compiler.go` → `compileDebug` | Existing per-file obj cache path (no optimization passes) |
| `internal/codegen/codegen.go` → `NewMergeTarget` | Creates the bare destination LLVM module for linking |
