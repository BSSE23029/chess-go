package main

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"chess-go"
	"chess-go/protocol"
	"chess-go/transport"
)

func newNetworkTestServer(t *testing.T, token string) (*httptest.Server, *protocol.Server) {
	t.Helper()
	authority := protocol.NewServer()
	server := httptest.NewServer(transport.NewHTTPServer(authority, token))
	t.Cleanup(server.Close)
	return server, authority
}

func TestNetworkCommandsExerciseServiceLifecycle(t *testing.T) {
	clearChessEnv(t)
	server, _ := newNetworkTestServer(t, "secret")
	ctx := context.Background()
	var output bytes.Buffer

	if err := runNetworkCommand(ctx, "matchmake", []string{server.URL, "--player", "alice", "--color", "white", "--token", "secret", "--clock-millis", "60000", "--increment-millis", "1000"}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Match match-1") || !strings.Contains(output.String(), "White 60000ms") {
		t.Fatalf("matchmake output = %q", output.String())
	}
	output.Reset()
	if err := runNetworkCommand(ctx, "list", []string{server.URL, "--token", "secret"}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "match-1") {
		t.Fatalf("list output = %q", output.String())
	}
	output.Reset()
	if err := runNetworkCommand(ctx, "join", []string{server.URL, "--match", "match-1", "--player", "bob", "--color", "black", "--token", "secret"}, &output); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := runNetworkCommand(ctx, "spectate", []string{server.URL, "--match", "match-1", "--player", "viewer", "--token", "secret"}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Match match-1") {
		t.Fatalf("spectate output = %q", output.String())
	}
	if err := runNetworkCommand(ctx, "join", []string{server.URL, "--match", "missing", "--player", "nobody", "--color", "white", "--token", "secret"}, &output); err == nil {
		t.Fatal("missing match unexpectedly succeeded")
	}
	if err := runNetworkCommand(ctx, "join", []string{server.URL}, &output); err == nil {
		t.Fatal("incomplete join unexpectedly succeeded")
	}
	if err := runNetworkCommand(ctx, "list", []string{server.URL, "--token", "wrong"}, &output); err == nil {
		t.Fatal("unauthorized list unexpectedly succeeded")
	}
}

func TestRemoteCommandExercisesMoveRefreshDrawAndResign(t *testing.T) {
	clearChessEnv(t)
	server, _ := newNetworkTestServer(t, "secret")
	ctx := context.Background()
	var output bytes.Buffer
	input := strings.NewReader("help\ne2e4\nrefresh\ne7e5\ndraw\nquit\n")
	if err := runRemote(ctx, []string{server.URL, "--match", "remote", "--player", "alice", "--color", "white", "--token", "secret", "--create"}, input, &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"Remote commands:", "waiting for the opponent", "remote >"} {
		if !strings.Contains(text, want) {
			t.Fatalf("remote output missing %q:\n%s", want, text)
		}
	}

	output.Reset()
	if err := runRemote(ctx, []string{server.URL, "--match", "remote", "--player", "bob", "--color", "black", "--token", "secret"}, strings.NewReader("resign\n"), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Game over: 1-0") {
		t.Fatalf("resign output = %q", output.String())
	}
}

