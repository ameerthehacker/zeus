#!/usr/bin/env bash
# Build everything the benchmarks need:
#   1. the Boehm GC (once, via cmake) if it is missing
#   2. the Zig runtime
#   3. the Zeus compiler (./zeus)
#   4. a release Zeus binary and an optimized Go binary for every case
#
# Node needs no build step — it runs the .js source directly.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$SCRIPT_DIR/../.." && pwd)"
BENCH="$REPO/bench"
export ZEUS_HOME="$REPO"

CASES=(fib-recursive loop-sum prime-sieve sort matmul)

cd "$REPO"

if [[ ! -f "$REPO/third_party/bdwgc/lib/libgc.a" ]]; then
  echo "==> Building Boehm GC (cmake)"
  cmake -B build -DCMAKE_BUILD_TYPE=Release
  make -C build
fi

echo "==> Building Zig runtime"
make build-runtime

echo "==> Building Zeus compiler (./zeus)"
make build

echo "==> Compiling benchmark programs (release)"
for c in "${CASES[@]}"; do
  out="$BENCH/.build/$c"
  mkdir -p "$out"
  echo "  - $c (zeus)"
  "$REPO/zeus" build "$BENCH/cases/$c/$c.zs" --release -o "$out/zeus_$c"
  echo "  - $c (go)"
  go build -o "$out/go_$c" "$BENCH/cases/$c/$c.go"
done

echo "==> Verifying self-checks (every program must exit 0)"
fail=0
for c in "${CASES[@]}"; do
  out="$BENCH/.build/$c"
  "$out/zeus_$c"                 || { echo "    ZEUS $c FAILED"; fail=1; }
  "$out/go_$c"                   || { echo "    GO   $c FAILED"; fail=1; }
  node "$BENCH/cases/$c/$c.js"   || { echo "    NODE $c FAILED"; fail=1; }
done
if [[ "$fail" -ne 0 ]]; then
  echo "One or more self-checks failed — results would be invalid." >&2
  exit 1
fi
echo "==> Build OK: all self-checks passed."
