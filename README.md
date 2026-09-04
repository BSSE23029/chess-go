# chess-go

`chess-go` is a dependency-light chess rules library, classical search engine,
interactive terminal client, authoritative JSON/HTTP/WebSocket match server,
LAN DNS-SD discovery helper, and deterministic tournament runner. It does not
assume or publish a GitHub owner; the module path is intentionally `chess-go`.

## Install and run

Use the checked-out source directly:

```console
go run ./cmd/chess play local
go run ./cmd/chess play bot --level Club --color black
go run ./cmd/chess matchmake http://127.0.0.1:8080 --player alice --color random
go run ./cmd/chess version
```

Install the command into the Go toolchain's configured bin directory:

```console
go install ./cmd/chess
```

The terminal UI uses arrows or `h j k l` to move, Enter to select, `Esc` to
clear a selection, `u`/`r` for undo/redo, `n` for a new game, `:` for commands,
and `q` to quit. Press `?` for the in-app keyboard guide. The full-screen view
keeps the board central and adds player cards, live clocks, captures, recent
moves, and a visible state legend. A bot game with `--color white` waits for
White's first input by design. When output is captured as scrollback, the
full-screen ANSI redraw can appear as many blank lines; a real terminal
replaces the alternate screen in place.

The `:` command palette also supports `theme ascii|unicode`, `flip`, `draw`,
and `resign`; `n` asks for confirmation before resetting an active game.

Useful runtime settings are environment-backed, including `CHESS_THEME`,
`CHESS_BOT_LEVEL`, `CHESS_BOT_PERSONALITY`, `CHESS_BOT_SEED`, `CHESS_CLOCK`,
`CHESS_NETWORK_URL`, `CHESS_NETWORK_TOKEN`, `CHESS_MATCH_STORE`, and
`CHESS_LAN_DISCOVERY`. See [`docs/api.md`](docs/api.md) for the complete API,
protocol, LAN, UCI, and tournament examples.

## Development

```console
make verify       # tests, race detector, vet, formatting, perft, file sizes
make build        # deterministic, CGO-free dist/chess binary
make bench        # engine and TUI benchmarks with allocation counts
make profile      # CPU/allocation profiles under dist/profiles/
go run ./cmd/perft --depth 4
```

The library's public packages are the root `chess-go`, `engine`, `perft`,
`protocol`, `transport`, `storage`, `lan`, and `tournament` packages. The
wire format is documented in [`docs/protocol.schema.json`](docs/protocol.schema.json).
Contribution and release procedures are in [`docs/contributing.md`](docs/contributing.md)
and [`docs/releasing.md`](docs/releasing.md); the monorepo decision is recorded
in [`docs/architecture.md`](docs/architecture.md).

This repository currently has no license file. Reuse or redistribution should
wait for the copyright owner to choose and add an explicit license.
