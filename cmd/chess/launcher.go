package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type launcherItem struct {
	label string
	hint  string
}

var launcherItems = []launcherItem{
	{label: "Local game", hint: "play standard chess locally"},
	{label: "Play against bot", hint: "choose strength, personality, and variation"},
	{label: "Remote game", hint: "create or join an online match"},
	{label: "Network tools", hint: "host, join, spectate, matchmake, list, discover"},
	{label: "Load PGN", hint: "open a saved game"},
	{label: "Help", hint: "keyboard and command reference"},
	{label: "Version", hint: "show the installed chess-go version"},
	{label: "Quit", hint: "leave the launcher"},
}

func runLauncherCommand(ctx context.Context, args []string, input io.Reader, output io.Writer) (bool, error) {
	if len(args) != 0 && args[0] != "menu" {
		return false, nil
	}
	if len(args) > 1 {
		return true, errors.New("usage: chess menu")
	}
	if !isInteractiveTerminal(input, output) {
		if len(args) == 0 {
			return true, errors.New("usage: chess version | chess play local|bot|remote [options] | chess host|join|connect|spectate|matchmake|list|discover ... | chess load FILE")
		}
		return true, errors.New("chess menu requires an interactive terminal")
	}
	selected, err := runLauncher(ctx, input, output)
	if err != nil || len(selected) == 0 {
		return true, err
	}
	return true, run(ctx, selected, input, output)
}

func runLauncher(ctx context.Context, input io.Reader, output io.Writer) ([]string, error) {
	in, ok := input.(*os.File)
	if !ok || !isInteractiveTerminal(input, output) {
		return nil, errors.New("launcher requires an interactive terminal")
	}
	state, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return nil, fmt.Errorf("enable launcher terminal: %w", err)
	}
	defer term.Restore(int(in.Fd()), state) //nolint:errcheck -- best effort during shutdown
	fmt.Fprint(output, "\x1b[?1049h\x1b[?25l")
	defer fmt.Fprint(output, "\x1b[?25h\x1b[?1049l")

	reader := bufio.NewReader(input)
	selected := 0
	message := ""
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		renderLauncher(output, launcherItems, selected, message)
		pressed, err := readKey(reader)
		if err != nil {
			return nil, err
		}
		switch pressed {
		case keyUp:
			selected = (selected + len(launcherItems) - 1) % len(launcherItems)
			message = ""
		case keyDown:
			selected = (selected + 1) % len(launcherItems)
			message = ""
		case keyQuit, keyEscape:
			return nil, nil
		case keyHelp:
			message = "Use ↑/↓ or j/k, Enter to choose, q or Esc to quit"
		case keySelect:
			args, err := launcherAction(reader, output, selected)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil, nil
				}
				message = "Error: " + err.Error()
				continue
			}
			if len(args) != 0 {
				return args, nil
			}
			message = ""
		}
	}
}

func launcherAction(reader *bufio.Reader, output io.Writer, selected int) ([]string, error) {
	switch selected {
	case 0:
		return launcherLocal(reader, output)
	case 1:
		return launcherBot(reader, output)
	case 2:
		return launcherRemote(reader, output)
	case 3:
		return launcherNetwork(reader, output)
	case 4:
		path, err := launcherPrompt(reader, output, "PGN file", "")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(path) == "" {
			return nil, errors.New("a PGN file is required")
		}
		return []string{"load", path}, nil
	case 5:
		fmt.Fprint(output, "\r\n")
		printTopLevelHelp(output)
		fmt.Fprint(output, "\r\nPress any key to return")
		_, err := readKey(reader)
		return nil, err
	case 6:
		fmt.Fprintf(output, "\r\nchess-go %s\r\n\r\nPress any key to return", version)
		_, err := readKey(reader)
		return nil, err
	case 7:
		return nil, nil
	default:
		return nil, errors.New("unknown launcher selection")
	}
}

