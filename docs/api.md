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

The current engine uses fixed-depth alpha-beta search and material evaluation.
Callers may replace `Bot.Evaluator` with any value implementing
`engine.Evaluator`.

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
