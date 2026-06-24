# Zeus IR

Zeus uses a two-level intermediate representation (IR) between the AST and LLVM IR. The higher-level form (HIR) carries rich type information used during type checking. The lower-level form (LIR) is what reaches codegen: no high-level sugar, every operation maps directly to LLVM primitives.

---

## Overview

```
Source → Lexer → Parser → AST
                           ↓
                    Generate Zeus IR  ← HIR generation (ir/ir.go)
                           ↓
                    Type Check IR     ← type inference & validation (ir/tc.go)
                           ↓
                    Lower IR          ← HIR → LIR (ir/lowering.go)
                           ↓
                    Generate LLVM IR  ← codegen (codegen/)
                           ↓
                    Optimize → Emit → Link
```

Both HIR and LIR use the same in-memory representation (`Instr`, `BasicBlock`). The distinction is purely about *which passes have run*:

| Phase | What it is | Used for |
|---|---|---|
| **HIR** | IR immediately after `Generate()` | Type checking — instructions carry full type annotations, high-level constructs like `GET_INDEX` and string operators are still present |
| **LIR** | HIR after `Lower()` | Codegen — all high-level constructs are replaced by primitive sequences, types are fully resolved |

---

## HIR — High-level IR

HIR is produced by `IRModule.Generate(program)` in `internal/ir/ir.go`. It walks the AST using the visitor pattern and emits instructions via the `IRBuilder`.

**Key properties of HIR:**
- String `+`, `==`, `!=`, `<`, etc. are represented as binary op instructions (`ADD`, `EQ_EQ`, …) — the same opcodes used for integers
- Array indexing `arr[i]` emits `GET_INDEX`; the lowering pass converts it to `.get()` later
- Casts for mixed-type arithmetic and implicit type coercions are emitted during type checking (not HIR gen)
- Basic blocks exist only inside function/method bodies; top-level code is a flat instruction list

**Debugging HIR:**

```bash
DEBUG=true zeus build yourfile.zs
```

This dumps the Zeus IR (post-HIR, pre-lowering) for every source file to stdout.

---

## HIR Instruction Reference

Every instruction has the shape:

```go
type Instr struct {
    Id     int         // unique per builder
    Type   InstrType   // opcode
    Output *Var        // nil if instruction produces no value
    Input  InstrInput  // opcode-specific operands
    Span   *token.Span // source location
}
```

### Arithmetic

| Instruction | Input type | Output | Use |
|---|---|---|---|
| `ADD` | `BinaryOpInstrInput{Left, Right Value}` | `*Var` | Addition: integers, floats, **and strings at HIR stage** |
| `SUB` | `BinaryOpInstrInput{Left, Right}` | `*Var` | Subtraction |
| `MUL` | `BinaryOpInstrInput{Left, Right}` | `*Var` | Multiplication |
| `DIV` | `BinaryOpInstrInput{Left, Right}` | `*Var` | Division |
| `MOD` | `BinaryOpInstrInput{Left, Right}` | `*Var` | Modulo (`%`) |
| `POWER` | `BinaryOpInstrInput{Left, Right}` | `*Var` | Exponentiation (`**`); operands must be f64 — HIR gen auto-inserts `CAST` for int operands |
| `NEG` | `UnaryOpInstrInput{Value}` | `*Var` | Numeric negation (`-x`) |

### Comparison

| Instruction | Input type | Output | Use |
|---|---|---|---|
| `EQ_EQ` | `BinaryOpInstrInput{Left, Right}` | `*Var` (bool) | Equality (`==`) — for strings at HIR stage; lowered to `.equals()` in LIR |
| `NOT_EQ` | `BinaryOpInstrInput{Left, Right}` | `*Var` (bool) | Inequality (`!=`) |
| `LESS_THAN` | `BinaryOpInstrInput{Left, Right}` | `*Var` (bool) | `<` |
| `LESS_THAN_EQ` | `BinaryOpInstrInput{Left, Right}` | `*Var` (bool) | `<=` |
| `GREATER_THAN` | `BinaryOpInstrInput{Left, Right}` | `*Var` (bool) | `>` |
| `GREATER_THAN_EQ` | `BinaryOpInstrInput{Left, Right}` | `*Var` (bool) | `>=` |

