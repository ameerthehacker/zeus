# LSP Roadmap — compiler gaps that cap language-server quality

The Zeus language server (`internal/lsp/`) is production-usable **today**: all features work for
the common single-file case, and it is crash-safe (root-cause fixes + a top-level recover net).
The items below are **enhancements that raise the quality ceiling**, not bugs — each is gated
behind a compiler-side gap and only shows up at the edges. They are ranked by LSP impact so this
can be picked up later without re-deriving the analysis.

The through-line: a purpose-built **LSP semantic model** derived from the type-checked AST is the
single highest-leverage investment. The compiler-side changes (gaps 3 and 5) are secondary.

---

## Foundational gaps

### 1. Flat, name-keyed symbol table

`internal/symbol_table/symbol_table.go` `Walk` and `ir.IRModule.GetAllSymbols` flatten every
retained scope into a single `name → value` map.

- **Symptoms:** completion surfaces symbols from unrelated scopes; `this.` / `super.` can't
  resolve the enclosing class; hover / go-to-definition can pick the wrong same-named symbol;
  inlay hints silently drop a shadowed or unused local (observed with `n` in the LSP
  `sampleProgram` test — the symbol was absent from the walked tables).
- **Fix:** a scope tree addressable by source span, plus a query API such as
  `SymbolsInScopeAt(pos)` and `EnclosingClassAt(pos)`.

### 2. No position→AST-node index; types not on AST nodes

The compiler keeps `SourceFile.Program`, but resolved types live on IR values
(`zeus_value.Var`, looked up by name); AST nodes carry no resolved type, and there is no
`NodeAt(pos)` lookup.

- **Symptoms:** LSP features are text/token heuristics — `parseMemberContext`,
  `signatureContext`, `wordAt`, and import-context detection in `internal/lsp/analysis.go`,
  `navigation.go`, and `imports.go`. Complex receivers like `getFoo().bar` can't complete, and
  hover on sub-expressions is approximate.
- **Fix:** retain the AST in the LSP; add a position→node lookup; associate resolved types with
  AST nodes (bridge IR values ↔ AST spans, or annotate nodes during type checking).

### 3. Panic-based front-end error model

`parser.pushError` panics with a `*ZeusError`; there are many `Assert` / `panic` sites in `ir`
and `zeus_value`; and `zeus_compiler.TrapCompilerException` re-panics.

- **Symptoms:** the LSP must wrap every request in a catch-all `recover`; roughly 18 root-cause
  crashes had to be fixed reactively (guarded by the `FuzzLSP` harness and the regression corpus
  under `internal/lsp/testdata/`, plus the curated `brokenSeeds` in `internal/lsp/fuzz_test.go`).
- **Fix:** a tolerant front-end mode that accumulates errors and never panics. This also makes
  `zeus build` itself crash-free on malformed input.

---

## Secondary gaps

### 4. Coarse parser error recovery

`pushError` drops any error on the same line as the previous one, and `synchronize` bails to the
next statement boundary — so the LSP shows fewer diagnostics than the code actually has.

- **Fix:** finer recovery and multiple diagnostics per construct (at least on the LSP path).

### 5. No reusable compile-file service; disk-only module resolution

The LSP reimplements a slim dependency pipeline in `Server.makeModuleResolver`
(`internal/lsp/pipeline.go`), reads imported files from disk (so unsaved edits to an imported
file aren't reflected), and re-parses imports on every keystroke.

- **Fix:** factor a shared "compile file → `IRModule` (with resolver)" service used by both the
  CLI and the LSP; add a filesystem overlay so the LSP can feed in-memory buffers of open files;
  cache resolved modules across edits.

### 6. Incomplete / synthetic spans; exports keyed by IR name

Primordials get `{1,1}` spans, some `Var`s have `nil` spans (guarded in the LSP), and
`VisitExportStmt` keys exports by `.Name` (the IR-uniquified name).

- **Fix:** an accurate source span on every node and IR value; key exports by source name.

### 7. No occurrence / reference index

Nothing binds each identifier occurrence to its declaration, so find-references and rename are
single-file and text-based.

- **Fix:** a resolved reference graph (each use → its declaration), enabling real cross-file,
  scope-aware references and safe rename.

---

## Recommended phasing

- **Phase 1 — LSP semantic index (highest leverage).** One structure built from the type-checked
  AST providing a scope tree by span, a position→node index, a resolved type per node, and
  (later) occurrence→declaration bindings. Addresses gaps 1, 2, and 7 coherently and retires most
  of the text-heuristics.
- **Phase 2 — non-panicking front-end mode.** Gaps 3 and 4. Lets the LSP drop its reliance on the
  catch-all recover and makes the compiler crash-free too.
- **Phase 3 — reusable compile/resolve service + filesystem overlay.** Gap 5. Cross-file
  correctness, unsaved-buffer resolution, and no drift between CLI and LSP.
- **Phase 4 — span accuracy, export keys, reference graph.** Gaps 6 and 7. Enables safe rename
  and real cross-file references.

---

## Status

None of the above is implemented. The LSP ships as-is; these are deferred enhancements. The
crash-hardening already done is guarded by the `FuzzLSP` harness and regression corpus under
`internal/lsp/`; `docs/src/content/docs/tooling/lsp.mdx` has the user-facing feature list.
