package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"chess-go"
	"chess-go/engine"
	"chess-go/protocol"
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
	if !strings.Contains(text, "8 ♜ ♞ ♝ ♛ ♚ ♝ ♞ ♜") || !strings.Contains(text, "Moves: e2e4 e7e5") {
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

func TestClaimDrawCommandUsesFIDERules(t *testing.T) {
	position, err := chess.ParseFEN("7k/8/8/8/8/8/8/R5K1 w - - 100 1")
	if err != nil {
		t.Fatal(err)
	}
	s := session{game: chess.NewGameFromPosition(position)}
	if err := s.command("claim draw", io.Discard); err != nil {
		t.Fatal(err)
	}
	if s.timeout != "Draw claimed under FIDE rules" || s.game.ResultFIDE() != "1/2-1/2" {
		t.Fatalf("claim state = %q, result = %q", s.timeout, s.game.ResultFIDE())
	}
}

func TestTopLevelHelp(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), []string{"help"}, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Usage: chess") || !strings.Contains(output.String(), "chess menu") || !strings.Contains(output.String(), "docs/cli.md") {
		t.Fatalf("top-level help = %q", output.String())
	}
}

func TestLauncherFormsBuildCLIArguments(t *testing.T) {
	clearChessEnv(t)
	t.Setenv("USER", "tester")
	var output bytes.Buffer
	bot, err := launcherBot(bufio.NewReader(strings.NewReader("\n2\nblack\n\n42\nno\n\n\nascii\n")), &output)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"play", "bot", "--depth", "2", "--color", "black", "--seed", "42", "--random=false", "--theme", "ascii"}
	if strings.Join(bot, " ") != strings.Join(want, " ") {
		t.Fatalf("bot launcher args = %#v, want %#v", bot, want)
	}
	remote, err := launcherSeat(bufio.NewReader(strings.NewReader("https://example.invalid\ngame\nviewer\nwhite\n\n")), &output, "join")
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"join", "https://example.invalid", "--match", "game", "--player", "viewer", "--color", "white"}
	if strings.Join(remote, " ") != strings.Join(want, " ") {
		t.Fatalf("seat launcher args = %#v, want %#v", remote, want)
	}
}

func TestLauncherSettingsUpdatesEnvironmentBackedOptions(t *testing.T) {
	clearChessEnv(t)
	var output bytes.Buffer
	input := strings.NewReader("ascii\nsprite\nno\n123\nprotobuf\nyes\nca.pem\nclient.pem\nclient.key\n")
	if err := launcherSettings(bufio.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"CHESS_THEME":            "ascii",
		"CHESS_PIECE_STYLE":      "sprite",
		"CHESS_BOT_RANDOM":       "false",
		"CHESS_BOT_SEED":         "123",
		"CHESS_NETWORK_FORMAT":   "protobuf",
		"CHESS_NETWORK_INSECURE": "true",
		"CHESS_TLS_CA":           "ca.pem",
		"CHESS_TLS_CLIENT_CERT":  "client.pem",
		"CHESS_TLS_CLIENT_KEY":   "client.key",
	} {
		if got := os.Getenv(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestLauncherSettingsRejectsHalfConfiguredClientCertificate(t *testing.T) {
	clearChessEnv(t)
	var output bytes.Buffer
	input := strings.NewReader("unicode\nauto\nyes\nnone\njson\nno\nnone\nclient.pem\nnone\n")
	if err := launcherSettings(bufio.NewReader(input), &output); err == nil || !strings.Contains(err.Error(), "certificate and private key") {
		t.Fatalf("half-configured client certificate error = %v", err)
	}
}

func TestLauncherHostFormCoversLANAdvertisedHost(t *testing.T) {
	clearChessEnv(t)
	var output bytes.Buffer
	input := strings.NewReader(":0\n\nnone\nnone\nyes\nnone\nyes\nlab\nboard.local\n")
	args, err := launcherHost(bufio.NewReader(input), &output)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(args, " "); !strings.Contains(got, "--insecure") || !strings.Contains(got, "--lan-instance lab") {
		t.Fatalf("host launcher args = %q", got)
	}
	if got := os.Getenv("CHESS_LAN_HOST"); got != "board.local" {
		t.Fatalf("CHESS_LAN_HOST = %q, want board.local", got)
	}
}

func TestLauncherRequiresInteractiveTerminalForMenu(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), []string{"menu"}, strings.NewReader(""), &output); err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("menu error = %v", err)
	}
}

