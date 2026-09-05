# User-facing coverage matrix

Chess-go treats coverage as an operation contract: every supported command,
launcher form, keyboard path, validation failure, and terminal layout must have
an automated check at the smallest boundary that proves the behavior. The
matrix below is the release checklist; generated example programs are not
counted as application behavior.

| Surface | Covered by | Evidence |
|---|---|---|
| `version`, top-level help, every subcommand help | `cmd/chess/main_test.go`, instrumented binary | `TestSubcommandHelpListsAllFlags`, `scripts/coverage-integration.sh` |
| Local play, clocks, SAN/UCI, undo/redo, save/load, draw commands | command lifecycle tests | `TestLocalGameLifecycleAndSave`, `TestCommandPaletteCoversDocumentedOperations` |
| Bot depth/level/personality/color/seed/randomness | config and bot tests | `TestLauncherFormsBuildCLIArguments`, `TestRandomBotSamplesNearBestMoves` |
| Remote create/join/move/refresh/draw/resign/spectate | real HTTP integration tests | `network_test.go` lifecycle tests with `httptest` service |
| Host validation, shutdown, persistence, discovery validation | host/discovery tests | `TestHostAndDiscoveryCommandLifecycle` |
| TLS, bearer authorization, JSON/protobuf settings | transport and launcher settings tests | `TestLauncherSettingsUpdatesEnvironmentBackedOptions` and transport package tests |
| Launcher root and network menus | registry, form, and action tests | `TestCommandRegistryDrivesLauncherAndHelp`, `launcher_forms_test.go` |
| Keyboard movement, selection, promotion, help, confirmation, commands | TUI state-machine tests | `TestInteractiveKeyboardStateMachineCoversGuidesAndConfirmation`, promotion tests |
| Narrow, compact, normal, wide, and resized terminal layouts | renderer tests and PTY harness | `TestResponsiveFramesStayWithinTerminalViewport`, raw cases in `scripts/coverage-integration.sh` |
| Engine legality, tactical positions, draw semantics, performance | engine and game suites | `engine/strength_test.go`, FIDE rule tests, benchmark suite |

## Release commands

```console
make coverage          # package-level line report
make coverage-integration
make coverage-gate     # both reports plus real-binary command/PTY cases
make verify            # tests, race, vet, formatting, perft, size, and gate
```

`make coverage-gate` is required by `make verify` and therefore by release
targets. The integration script fails on an unexpected command error; it uses
Go's `GOCOVERDIR` for actual-process evidence and drives a raw terminal when
the host provides the BSD/macOS `script` helper.

When adding a command or flag, update the single registry/config form, its CLI
help and this matrix, then add both a form test and an integration case. Keep
the machine- and user-specific values in environment variables or prompts;
never encode local identities, paths, or credentials in the tests.
