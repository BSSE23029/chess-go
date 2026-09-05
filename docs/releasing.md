# Releasing

Release binaries are built with the Go toolchain from `go.mod`, disabled CGO,
`-trimpath`, disabled VCS stamping, an empty build ID, and an explicit version
string. These settings make repeated binaries from the same source and
toolchain byte-for-byte comparable.

Run the full test, command-coverage, and PTY gate before building a candidate:

```console
VERSION=v0.1.0 make release
sha256sum dist/chess
```

To create cross-platform archives and a checksum manifest, use the release
packager. It targets macOS, Linux, and Windows by default:

```console
VERSION=v0.1.0 make release-all
find dist/releases/v0.1.0 -maxdepth 1 -type f -print
cat dist/releases/v0.1.0/SHA256SUMS
```

Override the output directory with `DIST` or the target matrix with
`RELEASE_TARGETS`, for example `RELEASE_TARGETS="linux/amd64 linux/arm64"`.
The tag-triggered GitHub Actions workflow runs this same target and publishes
the archives only after the repository owner creates a `v*` tag.

After review, create and push a signed or otherwise policy-approved tag through
the project owner's normal repository workflow:

```console
git tag -a v0.1.0 -m "chess-go v0.1.0"
git show --stat v0.1.0
```

Do not claim a public release until the owner has selected a license, chosen the
canonical module/repository URL, reviewed the generated checksums, and
published the release notes. External UCI engines such as Stockfish remain
separately installed and licensed; configure their executable through
`CHESS_UCI_ENGINE`.
