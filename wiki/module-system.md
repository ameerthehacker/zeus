# Module System

Zeus uses a file-based module system similar to JavaScript/TypeScript. The import path is the
module identity — there are no separate package declarations.

---

## Syntax

```zeus
// Named imports
import { add, multiply } from "./math";
import { Point } from "./shapes/point";

// Stdlib (@ prefix resolves from ZEUS_HOME/lib/)
import { abs, max } from "@std/math";

// Export a function
export function add(a: i32, b: i32): i32 {
    return a + b;
}

// Export a class
export class Point {
    public x: i32;
    public y: i32;
    constructor(x: i32, y: i32) { this.x = x; this.y = y; }
    public getSum(): i32 { return this.x + this.y; }
}
```

Only functions and classes can be exported. Variables cannot be exported.

---

## Path Resolution (`internal/module/module.go`)

`ResolveFilePath(sourcePath, importPath)` is the single entry point. It is called with the
**absolute path of the importing file** and the raw import string from source.

| Import form | Resolution |
|---|---|
| `"./math"` | `<importing_file_dir>/math.zs` (auto-appends `.zs` if needed) |
| `"./utils"` | `<importing_file_dir>/utils/index.zs` (directory → index.zs) |
| `"./math.zs"` | `<importing_file_dir>/math.zs` (explicit extension also works) |
| `"@std/math"` | `$ZEUS_HOME/lib/std/math/index.zs` |

**ZEUS_HOME** is an env var that points to the zeus installation root (e.g. the repo root in
development, or the Homebrew prefix in production). `lib/` lives directly inside it.

The `.zs` auto-append logic: if `os.Stat(resolvedPath)` fails, try `resolvedPath + ".zs"`. If
that exists, use it. This is what lets you write `"./math"` instead of `"./math.zs"`.

---

## Compilation Pipeline

Multi-file compilation is orchestrated in `internal/zeus_compiler/compiler.go`.

### 1. Dependency collection — `CollectDependencies`

BFS from the entry file. For each file:
1. Lex + parse it (`CompileFile`)
2. Walk its AST import statements (`GetDependencies`) — resolve each path, `ReadSourceFile`
3. Detect circular deps by tracking `inProgress` (set when a file starts processing, never
   cleared — so if a dep's dep sees the original file still in-progress, it's circular)
4. Prepend the current file to `sourceFiles` and enqueue its dependencies

Result: `sourceFiles` slice where **dependencies always appear before the files that import
them** (deepest deps first). This ordering is critical for IR generation.

### 2. Zeus IR generation — `GenerateZeusIR`

Iterates `sourceFiles` in order (deps first). For each file:
- Creates a fresh `IRBuilder` and `IRModule`
- Stores the `IRModule` in `irModuleFilePathMap[path]`
- Calls `irModule.Generate(program)`

When `VisitImportStmt` runs:
1. Resolves the import path to an absolute path
2. Looks up `irModuleFilePathMap[absolutePath]` (guaranteed to exist because deps were processed first)
3. Calls `irModule.GetExportedSymbol(name)` on the dependency's IRModule
4. Emits `BuildImport(modulePath, name, value)` — adds an IMPORT IR instruction and declares
   the imported value in the **global** symbol table of the current IRBuilder

When `VisitExportStmt` runs:
1. Visits the expression (usually a `FunctionDeclExpr` or `ClassDeclExpr`)
2. Stores the result in `g.exportedSymbols[name]` so other modules can look it up
3. Emits `BuildExport(modulePath, value)` — adds an EXPORT IR instruction

### 3. Type checking, IR lowering

Standard passes, both import and export instructions are handled (type-check verifies only
functions and classes can be imported/exported).

### 4. LLVM codegen — `GenerateLLVMIR`

Each `SourceFile` gets its own `CodegenModule` with its own LLVM context and module. They are
compiled independently and then linked.

#### Export (`genExport`)

For a **function**: rename the LLVM function to the module-scoped name and set `ExternalLinkage`.
```
"add"  →  "$_Users_alice_project_math_zs_add"   (ExternalLinkage)
```

For a **class**: rename the object-header global to the module-scoped name. The constructor
method is also renamed. The vtable and factory function keep their original names (they are not
directly referenced by name from importing modules).

#### Import (`genImport`)

For a **function**: declare an external LLVM function with the module-scoped name. Register it in
the current module's symbol table under the **original** name so call sites just use `add`.

For a **class**: call `genImportedClass` which:
1. Creates struct types (vtable, object header, main struct) — all with correct layout because
   the `zeus_value.Class` metadata travels with the IR
2. Declares the object-header global as external (module-scoped name)
3. Declares the constructor as external (module-scoped name)
4. Calls `declareFactoryFunction(class)` — declares `zeus_new_ClassName` as external so
   `new ImportedClass()` can be used in the importing module