func TestSharedCommandConfigsResolveEnvironmentDefaults(t *testing.T) {
	clearChessEnv(t)
	t.Setenv("CHESS_BOT_DEPTH", "5")
	t.Setenv("CHESS_BOT_RANDOM", "false")
	t.Setenv("CHESS_PLAYER_COLOR", "black")
	t.Setenv("CHESS_NETWORK_URL", "https://chess.example")
	t.Setenv("CHESS_MATCH_ID", "round-1")
	bot, err := botConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if bot.Depth != 5 || bot.Random || bot.Color != "black" {
		t.Fatalf("bot environment config = %#v", bot)
	}
	remote := remoteConfigFromEnv()
	if remote.Address != "https://chess.example" || remote.Match != "round-1" || remote.Color != "black" {
		t.Fatalf("remote environment config = %#v", remote)
	}
	if got := strings.Join((SeatConfig{Command: "spectate", Address: "https://chess.example", Match: "round-1", Color: "black"}).args(), " "); !strings.Contains(got, "--color spectator") {
		t.Fatalf("spectator config args = %q", got)
	}
}

func TestSubcommandHelpListsAllFlags(t *testing.T) {
	clearChessEnv(t)
	for _, test := range []struct {
		args []string
		want []string
	}{
		{args: []string{"play", "local", "--help"}, want: []string{"--clock", "--increment", "--theme"}},
		{args: []string{"play", "bot", "--help"}, want: []string{"--level", "--depth", "--personality", "--seed", "--random", "--color", "--theme"}},
		{args: []string{"host", "--help"}, want: []string{"--addr", "--token", "--cert", "--key", "--insecure", "--store", "--lan", "--lan-instance"}},
		{args: []string{"play", "remote", "--help"}, want: []string{"--match", "--player", "--color", "--token", "--create", "--clock-millis", "--increment-millis", "--theme"}},
		{args: []string{"join", "--help"}, want: []string{"--match", "--player", "--color", "--token"}},
		{args: []string{"connect", "--help"}, want: []string{"--match", "--player", "--color", "--token"}},
		{args: []string{"spectate", "--help"}, want: []string{"--match", "--player", "--color", "--token"}},
		{args: []string{"matchmake", "--help"}, want: []string{"--player", "--color", "--token", "--clock-millis", "--increment-millis"}},
		{args: []string{"list", "--help"}, want: []string{"--token"}},
		{args: []string{"discover", "--help"}, want: []string{"--seconds"}},
	} {
		t.Run(strings.Join(test.args, "-"), func(t *testing.T) {
			var output bytes.Buffer
			if err := run(context.Background(), test.args, strings.NewReader(""), &output); err != nil {
				t.Fatal(err)
			}
			for _, want := range test.want {
				if !strings.Contains(output.String(), want) {
					t.Fatalf("help output missing %q:\n%s", want, output.String())
				}
			}
		})
	}
}

func TestSpectateForcesSpectatorRole(t *testing.T) {
	previousClient := http.DefaultClient
	defer func() { http.DefaultClient = previousClient }()
	response, err := protocol.Encode(protocol.Snapshot, "snapshot", protocol.MatchSnapshot{MatchID: "game", Turn: "white", Result: "*"})
	if err != nil {
		t.Fatal(err)
	}
	var request protocol.JoinMatchRequest
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(httpRequest *http.Request) (*http.Response, error) {
		body, readErr := io.ReadAll(httpRequest.Body)
		if readErr != nil {
			return nil, readErr
		}
		envelope, decodeErr := protocol.Decode(body)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if decodeErr := envelope.UnmarshalPayload(&request); decodeErr != nil {
			return nil, decodeErr
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(response))}, nil
	})}
	var output bytes.Buffer
	if err := run(context.Background(), []string{"spectate", "http://example.invalid", "--match", "game", "--player", "viewer", "--color", "white"}, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}
	if request.Color != "spectator" {
		t.Fatalf("spectate sent color %q, want spectator", request.Color)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
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
		{"play", "bot", "--seed", "not-a-number"},
		{"play", "bot", "--random", "maybe"},
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
	if err != nil || line != "save game.pgn" || output.String() != "savx\b \be game.pgn\r\n" {
		t.Fatalf("raw command = %q, %v; output %q", line, err, output.String())
	}
}

