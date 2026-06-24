# Parser

The Zeus parser converts a flat stream of tokens (produced by the lexer) into an Abstract Syntax Tree (AST). It reports zero or more `ZeusError` values and — thanks to panic/recover synchronization — keeps parsing even after encountering errors so that multiple problems can be surfaced in a single pass.

---

## Overview

```
Source → Lexer (+ ASI) → []*token.Token → Parser → *ast.ProgramNode + []*zeus_error.ZeusError
```

**Entry points:**

| Method | Use |
|---|---|
| `ParseProgram()` | Parse a complete source file. Returns the root `ProgramNode` and all errors. |
| `ParseStmt()` | Parse a single statement. Used internally and in tests. |
| `ParseExpr()` | Parse a single expression. Used internally and in tests. |

**Key types:**

- `internal/token` — `Token`, `TokenType`, `Span`, `Position`
- `internal/ast` — all AST node types
- `internal/zeus_error` — `ZeusError`, `ErrorSeverity`

---

## Automatic Semicolon Insertion (ASI)

Semicolons are **optional**. The lexer inserts a virtual `TokenTypeSemicolon` into the token stream when a newline (or end-of-file) follows a token that can end a statement. The parser never sees the newline; it only sees the semicolon the lexer injected.

### Trigger tokens

A virtual semicolon is inserted after a newline or EOF when the last emitted token is one of:

| Category | Tokens |
|---|---|
| Identifiers | `identifier` |
| Literals | number, string, char, `true`, `false`, `null` |
| Closers | `)`, `]` |
| Postfix operators | `++`, `--` |

`}` is **not** a trigger. Block-ending constructs (functions, classes, if, while, for, try) do not need a semicolon after their closing brace, so including `}` would insert spurious semicolons before subsequent statements.

### Positioning

The virtual semicolon is given the same span as the `End` position of the token that triggered it. This keeps error messages pointing at the end of the statement rather than at invisible whitespace.

### Conventions

Because `)` is a trigger token, a few style rules apply:

- **`if`/`for` bodies**: the `{` (or any single-statement body) must start on the **same line** as the closing `)`. A newline after `)` inserts `;`, which the parser would then treat as the body.
- **Multi-line expressions**: put the continuation operator at the **end** of the line, not the beginning of the next. `a +\nb` is fine; `a\n+ b` inserts `;` after `a`.
- **Multi-line function call arguments**: put `,` at the end of each argument line. `,` is not a trigger, so the call continues on the next line.

### Examples

```
// No explicit semicolons needed:
let x: i8 = 5
let y: i8 = x + 1

function add(a: i32, b: i32): i32 {
    return a + b
}

// Explicit semicolons still work:
let z: i32 = 0;

// Multi-line call — trailing comma prevents ASI:
foo(
    a,
    b,
)

// This is TWO statements (ASI fires after `)` on line 1):
if (x > 0)
    return x    // ← WRONG: parsed as `if (x > 0);` then `return x;`

// Correct multi-line if:
if (x > 0) {
    return x
}
```

---

## Pratt Parsing

Expressions are parsed with a **Pratt (top-down operator precedence) parser**. The core idea: each token type can have a *prefix parselet* (used when the token starts an expression) and/or an *infix parselet* (used when the token appears between two expressions).

### Operator Precedence

Higher numbers bind more tightly:

| Precedence | Operators |
|---|---|
| 14 | `.` (member access), `[` (indexing) |
| 13 | `(` (function call), `new` |
| 12 | Postfix `++`, `--` |
| 11 | Prefix `!`, unary `-`, prefix `++`, `--` |
| 10 | `**` (power, right-associative) |
| 9 | `*`, `/`, `%` |
| 8 | `+`, `-` |
| 7 | `>`, `>=`, `<`, `<=` |
| 6 | `==`, `!=` |
| 5 | `&&` |
| 4 | `\|\|` |
| 1 | `=`, `+=`, `-=`, `*=`, `/=`, `%=` (all right-associative) |

### Right-Associativity

Assignment operators and `**` are right-associative. The Pratt loop lowers the precedence threshold by 1 for these operators so the right operand grabs them first:

```
2 ** 3 ** 4  →  2 ** (3 ** 4)   // right-associative
a = b = c    →  a = (b = c)     // right-associative
```

### How parselets work

`parseExprOfPrecedence(minPrec, optional, extraInfo...)` drives the loop:

