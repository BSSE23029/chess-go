# Troubleshooting

## Pieces look small

Terminal programs cannot change the terminal font size. Unicode
`CHESS_PIECE_STYLE=auto` selects centered `TEXT` chess glyphs on normal
dashboards and uses scalable half-block pieces only in genuinely wide/tall
cells. `CHESS_PIECE_STYLE=sprite` opts into the scalable mode explicitly.
Compact viewports keep centered one-cell text glyphs and show a compact-viewport notice. Make
the terminal wider or use the launcher Settings screen to select a style.

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