func TestInteractiveRendererResetsColumnForRawTerminals(t *testing.T) {
	var output bytes.Buffer
	ui := boardUI{cursor: chess.NoSquare}
	renderInteractive(&output, chess.NewGame(), &ui, false, "", asciiTheme)
	if !strings.Contains(output.String(), "\r\n") {
		t.Fatal("interactive frame did not use CRLF line starts")
	}
}

func TestInteractiveRendererShowsDashboard(t *testing.T) {
	game := chess.NewGame()
	if err := game.PlayUCI("e2e4"); err != nil {
		t.Fatal(err)
	}
	ui := boardUI{cursor: chess.NoSquare, whiteName: "Ada", blackName: "HAL", message: "Played e4", showHelp: true}
	var output bytes.Buffer
	renderInteractive(&output, game, &ui, false, "White 05:00 · Black 04:58 · +00:03", asciiTheme)
	text := output.String()
	for _, want := range []string{"CHESS-GO", "MATCH", "Ada", "HAL", "STATUS", "Black to move", "RECENT MOVES", "1. e4", "e2e4", "KEYBOARD", "LEGEND"} {
		if !strings.Contains(text, want) {
			t.Errorf("dashboard rendering lacks %q:\n%s", want, text)
		}
	}
}

func TestInteractiveRendererRedrawsWhenClockChanges(t *testing.T) {
	game := chess.NewGame()
	ui := boardUI{cursor: chess.NoSquare}
	var output bytes.Buffer
	renderInteractive(&output, game, &ui, false, "White 00:10 · Black 00:10 · +00:00", asciiTheme)
	output.Reset()
	renderInteractive(&output, game, &ui, false, "White 00:09 · Black 00:10 · +00:00", asciiTheme)
	text := output.String()
	if !strings.Contains(text, "\x1b[2J") || !strings.Contains(text, "White 00:09") {
		t.Fatalf("clock redraw missing: %q", text)
	}
	if !strings.Contains(text, "CHESS-GO") {
		t.Fatalf("clock update did not redraw the frame: %q", text)
	}
	if !strings.Contains(text, tuiSurface) {
		t.Fatalf("clock redraw did not set an explicit terminal surface: %q", text)
	}
}

func TestInteractiveRendererUsesASCIIChrome(t *testing.T) {
	game := chess.NewGame()
	ui := boardUI{cursor: chess.NoSquare}
	var output bytes.Buffer
	renderInteractive(&output, game, &ui, false, "", asciiTheme)
	text := output.String()
	if !strings.Contains(text, "+") || !strings.Contains(text, "----") || strings.Contains(text, "┌") || strings.Contains(text, "·") || strings.Contains(text, "●") {
		t.Fatalf("ASCII theme emitted Unicode chrome:\n%s", text)
	}
}

func TestInteractiveRendererStacksTheRailWhenNarrow(t *testing.T) {
	game := chess.NewGame()
	ui := boardUI{cursor: chess.NoSquare}
	model := ui.model(game, game.Position(), asciiTheme)
	var output bytes.Buffer
	renderFullInteractive(&output, game, &ui, model, false, "", asciiTheme, boardScale{cellWidth: 4, cellHeight: 1}, true, 60, 30)
	for _, line := range strings.Split(output.String(), "\n") {
		if strings.Contains(line, "8 |") && strings.Contains(line, "MATCH") {
			t.Fatalf("narrow board still placed the rail beside a rank: %q", line)
		}
	}
}

func TestBoardScaleUsesAvailableTerminalSpace(t *testing.T) {
	wide, compact := boardScaleForTerminal(213, 60)
	if compact || wide.cellWidth != 18 || wide.cellHeight != 4 {
		t.Fatalf("wide scale = %#v, compact %v", wide, compact)
	}
	huge, compact := boardScaleForTerminal(266, 60)
	if compact || huge.cellWidth != 20 || huge.cellHeight != 4 {
		t.Fatalf("huge scale = %#v, compact %v", huge, compact)
	}
	narrow, compact := boardScaleForTerminal(60, 30)
	if !compact || narrow.cellWidth != 4 || narrow.cellHeight != 1 {
		t.Fatalf("narrow scale = %#v, compact %v", narrow, compact)
	}
	tiny, compact := boardScaleForTerminal(30, 24)
	if !compact || tiny.cellWidth != 1 {
		t.Fatalf("tiny scale = %#v, compact %v", tiny, compact)
	}
}

