package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestLauncherFormsCoverEveryOperation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		call  func(*bufio.Reader, *bytes.Buffer) ([]string, error)
		want  string
	}{
		{name: "local", input: "\n\nunicode\n", call: func(r *bufio.Reader, b *bytes.Buffer) ([]string, error) { return launcherLocal(r, b) }, want: "play local --theme unicode"},
		{name: "remote", input: "https://example.invalid\nmatch\nplayer\nblack\ntoken\nyes\n60000\n1000\nascii\n", call: func(r *bufio.Reader, b *bytes.Buffer) ([]string, error) { return launcherRemote(r, b) }, want: "play remote https://example.invalid --match match --player player --color black --token token --create --clock-millis 60000 --increment-millis 1000 --theme ascii"},
		{name: "matchmake", input: "https://example.invalid\nplayer\nrandom\ntoken\n60000\n1000\n", call: func(r *bufio.Reader, b *bytes.Buffer) ([]string, error) { return launcherMatchmake(r, b) }, want: "matchmake https://example.invalid --player player --color random --token token --clock-millis 60000 --increment-millis 1000"},
		{name: "list", input: "https://example.invalid\ntoken\n", call: func(r *bufio.Reader, b *bytes.Buffer) ([]string, error) { return launcherList(r, b) }, want: "list https://example.invalid --token token"},
		{name: "discover", input: "3\n", call: func(r *bufio.Reader, b *bytes.Buffer) ([]string, error) { return launcherDiscover(r, b) }, want: "discover --seconds 3"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearChessEnv(t)
			var output bytes.Buffer
			args, err := test.call(bufio.NewReader(strings.NewReader(test.input)), &output)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(args, " "); got != test.want {
				t.Fatalf("launcher args = %q, want %q", got, test.want)
			}
		})
	}

	for _, command := range []string{"join", "connect", "spectate"} {
		t.Run(command, func(t *testing.T) {
			input := "https://example.invalid\nmatch\nplayer\nwhite\ntoken\n"
			if command == "spectate" {
				input = "https://example.invalid\nmatch\nplayer\nwhite\ntoken\n"
			}
			args, err := launcherSeat(bufio.NewReader(strings.NewReader(input)), &bytes.Buffer{}, command)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(strings.Join(args, " "), command+" https://example.invalid --match match") {
				t.Fatalf("seat args = %#v", args)
			}
		})
	}
}

func TestLauncherActionDispatchesEveryRootAction(t *testing.T) {
	for _, test := range []struct {
		action string
		input  string
		want   string
	}{
		{action: "load", input: "game.pgn\n", want: "load game.pgn"},
		{action: "quit"},
		{action: "network", input: "\x1b", want: ""},
		{action: "help", input: "q", want: "Usage: chess"},
		{action: "version", input: "q", want: "chess-go"},
	} {
		t.Run(test.action, func(t *testing.T) {
			var output bytes.Buffer
			args, err := launcherAction(bufio.NewReader(strings.NewReader(test.input)), &output, test.action)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(args, " "); got != "" && got != test.want {
				t.Fatalf("action args = %q, want %q", got, test.want)
			}
			if test.want != "" && test.action != "load" && !strings.Contains(output.String(), test.want) {
				t.Fatalf("action output lacks %q:\n%s", test.want, output.String())
			}
		})
	}
	if _, err := launcherAction(bufio.NewReader(strings.NewReader("")), &bytes.Buffer{}, "unknown"); err == nil {
		t.Fatal("unknown launcher action was accepted")
	}
}