### Logical / Boolean

| Instruction | Input type | Output | Use |
|---|---|---|---|
| `AND` | `BinaryOpInstrInput{Left, Right}` | `*Var` (bool) | Logical AND (`&&`) |
| `OR` | `BinaryOpInstrInput{Left, Right}` | `*Var` (bool) | Logical OR (`\|\|`) |
| `NOT` | `UnaryOpInstrInput{Value}` | `*Var` (bool) | Logical NOT (`!x`) |

### Memory

| Instruction | Input type | Output | Use |
|---|---|---|---|
| `DECLARE_VAR` | `DeclareVarInstrInput{Variable *Var, Initializer Value, IsConst bool}` | none | Declare a variable; `IsConst=true` for `const` declarations; `Initializer` is nil for uninitialized `let` |
| `LOAD` | `LoadInstrInput{Addr *Var}` | `*Var` (value) | Load the value stored in a pointer variable (`IsPtr=true`) |
| `STORE` | `StoreInstrInput{Addr *Var, Value Value}` | none | Store a value into a pointer variable; emitted for assignments (`x = expr`) |
| `CAST` | `CastInstrInput{Value Value, CastType ValueType}` | `*Var` | Type coercion — emitted by HIR gen for `**` int operands and by the type checker for implicit conversions (e.g. `string → u8[]`) |

### Functions

| Instruction | Input type | Output | Use |
|---|---|---|---|
| `DECLARE_FUNC` | `DeclFuncInstrInput{Function *Function, Body *BasicBlock}` | `*Var` | Declare a named function; `Body` is the entry basic block of the function body |
| `DECLARE_PRIMORDIAL_FUNC` | `DeclPrimordialFuncInstrInput{Function *Function}` | `*Var` | Declare a built-in function (e.g. `print`, `log`); always emitted at the head of every module |
| `CALL_FUNC` | `CallFuncInstrInput{Callee Value, Args []Value}` | `*Var` | Call a directly named function |
| `CALL_INDIRECT_FUNC` | `IndirectFuncCallInstrInput{Function Value, Args []Value}` | `*Var` | Call via function pointer; used for method calls after loading the method from an object |
| `RETURN` | `ReturnInstrInput{Value Value}` | none | Return from a function; `Value` is nil for `void` functions |

### Control Flow

| Instruction | Input type | Output | Use |
|---|---|---|---|
| `JMP` | `JmpInstrInput{Target *BasicBlock}` | none | Unconditional branch |
| `COND_JMP` | `CondJmpInstrInput{TrueTarget, FalseTarget *BasicBlock, Condition Value}` | none | Conditional branch — used for `if`, `while`, `for`, and bounds checks |

### Classes & Objects

| Instruction | Input type | Output | Use |
|---|---|---|---|
| `DECLARE_CLASS` | `DeclClassInstrInput{Class *Class}` | `*Var` | Declare a class (user-defined or primordial); primordial classes have `Class.PrimordialName != ""` |
| `DECLARE_CLASS_METHOD` | `DeclClassMethodInstrInput{Method *Function, Body *BasicBlock, Class *Class}` | `*Var` | Declare a method (or constructor) on a class |
| `NEW_OBJ` | `NewObjInstrInput{Callee Value, Args []Value}` | `*Var` (object) | Instantiate a class (`new Foo(...)`) or create an array (`new i32[]`) |
| `OBJECT_PROPERTY_ACCESS` | `ObjectPropertyAccessInstrInput{Object Value, Property string, IsLValue bool}` | `*Var` (pointer to field) | Access a field or method slot. `IsLValue=true` when the access is for a write (left-hand side of assignment). Pair with `LOAD` to read, or `STORE` to write. |

### Arrays (HIR only)

| Instruction | Input type | Output | Use |
|---|---|---|---|
| `GET_INDEX` | `GetIndexInstrInput{Array Value, Indices []Value}` | `*Var` | Array element access (`arr[i]` or `arr[i][j]`). **HIR only** — the lowering pass replaces every `GET_INDEX` with one or more `.get()` method calls |

### Modules