1. Look up a **prefix parselet** for the current token. If none exists and `optional` is false, emit an error.
2. Consume the token and call the parselet to build the left-hand node.
3. Loop: peek at the next token, look up an **infix parselet**, and compare its precedence to `minPrec`. Stop if the next operator's precedence is ≤ `minPrec`.
4. Consume and call the infix parselet, passing the current left node.

---

## Grammar Constructs

### Variable Declaration (`let` / `const`)

```
VarDeclStmt  ::= ('let' | 'const') VarDecl (',' VarDecl)* ';'
VarDecl      ::= identifier ':' DataType ('=' Expr)?
DataType     ::= primitiveType | identifier | DataType '[' ']'
```

Example: `let x: i8 = 5, y: f32 = 1.5;`

At least one `VarDecl` is required. A missing `:` after the identifier produces:
> `expected : after identifier in variable declaration, but found <token>`

### If Statement

```
IfStmt ::= 'if' '(' Expr ')' Stmt ('else' Stmt)?
```

Example: `if (x > 0) { return x; } else { return -x; }`

The condition is not optional — `while` and `for` follow the same pattern. Parentheses around the condition are **required** (unlike JavaScript where many constructs omit them).

### While Statement

```
WhileStmt ::= 'while' Expr Stmt
```

Example: `while x > 0 { x = x - 1; }`

Note: parentheses are **not** required around the condition.

### For Statement

```
ForStmt ::= 'for' '(' ForInit? ';' Expr? ';' Expr? ')' Stmt
ForInit ::= VarDeclStmt | Expr
```

Example: `for (let i: i8 = 0; i < 10; i++) { ... }`

All three clauses (init, condition, update) are optional. The two semicolons are required.

### Block Statement

```
BlockStmt ::= '{' Stmt* '}'
```

Blocks are statements, so `if (x) { }` and `if (x) return;` are both valid.

### Return Statement

```
ReturnStmt ::= 'return' Expr? ';'
```

The expression is optional (`return;` is valid for void functions).

### Function Declaration

```
FunctionDecl ::= 'function' identifier '(' ParamList ')' ':' DataType BlockStmt
ParamList    ::= (VarDecl (',' VarDecl)*)?
```

Example: `function add(x: i32, y: i32): i32 { return x + y; }`

Functions are parsed as **expressions** (they are `FunctionDeclExprNode`), so they can be used anywhere an expression is valid. When used at statement level they are wrapped in `ExprStmtNode`.

### Class Declaration

```
ClassDecl  ::= 'class' identifier ('extends' identifier)? '{' ClassMember* '}'
ClassMember ::= AccessModifier? (MethodDecl | PropertyDecl)
MethodDecl  ::= identifier '(' ParamList ')' ':' DataType BlockStmt
PropertyDecl ::= identifier ':' DataType ';'
AccessModifier ::= 'public' | 'private' | 'protected'
```

Example:
```
class Point extends Shape {
  public x: i32;
  public y: i32;
  constructor(): void { }
  distance(other: Point): f64 { ... }
}
```

The `constructor` method may omit the return type colon (it is always `void`).

### Import / Export

```
ImportStmt ::= 'import' '{' (identifier (',' identifier)*)? '}' 'from' string ';'
ExportStmt ::= 'export' (FunctionDecl | ClassDecl)
```

Export is restricted to functions and classes; exporting variables or literals is an error.

### Try / Catch

```
TryCatchStmt ::= 'try' BlockStmt CatchClause+
CatchClause  ::= 'catch' '(' identifier ':' DataType ')' BlockStmt
```

At least one `catch` clause is required. Multiple catch clauses are allowed for catching different error types.

### Throw Statement

```
ThrowStmt ::= 'throw' Expr ';'
```

---

## Error Reporting

### Error creation helpers

| Helper | When to use |
|---|---|
| `p.expectedError(expected, extraInfo...)` | Called at EOF: formats `"expected X Y"` |
| `p.expectedButGotError(expected, token, extraInfo...)` | Called mid-stream: formats `"expected X Y, but found Z"`. Delegates to `expectedError` when at EOF. |
| `p.pushError(err)` | Final sink. Deduplicates errors on the same source line, then **panics** with the error to unwind the call stack. |

### Context strings

Every `consumeToken` call should include a human-readable `extraInfo` argument that names the surrounding construct:

```go
p.consumeToken(token.TokenTypeLeftParen, "after if")
p.consumeToken(token.TokenTypeColon, fmt.Sprintf("after identifier in %s", cxt))
p.consumeToken(token.TokenTypeLeftBrace, "to begin block")
```

