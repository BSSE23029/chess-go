# Go API guide

The module exposes chess rules through `chess-go`, fixed-depth search through
`chess-go/engine`, and move-tree verification through `chess-go/perft`.

All position values are safe to copy. Prefer `Position.Apply` in application
code. Search code can use `MakeLegalMove` and `UnmakeMove` to avoid copying,
provided every successful make is paired with an unmake.

## Positions and legal moves

```go
position := chess.NewPosition()

for _, move := range position.LegalMoves() {
    fmt.Println(move.UCI())
}

next, err := position.ApplyUCI("e2e4")
if err != nil {
    log.Fatal(err)
}
fmt.Println(next.FEN())
```

`ParseFEN` requires all six FEN fields and rejects semantically impossible
state, including missing or adjacent kings, invalid pawn placement, inconsistent
castling or en-passant state, and a side that illegally left its king attacked.

```go
position, err := chess.ParseFEN(chess.InitialFEN)
if err != nil {
    log.Fatal(err)
}
```

## SAN and game lifecycle

`Game` owns move history, undo/redo navigation, automatic terminal-state
detection, declared PGN results, and PGN metadata.

```go
game := chess.NewGame()
for _, san := range []string{"e4", "e5", "Nf3", "Nc6"} {
    if err := game.PlaySAN(san); err != nil {
        log.Fatal(err)
    }
}

fmt.Println(game.Position().FEN())
game.Undo()
game.Redo()
```

`Position.SAN` formats a legal move. `Position.ParseSAN` resolves SAN against
the position's legal moves, so ambiguous or illegal notation returns an error.

## PGN metadata and round-trips

`ParsePGN` reads ordered tag pairs, the main-line SAN moves, comments,
variations, NAGs, and results. Comments, variations, and NAGs are ignored as
annotations; malformed comment or variation delimiters are rejected. Tag values
support the PGN escapes `\\` and `\"`.

```go
game, err := chess.ParsePGN(`[Event "Example"]
[Annotator "A \\ B"]
[Result "*"]

1. e4 {main line} (1. d4) e5 *`)
if err != nil {
    log.Fatal(err)
}

for _, tag := range game.Tags() {
    fmt.Printf("%s=%s\n", tag.Name, tag.Value)
}
fmt.Println(game.PGN())
```

`Tags` returns a copy in source order. Arbitrary tags and escaped values survive
a parse/export/reparse lifecycle. The writer synchronizes `Result`, `SetUp`, and
`FEN` with the actual game state and supplies missing standard roster tags.

## Reversible search moves

Moves returned by `LegalMoves` can use the fast path:

```go
for _, move := range position.LegalMoves() {
    undo := position.MakeLegalMove(move)
    score(position)
    position.UnmakeMove(undo)
}
```

Use `MakeMove` for untrusted moves; it validates legality first. `Undo` is
opaque, belongs to the exact make operation that created it, and should be
unmade in strict last-in-first-out order. `Hash` is an incrementally maintained
Zobrist key for search positions and intentionally excludes move clocks.

## Engine

`engine.Bot` implements `chess.Player`. Cancellation is propagated through its
search.

```go
bot := engine.New(3)
move, err := bot.ChooseMove(context.Background(), position)
if err != nil {
    log.Fatal(err)
}
fmt.Println(move.UCI())
```

The current engine uses fixed-depth alpha-beta search. `engine.New` retains a
material evaluator for compatibility; named profiles use the deterministic
`PositionalEvaluator`, which adds piece-square, mobility, pawn-structure,
bishop-pair, passed-pawn, and king-safety terms. Callers may replace
`Bot.Evaluator` with any value implementing `engine.Evaluator`.

`EndgameEvaluator` adds sparse-position king centralization and king-to-pawn
proximity; Advanced, Expert, and Maximum profiles use it automatically.

