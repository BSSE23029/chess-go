SHELL := /bin/sh

GO ?= go
DIST ?= dist
VERSION ?= dev
BUILD_FLAGS := -trimpath -buildvcs=false
LDFLAGS := -s -w -buildid= -X main.version=$(VERSION)

.PHONY: test race vet fmt perft file-size bench profile verify build release release-all

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

bench:
	GOCACHE=$${GOCACHE:-/tmp/chess-go-build-cache} $(GO) test -run '^$$' -bench . -benchmem ./engine ./cmd/chess

profile:
	@mkdir -p "$(DIST)/profiles"
	@GOCACHE=$${GOCACHE:-/tmp/chess-go-build-cache} $(GO) test -run '^$$' -bench '^BenchmarkSearchDepth3$$' -benchtime=5s -cpuprofile "$(DIST)/profiles/engine.cpu.pprof" -memprofile "$(DIST)/profiles/engine.mem.pprof" ./engine
	@GOCACHE=$${GOCACHE:-/tmp/chess-go-build-cache} $(GO) test -run '^$$' -bench '^BenchmarkInteractiveRender$$' -benchtime=5s -cpuprofile "$(DIST)/profiles/tui.cpu.pprof" -memprofile "$(DIST)/profiles/tui.mem.pprof" ./cmd/chess

verify:
	@sh scripts/verify.sh

build:
	@mkdir -p "$(DIST)"
	@GOCACHE=$${GOCACHE:-/tmp/chess-go-build-cache} CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o "$(DIST)/chess" ./cmd/chess

release: verify build

release-all: verify
	VERSION=$(VERSION) DIST=$(DIST) sh scripts/release.sh
