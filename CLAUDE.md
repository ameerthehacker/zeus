# Zeus Compiler — Developer Reference

## Quick orientation

Zeus is a compiled language that emits native binaries via LLVM. The pipeline is:

```
.zs source  →  Lexer  →  Parser  →  IR gen  →  Type check  →  Lowering  →  Codegen  →  LLVM  →  binary
```

The runtime is written in Zig (`runtime/`). Build tags: `-tags llvm19` are required for all `go` commands.

## Repo layout

```
cmd/                  CLI (cobra commands: build, fmt, lsp, ...)
internal/
  token/              TokenType enum + Span/Position types
  lexer/              Lexer — produces []Token from source text; comments captured off-stream via Comments()
  ast/                AST nodes + ExprVisitor[T] interface
  parser/             Pratt parser (precedence climbing)
  ir/
    ir.go             IR generator — walks AST, emits HIR via IRBuilder
    builder.go        IRBuilder — creates BasicBlocks and Instr lists
    instr.go          InstrType enum, VarDecl, input/output structs
    tc.go             Type-checker driver (pass list, context management, shared utilities)
    tc_type_check.go  TypeCheckingPass — type validation, implicit casts, safety-net resolution
    tc_unused.go      UnusedWarningPass
    tc_undefined.go   UndefinedTypeCheckPass
    type_inference.go TypeInferencePass — NEW_OBJ + OBJECT_PROPERTY_ACCESS bidirectional sync
    ir_type_infer.go  inferFunctionEnv — AST pre-scan for local var types + return type inference
    ir_passes.go      DeclCheckPass (pre-IR stub registration) + other pre-IR passes
    lowering.go       HIR → LIR lowering (e.g. array indexing → method calls)
  codegen/
    codegen.go        LIR → LLVM IR via tinygo.org/x/go-llvm
    llvm_type.go      Zeus type → LLVM type conversion
  zeus_value/         Value and ValueType interfaces, concrete types (IntType, FloatType, ...)
  zeus_compiler/      Orchestrates all passes in order
  formatter/          `zeus fmt` — Prettier-style formatter (doc.go IR, printer.go, formatter.go). See wiki/formatter.md
  lsp/                Language-server protocol implementation
test/
  e2e/                End-to-end tests; compiled via the zeus binary
  parser/             Parser unit tests (AST structure)
  formatter/          Formatter golden + idempotency/parse-stability tests
  ir/                 HIR/LIR blackbox tests
  lexer/              Lexer unit tests
```

## How to run tests

```bash
# All tests (requires LLVM headers on PATH)
go test -tags llvm19 ./test/...

# Just e2e (builds the zeus binary first)
go test -tags llvm19 ./test/e2e/... -v -count=1

# Parser/lexer/IR tests (no LLVM needed)
go test ./test/parser/... ./test/lexer/... ./test/ir/...

# Compile a playground file (needs cmake build + zig runtime)
make compile file=hello
make run file=hello
```

## Adding a new operator — complete checklist

### Step 1 — Token (`internal/token/token.go`)

Add the new `TokenType` constant inside the `operator_beg`/`operator_end` sentinels and add its `String()` case.

### Step 2 — Lexer (`internal/lexer/lexer.go`)

Add a `case char == 'X':` branch in the main `Lex()` switch. For multi-character tokens, use `l.matchNextRune('Y')` to peek ahead. Emit with `l.pushToken(token.NewToken(...))`.

### Step 3 — Parser (`internal/parser/parser.go`)

**Precedence table** — add the new token to `BinaryOperatorPrecedence` with the right integer:

