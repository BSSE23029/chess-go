#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
output_dir=${1:-}
work=$(mktemp -d "${TMPDIR:-/tmp}/chess-go-cover.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
binary="$work/chess"
cover_dir="$work/cover"
mkdir -p "$cover_dir"

cd "$root"
# The matrix is a deterministic release check. Runtime overrides remain
# supported by the application, but a developer's shell must not change which
# documented paths are exercised here.
unset CHESS_BOT_DEPTH CHESS_BOT_LEVEL CHESS_BOT_PERSONALITY CHESS_BOT_SEED CHESS_BOT_RANDOM
unset CHESS_PLAYER_COLOR CHESS_PLAYER_NAME CHESS_BOT_NAME CHESS_CLOCK CHESS_INCREMENT CHESS_THEME CHESS_PIECE_STYLE
unset CHESS_NETWORK_ADDR CHESS_NETWORK_URL CHESS_NETWORK_TOKEN CHESS_NETWORK_FORMAT CHESS_NETWORK_INSECURE CHESS_MATCH_ID CHESS_PLAYER_ID
unset CHESS_TLS_CERT CHESS_TLS_KEY CHESS_TLS_CA CHESS_TLS_CLIENT_CERT CHESS_TLS_CLIENT_KEY CHESS_MATCH_STORE
unset CHESS_LAN_DISCOVERY CHESS_LAN_INSTANCE CHESS_LAN_HOST
go build -cover -trimpath -buildvcs=false -o "$binary" ./cmd/chess

host_pid=""
cleanup() {
	status=$?
	if [ -n "$host_pid" ]; then
		kill "$host_pid" 2>/dev/null || true
		wait "$host_pid" 2>/dev/null || true
	fi
	rm -rf "$work"
	exit "$status"
}
trap cleanup EXIT HUP INT TERM

run_case() {
	GOCOVERDIR="$cover_dir" "$binary" "$@" >/dev/null 2>&1
}

run_case version
run_case help
run_case play local --help
run_case play bot --help
run_case play remote --help
run_case host --help
run_case join --help
run_case connect --help
run_case spectate --help
run_case matchmake --help
run_case list --help
run_case discover --help
run_case load --help

printf 'quit\n' | GOCOVERDIR="$cover_dir" "$binary" play local >/dev/null 2>&1 || true
printf 'quit\n' | GOCOVERDIR="$cover_dir" "$binary" play bot --depth 1 --random=false >/dev/null 2>&1 || true

pgn="$work/sample.pgn"
printf '[Event "coverage"]\n\n1. e4 *\n' >"$pgn"
printf 'quit\n' | GOCOVERDIR="$cover_dir" "$binary" load "$pgn" >/dev/null 2>&1

# Drive the documented network operations through the instrumented binary, not
# only through package tests. A loopback, plaintext server keeps this harness
# self-contained; TLS behavior has its own certificate-backed test coverage.
host_log="$work/host.log"
GOCOVERDIR="$cover_dir" "$binary" host --addr 127.0.0.1:0 --insecure >"$host_log" 2>&1 &
host_pid=$!
host_address=""
attempt=0
while [ "$attempt" -lt 100 ]; do
	if [ -s "$host_log" ]; then
		host_address=$(sed -n 's/^Hosting chess server on //p' "$host_log" | head -n 1)
		if [ -n "$host_address" ]; then
			break
		fi
	fi
	if ! kill -0 "$host_pid" 2>/dev/null; then
		cat "$host_log" >&2
		exit 1
	fi
	attempt=$((attempt + 1))
	sleep 0.05
done
if [ -z "$host_address" ]; then
	cat "$host_log" >&2
	exit 1
fi
server_url="http://$host_address"

run_case list "$server_url"
printf 'quit\n' | GOCOVERDIR="$cover_dir" "$binary" play remote "$server_url" --match coverage-remote --create --player white --color white >/dev/null 2>&1
run_case join "$server_url" --match coverage-remote --player black --color black
run_case connect "$server_url" --match coverage-remote --player watcher --color spectator
run_case spectate "$server_url" --match coverage-remote --player spectator --color spectator
printf 'quit\n' | GOCOVERDIR="$cover_dir" "$binary" play remote "$server_url" --match coverage-connect --create --player creator --color white >/dev/null 2>&1
run_case connect "$server_url" --match coverage-connect --player connector --color black
run_case matchmake "$server_url" --player waiting --color random
run_case list "$server_url"

# Exercise the real raw-terminal launcher/game path when the host provides the
# standard BSD/macOS script form. Unit tests still cover rendering on systems
# without a PTY helper.
if script -q "$work/probe" sh -c 'exit 0' >/dev/null 2>&1; then
	printf 'q' | script -q "$work/menu.raw" sh -c "stty cols 60 rows 30; GOCOVERDIR='$cover_dir' '$binary' menu" >/dev/null 2>&1 || true
	printf 'q' | script -q "$work/game.raw" sh -c "stty cols 106 rows 30; GOCOVERDIR='$cover_dir' '$binary' play local --theme unicode" >/dev/null 2>&1 || true
	test -s "$work/menu.raw"
	test -s "$work/game.raw"
fi

# Flush the host process's instrumented coverage before converting the data.
kill "$host_pid" 2>/dev/null || true
wait "$host_pid" 2>/dev/null || true
host_pid=""

if [ -n "$output_dir" ]; then
	mkdir -p "$output_dir"
	go tool covdata textfmt -i="$cover_dir" -o="$output_dir/coverage.out"
fi
summary=$(go tool covdata percent -i="$cover_dir")
printf '%s\n' "$summary"
printf '%s\n' "$summary" | grep -q 'chess-go/cmd/chess'