func TestScaledBoardRepeatsRanksWithoutFixedClockCoordinates(t *testing.T) {
	t.Setenv("CHESS_PIECE_STYLE", "text")
	game := chess.NewGame()
	ui := boardUI{cursor: chess.NoSquare}
	model := ui.model(game, game.Position(), unicodeTheme)
	var output bytes.Buffer
	renderFullInteractive(&output, game, &ui, model, false, "White 05:00 · Black 05:00 · +00:03", unicodeTheme, boardScale{cellWidth: 10, cellHeight: 3}, false, 213, 60)
	text := output.String()
	if strings.Contains(text, "\x1b[10;50H") || strings.Count(text, "──────────") < 8 || strings.Count(text, "♜") != 2 || !strings.Contains(text, "    │") {
		t.Fatalf("scaled frame retained fixed coordinates or rank height:\n%s", text)
	}
	lines := strings.Split(text, "\n")
	var top, rank, coordinates string
	for _, line := range lines {
		clean := stripSGR(line)
		switch {
		case strings.Contains(clean, "┌──────────"):
			top = clean
		case strings.Contains(clean, "8 │"):
			rank = clean
		case strings.Contains(clean, "a         b"):
			coordinates = clean
		}
	}
	firstVisible := func(line string) int {
		return strings.IndexFunc(line, func(r rune) bool { return r != ' ' })
	}
	if top == "" || rank == "" || coordinates == "" || firstVisible(top) != firstVisible(rank)+2 || firstVisible(coordinates) != firstVisible(rank)+6 {
		t.Fatalf("board geometry is not aligned: top=%q rank=%q coordinates=%q", top, rank, coordinates)
	}
}

func TestScaledUnicodePiecesAreCenteredAndEmphasized(t *testing.T) {
	t.Setenv("CHESS_PIECE_STYLE", "text")
	game := chess.NewGame()
	ui := boardUI{cursor: chess.NoSquare}
	position := game.Position()
	files, ranks := boardOrientation(false)
	lines := boardLines(position, files, ranks, &ui, [64]bool{}, [64]bool{}, chess.NoSquare, unicodeTheme, boardScale{cellWidth: 10, cellHeight: 3})
	var pieceRows []int
	for index, line := range lines {
		if strings.Contains(line, "♜") {
			pieceRows = append(pieceRows, index)
			if !strings.Contains(line, tuiBold) {
				t.Fatalf("wide Unicode piece is not emphasized: %q", line)
			}
		}
	}
	if len(pieceRows) != 1 || pieceRows[0] != 2 {
		t.Fatalf("wide Unicode pieces rendered on rows %v, want both on centered row 2", pieceRows)
	}
}

func TestCompactUnicodePiecesStayCenteredAndReadable(t *testing.T) {
	ui := boardUI{cursor: chess.NoSquare}
	position := chess.NewGame().Position()
	files, ranks := boardOrientation(false)
	lines := boardLines(position, files, ranks, &ui, [64]bool{}, [64]bool{}, chess.NoSquare, unicodeTheme, boardScale{cellWidth: 5, cellHeight: 1})
	for _, line := range lines {
		clean := stripSGR(line)
		if !strings.Contains(clean, "♜") {
			continue
		}
		lineRunes := []rune(clean)
		start := slices.Index(lineRunes, '│')
		end := slices.Index(lineRunes[start+1:], '│')
		if start < 0 || end < 0 {
			t.Fatalf("compact board lost cell separators: %q", clean)
		}
		cell := string(lineRunes[start+1 : start+1+end])
		if cell != "  ♜  " {
			t.Fatalf("compact Unicode piece is not centered: %q", cell)
		}
		if !strings.Contains(line, tuiBold) {
			t.Fatalf("compact Unicode piece is not emphasized: %q", line)
		}
		return
	}
	t.Fatal("compact board did not render a rook")
}

