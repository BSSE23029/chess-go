# Performance workflow

The engine is optimized against a small, deterministic depth-3 suite rather
than one opening position. It covers opening, tactical, quiet, and endgame
positions and reports elapsed time, allocations, and average nodes per search:

```console
go test ./engine -run '^$' -bench '^BenchmarkSearchSuiteDepth3$' -benchmem
```

The suite reports `nodes/search`, cache-hit metrics, and `tt-hits/search` in
addition to elapsed time and allocations. A representative Apple M1 Pro run
currently measures single-digit allocations for the opening and tactical
positions after deterministic candidate-selection reuse; treat those numbers
as a local regression baseline, not a portable performance guarantee.

On the current M1 Pro baseline, the default 16K table measured about 6.2 ms for
the depth-3 opening position, 1,128 nodes/search, and 7 allocations/search.
PGO reduced one same-suite sample from about 7.2 ms to 6.6 ms without changing
the node or cache counters; repeat measurements on the target machine before
drawing conclusions.

Run the broader benchmark set with:

```console
make bench
```

The CI guard checks deterministic search work rather than machine-dependent
wall-clock time:

```console
make benchmark-regression
```

It protects the depth-3 opening baseline from excessive node growth and
requires the transposition table to continue producing hits. The table-size
tuning benchmark compares 2K, 4K, 8K, and 16K entries:

```console
go test ./engine -run '^$' -bench '^BenchmarkSearchSuiteDepth3TableSizes$' -benchmem
```

The default is currently 16K entries: local depth-3 measurements showed less
replacement churn than 2K while keeping the fixed table under 1 MiB. Set
`Bot.TranspositionTableSize` explicitly when embedding the engine in a tighter
memory budget and re-run the benchmark on that target.

Profile work can be compared with the actual Club, Advanced, and Maximum
presets (the benchmark disables the opening book):

```console
go test ./engine -run '^$' -bench '^BenchmarkStrengthProfiles$' -benchmem
```

SEE remains opt-in for the tactical personality until its cost is measured
against the cheap default capture signal:

```console
go test ./engine -run '^$' -bench '^BenchmarkQuiescenceCaptureScoring$' -benchmem
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

Compare the same opening search suite with and without that profile:

```console
make pgo-compare
```

The command reports both benchmark lines; compare them on the same machine and
toolchain, and treat node counts as the correctness invariant.

Compare changes on the same machine and Go toolchain. The benchmark is a
diagnostic baseline, not a strength claim; tactical correctness and legal-move
tests remain release gates.
