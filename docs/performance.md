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
positions after deterministic candidate-selection reuse; the randomized Club
and Advanced profiles also avoid validating each styled candidate twice. Treat
those numbers as a local regression baseline, not a portable performance
guarantee.

On the current M1 Pro baseline, the default 8K table measures roughly 3.7–4.0
ms for the depth-3 opening position, 1,128 nodes/search, and 4 allocations/
search. Deterministic searches now skip the root candidate buffer entirely;
randomized profiles still retain candidate scoring for near-best selection.
The move-generation path also avoids constructing a castling-rights map for
every make/unmake operation. Repeated table-size runs kept the same node and
TT-hit counters while
the 8K table avoided the 16K table's extra memory footprint. The current
one-iteration PGO comparison measured about 3.9 ms without PGO versus 3.9 ms
with PGO, with identical search counters; this is effectively neutral and
remains noisy diagnostic data, not a claim that PGO improves this workload.
The current randomized-profile run also keeps the search counters unchanged
while reducing Club and Advanced from 14 to roughly 10–11 allocations/search
by using bounded stack scratch space for near-best move weights; Maximum remains
at 4 allocations/search. These figures are fresh Apple M1 Pro measurements,
not a portable timing promise.
Repeat measurements on the target
machine before drawing conclusions.

Run the broader benchmark set with:

```console
make bench
```

The CI guard checks deterministic search work and the stable opening allocation
count rather than machine-dependent wall-clock time:

```console
make benchmark-regression
```

It protects the depth-3 opening baseline from excessive node growth and
requires the transposition table to continue producing hits. The table-size
tuning benchmark compares 2K, 4K, 8K, and 16K entries:

```console
go test ./engine -run '^$' -bench '^BenchmarkSearchSuiteDepth3TableSizes$' -benchmem
```

The default is currently 8K entries: repeated local depth-3 measurements kept
the same search work as 16K while using less memory. Set
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
go tool pprof -http=:0 dist/profiles/engine-strength.cpu.pprof
```

`make profile` now profiles both the deterministic depth-3 suite and the real
Club, Advanced, and Maximum presets (with their opening book disabled). The
second profile is written separately as `engine-strength.cpu.pprof` and
`engine-strength.mem.pprof`, so profile-guided builds can continue using the
stable generic engine profile while strength tuning has its own evidence.

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

## Deterministic self-play smoke check

The tournament runner derives each game's bot seed from the supplied seed and
game number. Run the same command twice to verify that the archived report and
PGN are byte-for-byte reproducible without spending a full match budget:

```console
go run ./cmd/tournament \
  --profiles Learner,Beginner --games 1 --plies 12 --seed 42 \
  --node-budget 128 --engine-version v0.1.0 --hardware-class local \
  --json /tmp/chess-first.json --pgn /tmp/chess-first.pgn
go run ./cmd/tournament \
  --profiles Learner,Beginner --games 1 --plies 12 --seed 42 \
  --node-budget 128 --engine-version v0.1.0 --hardware-class local \
  --json /tmp/chess-second.json --pgn /tmp/chess-second.pgn
shasum -a 256 /tmp/chess-first.json /tmp/chess-second.json \
  /tmp/chess-first.pgn /tmp/chess-second.pgn
```

The two JSON hashes and two PGN hashes should match. The tournament package
also asserts this invariant in `TestRoundRobinReportIsReproducibleAndPortable`.