func TestScaledUnicodeSpritesFillAndCenterLargeCells(t *testing.T) {
	t.Setenv("CHESS_PIECE_STYLE", "sprite")
	ui := boardUI{cursor: chess.NoSquare}
	position := chess.NewGame().Position()
	files, ranks := boardOrientation(false)
	lines := boardLines(position, files, ranks, &ui, [64]bool{}, [64]bool{}, chess.NoSquare, unicodeTheme, boardScale{cellWidth: 16, cellHeight: 4})
	for _, line := range lines[1:5] {
		clean := stripSGR(line)
		if !strings.ContainsAny(clean, "▀▄█") {
			continue
		}
		cells := strings.Split(clean[strings.Index(clean, "│"):], "│")
		if len(cells) < 9 {
			t.Fatalf("sprite row lost board separators: %q", clean)
		}
		for _, cell := range cells[1:9] {
			if got := len([]rune(cell)); got != 16 {
				t.Fatalf("sprite cell width = %d, want 16: %q", got, cell)
			}
		}
	}
	if got := pieceSpriteRow(chess.Piece{Type: chess.Rook}, 16, 0); got == "" || strings.TrimSpace(got) == got {
		t.Fatalf("sprite row lost vertical breathing room: %q", got)
	}
}

func TestSpriteSilhouettesStayHorizontallyCentered(t *testing.T) {
	for _, pieceType := range []chess.PieceType{chess.Pawn, chess.Knight, chess.Bishop, chess.Rook, chess.Queen, chess.King} {
		row := centerSpriteRow(pieceSpriteBitmap[pieceType][0])
		left := strings.IndexByte(row, '#')
		right := strings.LastIndexByte(row, '#')
		if left < 0 || right < left {
			t.Fatalf("piece %v has no silhouette: %q", pieceType, row)
		}
		if left > len(row)-right-1+1 || len(row)-right-1 > left+1 {
			t.Fatalf("piece %v silhouette is not centered: %q", pieceType, row)
		}
	}
}

func TestAutoPieceStyleUsesSpritesOnlyWhenTheyCanScale(t *testing.T) {
	t.Setenv("CHESS_PIECE_STYLE", "auto")
	piece := chess.Piece{Color: chess.Black, Type: chess.Queen}
	if !pieceSpriteEnabled(piece, unicodeTheme, 18, 4) {
		t.Fatal("auto style did not select a scalable sprite for a large Unicode cell")
	}
	if !pieceSpriteEnabled(piece, unicodeTheme, 5, 2) {
		t.Fatal("auto style did not select a two-row sprite for a compact dashboard cell")
	}
	if pieceSpriteEnabled(piece, unicodeTheme, 4, 2) || pieceSpriteEnabled(piece, unicodeTheme, 5, 1) {
		t.Fatal("auto style selected a sprite where the cell cannot support it")
	}
	t.Setenv("CHESS_PIECE_STYLE", "text")
	if pieceSpriteEnabled(piece, unicodeTheme, 18, 4) {
		t.Fatal("text style unexpectedly selected a sprite")
	}
}

func TestResponsive106RowBoardUsesCenteredSprites(t *testing.T) {
	t.Setenv("CHESS_PIECE_STYLE", "auto")
	scale, compact := boardScaleForTerminal(106, 30)
	if compact || scale.cellWidth != 5 || scale.cellHeight != 2 {
		t.Fatalf("106x30 scale = %#v, compact %v; want 5x2 dashboard cells", scale, compact)
	}
	ui := boardUI{cursor: chess.NoSquare}
	position := chess.NewGame().Position()
	files, ranks := boardOrientation(false)
	lines := boardLines(position, files, ranks, &ui, [64]bool{}, [64]bool{}, chess.NoSquare, unicodeTheme, scale)
	var spriteRows int
	for _, line := range lines[1:3] {
		clean := stripSGR(line)
		if !strings.ContainsAny(clean, "▀▄█") {
			continue
		}
		spriteRows++
		cells := strings.Split(clean[strings.Index(clean, "│"):], "│")
		if len(cells) < 9 {
			t.Fatalf("sprite row lost board separators: %q", clean)
		}
		for _, cell := range cells[1:9] {
			if got := len([]rune(cell)); got != scale.cellWidth {
				t.Fatalf("compact sprite cell width = %d, want %d: %q", got, scale.cellWidth, cell)
			}
		}
	}
	if spriteRows != 2 {
		t.Fatalf("compact sprite rows = %d, want 2", spriteRows)
	}
}