| Prec | Operators |
|------|-----------|
| 20 | `.` `[]` member access / indexing |
| 19 | `()` function call, `new` |
| 18 | postfix `++` `--` |
| 17 | unary `-` `!` `~` prefix `++` `--` (`UnaryOperatorPrecedence`) |
| 16 | `**` right-assoc |
| 15 | `*` `/` `%` |
| 14 | `+` `-` |
| 13 | `<<` `>>` |
| 12 | `<` `<=` `>` `>=` |
| 11 | `==` `!=` |
| 10 | `&` bitwise AND |
| 9 | `^` bitwise XOR |
| 8 | `\|` bitwise OR |
| 7 | `&&` logical AND |
| 6 | `\|\|` logical OR |
| 3 | `?:` ternary |
| 1 | `=` `+=` `-=` `*=` `/=` `%=` `**=` assignments |

**Right-associative** — add to `RightAssociativeOperators` if needed (assignments, `**` are right-assoc).

**Parselet** — add to `infixParselets` (binary) or `prefixParselets` (unary). Most binary operators use the shared `binaryOperatorParseLet`. Special operators (ternary, function call, indexing) get their own closure.

### Step 4 — AST node (only for non-binary operators)

If the operator requires a new AST shape (e.g. `TernaryExprNode` with three children), add the node in `internal/ast/expr.go` and add the corresponding `VisitXxx(*XxxNode) T` method to the `ExprVisitor[T]` interface. There are two implementations to update: `*IRModule` in `ir.go` (full IR emission) and `astWalker` in `ir/closure.go` (closure analysis — add a stub that walks any sub-expressions via `w.walkExpr`).

### Step 5 — IR instruction (`internal/ir/instr.go`)

Add a new `InstrTypeXxx` constant in the `const (...)` block and a `String()` case.

### Step 6 — IR generation (`internal/ir/ir.go`)

Add a case in `VisitBinaryExpr` (or `VisitUnaryExpr`, or a new `VisitTernaryExpr` method). Use `g.irBuilder.BuildBinaryOp` / `BuildUnaryOp`. For control-flow operators (short-circuit, ternary), use `BuildSuccessorBlock`, `BuildCondJmp`, `BuildJmp`, `BuildStore`, `BuildLoad` — see `emitShortCircuitAnd` as the pattern.

**Power operator special case**: both operands must be cast to f64 before the LLVM `pow` intrinsic. Use the `buildPowerOp` helper.

### Step 7 — Type checker (`internal/ir/tc.go` and `tc_type_check.go`)

Add cases in the two passes that switch on `InstrType`:

1. **`TypeCheckingPass.HandleInstruction`** (`tc_type_check.go`) — add the actual type-checking logic (`tcBinaryOp` / `tcUnaryOp`).
2. **`UnusedWarningPass.HandleInstruction`** (`tc_unused.go`) — add to the appropriate `handleBinaryOp` / `handleUnaryOp` case list.

There is no `ToKnownTypesPass` — it was removed. Types are resolved inline during IR generation via `resolveTypeForIRGen`. Ternary result vars with `nil` type are handled by `TypeCheckingPass.tcStore` inferring from the first store. See `wiki/type-inference.md` for the full architecture.

### Step 8 — Codegen (`internal/codegen/codegen.go`)

Add a `case ir.InstrTypeXxx:` in the giant `switch instr.Type` inside `Generate`. For binary/unary ops, the fallthrough chains to `genBinaryOp` / `genUnaryOp`. Add the new case inside those functions too.

Also add to the `switch instr.Type` in `genBinaryOp` / `genUnaryOp`. Bitwise ops use `genLLVMBinaryOp(left, right, name, signedFn, unsignedFn, nil)` (nil for the float callback = integer-only).

### Step 9 — E2E test

Create `test/e2e/specs/<group>/<name>.zs` and add an entry to `test/e2e/specs/<group>/spec.json`. The spec format:

```json
{ "name": "Human readable name", "exit": "0", "entry": "filename.zs" }
```

`"exit": "0"` means the program must exit with code 0. Use non-zero returns as error sentinels inside the Zeus program.

## IR builder patterns

### Short-circuit / ternary control flow