The search horizon uses quiescence search: tactical captures, promotions, and
forced check evasions are explored before a leaf is statically evaluated.
Each search owns a Zobrist-keyed transposition table with exact, lower-bound,
upper-bound, and preferred-move entries; callers can therefore reuse a single
`Search` call safely without sharing mutable engine state across games. Its
statistics include reduced late-move searches for profiling.
Quiet cutoffs update per-ply killer and history tables for move ordering, while
completed iterative scores seed a narrow aspiration window and automatically
retry with a full window when the score falls outside it.
At depth three and deeper, late quiet moves use a reduced null-window search
and are re-searched at full depth only when they raise alpha.

`OpeningBook` can provide deterministic hash-keyed moves before search. Named
profiles use `BuiltinOpeningBook`; an absent or illegal entry automatically
falls back to iterative search.

For callers that own time controls, `Bot.Search` performs iterative deepening
with explicit node and wall-clock limits and returns statistics for the deepest
completed iteration:

```go
move, stats, err := bot.Search(ctx, position, engine.SearchLimits{
    MaxDepth: 8,
    MaxNodes: 250_000,
    Time:     750 * time.Millisecond,
})
```

If a node limit interrupts after a completed iteration, the last complete move
is returned with its depth and node count. Context cancellation and a timeout
before any complete iteration are returned as errors.

For user-facing play, `engine` provides deterministic named strength presets:
`Learner`, `Beginner`, `Casual`, `Club`, `Advanced`, `Expert`, and `Maximum`.
The CLI accepts either a preset or the legacy raw depth setting:

```console
CHESS_BOT_LEVEL=club go run ./cmd/chess play bot
go run ./cmd/chess play bot --level Expert --color black
```

`ParseStrengthProfile` accepts names case-insensitively. Each preset controls
search depth and the maximum evaluation loss allowed when ranking candidate
moves; no hidden random source is used, so repeated tests are reproducible.

Strength and personality are separate. `ParsePersonality` supports Cautious,
Aggressive, Materialist, Tactician, Positional, Simplifier, and Trickster.
`Bot.SetPersonality` accepts an explicit seed; positive temperatures select
among eligible candidates deterministically from that seed.

## Perft

Use `perft.Count` for a total or `perft.Divide` for one count per legal root
move. Both preserve their input and honor context cancellation.

```go
nodes, err := perft.Count(context.Background(), chess.NewPosition(), 4)
if err != nil {
    log.Fatal(err)
}
fmt.Println(nodes) // 197281
```

The command-line equivalent is:

```console
go run ./cmd/perft --depth 4
go run ./cmd/perft --fen "7k/7p/8/8/8/8/P7/K7 b - - 0 1" --depth 2 --divide
```

## Runnable example

Run the checked example from the repository root:

```console
go run ./examples/basic
```

The normal `go test ./...` gate compiles this example with the public API.

The terminal renderer supports ASCII letters and Unicode chess glyphs. Select
one with `--theme ascii|unicode` or the `CHESS_THEME` environment variable; an
invalid value fails before the game starts.

## Network protocol foundation

`chess-go/protocol` defines version-one JSON envelopes and an authoritative
in-memory `Match`. Every move request includes the match ID, expected sequence,
and expected Zobrist hash; the match validates the player seat and legal chess
move before incrementing its sequence. Snapshots include the current turn,
result (`*`, `1-0`, `0-1`, or `1/2-1/2`), draw offer, and spectator count.

`protocol.Server` adds session-aware match creation, joining, reconnecting,
spectating, resignation, draw offers, deterministic match listing, and strict
envelope dispatch. A disconnected player's seat remains reserved and is
restored by `Connect` with the same player ID. Domain failures are returned as
`error` envelopes with stable codes such as `sequence_conflict`,
`position_mismatch`, `seat_taken`, and `match_over`; malformed JSON or payloads
remain ordinary decoding errors.

The wire envelope is documented in [`protocol.schema.json`](protocol.schema.json)
and is transport-independent, so HTTP and WebSocket adapters can share the same
messages.
