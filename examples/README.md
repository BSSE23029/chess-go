# Runnable examples

This directory contains 55 numbered, small, focused Go programs for the public
`chess-go` API, plus the original `basic` example. Every numbered folder is
independently runnable and compiled by the repository test gate.

Run one example:

```console
go run ./examples/01-new-game
go run ./examples/40-engine-search
go run ./examples/51-transport-http
```

Compile every example without running them:

```console
GOCACHE=/tmp/chess-go-build-cache go test ./examples/...
```

## Index

| # | Example | Demonstrates |
|---:|---|---|
| 01 | [new-game](01-new-game) | Create a standard game and print its FEN |
| 02 | [position-copy](02-position-copy) | Apply a move without mutating the source |
| 03 | [legal-moves](03-legal-moves) | Generate legal moves |
| 04 | [parse-square](04-parse-square) | Parse algebraic coordinates |
| 05 | [parse-uci](05-parse-uci) | Inspect structured UCI moves |
| 06 | [uci-roundtrip](06-uci-roundtrip) | Serialize a move back to UCI |
| 07 | [apply-uci](07-apply-uci) | Apply UCI to an immutable position |
| 08 | [apply-legal-move](08-apply-legal-move) | Apply a move from `LegalMoves` |
| 09 | [make-unmake](09-make-unmake) | Fast search-style make/unmake |
| 10 | [side-to-move](10-side-to-move) | Read the active color |
| 11 | [piece-inspection](11-piece-inspection) | Inspect one board square |
| 12 | [board-hash](12-board-hash) | Read the Zobrist position hash |
| 13 | [castling-rights](13-castling-rights) | Inspect castling availability |
| 14 | [en-passant-square](14-en-passant-square) | Inspect the en-passant target |
| 15 | [move-clocks](15-move-clocks) | Read FEN move counters |
| 16 | [san-format](16-san-format) | Parse and format SAN |
| 17 | [san-play](17-san-play) | Play a human-readable SAN line |
| 18 | [from-san](18-from-san) | Build a game from SAN values |
| 19 | [undo-redo](19-undo-redo) | Navigate reversible game history |
| 20 | [captured-pieces](20-captured-pieces) | Read captured pieces |
| 21 | [game-result](21-game-result) | Detect checkmate and result markers |
| 22 | [check-state](22-check-state) | Detect check |
| 23 | [fen-parse](23-fen-parse) | Parse a custom FEN |
| 24 | [fen-roundtrip](24-fen-roundtrip) | Verify FEN serialization |
| 25 | [promotion](25-promotion) | Apply a promotion |
| 26 | [castle-move](26-castle-move) | Apply castling |
| 27 | [en-passant](27-en-passant) | Apply en passant |
| 28 | [move-flags](28-move-flags) | Inspect capture flags |
| 29 | [game-status](29-game-status) | Read automatic game status |
| 30 | [pgn-parse](30-pgn-parse) | Parse PGN |
| 31 | [pgn-export](31-pgn-export) | Export PGN |
| 32 | [pgn-tags](32-pgn-tags) | Add and read PGN tags |
| 33 | [pgn-result](33-pgn-result) | Declare a PGN result |
| 34 | [pgn-custom-fen](34-pgn-custom-fen) | Export a PGN from a FEN |
| 35 | [perft-count](35-perft-count) | Count legal move-tree nodes |
| 36 | [perft-divide](36-perft-divide) | Count each root move |
| 37 | [perft-cancel](37-perft-cancel) | Cancel a perft calculation |
| 38 | [engine-material](38-engine-material) | Evaluate material |
| 39 | [engine-positional](39-engine-positional) | Evaluate positional terms |
| 40 | [engine-search](40-engine-search) | Search and inspect statistics |
| 41 | [engine-node-limit](41-engine-node-limit) | Bound search by nodes |
| 42 | [engine-profile](42-engine-profile) | Select a named strength preset |
| 43 | [engine-personality](43-engine-personality) | Configure deterministic style |
| 44 | [engine-cancel](44-engine-cancel) | Cancel an engine search |
| 45 | [uci-adapter-config](45-uci-adapter-config) | Configure an external UCI engine |
| 46 | [protocol-create](46-protocol-create) | Create an authoritative match |
| 47 | [protocol-move](47-protocol-move) | Submit a synchronized move |
| 48 | [protocol-draw](48-protocol-draw) | Offer and accept a draw |
| 49 | [protocol-resign](49-protocol-resign) | Resign an authoritative match |
| 50 | [protocol-json](50-protocol-json) | Encode and decode a wire envelope |
| 51 | [transport-http](51-transport-http) | Exercise the HTTP adapter locally |
| 52 | [storage-save](52-storage-save) | Persist match state to JSON |
| 53 | [lan-service](53-lan-service) | Build a DNS-SD service descriptor |
| 54 | [tournament-run](54-tournament-run) | Run a tiny deterministic tournament |
| 55 | [player-interface](55-player-interface) | Use the shared `chess.Player` interface |

The examples avoid hard-coded machine paths, network tokens, or external engine
paths; protocol snippets use stable illustrative IDs. Set machine-specific
values such as `CHESS_UCI_ENGINE` in your shell when needed.
