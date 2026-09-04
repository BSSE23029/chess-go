# Releasing

Release binaries are built with a fixed Go toolchain from `go.mod`, disabled
CGO, `-trimpath`, disabled VCS stamping, an empty build ID, and an explicit
version string. These settings make repeated builds from the same source and
toolchain byte-for-byte comparable.

Run the full gate and build a candidate:

```console
VERSION=v0.1.0 make release
sha256sum dist/chess
```

After review, create and push a signed or otherwise policy-approved tag through
the project owner's normal repository workflow:

```console
git tag -a v0.1.0 -m "chess-go v0.1.0"
git show --stat v0.1.0
```

Do not claim a public release until the owner has selected a license, chosen the
canonical repository URL, and published the generated checksums alongside the
binary. External UCI engines such as Stockfish remain separately installed and
licensed; configure their executable through `CHESS_UCI_ENGINE`.