func TestKeyboardGuideReservesRowsForEveryShortcut(t *testing.T) {
	scale, compact := boardScaleForTerminal(106, 30)
	guideScale := scaleForInteractiveContent(scale, compact, 30, true)
	if guideScale.cellHeight != 1 {
		t.Fatalf("guide scale = %#v, want one-row board to keep all guide lines", guideScale)
	}
	normalScale := scaleForInteractiveContent(scale, compact, 30, false)
	if normalScale.cellHeight != 2 {
		t.Fatalf("normal scale = %#v, want two-row board", normalScale)
	}
	game := chess.NewGame()
	ui := boardUI{cursor: chess.NoSquare, showHelp: true}
	model := ui.model(game, game.Position(), unicodeTheme)
	var output bytes.Buffer
	renderFullInteractive(&output, game, &ui, model, false, "", unicodeTheme, guideScale, false, 106, 30)
	frame := formatInteractiveFrame(output.String(), 106, 30)
	if !strings.Contains(frame, "n  new game") || !strings.Contains(frame, "q / ctrl-c  quit") {
		t.Fatalf("keyboard guide was clipped:\n%s", stripSGR(frame))
	}
}

func TestResponsiveFramesStayWithinTerminalViewport(t *testing.T) {
	t.Setenv("CHESS_PIECE_STYLE", "auto")
	game := chess.NewGame()
	ui := boardUI{cursor: chess.NoSquare, whiteName: "White", blackName: "Bot", mode: "LOCAL MATCH"}
	model := ui.model(game, game.Position(), unicodeTheme)
	for _, size := range [][2]int{{60, 30}, {95, 24}, {106, 30}, {120, 30}, {125, 30}, {150, 30}, {180, 30}, {213, 60}} {
		width, height := size[0], size[1]
		scale, compact := boardScaleForTerminal(width, height)
		scale = scaleForInteractiveContent(scale, compact, height, false)
		var output bytes.Buffer
		renderFullInteractive(&output, game, &ui, model, false, "", unicodeTheme, scale, compact, width, height)
		body := strings.TrimPrefix(output.String(), tuiFrameStart)
		for index, line := range strings.Split(body, "\r\n") {
			if got := len([]rune(stripSGR(line))); got > width {
				t.Fatalf("%dx%d frame line %d is %d columns wide: %q", width, height, index, got, line)
			}
		}
	}
}

func TestInteractiveFramePaintsTheWholeViewport(t *testing.T) {
	frame := formatInteractiveFrame("\x1b[H\x1b[2Jone\n", 8, 3)
	lines := strings.Split(strings.TrimPrefix(frame, "\x1b[H\x1b[2J"), "\n")
	if len(lines) != 3 {
		t.Fatalf("painted frame has %d lines, want 3", len(lines))
	}
	for _, line := range lines {
		if got := len([]rune(stripSGR(line))); got != 8 {
			t.Fatalf("painted line width = %d, want 8: %q", got, line)
		}
	}
}

func TestUnicodePiecePresentationScalesForCapableTerminals(t *testing.T) {
	ui := boardUI{cursor: chess.NoSquare}
	rook := chess.Piece{Color: chess.Black, Type: chess.Rook}
	square, err := chess.ParseSquare("a8")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHESS_PIECE_STYLE", "emoji")
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	got := boardCell(rook, square, 0, &ui, [64]bool{}, [64]bool{}, chess.NoSquare, unicodeTheme, 10)
	if !strings.Contains(got, "♜\ufe0f") {
		t.Fatalf("emoji presentation missing from wide Unicode cell: %q", got)
	}
	t.Setenv("CHESS_PIECE_STYLE", "text")
	got = boardCell(rook, square, 0, &ui, [64]bool{}, [64]bool{}, chess.NoSquare, unicodeTheme, 10)
	if strings.Contains(got, "\ufe0f") {
		t.Fatalf("text presentation unexpectedly contains variation selector: %q", got)
	}
	t.Setenv("CHESS_PIECE_STYLE", "emoji")
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")
	got = boardCell(rook, square, 0, &ui, [64]bool{}, [64]bool{}, chess.NoSquare, unicodeTheme, 10)
	if strings.Contains(got, "\ufe0f") {
		t.Fatalf("Apple Terminal received a width-unstable emoji presentation: %q", got)
	}
}

