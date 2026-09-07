# Changelog

All notable changes to chess-go are documented here.

## [v0.1.0] - 2026-09-07

The first release candidate of the dependency-light Go chess toolkit.

### Added

- Legal chess rules, FEN/SAN/PGN helpers, perft, and undo/redo game state.
- Classical alpha-beta search with iterative deepening, transposition tables,
  move ordering, null-move and late-move reductions, quiescence search, and
  selectable strength/personality profiles.
- Material, positional, endgame, pawn-structure, passed-pawn, rook-activity,
  and king-safety evaluation terms.
- UCI-compatible engine mode, local and network match transports, TLS/mTLS,
  tournament orchestration, and storage examples.
- Responsive terminal UI with launcher settings, live resize handling,
  Unicode/text/icon piece styles, compact fallbacks, and real terminal
  screenshots in `docs/images/`.
- Reproducible multi-platform release archives with SHA-256 manifests.

### Verification

- `make verify` runs unit, race, vet, formatting, perft, file-size, package
  coverage, integration coverage, and PTY resize gates.
- Release archives target Darwin amd64/arm64, Linux amd64/arm64, and Windows
  amd64. Verify `SHA256SUMS` before installing an archive.

[v0.1.0]: https://github.com/BSSE23029/chess-go/releases/tag/v0.1.0
