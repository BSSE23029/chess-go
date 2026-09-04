#!/bin/sh
set -eu

cache="${GOCACHE:-/tmp/chess-go-build-cache}"
export GOCACHE="$cache"

go test ./...
go test -race ./...
go vet ./...

format_errors=""
for file in $(rg --files -g '*.go'); do
	formatted=$(gofmt -l "$file")
	if [ -n "$formatted" ]; then
		format_errors="$format_errors\n$formatted"
	fi
done
if [ -n "$format_errors" ]; then
	echo "gofmt required:$format_errors" >&2
	exit 1
fi

perft=$(go run ./cmd/perft --depth 4)
if [ "$perft" != "197281" ]; then
	echo "perft regression: got $perft, want 197281" >&2
	exit 1
fi
sh scripts/check-file-size.sh