func TestRemoteSpectatorCannotMove(t *testing.T) {
	clearChessEnv(t)
	server, _ := newNetworkTestServer(t, "secret")
	ctx := context.Background()
	if err := runRemote(ctx, []string{server.URL, "--match", "spectator", "--player", "alice", "--color", "white", "--token", "secret", "--create"}, strings.NewReader("quit\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runRemote(ctx, []string{server.URL, "--match", "spectator", "--player", "viewer", "--color", "spectator", "--token", "secret"}, strings.NewReader("e2e4\nquit\n"), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "spectators cannot move") {
		t.Fatalf("spectator output = %q", output.String())
	}
}

func TestHostAndDiscoveryCommandLifecycle(t *testing.T) {
	clearChessEnv(t)
	var output bytes.Buffer
	if err := runHost(context.Background(), []string{"--help"}, &output); err != nil || !strings.Contains(output.String(), "--lan-instance") {
		t.Fatalf("host help err=%v output=%q", err, output.String())
	}
	for _, args := range [][]string{{}, {"--cert", "cert.pem"}, {"--insecure", "--cert", "cert.pem", "--key", "key.pem"}} {
		if err := runHost(context.Background(), args, &bytes.Buffer{}); err == nil {
			t.Fatalf("host args %#v unexpectedly succeeded", args)
		}
	}
	if err := runDiscover(context.Background(), []string{"--seconds", "0"}, &bytes.Buffer{}); err == nil {
		t.Fatal("zero-second discovery unexpectedly succeeded")
	}
	if err := runDiscover(context.Background(), []string{"--help"}, &output); err != nil || !strings.Contains(output.String(), "--seconds") {
		t.Fatalf("discover help err=%v output=%q", err, output.String())
	}

	storePath := filepath.Join(t.TempDir(), "matches.json")
	writer := &notifyingWriter{ready: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runHost(ctx, []string{"--addr", "127.0.0.1:0", "--insecure", "--store", storePath}, writer)
	}()
	select {
	case <-writer.ready:
		cancel()
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("host did not start listening")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("host shutdown error = %v", err)
	}
	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("host did not persist store: %v", err)
	}
}

type notifyingWriter struct {
	mu    sync.Mutex
	data  bytes.Buffer
	ready chan struct{}
}

func (w *notifyingWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.data.Write(value)
	select {
	case w.ready <- struct{}{}:
	default:
	}
	return n, err
}

func TestNetworkParsingAndSnapshotFallbacks(t *testing.T) {
	for _, test := range []struct {
		value string
		want  chess.Color
		ok    bool
	}{
		{value: "white", want: chess.White, ok: true},
		{value: "BLACK", want: chess.Black, ok: true},
		{value: "watcher", want: remoteSpectator, ok: true},
		{value: "green", ok: false},
	} {
		got, err := parseRemoteColor(test.value)
		if (err == nil) != test.ok || test.ok && got != test.want {
			t.Fatalf("parseRemoteColor(%q) = %v, %v; want %v, ok=%v", test.value, got, err, test.want, test.ok)
		}
	}
	if colorNameLower(chess.White) != "white" || colorNameLower(chess.Black) != "black" {
		t.Fatal("colorNameLower returned the wrong names")
	}
	if !envBool("CHESS_MISSING_BOOL", true) {
		t.Fatal("envBool did not use its fallback")
	}
	t.Setenv("CHESS_MISSING_BOOL", "off")
	if envBool("CHESS_MISSING_BOOL", true) {
		t.Fatal("envBool did not parse false")
	}

	position := chess.NewGame().Position()
	fen := position.FEN()
	for _, test := range []struct {
		name     string
		snapshot protocol.MatchSnapshot
		wantFEN  string
		wantErr  bool
	}{
		{name: "fen", snapshot: protocol.MatchSnapshot{FEN: fen}, wantFEN: fen},
		{name: "bad move fallback", snapshot: protocol.MatchSnapshot{Moves: []string{"bad"}, FEN: fen}, wantFEN: fen},
		{name: "hash fallback", snapshot: protocol.MatchSnapshot{FEN: fen, PositionHash: 1}, wantFEN: fen},
		{name: "invalid fen", snapshot: protocol.MatchSnapshot{FEN: "not-fen"}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			game, err := gameFromSnapshot(test.snapshot)
			if test.wantErr {
				if err == nil {
					t.Fatal("invalid snapshot unexpectedly succeeded")
				}
				return
			}
			if err != nil || game.Position().FEN() != test.wantFEN {
				t.Fatalf("game=%v err=%v", game, err)
			}
		})
	}
}
