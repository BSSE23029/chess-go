#!/bin/sh
set -eu

cache=${GOCACHE:-/tmp/chess-go-build-cache}
output=$(GOCACHE="$cache" go test ./engine -run '^$' -bench '^BenchmarkSearchSuiteDepth3/opening$' -benchtime=1x -count=1)
nodes=$(printf '%s\n' "$output" | awk '{ for (i = 1; i <= NF; i++) if ($i == "nodes/search") print $(i - 1) }' | tail -n 1)
tt_hits=$(printf '%s\n' "$output" | awk '{ for (i = 1; i <= NF; i++) if ($i == "tt-hits/search") { printf "%.0f\n", $(i - 1); exit } }')
allocs=$(printf '%s\n' "$output" | awk '{ for (i = 1; i <= NF; i++) if ($i == "allocs/op") { printf "%.0f\n", $(i - 1); exit } }')

case "$nodes" in
  '') echo "benchmark did not report an integer nodes/search value" >&2; exit 1 ;;
  *[!0-9]*) echo "benchmark did not report an integer nodes/search value" >&2; exit 1 ;;
esac
case "$tt_hits" in
  '') echo "benchmark did not report an integer tt-hits/search value" >&2; exit 1 ;;
  *[!0-9]*) echo "benchmark did not report an integer tt-hits/search value" >&2; exit 1 ;;
esac
case "$allocs" in
  '') echo "benchmark did not report an integer allocs/op value" >&2; exit 1 ;;
  *[!0-9]*) echo "benchmark did not report an integer allocs/op value" >&2; exit 1 ;;
esac

# This is a deterministic search-work threshold, not a wall-clock promise.
# Raise it only with a benchmark result and a reviewed search-quality reason.
if [ "$nodes" -gt 1800 ]; then
  echo "search node regression: opening depth-3 used $nodes nodes (limit 1800)" >&2
  exit 1
fi
if [ "$tt_hits" -lt 10 ]; then
  echo "transposition-table regression: opening depth-3 found $tt_hits hits (minimum 10)" >&2
  exit 1
fi
if [ "$allocs" -gt 8 ]; then
  echo "allocation regression: opening depth-3 used $allocs allocs/op (limit 8)" >&2
  exit 1
fi

echo "benchmark guard: $nodes nodes/search, $tt_hits tt-hits/search, $allocs allocs/op"
