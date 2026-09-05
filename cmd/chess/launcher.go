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

func runLauncherCommand(ctx context.Context, args []string, input io.Reader, output io.Writer) (bool, error) {
	if len(args) != 0 && args[0] != "menu" {
		return false, nil
	}
	if len(args) > 1 {
		return true, errors.New("usage: chess menu")
	}
	if !isInteractiveTerminal(input, output) {
		if len(args) == 0 {
			return true, errors.New(commandUsageSummary())
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
	defer fmt.Fprint(output, "\x1b[0m\x1b[?25h\x1b[?1049l")

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
			args, err := launcherAction(reader, output, launcherItems[selected].action)
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
			if launcherItems[selected].label == "Settings" {
				message = "Settings saved"
			} else {
				message = ""
			}
		}
	}
}

func launcherAction(reader *bufio.Reader, output io.Writer, action string) ([]string, error) {
	switch action {
	case "local":
		return launcherLocal(reader, output)
	case "bot":
		return launcherBot(reader, output)
	case "remote":
		return launcherRemote(reader, output)
	case "network":
		return launcherNetwork(reader, output)
	case "load":
		path, err := launcherPrompt(reader, output, "PGN file", "")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(path) == "" {
			return nil, errors.New("a PGN file is required")
		}
		return []string{"load", path}, nil
	case "settings":
		return nil, launcherSettings(reader, output)
	case "help":
		fmt.Fprint(output, "\r\n")
		printTopLevelHelp(output)
		fmt.Fprint(output, "\r\nPress any key to return")
		_, err := readKey(reader)
		return nil, err
	case "version":
		fmt.Fprintf(output, "\r\nchess-go %s\r\n\r\nPress any key to return", version)
		_, err := readKey(reader)
		return nil, err
	case "quit":
		return nil, nil
	default:
		return nil, errors.New("unknown launcher selection")
	}
}

func renderLauncher(output io.Writer, items []launcherItem, selected int, message string) {
	renderLauncherAtSize(output, items, selected, message, terminalWidth(output), terminalHeight(output))
}

func renderLauncherAtSize(output io.Writer, items []launcherItem, selected int, message string, width, height int) {
	var frame strings.Builder
	fmt.Fprintf(&frame, "%s%s  CHESS-GO%s\n", tuiTitle, tuiBold, tuiReset)
	fmt.Fprintf(&frame, "%s  Interactive launcher%s\n\n", tuiDim, tuiReset)
	compact := width > 0 && width < 72
	for index, item := range items {
		marker := "  "
		style := tuiDim
		if index == selected {
			marker = "▶ "
			style = tuiAccent + tuiBold
		}
		if compact {
			fmt.Fprintf(&frame, "%s%s%s%s\n", style, marker, item.label, tuiReset)
		} else {
			fmt.Fprintf(&frame, "%s%s%-22s%s  %s\n", style, marker, item.label, tuiReset, item.hint)
		}
	}
	footer := "↑/↓ or j/k move · Enter select · ? help · q quit"
	if compact {
		footer = "↑/↓ or j/k · Enter choose · ? help · q quit"
	}
	fmt.Fprintf(&frame, "\n%s  %s%s\n", tuiDim, footer, tuiReset)
	if message != "" {
		fmt.Fprintf(&frame, "%s%s  %s%s\n", tuiBold, tuiAccent, message, tuiReset)
	}
	writeFrame(output, strings.ReplaceAll(formatInteractiveFrame(tuiFrameStart+frame.String(), width, height), "\n", "\r\n"))
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
	items := networkLauncherItems()
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
			switch items[selected].action {
			case "host":
				return launcherHost(reader, output)
			case "join", "connect", "spectate":
				return launcherSeat(reader, output, items[selected].action)
			case "matchmake":
				return launcherMatchmake(reader, output)
			case "list":
				return launcherList(reader, output)
			case "discover":
				return launcherDiscover(reader, output)
			case "back":
				return nil, nil
			}
		}
	}
}