func TestStripSGRPreservesTerminalControls(t *testing.T) {
	got := stripSGR("\x1b[31mred\x1b[0m\x1b[2J")
	if got != "red\x1b[2J" {
		t.Fatalf("stripSGR() = %q", got)
	}
}

func TestCompactFrameFitsTerminalViewport(t *testing.T) {
	game := chess.NewGame()
	ui := boardUI{cursor: chess.NoSquare, whiteName: "White", blackName: "Black", mode: "LOCAL MATCH"}
	model := ui.model(game, game.Position(), unicodeTheme)
	var output bytes.Buffer
	renderFullInteractive(&output, game, &ui, model, false, "", unicodeTheme, boardScale{cellWidth: 5, cellHeight: 1}, true, 106, 30)
	frame := formatInteractiveFrame(output.String(), 106, 30)
	body := strings.TrimPrefix(frame, tuiFrameStart)
	for index, line := range strings.Split(body, "\n") {
		if got := len([]rune(stripSGR(line))); got > 106 {
			t.Fatalf("compact frame line %d is %d columns wide: %q", index, got, line)
		}
	}
}

func TestInteractiveFrameFitsViewport(t *testing.T) {
	frame := "\x1b[H\x1b[2Jone\ntwo\nthree\n"
	got := formatInteractiveFrame(frame, 20, 2)
	if !strings.HasPrefix(got, "\x1b[H\x1b[2J") || strings.Contains(got, "three") || strings.Count(got, "\n") != 1 {
		t.Fatalf("frame was not vertically fitted: %q", got)
	}
	if got := truncateTerminalLine("\x1b[31mabcdef\x1b[0m", 3); !strings.Contains(got, "abc") || strings.Contains(got, "def") {
		t.Fatalf("line was not horizontally fitted: %q", got)
	}
}

