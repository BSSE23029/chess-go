# chess-go — Go chess library, classical chess engine, and terminal chess server

`chess-go` is a dependency-light, rules-first chess toolkit for Go. It combines
a reusable chess rules library, FEN/SAN/UCI/PGN support, a classical alpha-beta
chess engine, a keyboard-driven terminal chess UI, an authoritative JSON/HTTP/
WebSocket match server, LAN discovery, and deterministic engine tournaments.

It uses ordinary algorithms rather than neural networks or machine learning:
legal move generation, make/unmake, Zobrist hashing, iterative alpha-beta
search, move ordering, quiescence search, transposition tables, and handcrafted
position evaluation.

> The module path is intentionally `chess-go` until the project owner selects a
> canonical repository/module URL. No GitHub owner or remote account is assumed.

## Contents

- [Why chess-go](#why-chess-go)
- [Install and play](#install-and-play)
- [Go API quick start](#go-api-quick-start)
- [Features](#features)
- [CLI command reference](#cli-command-reference)
- [55 runnable examples](#55-runnable-examples)
- [Network chess](#network-chess)
- [Performance and reproducible builds](#performance-and-reproducible-builds)
- [Development](#development)
- [Release readiness](#release-readiness)
- [Documentation](#documentation)
- [License](#license)

## Why chess-go

Use this project when you need a Go chess library for legal moves, a small
classical chess bot, a terminal chess game, a portable chess protocol, or a
headless tournament runner. The rules package stays independent from the
terminal and network layers, so applications can use only the pieces they need.

The project is a good fit for:

- Go chess applications and teaching projects.
- FEN/SAN/PGN conversion and chess analysis tools.
- Deterministic perft and move-generation verification.
- Local human-versus-human or human-versus-bot terminal games.
- Authoritative LAN or online chess match prototypes.
- Bot strength calibration and UCI engine comparisons.

## Install and play

Run directly from a checkout:

```console
go run ./cmd/chess play local --theme unicode
go run ./cmd/chess play bot --level Club --color black --theme unicode
go run ./cmd/chess version
```

Install the command into Go's configured binary directory:

```console
go install ./cmd/chess
chess play bot --level Casual
```

The terminal board uses real Unicode chess symbols by default. Use
`--theme ascii` or `CHESS_THEME=ascii` for plain letters and ASCII borders.
Set `NO_COLOR=1` when ANSI color is not desired. The TUI scales its board and
sidebar to the current terminal size, clips safely at very small viewports,
restores the terminal on exit, and redraws cleanly when state or clocks change.

Interactive controls are Arrow keys or `h`/`j`/`k`/`l`, Enter/Space to select,
`Esc` to clear, `u`/`r` for undo/redo, `n` for a confirmed new game, `:` for
commands, `?` for help, and `q` or Ctrl-C to quit. Promotion choices use
Left/Right and Enter.

### Terminal previews

The board grows into the available space on a wide terminal and switches to a
stacked, compact layout when the window is narrow. Both previews are rendered
from the same themes available at runtime:

![Wide Unicode chess-go terminal board](docs/images/preview-unicode.svg)

![Compact ASCII chess-go terminal board](docs/images/preview-ascii.svg)

## Go API quick start

```go
package main

import (
	"context"
	"fmt"
	"log"

	chess "chess-go"
	"chess-go/engine"
)

func main() {
	game, err := chess.FromSAN([]string{"e4", "e5", "Nf3"})
	if err != nil {
		log.Fatal(err)
	}

	position := game.Position()
	fmt.Println("FEN:", position.FEN())
	fmt.Println("legal moves:", len(position.LegalMoves()))

	bot := engine.New(3)
	move, err := bot.ChooseMove(context.Background(), position)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("engine move:", move.UCI())
}
```

The root `chess-go` package is safe to use without the CLI. For now, consume it
from a checkout; the public `go get` command must wait until the owner selects a
domain-qualified module path.

## Features

| Area | Included |
|---|---|
| Chess rules | Legal moves, check, checkmate, stalemate, castling, promotion, en passant, 50-move rule, repetition, insufficient material |
| Notation | FEN parsing/serialization, SAN parsing/formatting, UCI coordinates, PGN tags/comments/variations/results |
| Search | Material and positional evaluators, iterative alpha-beta, quiescence, transposition tables, aspiration windows, null-move pruning, late-move reductions |
| Bot profiles | Learner, Beginner, Casual, Club, Advanced, Expert, Maximum plus deterministic personalities |
| Terminal UI | Unicode/ASCII themes, responsive narrow layout, clocks, captures, move history, promotion picker, command palette, bot statistics |
| Networking | Versioned JSON protocol, authoritative server, HTTP, WebSocket, reconnectable sessions, spectators, draw/resign, server clocks |
| LAN | Dependency-free DNS-SD/mDNS advertisement and discovery |
| Calibration | Deterministic round-robin tournaments, PGN/JSON reports, Elo-style estimates and confidence intervals |
| Verification | Perft counts, race/vet gates, benchmark suite, CPU/allocation profiles, reproducible CGO-free builds |

## CLI command reference

Every top-level command, flag, accepted value, environment fallback, line-mode
command, interactive key, and remote action is documented in the exhaustive
[`docs/cli.md`](docs/cli.md) reference.

Common commands:

```console
chess play local [--clock 10m] [--increment 3s] [--theme unicode]
chess play bot [--level Club | --depth 3] [--color white|black]
chess play remote http://127.0.0.1:8080 --match game --player alice
chess host --addr :8080 --token "$CHESS_NETWORK_TOKEN"
chess join http://127.0.0.1:8080 --match game --player alice --color white
chess spectate http://127.0.0.1:8080 --match game --player viewer
chess matchmake http://127.0.0.1:8080 --player alice --color random
chess list http://127.0.0.1:8080
chess discover --seconds 2
chess load game.pgn
```

For local line mode, moves can be SAN (`Nf3`, `O-O`) or UCI (`g1f3`). The
palette commands are `moves`, `undo`, `redo`, `fen FEN`, `load FILE`,
`save FILE`, `theme ascii|unicode`, `flip`, `draw`, `resign`, `help`, and
`quit`/`exit`/`q`.

## 55 runnable examples

The [`examples/`](examples) directory contains 55 numbered, independently
runnable Go programs (plus the original `basic` example) covering positions,
move generation, notation, special moves, PGN, perft, engine search, UCI
configuration, protocol messages, HTTP transport, storage, LAN descriptors,
tournaments, and the shared player interface.

```console
go run ./examples/01-new-game
go run ./examples/25-promotion
go run ./examples/40-engine-search
go run ./examples/51-transport-http
GOCACHE=/tmp/chess-go-build-cache go test ./examples/...
```

See the complete numbered index in [`examples/README.md`](examples/README.md).

## Network chess

The `protocol` package is transport-independent and authoritative. Every move
contains a match ID, expected sequence, and expected position hash. The server
validates the player seat and chess legality before accepting a move; clients
rebuild from authoritative snapshots.

Start a local host with optional persistence and LAN discovery:

```console
CHESS_NETWORK_ADDR=:8080 \
CHESS_NETWORK_TOKEN=local \
CHESS_MATCH_STORE=matches.json \
chess host --lan
```

Then create/join a match from another terminal:

```console
chess play remote http://127.0.0.1:8080 --create \
  --match demo --player alice --color white --token local
chess play remote http://127.0.0.1:8080 \
  --match demo --player bob --color black --token local
```

The versioned envelope and payload shapes are defined in
[`docs/protocol.schema.json`](docs/protocol.schema.json). TLS is enabled by
providing `CHESS_TLS_CERT` and `CHESS_TLS_KEY`; do not expose an unauthenticated
development server to the public internet.

## Performance and reproducible builds

Run the measured baselines and profiles:

```console
make bench
make profile
```

Profiles are written to `dist/profiles/`. Release binaries use the Go version
from `go.mod`, `CGO_ENABLED=0`, `-trimpath`, disabled VCS stamping, and an
explicit version string.

Build one candidate binary:

```console
VERSION=v0.2.0 make release
sha256sum dist/chess
```

Build macOS, Linux, and Windows archives with checksums:

```console
VERSION=v0.2.0 make release-all
find dist/releases/v0.2.0 -maxdepth 1 -type f -print
```

## Development

```console
make verify       # tests, race detector, vet, formatting, perft, file size
make build        # deterministic CGO-free dist/chess binary
make bench        # engine and TUI benchmark baselines
make profile      # CPU and allocation profiles
go run ./cmd/perft --depth 4
go run ./cmd/tournament --profiles Learner,Club,Expert --games 20
```

Keep rules independent from UI and transport code. Add focused lifecycle tests
for behavior changes, use environment variables for machine-specific values,
and keep maintained production Go files below the 500-line hard limit.

## Release readiness

The repository now has CI verification, cross-platform packaging, checksum
generation, 55 examples, and an automatic tag-triggered release workflow.
Before a public release, the owner still needs to choose and add an explicit
license, select the canonical repository/module URL, review dependency and
security policy, create a version tag, and publish release notes.

The project does not push tags, publish artifacts, or claim ownership of any
GitHub account automatically.

## Documentation

- [`docs/api.md`](docs/api.md) — library, engine, protocol, LAN, and tournament API.
- [`docs/cli.md`](docs/cli.md) — exhaustive command and environment reference.
- [`docs/core.md`](docs/core.md) — rules, notation, PGN, hashing, and engine design.
- [`docs/algos.md`](docs/algos.md) — algorithm notes and search trade-offs.
- [`docs/architecture.md`](docs/architecture.md) — package boundaries and monorepo decision.
- [`docs/protocol.schema.json`](docs/protocol.schema.json) — language-neutral protocol schema.
- [`docs/contributing.md`](docs/contributing.md) — development gates and contribution rules.
- [`docs/releasing.md`](docs/releasing.md) — candidate, archive, checksum, and tag workflow.

## License

No license has been selected yet. Until the copyright owner adds one, treat the
source as all-rights-reserved and do not redistribute it as a public package.
