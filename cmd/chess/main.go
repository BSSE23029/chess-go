package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"chess-go"
	"chess-go/engine"
)

type session struct {
	game      *chess.Game
	bot       chess.Player
	human     chess.Color
	humanName string
	botName   string
	botLevel  string
	clock     *gameClock
	timeout   string
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "chess:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: chess play local [options] | chess play bot [--level NAME | --depth N] [options] | chess load FILE")
	}
	s := session{
		game:      chess.NewGame(),
		human:     chess.White,
		humanName: firstSet(os.Getenv("CHESS_PLAYER_NAME"), os.Getenv("USER"), "Player"),
		botName:   firstSet(os.Getenv("CHESS_BOT_NAME"), "Bot"),
	}
	switch args[0] {
	case "play":
		if len(args) < 2 {
			return errors.New("play requires local or bot")
		}
		switch args[1] {
		case "local":
			options := flag.NewFlagSet("play local", flag.ContinueOnError)
			options.SetOutput(io.Discard)
			limit, increment := clockFlags(options)
			if err := options.Parse(args[2:]); err != nil || options.NArg() != 0 {
				return errors.New("usage: chess play local [--clock DURATION] [--increment DURATION]")
			}
			clock, err := parseClock(*limit, *increment)
			if err != nil {
				return err
			}
			s.clock = clock
		case "bot":
			defaultDepth, err := envInt("CHESS_BOT_DEPTH", 3)
			if err != nil {
				return err
			}
			options := flag.NewFlagSet("play bot", flag.ContinueOnError)
			options.SetOutput(io.Discard)
			depth := options.Int("depth", defaultDepth, "search depth")
			level := options.String("level", os.Getenv("CHESS_BOT_LEVEL"), "named strength profile")
			color := options.String("color", firstSet(os.Getenv("CHESS_PLAYER_COLOR"), "white"), "human color")
			limit, increment := clockFlags(options)
			if err := options.Parse(args[2:]); err != nil || options.NArg() != 0 || *depth < 1 {
				return errors.New("usage: chess play bot [--level NAME | --depth N] [--color white|black]")
			}
			switch *color {
			case "white":
				s.human = chess.White
			case "black":
				s.human = chess.Black
			default:
				return fmt.Errorf("invalid color %q", *color)
			}
			if *level != "" {
				profile, err := engine.ParseStrengthProfile(*level)
				if err != nil {
					return err
				}
				s.bot = engine.NewProfile(profile)
				s.botLevel = profile.String()
			} else {
				s.bot = engine.New(*depth)
			}
			s.clock, err = parseClock(*limit, *increment)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown play mode %q", args[1])
		}
	case "load":
		if len(args) != 2 {
			return errors.New("load requires one PGN file")
		}
		game, err := loadPGN(args[1])
		if err != nil {
			return err
		}
		s.game = game
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return s.play(ctx, input, output)
}

func (s *session) play(ctx context.Context, input io.Reader, output io.Writer) error {
	if isInteractiveTerminal(input, output) {
		return s.playInteractive(ctx, input, output)
	}
	scanner := bufio.NewScanner(input)
	lines := make(chan string)
	scanDone := make(chan error, 1)
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		scanDone <- scanner.Err()
	}()
	fmt.Fprintln(output, "Commands: SAN or UCI move, moves, undo, redo, fen FEN, load FILE, save FILE, help, quit")
	for {
		render(output, s.game, s.human == chess.Black, s.clockSummary())
		if s.timeout != "" {
			fmt.Fprintln(output, "Game over:", s.timeout)
			return nil
		}
		if result := s.game.Result(); result != "*" {
			fmt.Fprintln(output, "Game over:", result)
			return nil
		}
		if s.bot != nil && s.game.Position().Turn() != s.human {
			mover := s.game.Position().Turn()
			moveCtx, cancel := s.clock.context(ctx, mover)
			move, err := s.bot.ChooseMove(moveCtx, s.game.Position())
			cancel()
			if err != nil {
				if s.clock != nil && s.clock.values()[mover] <= 0 {
					s.flag(mover)
					continue
				}
				return fmt.Errorf("bot move: %w", err)
			}
			san, _ := s.game.Position().SAN(move)
			if err := s.game.Play(move); err != nil {
				return fmt.Errorf("bot returned invalid move: %w", err)
			}
			if !s.clock.completeMove(mover) {
				s.flag(mover)
				continue
			}
			fmt.Fprintf(output, "%s played %s (%s)\n", s.botLabel(), san, move.UCI())
			continue
		}
		name := colorName(s.game.Position().Turn())
		if s.bot != nil {
			name = s.humanName
		}
		fmt.Fprintf(output, "%s to move > ", name)
		mover := s.game.Position().Turn()
		s.clock.start(mover)
		var line string
		if s.clock == nil {
			select {
			case line = <-lines:
			case err := <-scanDone:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			timer := time.NewTimer(s.clock.untilFlag(mover))
			select {
			case line = <-lines:
				if !timer.Stop() {
					<-timer.C
				}
			case err := <-scanDone:
				timer.Stop()
				return err
			case <-timer.C:
				s.flag(mover)
				continue
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}
		before := len(s.game.Moves())
		if err := s.command(strings.TrimSpace(line), output); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			fmt.Fprintln(output, "Error:", err)
		} else if len(s.game.Moves()) > before && !s.clock.completeMove(mover) {
			s.flag(mover)
		}
	}
}

