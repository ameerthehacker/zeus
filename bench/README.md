# Zeus benchmarks

A small, reproducible suite that compares **Zeus** against **Go** and **Node.js**
on basic CPU-bound algorithms. Results are rendered into the docs site at
**Performance → Benchmarks**.

## What it measures

Each case is implemented three times — `<slug>.zs`, `<slug>.go`, `<slug>.js` —
running the *same* algorithm on the *same* input. We measure **whole-process
wall-clock time** with [`hyperfine`](https://github.com/sharkdp/hyperfine)
(warmups + repeated runs). Whole-process timing is used because Zeus exposes no
in-program clock; each program is sized so its compute dwarfs process startup.

Every program **self-verifies**: it computes a checksum, compares it to a
hard-coded expected value, and exits `0` only on a match. Identical exit codes
across languages prove the three implementations compute the same result, and it
stops the optimizer from deleting the work. The expected values were derived from
a reference Go run.

| Slug | Algorithm | Size | Exercises |
|------|-----------|------|-----------|
| `fib-recursive` | Naive recursive Fibonacci | `fib(40)` | function-call overhead |
| `loop-sum` | Sum `(i & 1023)` over `1..1e9` (i64) | `N = 1e9` | raw loop / integer ALU |
| `prime-sieve` | Sieve of Eratosthenes, count primes | `limit = 3e7` | array alloc + nested loops |
| `sort` | Insertion sort of an LCG-filled array | `N = 20000` | array get/set + comparisons |
| `matmul` | Naive flat NxN integer matrix multiply | `512 x 512` | 2D access, triple-nested loop |

## Requirements

- The Zeus toolchain prerequisites (LLVM 19, Zig, cmake — see the main README).
- `go` and `node` on `PATH`.
- `hyperfine` — install with `brew install hyperfine`.

## Running

```bash
# Full pipeline: build -> benchmark -> render report into docs/
bash bench/run.sh

# Or step by step:
bash bench/scripts/build.sh        # build zeus + compile every case
bash bench/scripts/bench.sh        # run hyperfine -> bench/results/*.json
node bench/scripts/gen-report.mjs  # render tables + SVG charts into the docs
```

Then preview: `cd docs && npm run dev`.

## Layout

```
bench/
  cases/<slug>/        <slug>.{zs,go,js} + meta.json
  scripts/             build.sh, bench.sh, gen-report.mjs
  results/             hyperfine JSON + env.json + summary.json (generated)
  .build/              compiled binaries (gitignored)
```

## Adding a benchmark

1. Create `cases/<slug>/` with `<slug>.zs`, `<slug>.go`, `<slug>.js` and a
   `meta.json` (`slug`, `name`, `description`, `param`, `expectedExit`).
2. Make all three self-check against the same checksum (derive it from the Go
   run first, then bake it into all three).
3. Add `<slug>` to the `CASES` array in `scripts/build.sh`, `scripts/bench.sh`
   and `scripts/gen-report.mjs`.

## Notes on fairness

- Zeus and Go are compiled optimized (`zeus build --release`, `go build`); Node
  runs the source under the V8 JIT. Numeric arrays in JS use typed arrays
  (`Int32Array`/`Uint8Array`), the idiomatic high-performance choice.
- These are single-machine, single-run-set numbers for a young language — a
  snapshot to guide optimization, not a definitive ranking.