func renderLauncher(output io.Writer, items []launcherItem, selected int, message string) {
	var frame strings.Builder
	fmt.Fprintf(&frame, "%s%s  CHESS-GO%s\n", tuiTitle, tuiBold, tuiReset)
	fmt.Fprintf(&frame, "%s  Interactive launcher%s\n\n", tuiDim, tuiReset)
	for index, item := range items {
		marker := "  "
		style := tuiDim
		if index == selected {
			marker = "▶ "
			style = tuiAccent + tuiBold
		}
		fmt.Fprintf(&frame, "%s%s%-22s%s  %s\n", style, marker, item.label, tuiReset, item.hint)
	}
	fmt.Fprintf(&frame, "\n%s  ↑/↓ or j/k move · Enter select · ? help · q quit%s\n", tuiDim, tuiReset)
	if message != "" {
		fmt.Fprintf(&frame, "%s%s  %s%s\n", tuiBold, tuiAccent, message, tuiReset)
	}
	writeFrame(output, strings.ReplaceAll(formatInteractiveFrame("\x1b[H\x1b[2J"+frame.String(), terminalWidth(output), terminalHeight(output)), "\n", "\r\n"))
}

func terminalWidth(output io.Writer) int {
	width, _ := terminalSize(output)
	return width
}

func terminalHeight(output io.Writer) int {
	_, height := terminalSize(output)
	return height
}

func launcherPrompt(reader *bufio.Reader, output io.Writer, label, defaultValue string) (string, error) {
	displayDefault := defaultValue
	if displayDefault == "" {
		displayDefault = "none"
	}
	fmt.Fprintf(output, "\r\n%s%s%s [%s]: %s", tuiAccent, label, tuiReset, displayDefault, "")
	value, err := readRawLine(reader, output)
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func launcherPromptSecret(reader *bufio.Reader, output io.Writer, label, defaultValue string) (string, error) {
	displayDefault := "not set"
	if defaultValue != "" {
		displayDefault = "set"
	}
	fmt.Fprintf(output, "\r\n%s%s%s [%s]: ", tuiAccent, label, tuiReset, displayDefault)
	value, err := readMaskedLine(reader, output)
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func readMaskedLine(reader *bufio.Reader, output io.Writer) (string, error) {
	line := make([]byte, 0, 32)
	for {
		character, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		switch {
		case character == '\r' || character == '\n':
			fmt.Fprint(output, "\r\n")
			return string(line), nil
		case (character == 8 || character == 127) && len(line) > 0:
			line = line[:len(line)-1]
			fmt.Fprint(output, "\b \b")
		case character >= 32 && character <= 126:
			line = append(line, character)
			fmt.Fprint(output, "*")
		}
	}
}

func launcherNetwork(reader *bufio.Reader, output io.Writer) ([]string, error) {
	items := []launcherItem{
		{label: "Host server", hint: "serve an encrypted match endpoint"},
		{label: "Join match", hint: "claim a player seat"},
		{label: "Connect", hint: "join alias"},
		{label: "Spectate match", hint: "watch without a seat"},
		{label: "Matchmake", hint: "find or create an open match"},
		{label: "List matches", hint: "show available matches"},
		{label: "Discover LAN", hint: "find advertised hosts"},
		{label: "Back", hint: "return to the launcher"},
	}
	selected := 0
	for {
		renderLauncher(output, items, selected, "Network tools")
		pressed, err := readKey(reader)
		if err != nil {
			return nil, err
		}
		switch pressed {
		case keyUp:
			selected = (selected + len(items) - 1) % len(items)
		case keyDown:
			selected = (selected + 1) % len(items)
		case keyEscape:
			return nil, nil
		case keyQuit:
			return nil, io.EOF
		case keySelect:
			switch selected {
			case 0:
				return launcherHost(reader, output)
			case 1, 2, 3:
				return launcherSeat(reader, output, []string{"join", "connect", "spectate"}[selected-1])
			case 4:
				return launcherMatchmake(reader, output)
			case 5:
				return launcherList(reader, output)
			case 6:
				return launcherDiscover(reader, output)
			case 7:
				return nil, nil
			}
		}
	}
}