```go
// Save parent block; create successor blocks; set insertion to each block in order
parentBlock := g.irBuilder.GetInsertionBlock()
thenBlock := g.irBuilder.BuildSuccessorBlock()   // added to parentBlock.Successors
elseBlock := g.irBuilder.BuildSuccessorBlock()   // added to parentBlock.Successors

// Emit then-block code first (to learn the result type if needed)
g.irBuilder.SetInsertionBlock(thenBlock)
thenValue := expr.Then.Accept(g)

// Return to parent block; declare result var; emit condJmp
g.irBuilder.SetInsertionBlock(parentBlock)
resultVar := g.irBuilder.BuildVarDecl(NewVarDecl("result", nil, false, nil, span))
g.irBuilder.BuildCondJmp(thenBlock, elseBlock, condition, span)

// Finish then-block: store + jmp to merge
g.irBuilder.SetInsertionBlock(thenBlock)
mergeBlock := g.irBuilder.BuildSuccessorBlock()   // added to thenBlock.Successors
g.irBuilder.BuildStore(resultVar, thenValue, span)
g.irBuilder.BuildJmp(mergeBlock, span)

// Else-block
g.irBuilder.SetInsertionBlock(elseBlock)
elseValue := expr.Else.Accept(g)
g.irBuilder.BuildStore(resultVar, elseValue, span)
g.irBuilder.BuildJmp(mergeBlock, span)

g.irBuilder.SetInsertionBlock(mergeBlock)
return g.irBuilder.BuildLoad(resultVar, span)
```

When `resultVar` is declared with `nil` type, `TypeCheckingPass.tcStore` infers the type from the first store.

### Compound assignments

`isCompoundAssignment()` and `getCompoundAssignmentOp()` in `ir.go` map `+=` etc. to `InstrTypeAdd` etc. The power compound assignment (`**=`) goes through `buildPowerOp` to handle the required f64 casting.

## Key gotchas

- **Hex/binary/octal literals**: `eatNumber` in the lexer uses `!isRadix10` to allow `a-f` digits. `strconv.ParseInt` calls in `zeus_value/value.go` and `codegen/llvm_type.go` use base `0` (auto-detect) to handle `0xFF`, `0b101`, `0o17`.
- **Power operator**: LLVM `pow` only accepts f64 operands. Always cast via `buildPowerOp` — it casts inputs to f64, computes power, and casts back.
- **Float→Int cast**: `genCast` in codegen uses `CreateFPToSI` / `CreateFPToUI` for float-to-int (not `CreateFPExt` which is float-to-wider-float).
- **Logical operators are short-circuit**: `&&` and `||` emit conditional jumps, not binary IR instructions directly. They're handled in `VisitBinaryExpr` before the right operand is evaluated.
- **Class methods**: `ir.go` does NOT push class methods to the symbol table (they're scoped to the class).
- **`ExprVisitor[T]` has two implementations**: `*IRModule` in `ir.go` (full IR emission) and `astWalker` in `ir/closure.go` (closure analysis — add a stub that walks any sub-expressions via `w.walkExpr`). Adding a new AST node requires updating both.
- **Type checker pass order**: `TypeInferencePass` → `TypeCheckingPass` → `UnusedWarningPass` → `UndefinedTypeCheckPass`. Types are resolved inline during IR gen (`resolveTypeForIRGen`); the type checker only validates. See `wiki/type-inference.md` for the full architecture.
- **Self-referential class properties** (`Node.next: Node`): resolved by stub back-fill at the end of `VisitClassDeclExpr` — after registering the full class, the DeclCheckPass stub's `*Var` objects are updated in-place so all `ObjectType{stub_copy}` instances see the resolved type through shared pointers.
- **User-defined array element types** (`new Point[]`): `VisitNewExpr` resolves the base element type via `resolveTypeForIRGen` before building the `ArrayType`. This ensures `getOrCreateArrayClass` creates the array class with concrete push/pop/get/set method types.
