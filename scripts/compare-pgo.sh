#!/bin/sh
set -eu

go_bin=${GO:-go}
dist=${DIST:-dist}
cache=${GOCACHE:-/tmp/chess-go-build-cache}
temporary=$(mktemp -d "${TMPDIR:-/tmp}/chess-go-pgo.XXXXXX")
trap 'rm -rf "$temporary"' EXIT HUP INT TERM
profile="$dist/profiles/default.pgo"
if [ ! -s "$profile" ]; then
	DIST="$dist" make profile
	cp "$dist/profiles/engine.cpu.pprof" "$profile"
fi

bench='^BenchmarkSearchSuiteDepth3/opening$'
baseline=$(GOCACHE="$cache" "$go_bin" test -o "$temporary/baseline.test" ./engine -run '^$' -bench "$bench" -benchtime=1x -count=1)
optimized=$(GOCACHE="$cache" "$go_bin" test -o "$temporary/optimized.test" -pgo="$profile" ./engine -run '^$' -bench "$bench" -benchtime=1x -count=1)

echo "non-PGO"
printf '%s\n' "$baseline" | sed -n '/^Benchmark/p'
echo "PGO"
printf '%s\n' "$optimized" | sed -n '/^Benchmark/p'
