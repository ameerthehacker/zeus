# Formatter (`zeus fmt`)

`zeus fmt <file>` formats a Zeus source file in place, Prettier-style: it parses
the source to an AST, throws away the original whitespace, and re-prints from an
intermediate **Doc IR** whose printer fits each construct to a print width.

The feature lives in `internal/formatter/` plus `cmd/fmt.go`, and drives only the
lexer and parser (via `lexer.NewLexer` + `parser.ParseProgram`) — no type checking,
IR, or codegen.

Comments are captured by the lexer as a **side channel**: they never enter the
token stream (so the parser is untouched), and are retrieved via `lexer.Comments()`
(`[]*token.Comment`). The formatter re-associates them with AST nodes by source
position.

## Pipeline

```
source → lexer → parser → AST → (formatter.go) Doc IR → (printer.go) string → file
```

## Files

- `doc.go` — the Doc IR: `text`, `concat`, `group`, `indent`, `line`, `softline`,
  `hardline`, `ifBreak`, `join`. `mustBreak` reports whether a doc contains a
  forced break (a `hardline`) so enclosing groups skip flat measurement.
- `printer.go` — the Wadler/Prettier printer. A stack of `(indent, mode, doc)`
  commands is processed LIFO. For each `group`, `fits()` measures whether its flat
  rendering stays within `PrintWidth`; if not (or it `mustBreak`), it renders in
  break mode. Output has trailing whitespace stripped per line.
- `options.go` — `Options{PrintWidth: 80, IndentWidth: 4, UseTabs: false}`.
- `formatter.go` — AST → Doc via `printStmt`/`printExpr` type switches over every
  node kind, plus `formatType` (renders a resolved `zeus_value.ValueType` back to
  source syntax).

## Comments

The lexer records `//` line comments and `/* */` block comments (added in
`eatLineComment`/`eatBlockComment`) as `token.Comment{Text, IsBlock, Span}`,
without emitting tokens — so ASI and the parser are unaffected.

The formatter attaches them by position in `printBody`, the routine that renders
any list of statements or class members. Walking items in source order with a
single advancing comment cursor (`nextComment`), for each item it emits:

- **leading** comments (own line, before the item),
- the item itself (recursing into nested blocks, which consumes their inner
  comments first), then
- **trailing** comments on the item's last line (` // ...`).

Remaining comments before the container's closing brace are emitted as **dangling**
comments. Because recursion consumes inner comments before the outer walk advances,
a single monotonic cursor over position-sorted comments places every comment at the
innermost enclosing container. JSDoc-style block comments (every continuation line
starts with `*`) are re-indented with the `*` aligned; any other multi-line block
comment is emitted verbatim (via `literalLine`) so tables and ASCII art keep their
layout. Blank lines around comments are preserved (one max), same as between
statements.

CRLF line endings are normalized to a single logical newline by the lexer
(`consumeNewLine`), so they neither miscount lines nor inject spurious blank lines.

## Key correctness constraints

- **ASI (automatic semicolon insertion).** The lexer inserts a `;` after a value
  token (identifier, number, `)`, `]`, `++`, `--`, template-end) at end of line.
  So a construct must never be broken such that a value token becomes line-final
  where a `;` would corrupt it. Consequences:
  - Control-flow conditions render inline (`condParen` is *not* a breakable group):
    breaking the closing `)` onto its own line would make the last condition token
    line-final and insert a spurious `;`.
  - Delimited lists (call args, params) break *with a trailing comma*, so the last
    element is followed by `,` (never line-final) — this is ASI-safe.
- **`formatType` cannot use `ValueType.String()` directly.** `BoolType` stringifies
  as `bool` but the keyword is `boolean`; function-type array elements need parens
  (`((i32) => i32)[]`). `formatType` handles these.
- **Anonymous functions render as fat arrows.** The AST stores `() => {}` and an
  anonymous `function() {}` identically (`FunctionDeclExprNode` with `Name == nil`);
  they are rendered back as `(params) => { body }`.
- **`while` conditions.** Unlike `if`/`for`, the while parser treats its parentheses
  as a grouping expression, so `printStmt` strips one grouping layer before
  re-adding canonical parens (`unwrapGrouping`).

## Tests (`test/formatter/`)

- `TestFormatGolden` — per-construct input → expected output.
- `TestFormatComments` — per-position comment cases (leading/trailing/dangling,
  inline + JSDoc block, class members), each also asserting idempotency.
- `TestCommentsKitchenSink` — one comment-heavy program: re-parses, is idempotent,
  and every source comment survives.
- `TestIdempotentAndStable` — over every parseable `test/e2e/specs/**/*.zs`:
  formatted output must re-parse cleanly (parse-stability) and `Format` must be
  idempotent (`Format(Format(x)) == Format(x)`).

As a safety net, `zeus fmt` never overwrites a file whose formatted output fails to
re-parse (`cmd/fmt.go`).

## Limitations

- **`new` of function-type arrays** (`new ((x: i32) => i32)[]`) parses to a lossy
  AST that cannot be reconstructed to round-tripping source; skipped in tests.
- Comments in unusual positions (mid-expression, between parameters) attach to the
  nearest statement/member rather than the exact token — never dropped, but may
  move to an adjacent line.
- Binary expressions and ternaries are not broken across lines.
