SHELL := /bin/sh

GO ?= go
DIST ?= dist
VERSION ?= dev
BUILD_FLAGS := -trimpath -buildvcs=false
LDFLAGS := -s -w -buildid= -X main.version=$(VERSION)

.PHONY: test race vet fmt perft file-size verify build release

test:
	GOCACHE=$${GOCACHE:-/tmp/chess-go-build-cache} $(GO) test ./...

race:
	GOCACHE=$${GOCACHE:-/tmp/chess-go-build-cache} $(GO) test -race ./...

vet:
	GOCACHE=$${GOCACHE:-/tmp/chess-go-build-cache} $(GO) vet ./...

fmt:
	@files=$$(rg --files -g '*.go'); test -z "$$(printf '%s\n' "$$files" | xargs gofmt -l)"

perft:
	@test "$$(GOCACHE=$${GOCACHE:-/tmp/chess-go-build-cache} $(GO) run ./cmd/perft --depth 4)" = 197281

file-size:
	@sh scripts/check-file-size.sh

verify:
	@sh scripts/verify.sh

build:
	@mkdir -p "$(DIST)"
	@GOCACHE=$${GOCACHE:-/tmp/chess-go-build-cache} CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o "$(DIST)/chess" ./cmd/chess

release: verify build