func (s *session) command(line string, output io.Writer) error {
	if line == "" {
		return nil
	}
	command, argument, _ := strings.Cut(line, " ")
	argument = strings.TrimSpace(argument)
	switch strings.ToLower(command) {
	case "quit", "exit":
		return io.EOF
	case "help":
		fmt.Fprintln(output, "Enter moves as SAN (Nf3, O-O) or UCI (g1f3). File commands accept local paths.")
		return nil
	case "moves":
		return printLegalMoves(output, s.game.Position())
	case "undo":
		return s.travel(false)
	case "redo":
		return s.travel(true)
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

func (s *session) travel(redo bool) error {
	steps := 1
	if s.bot != nil {
		steps = 2
		if !redo && len(s.game.Moves()) < steps {
			return errors.New("no complete human/bot turn to undo")
		}
	}
	completed := 0
	for range steps {
		if redo {
			if !s.game.Redo() {
				for range completed {
					s.game.Undo()
				}
				return errors.New("nothing to redo")
			}
		} else if !s.game.Undo() {
			return errors.New("nothing to undo")
		}
		completed++
	}
	s.clock.sync(s.game.Position().Turn())
	return nil
}

func envInt(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func firstSet(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (s *session) botLabel() string {
	if s.botLevel == "" {
		return s.botName
	}
	return s.botName + " [" + s.botLevel + "]"
}

func loadPGN(path string) (*chess.Game, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return chess.ParsePGN(string(data))
}

func render(output io.Writer, game *chess.Game, flipped bool, clocks string) {
	position := game.Position()
	files := []int{0, 1, 2, 3, 4, 5, 6, 7}
	ranks := []int{7, 6, 5, 4, 3, 2, 1, 0}
	if flipped {
		files, ranks = []int{7, 6, 5, 4, 3, 2, 1, 0}, []int{0, 1, 2, 3, 4, 5, 6, 7}
	}
	fmt.Fprintln(output)
	for _, rank := range ranks {
		fmt.Fprintf(output, "%d ", rank+1)
		for _, file := range files {
			fmt.Fprintf(output, "%c ", pieceSymbol(position.PieceAt(chess.Square(rank*8+file))))
		}
		fmt.Fprintln(output)
	}
	fmt.Fprint(output, "  ")
	for _, file := range files {
		fmt.Fprintf(output, "%c ", 'a'+file)
	}
	fmt.Fprintln(output)
	if clocks != "" {
		fmt.Fprintln(output, clocks)
	}
	fmt.Fprintln(output, capturedSummary(game))
	fmt.Fprint(output, "Moves:")
	for _, move := range game.Moves() {
		fmt.Fprint(output, " ", move.UCI())
	}
	fmt.Fprintln(output)
	if position.InCheck() {
		fmt.Fprintln(output, "Check")
	}
}

func pieceSymbol(piece chess.Piece) byte {
	symbol := map[chess.PieceType]byte{chess.NoPiece: '.', chess.Pawn: 'p', chess.Knight: 'n', chess.Bishop: 'b', chess.Rook: 'r', chess.Queen: 'q', chess.King: 'k'}[piece.Type]
	if !piece.IsEmpty() && piece.Color == chess.White {
		symbol = byte(strings.ToUpper(string(symbol))[0])
	}
	return symbol
}

func capturedSummary(game *chess.Game) string {
	white, black := "", ""
	for _, piece := range game.Captured() {
		if piece.Color == chess.Black {
			white += string(pieceSymbol(piece))
		} else {
			black += string(pieceSymbol(piece))
		}
	}
	if white == "" {
		white = "-"
	}
	if black == "" {
		black = "-"
	}
	return fmt.Sprintf("Captured by White: %s · Black: %s", white, black)
}

func printLegalMoves(output io.Writer, position chess.Position) error {
	for index, move := range position.LegalMoves() {
		san, err := position.SAN(move)
		if err != nil {
			return err
		}
		if index > 0 {
			fmt.Fprint(output, " ")
		}
		fmt.Fprintf(output, "%s(%s)", san, move.UCI())
	}
	fmt.Fprintln(output)
	return nil
}

func colorName(color chess.Color) string {
	if color == chess.White {
		return "White"
	}
	return "Black"
}
