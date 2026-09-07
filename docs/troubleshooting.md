# Troubleshooting

## Pieces look small

Terminal programs cannot resize one Unicode glyph independently of the rest of
the terminal grid. Unicode defines character width properties, but a terminal
still places text in fixed-size cells; the selected font controls the glyph's
actual pixels. `CHESS_PIECE_STYLE=auto` therefore keeps recognizable,
centered chess glyphs at every size. `CHESS_PIECE_STYLE=sprite` opts into an
experimental half-block pixel treatment when a larger silhouette matters more
than looking like a font-rendered chess symbol. Use the launcher Settings
screen to select a style.

True per-glyph font scaling is terminal-specific. [Kitty's text-sizing
protocol](https://github.com/kovidgoyal/kitty/blob/master/docs/text-sizing-protocol.rst)
and [graphics protocol](https://github.com/kovidgoyal/kitty/blob/master/docs/graphics-protocol.rst)
can render larger text or images in a cell rectangle, while [iTerm2's inline
image protocol](https://iterm2.com/documentation-images.html) exposes a
separate image path. Those features are not portable to macOS Terminal,
Windows Terminal, or basic SSH/tmux sessions. The portable default is
therefore centered Unicode text, with the experimental multi-cell sprite
available explicitly and literal text available through
`CHESS_PIECE_STYLE=text`.

## The board looks stretched

Terminal columns are narrower than terminal rows. The renderer therefore caps
wide-board cells at roughly twice their row height and reselects that tier after
every resize. If a terminal uses an unusual font aspect ratio, set a larger
font/zoom or choose `CHESS_PIECE_STYLE=text` for the most predictable alignment.

## The board shifts or clips after resizing

Resize the terminal once more after changing its font or zoom. The renderer
queries the live width and height and redraws on supported resize signals. If
the terminal reports an unusual size, use the ASCII theme:

```console
CHESS_THEME=ascii CHESS_PIECE_STYLE=text go run ./cmd/chess play local
```

## Colors are unreadable

Set `NO_COLOR=1` to disable ANSI colors. The board and status text remain
usable in monochrome terminals.

## Environment settings are ignored

The binary does not load dotenv files implicitly. Export the values or source a
local file explicitly:

```console
set -a; . ./.env; set +a
go run ./cmd/chess menu
```

Use `chess help`, `chess <command> --help`, or the launcher Help item for the
complete command and setting list.

## Network TLS failures

Use `CHESS_TLS_CA` for a private CA and provide both
`CHESS_TLS_CLIENT_CERT` and `CHESS_TLS_CLIENT_KEY` for mTLS. Keep
`CHESS_NETWORK_INSECURE` disabled except for an explicitly local development
server.
