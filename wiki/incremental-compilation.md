# Incremental Compilation

Zeus supports incremental compilation: source files whose content has not changed since the last build are skipped — their cached object files are reused directly without re-running the full LLVM pipeline.

---

## Motivation

Before incremental compilation, every `zeus build` invocation:

1. Compiled **all** source files from scratch.
2. Created anonymous temporary object files in `os.TempDir()` that were never reused.
3. Cleaned up nothing — temp files drifted until the OS purged them.

With incremental compilation, a file that hasn't changed produces a cache hit in ~0 ms instead of going through lexing → parsing → IR generation → type checking → LLVM IR → optimization → object file emission.

---

## Directory Layout

Zeus follows the **Cargo (Rust)** convention for its output directory:

```
<target-dir>/
  target/
    debug/
      obj/   ← per-file object files, named by SHA256 hash
      bin/   ← compiled executables
```

`<target-dir>` defaults to the current working directory. Override it with `--target-dir`.

### Without `-o`

```
zeus build main.zs
```

- Executable → `./target/debug/bin/main`
- Object files → `./target/debug/obj/<hash>.o`

### With `-o`

```
zeus build main.zs -o /tmp/my-program
```

- Executable → `/tmp/my-program` (user-specified)
- Object files → `./target/debug/obj/<hash>.o` (same cache, same cwd)

### With `--target-dir`

```
zeus build main.zs --target-dir /workspace
```

- Executable → `/workspace/target/debug/bin/main`
- Object files → `/workspace/target/debug/obj/<hash>.o`

`--target-dir` sets **where the `target/` folder is created**, not the obj path directly. `-o` and `--target-dir` are orthogonal: you can use either, both, or neither.

---

## Cache Key Design

Each source file's object file is named by the **SHA256 hash of its absolute path and its content**:

```
hash = SHA256(absoluteFilePath + "\n" + fileContent)
objPath = <objDir>/<hex(hash)>.o
```

**Why include the path?**
Zeus embeds the absolute file path into every exported LLVM symbol (module-scoped naming):

```
/home/alice/math.zs → "$_home_alice_math_zs_add"
```

Two files with identical source at different paths produce different object code, so the path must be part of the cache key.

**Why include the content?**
Content changes invalidate the cache, triggering a fresh compilation.

---

## Cache Hit / Miss Logic

`EmitObjFiles` in `internal/zeus_compiler/compiler.go`:

```
for each sourceFile:
    hash = SHA256(path + "\n" + content)
    objPath = objDir + "/" + hash + ".o"

    if objPath exists on disk:
        reuse it (cache hit)
    else:
        emit LLVM object file → objPath (cache miss)

return exact list of obj paths
```

The returned list is passed directly to the linker — no directory globbing. This means stale object files from deleted or renamed source files are never accidentally linked.

---

## Clearing the Cache

```bash
rm -rf target/
```

The next build re-compiles all source files from scratch.

---

## Known Limitation: No Transitive Invalidation

The cache is keyed on a single file's path + content. It does **not** track cross-file dependencies.

**Scenario that can produce stale results:**

1. Module B exports a class with fields `x: i32, y: i32`.
2. Module A imports that class — A's object file has B's struct layout baked in.
3. You change B's class to add a new field `z: i32` (A's source is unchanged).
4. A gets a **cache hit** → A's old object file is reused with the wrong layout.
5. Runtime memory corruption.

**Workaround:** After changing exported class layouts, clear the cache with `rm -rf target/` to force a full rebuild.

Transitive invalidation (tracking which files depend on which exports) is planned for a future version.

---

## Debug Mode

When the `ZEUS_DEBUG` environment variable is set, the compiler also writes human-readable LLVM IR alongside each object file:

```
target/debug/obj/<hash>.o   ← object file
target/debug/obj/<hash>.ll  ← LLVM IR dump
```

---

## Implementation Reference

| Location | Responsibility |
|---|---|
| `cmd/build.go` | Resolves `targetBase`, `objDir`, and `outputPath`; creates directories |
| `internal/zeus_compiler/compiler.go` → `EmitObjFiles` | SHA256 cache logic; returns exact obj file list |
| `internal/zeus_compiler/compiler.go` → `linkObjFiles` | Accepts obj list (not a dir glob); appends runtime `.o` files |
| `internal/zeus_compiler/compiler.go` → `sourceFileHash` | Computes SHA256(path + content) |
