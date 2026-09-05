# Performance workflow

The engine is optimized against a small, deterministic depth-3 suite rather
than one opening position. It covers opening, tactical, quiet, and endgame
positions and reports elapsed time, allocations, and average nodes per search:

```console
go test ./engine -run '^$' -bench '^BenchmarkSearchSuiteDepth3$' -benchmem
```

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
