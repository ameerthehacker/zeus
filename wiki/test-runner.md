# Test Runner

Zeus ships a Jest-like test runner: users write `*.test.zs` / `*.spec.zs` files with `describe` /
`it` / `assert`, and `zeus test` discovers, compiles, runs, and reports them. This document covers
the end-to-end architecture — the two halves (a pure-Zeus `@std/test` module and a Go CLI command),
the wire protocol between them, and the design constraints that shaped it.

```
zeus test [path]
   │  discover *.test.zs / *.spec.zs
   ▼
 for each file (worker pool):
   zeus build <file> ──► native binary        (subprocess)
        │                                      lib/std/test.zs is pulled in via `import`
        ▼
   run binary ──► stdout: \x1f-delimited ZTEST markers
        │
        ▼
   Go runner parses markers ──► Jest-style report + exit code
```

---

## Why the split: a Zeus module can't aggregate results

The obvious design — do everything in Zeus, keep a global pass/fail counter, print a summary at the
end — does **not** work, because of how Zeus executes modules:

1. **Only the entry file's top-level code runs.** The entry file's statements are wrapped in the
   synthetic `#_zeus_main` (`internal/ir/ir_passes.go`, `IREmitPass.Run`). An **imported** module
   contributes only its exported functions/classes; its module-level statements are *not* executed
   at program start. So a counter declared at module scope in `@std/test` would never initialize.
2. **Each module gets its own IRBuilder / symbol table.** `Compiler.GenerateZeusIR`
   (`internal/zeus_compiler/compiler.go`) calls `ir.NewIRBuilder()` **per source file**. There is no
   shared mutable state a stdlib module could hang a counter on.

