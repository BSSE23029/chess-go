package main

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestNamedBotLevelUsesEnvironmentProfile(t *testing.T) {
	clearChessEnv(t)
	t.Setenv("CHESS_BOT_LEVEL", "learner")
	t.Setenv("CHESS_PLAYER_COLOR", "black")
	var output bytes.Buffer
	if err := run(context.Background(), []string{"play", "bot"}, strings.NewReader("quit\n"), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Bot [Learner] played") {
		t.Fatalf("named bot level was not applied:\n%s", output.String())
	}
}

func TestNamedBotPersonalityUsesEnvironmentSeed(t *testing.T) {
	clearChessEnv(t)
	t.Setenv("CHESS_BOT_LEVEL", "learner")
	t.Setenv("CHESS_BOT_PERSONALITY", "trickster")
	t.Setenv("CHESS_BOT_SEED", "42")
	t.Setenv("CHESS_PLAYER_COLOR", "black")
	var output bytes.Buffer
	if err := run(context.Background(), []string{"play", "bot"}, strings.NewReader("quit\n"), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Bot [Learner/Trickster] played") {
		t.Fatalf("named bot personality was not applied:\n%s", output.String())
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

func TestLineModeAcceptsShortQuitCommand(t *testing.T) {
	clearChessEnv(t)
	var output bytes.Buffer
	if err := run(context.Background(), []string{"play", "local"}, strings.NewReader("q\n"), &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "Error:") {
		t.Fatalf("short quit was parsed as a move:\n%s", output.String())
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
		{"play", "bot", "--level", "unknown"},
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

func TestInteractiveSelectionPlaysLegalMove(t *testing.T) {
	s := session{game: chess.NewGame(), human: chess.White}
	e2, _ := chess.ParseSquare("e2")
	e4, _ := chess.ParseSquare("e4")
	ui := boardUI{cursor: e2, selected: chess.NoSquare}

	if s.handleKey(&ui, keySelect) || ui.selected != e2 {
		t.Fatalf("piece was not selected: %#v", ui)
	}
	ui.cursor = e4
	if s.handleKey(&ui, keySelect) || ui.selected != chess.NoSquare {
		t.Fatalf("move was not completed: %#v", ui)
	}
	if moves := s.game.Moves(); len(moves) != 1 || moves[0].UCI() != "e2e4" {
		t.Fatalf("interactive move history = %#v", moves)
	}

	s.handleKey(&ui, keyUndo)
	if s.game.Position().FEN() != chess.InitialFEN {
		t.Fatal("interactive undo did not restore the position")
	}
	s.handleKey(&ui, keyRedo)
	if len(s.game.Moves()) != 1 {
		t.Fatal("interactive redo did not restore the move")
	}
	s.handleKey(&ui, keyNew)
	if s.game.Position().FEN() != chess.InitialFEN || len(s.game.Moves()) != 0 {
		t.Fatal("interactive new game did not reset the session")
	}
}

func TestInteractiveNavigationAndKeyDecoding(t *testing.T) {
	d4, _ := chess.ParseSquare("d4")
	d5, _ := chess.ParseSquare("d5")
	if got := moveCursor(d4, keyUp, false); got != d5 {
		t.Fatalf("white up = %s", got)
	}
	if got := moveCursor(d4, keyDown, true); got != d5 {
		t.Fatalf("flipped down = %s", got)
	}
	a1, _ := chess.ParseSquare("a1")
	if got := moveCursor(a1, keyLeft, false); got != a1 {
		t.Fatalf("cursor escaped board to %s", got)
	}

	reader := bufio.NewReader(strings.NewReader("\x1b[A\x1b[B\x1b[C\x1b[D\r?:q"))
	want := []key{keyUp, keyDown, keyRight, keyLeft, keySelect, keyHelp, keyCommand, keyQuit}
	for index, expected := range want {
		got, err := readKey(reader)
		if err != nil || got != expected {
			t.Fatalf("key %d = %v, %v; want %v", index, got, err, expected)
		}
	}
	escape, err := readKey(bufio.NewReader(strings.NewReader("\x1b")))
	if err != nil || escape != keyEscape {
		t.Fatalf("standalone escape = %v, %v", escape, err)
	}
	escapeReader := bufio.NewReader(strings.NewReader("\x1bq"))
	if escape, err = readKey(escapeReader); err != nil || escape != keyEscape {
		t.Fatalf("escape prefix = %v, %v", escape, err)
	}
	if next, err := readKey(escapeReader); err != nil || next != keyQuit {
		t.Fatalf("key after escape = %v, %v", next, err)
	}
	var output bytes.Buffer
	line, err := readRawLine(bufio.NewReader(strings.NewReader("savx\x7fe game.pgn\r")), &output)
	if err != nil || line != "save game.pgn" || output.String() != "savx\b \be game.pgn\n" {
		t.Fatalf("raw command = %q, %v; output %q", line, err, output.String())
	}
}

func TestInteractiveRendererShowsDashboard(t *testing.T) {
	game := chess.NewGame()
	if err := game.PlayUCI("e2e4"); err != nil {
		t.Fatal(err)
	}
	ui := boardUI{cursor: chess.NoSquare, whiteName: "Ada", blackName: "HAL", message: "Played e4", showHelp: true}
	var output bytes.Buffer
	renderInteractive(&output, game, ui, false, "White 05:00 · Black 04:58 · +00:03", asciiTheme)
	text := output.String()
	for _, want := range []string{"CHESS-GO", "MATCH", "Ada", "HAL", "STATUS", "Black to move", "RECENT MOVES", "1. e2e4", "KEYBOARD", "LEGEND"} {
		if !strings.Contains(text, want) {
			t.Errorf("dashboard rendering lacks %q:\n%s", want, text)
		}
	}
}

func TestInteractiveEscapeClearsSelection(t *testing.T) {
	e2, _ := chess.ParseSquare("e2")
	ui := boardUI{cursor: e2, selected: e2, showHelp: true}
	s := session{game: chess.NewGame(), human: chess.White}
	if s.handleKey(&ui, keyEscape) {
		t.Fatal("escape unexpectedly quit")
	}
	if ui.selected != chess.NoSquare || ui.showHelp || ui.message != "Selection cleared" {
		t.Fatalf("escape state = %#v", ui)
	}
}

func TestTUITextKeepsTheRailBounded(t *testing.T) {
	if got := tuiText("  Ada\x1b[31m", 20); got != "Ada[31m" {
		t.Fatalf("sanitized label = %q", got)
	}
	if got := tuiText("abcdefgh", 5); got != "abcd…" {
		t.Fatalf("truncated label = %q", got)
	}
}

func TestInteractiveRendererHighlightsBoardState(t *testing.T) {
	game := chess.NewGame()
	if err := game.PlayUCI("e2e4"); err != nil {
		t.Fatal(err)
	}
	e7, _ := chess.ParseSquare("e7")
	e6, _ := chess.ParseSquare("e6")
	ui := boardUI{cursor: e6, selected: e7}
	var output bytes.Buffer
	renderInteractive(&output, game, ui, false, "White 05:00 · Black 05:00 · +00:03", unicodeTheme)
	text := output.String()
	for _, want := range []string{"\x1b[H\x1b[2J", "\x1b[7m", "\x1b[46m", "\x1b[42m", "\x1b[43m", "8 ", "  a  b  c  d  e  f  g  h", "White 05:00", "Black to move"} {
		if !strings.Contains(text, want) {
			t.Errorf("interactive rendering lacks %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "♜") || !strings.Contains(text, "♙") {
		t.Fatal("unicode theme did not render chess symbols")
	}
}

func TestThemeConfiguration(t *testing.T) {
	for _, value := range []string{"ascii", "letters", "unicode", "symbols", ""} {
		if _, err := parseTheme(value); err != nil {
			t.Errorf("parseTheme(%q): %v", value, err)
		}
	}
	if _, err := parseTheme("neon"); err == nil {
		t.Fatal("unknown theme accepted")
	}
	clearChessEnv(t)
	t.Setenv("CHESS_THEME", "unicode")
	var output bytes.Buffer
	if err := run(context.Background(), []string{"play", "local"}, strings.NewReader("quit\n"), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "♜") {
		t.Fatal("environment theme did not affect scripted rendering")
	}
}

func TestCapturedPieceSummary(t *testing.T) {
	position, _ := chess.ParseFEN("7k/8/8/3pP3/8/8/8/K7 w - d6 0 2")
	game := chess.NewGameFromPosition(position)
	if err := game.PlayUCI("e5d6"); err != nil {
		t.Fatal(err)
	}
	if got := capturedSummary(game); got != "Captured by White: p · Black: -" {
		t.Fatalf("captured summary = %q", got)
	}
}

func TestChessClockLifecycle(t *testing.T) {
	clock, err := parseClock("10s", "2s")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	clock.now = func() time.Time { return now }
	clock.start(chess.White)
	now = now.Add(3 * time.Second)
	if got := clock.values(); got != ([2]time.Duration{7 * time.Second, 10 * time.Second}) {
		t.Fatalf("running values = %v", got)
	}
	if !clock.completeMove(chess.White) {
		t.Fatal("white incorrectly lost on time")
	}
	if got := clock.values(); got != ([2]time.Duration{9 * time.Second, 10 * time.Second}) {
		t.Fatalf("incremented values = %v", got)
	}
	now = now.Add(10 * time.Second)
	if clock.completeMove(chess.Black) {
		t.Fatal("black move completed after flag fall")
	}
	if got := formatClock(1500 * time.Millisecond); got != "00:02" {
		t.Fatalf("rounded clock = %q", got)
	}
}

func TestClockConfigurationFromFlagsAndEnvironment(t *testing.T) {
	clearChessEnv(t)
	t.Setenv("CHESS_CLOCK", "1m")
	t.Setenv("CHESS_INCREMENT", "2s")
	var output bytes.Buffer
	if err := run(context.Background(), []string{"play", "local"}, strings.NewReader("quit\n"), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "White 01:00 · Black 01:00 · +00:02") {
		t.Fatalf("configured clocks not rendered:\n%s", output.String())
	}
	for _, args := range [][]string{
		{"play", "local", "--clock", "fast"},
		{"play", "local", "--increment", "1s"},
		{"play", "bot", "--clock", "1m", "--increment", "-1s"},
	} {
		clearChessEnv(t)
		if err := run(context.Background(), args, strings.NewReader(""), &bytes.Buffer{}); err == nil {
			t.Errorf("run(%q) accepted invalid clock", args)
		}
	}
}

func TestHostTLSFlagsRequireCertificatePair(t *testing.T) {
	if err := run(context.Background(), []string{"host", "--cert", "server.crt"}, strings.NewReader(""), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "must be provided together") {
		t.Fatalf("host TLS validation error = %v", err)
	}
}

func TestVersionCommand(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), []string{"version"}, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "chess-go dev" {
		t.Fatalf("version output = %q", output.String())
	}
}

func clearChessEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"CHESS_BOT_DEPTH", "CHESS_BOT_LEVEL", "CHESS_BOT_PERSONALITY", "CHESS_BOT_SEED", "CHESS_PLAYER_COLOR", "CHESS_PLAYER_NAME", "CHESS_BOT_NAME", "CHESS_CLOCK", "CHESS_INCREMENT", "CHESS_THEME", "CHESS_NETWORK_ADDR", "CHESS_NETWORK_URL", "CHESS_NETWORK_TOKEN", "CHESS_MATCH_ID", "CHESS_PLAYER_ID", "CHESS_TLS_CERT", "CHESS_TLS_KEY", "CHESS_MATCH_STORE", "CHESS_LAN_DISCOVERY", "CHESS_LAN_INSTANCE", "CHESS_LAN_HOST"} {
		t.Setenv(name, "")
	}
}