Without a context string the message is just `"expected {"` — correct but unhelpful. With it: `"expected { to begin block"`.

### Panic as control flow

Error recovery uses Go's `panic` / `recover` to jump out of deeply nested parse functions without requiring every function to return `(node, error)`:

```
pushError()  →  panic(ZeusError)
    ↑
    caught by handlePanic() which is deferred in ParseStmt()
    ↓
synchronize() — advances past erroneous tokens
```

This pattern allows any helper deep in the call stack (e.g., `consumeToken` inside `parseVarDecl` inside `parseFunctionSignatureAndBody`) to abort immediately without bubbling through intermediate return values.

Non-`ZeusError` panics (e.g., nil pointer dereferences) are **re-panicked** by `handlePanic` so genuine bugs are not silently swallowed.

---

## Synchronization & Recovery

After an error is recorded and the panic unwinds to `ParseStmt`, `synchronize()` advances the token cursor to a position where parsing can safely resume.

### Algorithm

```go
func (p *Parser) synchronize() {
    // Infinite-loop guard: if stuck on the same token, consume it and return.
    if p.lastSyncPos != nil && p.lastSyncPos == &p.peek().Span.Start {
        p.consume()
        return
    }

    // Consume tokens that are not "structural".
    for canConsume(p.peek()) { p.consume() }

    // Consume semicolons and 'else' (stop *after* them).
    if stopAfterTokens[p.peek().Type] { p.consume() }

    p.lastSyncPos = &p.peek().Span.Start
}
```

**`stopAtTokens`** — stop *before* consuming:
`let`, `const`, `function`, `(`, `)`, `{`, `}`, `if`, `try`, `catch`, `throw`

**`stopAfterTokens`** — stop *after* consuming:
`;`, `else`

The rationale: stopping before keywords and braces lets the parser attempt to parse the next statement from a clean boundary. Consuming semicolons (both explicit and ASI-inserted) avoids re-entering a half-parsed statement.

### Same-line error deduplication

`pushError` checks whether the last recorded error is on the same source line as the new error. If so, the new error is **silently dropped** (but the panic still fires for recovery). This prevents one missing token from cascading into five confusing follow-on errors.

### Nil body guards

When `ParseStmt()` is called to parse a loop or conditional body and it returns `nil` (because the body itself encountered an error), the parent parse function returns `nil` rather than dereferencing a nil pointer. This applies to `parseIfStmt`, `parseWhileStmt`, and `parseForStmt`.

---

## Adding a New Construct

Follow these steps to add, say, a `repeat { } until (Expr)` statement:

### 1. Add token types (if needed)

In `internal/token/token.go`, add `TokenTypeRepeat` and `TokenTypeUntil`. Make sure the lexer recognises the keywords.

### 2. Write the parse function

```go
func (p *Parser) parseRepeatStmt() *ast.RepeatStmtNode {
    repeatKeyword := p.consumeToken(token.TokenTypeRepeat)
    body := p.parseBlockStmt()
    p.consumeToken(token.TokenTypeUntil, "after repeat body")
    p.consumeToken(token.TokenTypeLeftParen, "after until")
    condition := p.parseExprOfPrecedence(0, false, "in until condition")
    p.consumeToken(token.TokenTypeRightParen, "after until condition")
    p.consumeSemicolon()  // accepts both an explicit ';' and an ASI-inserted one
    span := &token.Span{Start: repeatKeyword.Span.Start, End: p.tokens[p.current-1].Span.End}
    return &ast.RepeatStmtNode{Body: body, Condition: condition, Span: span}
}
```

Always add a human-readable context string to every `consumeToken` call.

### 3. Register in `ParseStmt`

```go
case token.TokenTypeRepeat:
    return p.parseRepeatStmt()
```

### 4. Add the AST node

In `internal/ast/`, add `RepeatStmtNode` with `GetSpan()` and `PrettyString()` methods.

### 5. Write tests

**Positive test** — verify the correct AST is produced for valid input.

**Negative tests** — one test per required token:
- Missing `{` → "expected { to begin block"
- Missing `until` → "expected until after repeat body, but found …"
- Missing `(` → "expected ( after until, but found …"
- Missing `)` → "expected ) after until condition, but found …"

**Synchronization test** — verify a bad `repeat` statement is recovered from and the next statement is still parsed:
```go
input: "repeat { } until;\nlet x: i8 = 1;",
errorCount: 1,
stmtCount: 1,
```
