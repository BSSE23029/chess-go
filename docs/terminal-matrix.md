# Terminal compatibility matrix

The renderer has two kinds of coverage:

| Environment | Automated coverage | Manual check |
|---|---|---|
| Linux terminal with a PTY | `make coverage-integration` exercises 80×24, 95×24, 100×30, 106×30, 120×30, and 213×60 frames plus a live resize. | Run the resize commands below with your terminal font. |
| macOS Terminal.app | CI compiles and tests `cmd/chess` on `macos-latest`. | Check Unicode text, wide icon mode, and resize in Terminal.app. |
| iTerm2 / WezTerm / Kitty | Renderer tests cover layout and emoji-width fallbacks; the local font is not available in CI. | Check `CHESS_PIECE_STYLE=emoji` and resize. |
| Windows Terminal | CI compiles and tests the Windows resize build tag. | Check redraw after dragging the window and use ASCII if the font reports unusual widths. |

## Manual resize check

Start a game in a real terminal:

```console
go run ./cmd/chess play local --theme unicode
```

Resize through these approximate viewport tiers while the game is running:

```text
80×24   compact text glyphs and stacked status rail
106×30  dashboard text glyphs with a side rail
120×30  larger dashboard text glyphs
213×60  proportional board cells with scalable icons
```

The header reports `TEXT` or `ICONS`. If the font makes chess glyphs look
misaligned, use the stable text renderer or the ASCII theme:

```console
CHESS_PIECE_STYLE=text go run ./cmd/chess play local --theme unicode
go run ./cmd/chess play local --theme ascii
```

Terminal columns are not pixels. The renderer caps a square at roughly twice
the row height, so a wide terminal can have unused horizontal space while the
board remains visually square.