func TestWriteFrameHonorsNoColorForTerminalWriters(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("NO_COLOR", "1")
	writeFrame(writer, "\x1b[31mred\x1b[0m")
	_ = writer.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	_ = reader.Close()
	if string(data) != "red" {
		t.Fatalf("NO_COLOR frame = %q", data)
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

func TestInteractivePromotionRequiresAnExplicitChoice(t *testing.T) {
	position, err := chess.ParseFEN("7k/P7/8/8/8/8/8/7K w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	a7, _ := chess.ParseSquare("a7")
	a8, _ := chess.ParseSquare("a8")
	s := session{game: chess.NewGameFromPosition(position), human: chess.White}
	ui := boardUI{cursor: a7, selected: chess.NoSquare}
	s.handleKey(&ui, keySelect)
	ui.cursor = a8
	s.handleKey(&ui, keySelect)
	if len(ui.promotion) != 4 || s.game.MoveCount() != 0 {
		t.Fatalf("promotion prompt state = %#v, moves = %d", ui, s.game.MoveCount())
	}
	s.handleKey(&ui, keyRight)
	s.handleKey(&ui, keySelect)
	if len(ui.promotion) != 0 || s.game.MoveCount() != 1 || s.game.Moves()[0].Promotion != chess.Rook {
		t.Fatalf("promotion choice = %#v, moves = %#v", ui, s.game.Moves())
	}
}

func TestCommandPaletteControlsPresentationAndGameEnd(t *testing.T) {
	s := session{game: chess.NewGame(), theme: asciiTheme}
	var output bytes.Buffer
	if err := s.command("theme unicode", &output); err != nil || s.theme.label() != "unicode" {
		t.Fatalf("theme command = %v, %q", err, s.theme.label())
	}
	if err := s.command("flip", &output); err != nil || !s.flip {
		t.Fatalf("flip command = %v, %v", err, s.flip)
	}
	if err := s.command("draw", &output); err != nil || s.timeout != "Draw by agreement" {
		t.Fatalf("draw command = %v, %q", err, s.timeout)
	}
	if err := s.command("resign", &output); err == nil {
		t.Fatal("resign command accepted an already finished game")
	}
}

func TestRemoteDashboardUsesTheInteractiveLayout(t *testing.T) {
	position := chess.NewPosition()
	snapshot := protocol.MatchSnapshot{MatchID: "demo", FEN: position.FEN(), PositionHash: position.Hash(), Turn: "white", Result: "*"}
	game, err := gameFromSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	ui := boardUI{cursor: chess.NoSquare, mode: "REMOTE MATCH"}
	var output bytes.Buffer
	if err := renderRemote(&output, game, snapshot, chess.White, asciiTheme, true, &ui); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "REMOTE MATCH") || !strings.Contains(output.String(), "MATCH") {
		t.Fatalf("remote dashboard was not rendered:\n%s", output.String())
	}
}

func TestGameFromSnapshotRestoresMoveHistory(t *testing.T) {
	game := chess.NewGame()
	for _, move := range []string{"e2e4", "e7e5"} {
		if err := game.PlayUCI(move); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := protocol.MatchSnapshot{FEN: game.Position().FEN(), PositionHash: game.Position().Hash(), Moves: []string{"e2e4", "e7e5"}}
	restored, err := gameFromSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if restored.MoveCount() != 2 || restored.Position().Hash() != snapshot.PositionHash {
		t.Fatalf("restored snapshot = %d moves, hash %x", restored.MoveCount(), restored.Position().Hash())
	}
}

func TestGameFromSnapshotRestoresCustomFENWithoutHistory(t *testing.T) {
	position, err := chess.ParseFEN("7k/P7/8/8/8/8/8/7K w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	restored, err := gameFromSnapshot(protocol.MatchSnapshot{FEN: position.FEN()})
	if err != nil {
		t.Fatal(err)
	}
	if restored.Position().Hash() != position.Hash() || restored.MoveCount() != 0 {
		t.Fatalf("restored custom FEN = %s with %d moves", restored.Position().FEN(), restored.MoveCount())
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
	t.Setenv("CHESS_PIECE_STYLE", "text")
	game := chess.NewGame()
	if err := game.PlayUCI("e2e4"); err != nil {
		t.Fatal(err)
	}
	e7, _ := chess.ParseSquare("e7")
	e6, _ := chess.ParseSquare("e6")
	ui := boardUI{cursor: e6, selected: e7}
	var output bytes.Buffer
	renderInteractive(&output, game, &ui, false, "White 05:00 · Black 05:00 · +00:03", unicodeTheme)
	text := output.String()
	for _, want := range []string{"\x1b[H\x1b[2J", "\x1b[7m", "\x1b[46m", "\x1b[42m", "\x1b[43m", "8 ", "a    b    c    d    e    f    g    h", "White 05:00", "Black to move"} {
		if !strings.Contains(text, want) {
			t.Errorf("interactive rendering lacks %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "♜") || !strings.Contains(text, "♙") {
		t.Fatal("unicode theme did not render chess symbols")
	}
}

func TestThemeConfiguration(t *testing.T) {
	defaultTheme, err := parseTheme("")
	if err != nil || defaultTheme.label() != "unicode" {
		t.Fatalf("default theme = %q, %v", defaultTheme.label(), err)
	}
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
	if err := run(context.Background(), []string{"host"}, strings.NewReader(""), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "TLS is required") {
		t.Fatalf("host secure-default error = %v", err)
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
	for _, name := range []string{"CHESS_BOT_DEPTH", "CHESS_BOT_LEVEL", "CHESS_BOT_PERSONALITY", "CHESS_BOT_SEED", "CHESS_BOT_RANDOM", "CHESS_PLAYER_COLOR", "CHESS_PLAYER_NAME", "CHESS_BOT_NAME", "CHESS_CLOCK", "CHESS_INCREMENT", "CHESS_THEME", "CHESS_PIECE_STYLE", "CHESS_NETWORK_ADDR", "CHESS_NETWORK_URL", "CHESS_NETWORK_TOKEN", "CHESS_NETWORK_FORMAT", "CHESS_NETWORK_INSECURE", "CHESS_MATCH_ID", "CHESS_PLAYER_ID", "CHESS_TLS_CERT", "CHESS_TLS_KEY", "CHESS_TLS_CA", "CHESS_TLS_CLIENT_CERT", "CHESS_TLS_CLIENT_KEY", "CHESS_MATCH_STORE", "CHESS_LAN_DISCOVERY", "CHESS_LAN_INSTANCE", "CHESS_LAN_HOST"} {
		t.Setenv(name, "")
	}
}