| Instruction | Input type | Output | Use |
|---|---|---|---|
| `IMPORT` | `ImportInstrInput{ModulePath string, Value Value}` | none | Import a symbol from another module |
| `EXPORT` | `ExportInstrInput{ModulePath string, Value Value}` | none | Export a symbol from the current module |

### Exception Handling

| Instruction | Input type | Output | Use |
|---|---|---|---|
| `THROW` | `ThrowInstrInput{ClassId int, ObjectPtr Value, SourceFile string, SourceLine int}` | none | Throw an exception object. `ClassId` identifies the exception class (0 at HIR stage; resolved by type checker). |
| `PUSH_HANDLER` | `PushHandlerInstrInput{HandlerBlock *BasicBlock, TryBodyBlock *BasicBlock, ClassIds []int}` | none | Register a catch handler at try-block entry; implements `setjmp` semantics |
| `POP_HANDLER` | none | none | Unregister the innermost handler at try-block exit (normal path) |
| `CHECK_EXCEPTION` | `CheckExceptionInstrInput{HandlerBlock, ContinueBlock *BasicBlock}` | none | After a function call inside a try block, branch to the handler if an exception is pending |
| `GET_EXCEPTION` | none | `*Var` (exception object) | Retrieve the currently pending exception in the catch block |
| `CLEAR_EXCEPTION` | none | none | Clear exception state after the catch body completes |

---

## LIR — Low-level IR

LIR is produced by running `Lowerer.Lower()` after type checking. It replaces high-level HIR constructs with sequences of primitive operations that map directly to LLVM.

**Key properties of LIR:**
- No `GET_INDEX` instructions remain — all replaced by `.get()` calls
- String operators (`+`, `==`, `!=`, `<`, `<=`, `>`, `>=`) are replaced by method calls
- Implicit type-coercion `CAST` instructions for string↔u8[] are expanded to explicit construction and copy sequences
- All `*Var` values have their `ValueType` set (done by the type checker before lowering)

### Lowering Rules

| HIR construct | LIR replacement |
|---|---|
| `GET_INDEX arr[i]` | `OBJECT_PROPERTY_ACCESS(arr, "get")` → `LOAD` → `CALL_INDIRECT_FUNC(get, [i])` |
| `GET_INDEX arr[i][j]` (two indices) | Two sequential `.get()` call sequences |
| `ADD(a, b)` where a, b are strings | `OBJECT_PROPERTY_ACCESS(a, "concat")` → `LOAD` → `CALL_INDIRECT_FUNC(concat, [b])` |
| `EQ_EQ(a, b)` where a, b are strings | `OBJECT_PROPERTY_ACCESS(a, "equals")` → `LOAD` → `CALL_INDIRECT_FUNC(equals, [b])` |
| `NOT_EQ(a, b)` where a, b are strings | `.equals()` call sequence + `NOT` |
| `LESS_THAN(a, b)` where a, b are strings | `.compare()` call sequence + `LESS_THAN(result, 0)` |
| `LESS_THAN_EQ(a, b)` where a, b are strings | `.compare()` call + `LESS_THAN_EQ(result, 0)` |
| `GREATER_THAN(a, b)` where a, b are strings | `.compare()` call + `GREATER_THAN(result, 0)` |
| `GREATER_THAN_EQ(a, b)` where a, b are strings | `.compare()` call + `GREATER_THAN_EQ(result, 0)` |
| `CAST(s, u8[])` (string → u8[] coercion) | `OBJECT_PROPERTY_ACCESS(s, "data")` → `LOAD` → `OBJECT_PROPERTY_ACCESS(data, "length")` → `LOAD` → `NEW_OBJ(u8[], [length])` → `CALL_INDIRECT_FUNC(copy, [source])` |
| `CAST(b, string)` (u8[] → string coercion) | `OBJECT_PROPERTY_ACCESS(b, "length")` → `LOAD` → `NEW_OBJ(u8[], [length])` → `.copy()` → `NEW_OBJ(string, [newArray])` |
| `arr.concat(other)` | Factory function call |
| `arr.slice(start, end)` | Factory function call |
| `arr.reverse()` | Factory function call |

---

## Lowering Passes

