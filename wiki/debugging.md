# Debugging the Zeus Compiler

Zeus provides a built-in IR dump facility to inspect every layer of the compilation pipeline for a given source file.

---

## `--internal-emit-ir`

Pass this flag to `zeus build` to emit three IR files per source file:

```bash
zeus build main.zs --internal-emit-ir
```

| File extension | Contents | Captured when |
|---|---|---|
| `.zhir` | High-level Zeus IR (HIR) | After type checking, **before** lowering |
| `.zlir` | Low-level Zeus IR (LIR) | After lowering, **before** LLVM codegen |
| `.ll` | LLVM IR (textual) | After codegen, pre-optimization in debug / pre-link in release |

---

## Output directory layout

IR files are written to `target/{mode}/ir/` and mirror the source tree structure.

```
<target-dir>/
  target/
    debug/
      obj/    ← object files (unchanged)
      bin/    ← compiled binary (unchanged)
      ir/
        main.zhir
        main.zlir
        main.ll
```

If your project has imports (e.g. `math/math.zs`), the folder structure is preserved:

```
      ir/
        main.zhir
        main.zlir
        main.ll
        math/
          math.zhir
          math.zlir
          math.ll
```

`--target-dir` shifts the root of the `target/` tree, so IR files move with it:

```bash
zeus build main.zs --internal-emit-ir --target-dir /workspace
# IR → /workspace/target/debug/ir/main.zhir, ...
```

---

## File format

Both `.zhir` and `.zlir` use the same textual Zeus IR format. Each file begins with a two-line comment header followed by the instruction listing:

```
; Zeus HIR
; source: /home/alice/myproject/main.zs

DECLARE_PRIMORDIAL_FUNC print (string) → void
DECLARE_FUNC main () → i32
0:
%1 = DECLARE_VAR i32 x = 42
%2 = ADD %1, 1
RETURN %2
```

**Header lines** (`;`-prefixed) are comments. A future parser can skip them and treat the rest as the program.

**Block headers** (`N:`) mark the start of a basic block. Instructions inside a block follow immediately after the header line.

**Instruction format:**

```
%output = OPCODE operand1, operand2   ; produces a value
OPCODE operand1, operand2             ; side-effect only
%output = OPCODE                      ; no input
OPCODE                                ; neither
```

### HIR vs LIR differences

The key differences to look for between `.zhir` and `.zlir`:

| Construct | HIR (`.zhir`) | LIR (`.zlir`) |
|---|---|---|
| Array indexing `arr[i]` | `GET_INDEX arr, i` | `OBJECT_PROPERTY_ACCESS arr, "get"` → `CALL_INDIRECT_FUNC` |
| String `+` | `ADD s1, s2` | `OBJECT_PROPERTY_ACCESS s1, "concat"` → `CALL_INDIRECT_FUNC` |
| String `==` | `EQ_EQ s1, s2` | `OBJECT_PROPERTY_ACCESS s1, "equals"` → `CALL_INDIRECT_FUNC` |
| String comparisons | `LESS_THAN` / `GREATER_THAN` etc. | `.compare()` call + integer comparison |
| `string → u8[]` cast | `CAST s, u8[]` | Explicit copy sequence |

---

## Clearing the IR cache

IR files are not automatically removed when source changes. To get a fresh dump, either delete the `ir/` folder or the entire `target/` tree:

```bash
rm -rf target/debug/ir/
# or
rm -rf target/
```

---

## Implementation reference

| Location | Responsibility |
|---|---|
| `cmd/build.go` → `FlagEmitIR` | CLI flag registration; checks `ZEUS_DEBUG` env var |
| `internal/zeus_compiler/compiler.go` → `EnableIREmission` | Configures `irDir` and `cwd` on the `Compiler` |
| `internal/zeus_compiler/compiler.go` → `Check` | Snapshots HIR and LIR text at the right pipeline stages |
| `internal/zeus_compiler/compiler.go` → `emitIRFiles` | Writes `.zhir`, `.zlir`, `.ll` preserving folder structure |
| `internal/zeus_compiler/compiler.go` → `formatZeusIR` | Adds file header; delegates to `IRBuilder.String()` |
| `internal/ir/builder.go` → `IRBuilder.String()` | Walks all instructions and blocks; returns textual IR |