The consequence: **`@std/test` is stateless.** It emits one marker line per event, and the **Go
runner** (which already captures each binary's stdout) does all counting, nesting, rendering, and
exit-code logic. This mirrors the existing e2e harness (`test/e2e/e2e_test.go`), which likewise
drives the compiler as a subprocess and inspects exit codes / stdout.

---

## Part 1 — `@std/test` (the Zeus API)

Source: **`lib/std/test.zs`**, resolved from `import { … } from "@std/test"` →
`$ZEUS_HOME/lib/std/test.zs` (see `wiki/module-system.md`).

```zeus
export function describe(name: string, fn: () => void): void {
  const out: Console = new Console();
  out.log("\x1fZTEST\x1fENTER\x1f" + name);
  fn();
  out.log("\x1fZTEST\x1fEXIT\x1f");
}

export function it(name: string, fn: () => void): void {
  const out: Console = new Console();
  try {
    fn();
    out.log("\x1fZTEST\x1fPASS\x1f" + name);
  } catch (e: Error) {
    out.log("\x1fZTEST\x1fFAIL\x1f" + name + "\x1f" + e.message);
  }
}

export function assert(cond: boolean, message: string): void {
  if (!cond) {
    throw new Error("AssertionError", message);
  }
}
```

Everything here leans on already-working language features:

- **Function-typed parameters called in pure Zeus** — `fn()` where `fn: () => void`. (Same
  mechanism as `test/e2e/specs/functors/cast-to-fn-type.zs`; see `wiki/functors.md`,
  `wiki/closures.md`.)
- **Lambdas as call arguments at any nesting** — `describe("x", () => { it("y", () => {…}) })`.
- **`try { } catch (e: Error) { … e.message }` + `throw new Error(name, msg)`** — an `it`
  swallows a failing assertion so the remaining tests still run (Jest-like), and reports
  `e.message`.

### Why `new Console()` instead of `console`

The global `console` **instance** is created only in the entry module, inside `#_zeus_main`
(`initPrimordialGlobals` in `internal/ir/ir.go`). An imported module never sees it. But the
`Console` **class** is a registered primordial declared into *every* module's symbol table
(`initializePrimordials` in `internal/ir/builder.go`), so `@std/test` constructs its own.

`new Console()` typechecks even though `Console` declares no constructor: `tcNewObj`
(`internal/ir/tc_type_check.go`) only raises "no constructor method found" when arguments are
**passed** to a ctor-less class — a zero-arg `new` is allowed. (See `wiki/module-system.md` and the
`project_global_vars_console` note.)

---

## Part 2 — The marker protocol (wire format)

`@std/test` communicates results as plain stdout lines. Fields are separated by **ASCII Unit
Separator `\x1f` (0x1F)** — chosen because it never appears in ordinary suite/test names or
assertion messages, and Zeus supports the `\xHH` string escape. Every marker line begins with the
tag `\x1fZTEST\x1f`; any other stdout line is treated as user output (e.g. a `console.log` inside a
test) and passed through.

| Event | Line | Emitted by |
|---|---|---|
| suite enter | `\x1fZTEST\x1fENTER\x1f<name>` | `describe`, before `fn()` |
| suite exit | `\x1fZTEST\x1fEXIT\x1f` | `describe`, after `fn()` |
| test passed | `\x1fZTEST\x1fPASS\x1f<name>` | `it`, on success |
| test failed | `\x1fZTEST\x1fFAIL\x1f<name>\x1f<message>` | `it`, on caught `Error` |

The paired `ENTER`/`EXIT` markers let the runner reconstruct **suite nesting depth** for
indentation without the Zeus side tracking any state. Run a `*.test.zs` binary directly (outside
`zeus test`) and you'll see these raw markers — useful for debugging.

The runner matches a line with `strings.HasPrefix(line, "\x1fZTEST\x1f")` and splits with
`strings.SplitN(line, "\x1f", 5)` → `["", "ZTEST", kind, name, message]`. `SplitN` with limit 5
keeps any stray `\x1f` inside a message from over-splitting.

---

## Part 3 — `zeus test` (the Go CLI)

Source: **`cmd/test.go`**, registered in `cmd/main.go` alongside `buildCmd()` / `runCmd()`.
Command shape: `zeus test [path]` (`cobra.MaximumNArgs(1)`); `path` defaults to the cwd.

### 3.1 Discovery — `discoverTestFiles`

`filepath.WalkDir` from the root, collecting files whose name ends in `.test.zs` or `.spec.zs`
(sorted for deterministic order). It skips `target/`, `node_modules/`, `.git/`, and any hidden
directory. A path to a single test file is accepted directly.

### 3.2 Per-file compile + run — `runTestFile`

Each file is handled by a subprocess, never in-process, because `Compiler.Compile` / `Check`
call `os.Exit(1)` on error (`internal/zeus_compiler/compiler.go`) — one bad file would otherwise
kill the whole run. The runner re-execs **itself** (`os.Executable()`):

1. **Compile** — `exec.Command(self, "build", file, "-o", bin, "--target-dir", fileDir)`, captured
   with `CombinedOutput`. A non-zero exit records a **compile error** for that file (the captured
   compiler diagnostic is shown) and the run continues. Each file gets its own temp `fileDir`
   (named from a hash of the path) so parallel builds never race on the shared obj cache.
2. **Run + stream** — `exec.Command(bin)` with a `StdoutPipe`, scanned line-by-line. Markers are
   parsed as they arrive (so progress is *live*, not batched), producing an ordered `[]event`
   (suite headers, test results, passthrough output) plus per-file pass/fail counts. A running
   `depth` counter is bumped on `ENTER` and decremented on `EXIT`.
3. **Crash detection** — the binary normally exits 0 (assertion throws are caught inside `it`). A
   **non-zero exit with no recorded failures** means the program threw outside a test / crashed;
   captured stderr is stored as a **runtime error** for the file.

### 3.3 Concurrency + live progress

File-level parallelism via a bounded worker pool of `runtime.NumCPU()` goroutines (a buffered
`sem` channel). Workers push incremental `progress{passDelta,failDelta,fileDone}` values onto a
channel; a dedicated **spinner goroutine** (`runSpinner`) renders a live status line to **stderr**:

```
⠋  23 tests · 2 failing · 3/7 files
```

The spinner is enabled only when stderr is an interactive terminal (`os.Stderr.Stat()` &
`os.ModeCharDevice`) and `NO_COLOR` is unset — otherwise it just drains the channel so workers
never block, keeping pipes / CI logs clean and deterministic. Each file's rendered block is
buffered and printed in **discovery order** after all workers finish, so parallelism never
scrambles the report (counts are order-independent).

> Note: this is **file-level** concurrency. Individual `it` blocks within one file run
> sequentially inside their compiled binary — see [Concurrency model](#concurrency-model).

### 3.4 Rendering + exit code — `renderReport`

Walks the buffered per-file results and prints a Jest-style tree: a `PASS`/`FAIL` badge per file,
`describe` names bolded and indented by depth, `✓`/`✗` per test with the failure message dimmed
after an em dash, and passthrough user output dimmed. Then the summary:

```
 PASS  math.test.zs
   math
     ✓ adds
     nested
       ✓ subtracts
 FAIL  strings.spec.zs
   strings
     ✓ concats
     ✗ fails intentionally — one is not two

 Test Files  1 failed, 1 passed, 2 total
      Tests  1 failed, 4 passed, 5 total
```

Colors are inline ANSI, gated on `NO_COLOR`. The process **exit code is 1** if any test failed,
any file failed to compile, or any binary crashed; otherwise 0 (so CI can gate on it).

---

## Concurrency model

`zeus test` parallelizes at the **file** granularity: one OS process per test file, `NumCPU` at a
time. It cannot parallelize the `it` blocks *inside* a file, because they execute sequentially in
that binary's `#_zeus_main` and the runner's unit of isolation is the process.

Running individual tests concurrently *within* a file would require the tests to be **async** —
`it("…", async () => { await … })` — so the runtime's libxev event loop could interleave them while
they wait. That needs Zeus `async` / `await` + `Promise<T>`, which are not implemented (async/await
is deferred past beta; `Promise<T>` needs generics — see `wiki/roadmap`). And even then the event
loop gives *cooperative* concurrency (overlapping tests that are waiting on I/O/timers), never true
parallelism of CPU-bound tests — that would need runtime threads, which Zeus doesn't have. So v1 is
**file-parallel + sequential tests within a file**; `it` and the runner are structured so an
async-test variant can slot in later without a rewrite.

---

## Design constraints (v1)

- **No number → string.** `` `value is ${anInt}` `` is a compile error (`invalid operands`; see
  `test/e2e/specs/strings/template-strings-err.zs` and `wiki/compiler-bugs.md`). So `assert`
  messages are static strings the caller supplies — the runner cannot print "expected 4, got 5".
- **A single `assert(cond: boolean, message: string)`**, not a fluent `expect(x).toBe(y)`, because
  Zeus has no generics or function overloading — one `expect` couldn't accept multiple types.
  Users write the comparison themselves: `assert(add(2, 2) == 4, "math broken")`.
- **Synchronous tests only** (see Concurrency model).

---

## Testing

Fixtures live under `test/e2e/testdata/test_runner/` (kept out of `test/e2e/specs/`, which is the
`spec.json`-driven harness's territory). `test/e2e/test_command_test.go` (`TestCommand`) builds the
compiler via the shared `buildCompiler` helper, then runs `zeus test` against the fixtures with
`NO_COLOR=1` and an explicit `ZEUS_HOME`, asserting exit codes and summary counts:

- `mixed/` (`pass.test.zs` + `fail.spec.zs`) → exit 1, `Tests  1 failed, 3 passed, 4 total`.
- `allpass/` → exit 0.
- an empty temp dir → exit 0, "No test files found".

Run it with `ZEUS_HOME=$(pwd) go test -tags llvm19 ./test/e2e/... -run TestCommand -count=1`.

---

## File map

| Path | Role |
|---|---|
| `lib/std/test.zs` | `@std/test`: `describe` / `it` / `assert`, marker emission |
| `cmd/test.go` | `zeus test` command: discovery, subprocess compile+run, worker pool, spinner, render, exit code |
| `cmd/main.go` | registers `testCmd()` |
| `test/e2e/testdata/test_runner/` | fixtures |
| `test/e2e/test_command_test.go` | `TestCommand` |

---

## Future work

- Fluent `expect(...)` matchers and value-aware failure diffs (blocked on number → string).
- `beforeEach` / `afterEach`, `.only` / `.skip`, watch mode, `--release`, test-name filtering.
- Async / concurrent individual tests (blocked on `async` / `await` + `Promise<T>`).