Lowering is a **pluggable pass architecture** defined in `internal/ir/lowering.go`. Each pass implements the `LowerPass` interface:

```go
type LowerPass interface {
    HandleInstruction(l *Lowerer, instr *Instr)  // called per instruction
    Finalize(l *Lowerer)                          // called after all instructions
    GetName() string
}
```

Passes run in this order (order matters — earlier passes may create instructions that later passes need to process):

| Order | Pass | What it does |
|---|---|---|
| 1 | `ArrayMethodLoweringPass` | Replaces `arr.concat()`, `arr.slice()`, `arr.reverse()` with factory function calls |
| 2 | `IndexLoweringPass` | Replaces every `GET_INDEX` with one or more `.get()` method call sequences |
| 3 | `StringOperatorLoweringPass` | Replaces string binary ops (`+`, `==`, `!=`, `<`, etc.) with method calls (`.concat()`, `.equals()`, `.compare()`) |
| 4 | `StringCastLoweringPass` | Replaces implicit `CAST` instructions for `string↔u8[]` with explicit construction and copy sequences |

`IndexLoweringPass` runs before `StringOperatorLoweringPass` because array accesses may involve string arrays. `StringOperatorLoweringPass` runs before `StringCastLoweringPass` because string operators need to be lowered to method calls before cast lowering can correctly identify string values.

---

## Control Flow

Zeus IR uses **basic blocks** for control flow inside functions. Each `BasicBlock` has:

```go
type BasicBlock struct {
    Id         int
    Instrs     []*Instr
    Successors []*BasicBlock
}
```

Top-level (module-scope) code is a flat instruction list in the `IRBuilder` rather than inside a block.

### How constructs map to blocks

**`if (cond) { then } else { else }`:**

```
entry block:   [condition instructions, COND_JMP(then_block, else_block)]
then_block:    [then body, JMP(merge_block)]
else_block:    [else body, JMP(merge_block)]
merge_block:   [subsequent statements]
```

**`while (cond) { body }`:**

```
entry block:       [init, JMP(condition_block)]
condition_block:   [condition instructions, COND_JMP(body_block, merge_block)]
body_block:        [body instructions, JMP(condition_block)]
merge_block:       [subsequent statements]
```

**`for (init; cond; update) { body }`:**

```
entry block:       [init DECLARE_VAR, JMP(condition_block)]
condition_block:   [condition instructions, COND_JMP(body_block, merge_block)]
body_block:        [body instructions, JMP(update_block)]
update_block:      [update instructions, JMP(condition_block)]
merge_block:       [subsequent statements]
```

**`try { } catch (e: Error) { }`:**

```
entry block:      [PUSH_HANDLER(handler_block, try_body_block, [classIds])]
try_body_block:   [try body, POP_HANDLER, JMP(merge_block)]
handler_block:    [JMP(catch_body_block)]
catch_body_block: [GET_EXCEPTION, DECLARE_VAR e, CLEAR_EXCEPTION, catch body, JMP(merge_block)]
merge_block:      [subsequent statements]
```

---

## Debugging IR

To dump the Zeus IR (between HIR generation and lowering) to stdout:

```bash
DEBUG=true zeus build yourfile.zs
```

Each instruction prints as:

```
%output = OPCODE input1, input2   # with output
OPCODE input1, input2             # without output
%output = OPCODE                  # without input
OPCODE                            # neither
```

Blocks print as `<block_id>:` followed by their instructions.

---

## Adding a New Instruction

1. Add the opcode to the `const (...)` block in `internal/ir/instr.go` and add a case to `InstrType.String()`.
2. Define an `*InstrInput` struct with `String()` and `As*InstrInput()` helpers in `instr.go`.
3. Add a `Build*()` method to `IRBuilder` in `builder.go`.
4. Emit the instruction from the relevant visitor in `ir.go`.
5. Handle the new opcode in the type checker (`tc.go`) if it produces or consumes typed values.
6. If the instruction is high-level (like `GET_INDEX`), add a lowering pass in `lowering.go`.
7. Handle codegen in `codegen/codegen.go`.
8. Add tests in `test/ir/hir_test.go` (and `lir_test.go` if it has a lowering rule).
