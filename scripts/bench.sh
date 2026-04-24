#!/usr/bin/env bash
# Run the in-process benchmarks and emit a structured JSON baseline at
# ./bench-results.json. Runs with a fixed benchtime so results are
# comparable across commits (not across machines — that requires the
# Phase 4 harness).
set -euo pipefail

cd "$(dirname "$0")/.."

BENCHTIME="${BENCHTIME:-1s}"
OUT="${OUT:-bench-results.json}"
SHA="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
GOVERSION="$(go version | awk '{print $3}')"

RAW=$(go test -bench=. -benchmem -run=^$ -benchtime="${BENCHTIME}" ./test/load/ 2>&1)

# Extract lines of the form:
#   BenchmarkName-N    ITERATIONS    NS_PER_OP ns/op    BYTES B/op    ALLOCS allocs/op
RESULTS=$(printf '%s\n' "$RAW" \
  | awk '/^Benchmark/ {
      name=$1; sub(/-[0-9]+$/, "", name);
      iters=$2; nsop=$3; bop=$5; allocs=$7;
      printf "{\"name\":\"%s\",\"iterations\":%s,\"ns_per_op\":%s,\"bytes_per_op\":%s,\"allocs_per_op\":%s}\n",
        name, iters, nsop, bop, allocs
    }')

JSON_BENCHES=$(printf '%s\n' "$RESULTS" | paste -sd, -)

cat > "$OUT" <<EOF
{
  "schema_version": 1,
  "commit": "$SHA",
  "timestamp": "$TS",
  "go_version": "$GOVERSION",
  "benchtime": "$BENCHTIME",
  "benchmarks": [$JSON_BENCHES]
}
EOF

echo "wrote $OUT"
cat "$OUT"
