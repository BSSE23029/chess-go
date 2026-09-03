package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"chess-go"
	"chess-go/engine"
)

func TestLocalGameLifecycleAndSave(t *testing.T) {
	clearChessEnv(t)
	path := filepath.Join(t.TempDir(), "game.pgn")
	input := strings.NewReader("e4\ne5\nundo\nredo\nmoves\nsave " + path + "\nquit\n")
	var output bytes.Buffer
	if err := run(context.Background(), []string{"play", "local"}, input, &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "8 r n b q k b n r") || !strings.Contains(text, "Moves: e2e4 e7e5") {
		t.Fatalf("terminal did not render board and history:\n%s", text)
	}
	if !strings.Contains(text, "Nf3(g1f3)") || !strings.Contains(text, "Saved "+path) {
		t.Fatalf("terminal commands did not complete:\n%s", text)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	game, err := chess.ParsePGN(string(data))
	if err != nil || len(game.Moves()) != 2 {
		t.Fatalf("saved PGN is invalid: %v", err)
	}
}

func TestBotGameUsesEnvironmentConfiguration(t *testing.T) {
	clearChessEnv(t)
	t.Setenv("CHESS_BOT_DEPTH", "1")
	t.Setenv("CHESS_PLAYER_COLOR", "black")
	t.Setenv("CHESS_PLAYER_NAME", "Ada")
	t.Setenv("CHESS_BOT_NAME", "HAL")
	var output bytes.Buffer
	if err := run(context.Background(), []string{"play", "bot"}, strings.NewReader("quit\n"), &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "HAL played") || !strings.Contains(text, "Ada to move >") {
		t.Fatalf("environment configuration was not applied:\n%s", text)
	}
	if !strings.Contains(text, "  h g f e d c b a") {
		t.Fatalf("black player's board was not flipped:\n%s", text)
	}
}

func TestFENReplacementCompletesGame(t *testing.T) {
	clearChessEnv(t)
	input := strings.NewReader("fen 7k/8/8/8/8/8/8/K7 w - - 0 1\n")
	var output bytes.Buffer
	if err := run(context.Background(), []string{"play", "local"}, input, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Game over: 1/2-1/2") {
		t.Fatalf("FEN game result missing:\n%s", output.String())
	}
}

func TestLoadPGNAndContinue(t *testing.T) {
	clearChessEnv(t)
	path := filepath.Join(t.TempDir(), "opening.pgn")
	if err := os.WriteFile(path, []byte("[Result \"*\"]\n\n1. e4 e5 *\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{"load", path}, strings.NewReader("Nf3\nquit\n"), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Moves: e2e4 e7e5 g1f3") {
		t.Fatalf("loaded game was not continued:\n%s", output.String())
	}
}

func TestInvalidInputIsRejectedWithoutEndingSession(t *testing.T) {
	clearChessEnv(t)
	var output bytes.Buffer
	if err := run(context.Background(), []string{"play", "local"}, strings.NewReader("e9e4\nquit\n"), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Error:") || !strings.Contains(output.String(), "Moves:\n") {
		t.Fatalf("invalid move handling is incorrect:\n%s", output.String())
	}
}

func TestCommandValidation(t *testing.T) {
	clearChessEnv(t)
	tests := [][]string{
		nil,
		{"play"},
		{"play", "unknown"},
		{"play", "local", "extra"},
		{"play", "bot", "--depth", "0"},
		{"play", "bot", "--color", "red"},
		{"load"},
	}
	for _, args := range tests {
		if err := run(context.Background(), args, strings.NewReader(""), &bytes.Buffer{}); err == nil {
			t.Errorf("run(%q) succeeded", args)
		}
	}
	t.Setenv("CHESS_BOT_DEPTH", "fast")
	if err := run(context.Background(), []string{"play", "bot"}, strings.NewReader(""), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "CHESS_BOT_DEPTH") {
		t.Fatalf("invalid environment error = %v", err)
	}
}

func TestBotUndoRedoTravelsCompleteTurnsAtomically(t *testing.T) {
	s := session{game: chess.NewGame(), bot: engine.New(1), human: chess.White}
	for _, move := range []string{"e2e4", "e7e5"} {
		if err := s.game.PlayUCI(move); err != nil {
			t.Fatal(err)
		}
	}
	after := s.game.Position().FEN()
	if err := s.travel(false); err != nil || len(s.game.Moves()) != 0 {
		t.Fatalf("paired undo failed: %v", err)
	}
	if err := s.travel(true); err != nil || s.game.Position().FEN() != after {
		t.Fatalf("paired redo failed: %v", err)
	}

	s = session{game: chess.NewGame(), bot: engine.New(1), human: chess.White}
	if err := s.game.PlayUCI("e2e4"); err != nil {
		t.Fatal(err)
	}
	s.game.Undo()
	if err := s.travel(true); err == nil {
		t.Fatal("incomplete paired redo succeeded")
	}
	if s.game.Position().FEN() != chess.InitialFEN {
		t.Fatal("failed paired redo partially changed position")
	}
}

func clearChessEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"CHESS_BOT_DEPTH", "CHESS_PLAYER_COLOR", "CHESS_PLAYER_NAME", "CHESS_BOT_NAME"} {
		t.Setenv(name, "")
	}
}