#### Module-scoped naming (`module.GetModuleScopedName`)

Every non-alphanumeric character in the absolute file path becomes `_`, prefixed with `$`:
```
/home/alice/project/math.zs  →  "$_home_alice_project_math_zs"
"$_home_alice_project_math_zs" + "_" + "add"  →  "$_home_alice_project_math_zs_add"
```

This guarantees globally unique LLVM symbol names for every export regardless of how many
modules are linked.

### 5. Optimization (mem2reg), object file emission, linking

Each module is optimized and emitted as a `.o` file independently, then linked together with
`clang` (macOS) or `gcc` (Linux) plus the Zig runtime.

---

## Primordial Classes in Multi-Module Compilation

This is the subtlest part. Every compilation unit independently generates the primordial class
infrastructure (string, u8[], Error): struct types, vtable globals, typeinfo globals, object
headers, factory functions, and method wrappers.

**Linkage strategy to avoid duplicate-symbol errors:**

| Symbol | Linkage | Rationale |
|---|---|---|
| Primordial globals (vtable, header, typeinfo) | `InternalLinkage` | Each TU has its own identical copy; runtime uses class IDs (integers), not pointer identity |
| Primordial factory functions (`zeus_new_string` etc.) | `LinkOnceODRLinkage` | Must be externally visible (Zig runtime calls them); linker deduplicates identical definitions |
| Primordial method wrappers (`string.concat` etc.) | `InternalLinkage` | Called only via vtable pointer; each TU has its own thin wrapper calling the Zig runtime |
| User-defined class globals | `ExternalLinkage` | Need to be reachable cross-module after export renaming |
| User-defined class methods | `InternalLinkage` | Always dispatched via vtable, never called by symbol name across modules |
| User-defined class factory (`zeus_new_MyClass`) | `ExternalLinkage` | Importing modules resolve `new MyClass()` to this symbol at link time |

**Primordial class ordering:** `GetAllClasses()` in `internal/zeus_value/primordial_registry.go`
must return classes in dependency order (u8[] before string before Error) because codegen
processes them sequentially and Error's constructor references string. The registry now uses an
explicit `classOrder` slice (insertion-ordered) instead of iterating a Go map (random order).
The Error class has a reserved ID of 1 which is smaller than the auto-incrementing IDs (101+),
so sorting by ID would place Error first — wrong. The insertion-order slice is the correct fix.

---

## ZEUS_HOME

`ZEUS_HOME` always means the **zeus installation root** — the directory that contains `lib/`
and `runtime/`. `GetRuntimeDir()` always appends `runtime/zig-out/out` to it.

```
$ZEUS_HOME/
  lib/
    std/
      math/
        index.zs      ← @std/math resolves here
  runtime/
    zig-out/
      out/
        zeus-runtime.o
```

In development: `ZEUS_HOME=$(pwd)` (the repo root).
In a Homebrew install: auto-detected from executable path — goes up from `bin/zeus` to the
Homebrew prefix, which has the same layout.

---

## Stdlib (`lib/std/`)

Currently only `math`:
- `abs(n: i32): i32`
- `max(a: i32, b: i32): i32`
- `min(a: i32, b: i32): i32`
- `clamp(value: i32, low: i32, high: i32): i32`

All implemented in pure Zeus — no runtime support needed.

---

## Known Limitations

- **No wildcard imports** — `import * as math from "./math"` is not supported
- **No re-exports** — `export { add } from "./math"` is not supported
- **No variable exports** — only functions and classes can be exported
- **No import aliases** — `import { add as sum }` is not supported
- **No package-private visibility** — symbols are either local to one file or exported to everyone; there is no middle ground for "visible within a logical module across multiple files"
- **Class name uniqueness across modules** — if two modules export classes with the same name (`class Point`), their factory functions (`zeus_new_Point`) will conflict at link time. Module-scoping the factory function name is the fix but not yet implemented.

---

## E2E Tests

All module tests live in `test/e2e/specs/module/`. The runner (`test/e2e/e2e_test.go`)
supports a `compile_error` field in spec.json for tests that expect the compiler to fail.

| Test | What it exercises |
|---|---|
| `func-main.zs` | Single function import |
| `func-multi-main.zs` | Multiple imports from same module |
| `class-main.zs` | Class import, `new`, property access |
| `class-method-main.zs` | Class import, constructor args, vtable method call |
| `folder-main.zs` | Directory import (`./utils` → `utils/index.zs`) |
| `stdlib-main.zs` | `@std/math` stdlib import |
| `bad-import.zs` | Compile error: symbol not exported |
| `missing-module.zs` | Compile error: file not found |
| `circular-main.zs` | Compile error: circular dependency |

---
