#!/usr/bin/env bash
# One command to (re)generate the benchmark report end to end:
#   build everything -> time with hyperfine -> render tables + charts into docs.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

bash "$SCRIPT_DIR/scripts/build.sh"
bash "$SCRIPT_DIR/scripts/bench.sh"
node "$SCRIPT_DIR/scripts/gen-report.mjs"

echo
echo "Done. Preview the report with:  cd docs && npm run dev"
