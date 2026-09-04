# Architecture decision

## Keep one repository for now

The project remains a Go monorepo while the public APIs and wire protocol are
still evolving. Splitting immediately would duplicate versioning, test fixtures,
release automation, and cross-package changes without improving the current
deployment model.

The package boundaries are explicit:

| Area | Packages | Boundary |
| --- | --- | --- |
| Rules and engine | root, `engine`, `perft` | no terminal or network imports |
| Terminal application | `cmd/chess` | consumes rules/engine and transport clients |
| Network authority | `protocol`, `transport`, `storage`, `lan` | server validates through the root rules package |
| Calibration | `tournament`, `cmd/tournament` | consumes `chess.Player`, including UCI adapters |

The JSON schema is language-neutral, so a future network implementation can be
split without changing clients. Consider separate `chess-core-go`,
`chess-tui-go`, and `chess-network-go` repositories only after a stable tagged
protocol, independent release cadence, and external consumers justify the
coordination cost.
