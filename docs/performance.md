# Performance workflow

The engine is optimized against a small, deterministic depth-3 suite rather
than one opening position. It covers opening, tactical, quiet, and endgame
positions and reports elapsed time, allocations, and average nodes per search:

```console
go test ./engine -run '^$' -bench '^BenchmarkSearchSuiteDepth3$' -benchmem
```

The suite reports `nodes/search` and `tt-hits/search` in addition to elapsed
time and allocations. A representative Apple M1 Pro run currently measures
about 70 allocations for the opening position and 77 for the tactical
position at depth 3; treat those numbers as a local regression baseline, not a
portable performance guarantee.

Run the broader benchmark set with:

```console
make bench
```

For profiles that can be opened with `go tool pprof`:

```console
make profile
go tool pprof -http=:0 dist/profiles/engine.cpu.pprof
```

After the baseline and correctness gates are green, `make pgo` reuses the
representative engine CPU profile as Go profile-guided optimization input and
writes an optimized binary to `dist/chess-pgo`. PGO is an optional last-mile
build comparison; it does not replace `make verify` or the tactical suite.

Compare changes on the same machine and Go toolchain. The benchmark is a
diagnostic baseline, not a strength claim; tactical correctness and legal-move
tests remain release gates.
