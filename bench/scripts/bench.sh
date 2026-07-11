#!/usr/bin/env bash
# Time every case with hyperfine (whole-process wall clock) and write one JSON
# file per case plus an environment fingerprint. Run scripts/build.sh first.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$SCRIPT_DIR/../.." && pwd)"
BENCH="$REPO/bench"
cd "$BENCH"

if ! command -v hyperfine >/dev/null 2>&1; then
  echo "hyperfine is required. Install it with:  brew install hyperfine" >&2
  exit 1
fi

CASES=(fib-recursive loop-sum prime-sieve sort matmul)
mkdir -p results

for c in "${CASES[@]}"; do
  zeus_bin=".build/$c/zeus_$c"
  go_bin=".build/$c/go_$c"
  js="cases/$c/$c.js"
  if [[ ! -x "$zeus_bin" || ! -x "$go_bin" ]]; then
    echo "Missing binaries for $c — run scripts/build.sh first." >&2
    exit 1
  fi
  echo "==> Benchmarking $c"
  hyperfine --warmup 3 --min-runs 10 \
    -n zeus "$zeus_bin" \
    -n go   "$go_bin" \
    -n node "node $js" \
    --export-json "results/$c.json"
done

CPU="$(sysctl -n machdep.cpu.brand_string 2>/dev/null || sysctl -n hw.model 2>/dev/null || echo unknown)"
OSV="$(sw_vers -productVersion 2>/dev/null || uname -r)"
ZEUS_VER="$(git -C "$REPO" describe --tags --always 2>/dev/null || echo dev)"
{
  printf '{\n'
  printf '  "date": "%s",\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '  "os": "%s %s (%s)",\n' "$(uname -s)" "$OSV" "$(uname -m)"
  printf '  "cpu": "%s",\n' "$CPU"
  printf '  "go": "%s",\n' "$(go version | sed 's/go version //')"
  printf '  "node": "%s",\n' "$(node --version)"
  printf '  "hyperfine": "%s",\n' "$(hyperfine --version)"
  printf '  "zeus": "%s"\n' "$ZEUS_VER"
  printf '}\n'
} > results/env.json

echo "==> Wrote results/*.json and results/env.json"
