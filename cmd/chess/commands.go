package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"chess-go"
)

func printTopLevelHelp(output io.Writer) {
	fmt.Fprintln(output, "Usage: chess [menu] | chess version | chess play local|bot|remote [options] | chess host|join|connect|spectate|matchmake|list|discover ... | chess load FILE")
	fmt.Fprintln(output, "Run chess or chess menu in a terminal for the interactive launcher.")
	fmt.Fprintln(output, "Run any subcommand with --help for its flags. Full reference: docs/cli.md")
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func printFlagHelp(output io.Writer, usage string, options *flag.FlagSet) {
	fmt.Fprintln(output, "Usage:", usage)
	fmt.Fprintln(output, "\nOptions:")
	options.SetOutput(output)
	options.PrintDefaults()
}

func (s *session) command(line string, output io.Writer) error {
	if line == "" {
		return nil
	}
	command, argument, _ := strings.Cut(line, " ")
	argument = strings.TrimSpace(argument)
	switch strings.ToLower(command) {
	case "quit", "exit", "q":
		return io.EOF
	case "help":
		fmt.Fprintln(output, "Moves: SAN (Nf3, O-O) or UCI (g1f3). Commands: moves, undo, redo, fen, load, save, theme, flip, resign, draw, quit.")
		return nil
	case "moves":
		return printLegalMoves(output, s.game.Position())
	case "undo":
		return s.travel(false)
	case "redo":
		return s.travel(true)
	case "theme":
		if argument == "" {
			fmt.Fprintln(output, "Theme:", s.theme.label())
			return nil
		}
		theme, err := parseTheme(argument)
		if err != nil {
			return err
		}
		s.theme = theme
		fmt.Fprintln(output, "Theme:", theme.label())
		return nil
	case "flip":
		s.flip = !s.flip
		fmt.Fprintln(output, "Board flipped:", s.flip)
		return nil
	case "resign":
		if s.timeout != "" || s.game.Result() != "*" {
			return errors.New("game is already over")
		}
		s.timeout = colorName(s.game.Position().Turn().Opponent()) + " wins by resignation"
		return nil
	case "draw":
		if s.timeout != "" || s.game.Result() != "*" {
			return errors.New("game is already over")
		}
		s.timeout = "Draw by agreement"
		return nil
	case "fen":
		position, err := chess.ParseFEN(argument)
		if err != nil {
			return err
		}
		s.game = chess.NewGameFromPosition(position)
		s.timeout = ""
		s.clock.reset()
		return nil
	case "load":
		game, err := loadPGN(argument)
		if err != nil {
			return err
		}
		s.game = game
		s.timeout = ""
		s.clock.reset()
		return nil
	case "save":
		if argument == "" {
			return errors.New("save requires a file path")
		}
		if err := os.WriteFile(argument, []byte(s.game.PGN()+"\n"), 0o644); err != nil {
			return err
		}
		fmt.Fprintln(output, "Saved", argument)
		return nil
	}
	if err := s.game.PlaySAN(line); err == nil {
		return nil
	}
	return s.game.PlayUCI(line)
}
