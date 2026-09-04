package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"chess-go"
	"golang.org/x/term"
)

type boardUI struct {
	cursor   chess.Square
	selected chess.Square
	message  string
}

func isInteractiveTerminal(input io.Reader, output io.Writer) bool {
	in, inputOK := input.(*os.File)
	out, outputOK := output.(*os.File)
	return inputOK && outputOK && term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}

func (s *session) playInteractive(ctx context.Context, input io.Reader, output io.Writer) error {
	in := input.(*os.File)
	state, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return fmt.Errorf("enable interactive terminal: %w", err)
	}
	defer term.Restore(int(in.Fd()), state) //nolint:errcheck -- best effort during shutdown
	fmt.Fprint(output, "\x1b[?1049h\x1b[?25l")
	defer fmt.Fprint(output, "\x1b[?25h\x1b[?1049l")

	ui := boardUI{cursor: initialCursor(s.human), selected: chess.NoSquare}
	reader := bufio.NewReader(input)
	for {
		if s.timeout != "" {
			ui.message = "Game over: " + s.timeout + " — n: new game, q: quit"
		} else if result := s.game.Result(); result != "*" {
			ui.message = "Game over: " + result + " — n: new game, q: quit"
		} else if s.bot != nil && s.game.Position().Turn() != s.human {
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
			ui.message = fmt.Sprintf("%s played %s (%s)", s.botLabel(), san, move.UCI())
		}
		renderInteractive(output, s.game, ui, s.human == chess.Black, s.clockSummary())
		if s.timeout != "" {
			return nil
		}
		mover := s.game.Position().Turn()
		if s.timeout == "" && s.game.Result() == "*" {
			s.clock.start(mover)
		}
		var pressed key
		var err error
		if s.clock == nil || s.timeout != "" || s.game.Result() != "*" {
			pressed, err = readKey(reader)
		} else {
			keys := make(chan key, 1)
			keyErrors := make(chan error, 1)
			go func() {
				value, readErr := readKey(reader)
				if readErr != nil {
					keyErrors <- readErr
					return
				}
				keys <- value
			}()
			timer := time.NewTimer(s.clock.untilFlag(mover))
			select {
			case pressed = <-keys:
				timer.Stop()
			case err = <-keyErrors:
				timer.Stop()
			case <-timer.C:
				s.flag(mover)
				continue
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if pressed == keyCommand {
			fmt.Fprint(output, "\n:")
			line, err := readRawLine(reader, output)
			if err != nil {
				return err
			}
			before := len(s.game.Moves())
			if err := s.command(strings.TrimSpace(line), output); errors.Is(err, io.EOF) {
				return nil
			} else if err != nil {
				ui.message = "Error: " + err.Error()
			} else {
				if len(s.game.Moves()) > before && !s.clock.completeMove(mover) {
					s.flag(mover)
				}
				ui.selected = chess.NoSquare
				ui.message = "Command completed"
			}
			continue
		}
		if quit := s.handleKey(&ui, pressed); quit {
			return nil
		}
	}
}

func initialCursor(human chess.Color) chess.Square {
	if human == chess.Black {
		square, _ := chess.ParseSquare("e7")
		return square
	}
	square, _ := chess.ParseSquare("e2")
	return square
}

func (s *session) handleKey(ui *boardUI, pressed key) bool {
	flipped := s.human == chess.Black
	switch pressed {
	case keyQuit:
		return true
	case keyUp, keyDown, keyLeft, keyRight:
		ui.cursor = moveCursor(ui.cursor, pressed, flipped)
		ui.message = ""
	case keySelect:
		s.selectSquare(ui)
	case keyUndo:
		if err := s.travel(false); err != nil {
			ui.message = err.Error()
		} else {
			ui.selected = chess.NoSquare
			ui.message = "Move undone"
		}
	case keyRedo:
		if err := s.travel(true); err != nil {
			ui.message = err.Error()
		} else {
			ui.selected = chess.NoSquare
			ui.message = "Move restored"
		}
	case keyNew:
		s.game = chess.NewGame()
		s.timeout = ""
		s.clock.reset()
		ui.cursor, ui.selected = initialCursor(s.human), chess.NoSquare
		ui.message = "New game"
	case keyHelp:
		ui.message = "Arrows/hjkl: move  Enter: select  u/r: undo/redo  n: new  : command  q: quit"
	}
	return false
}

func (s *session) selectSquare(ui *boardUI) {
	position := s.game.Position()
	if s.game.Result() != "*" {
		ui.message = "Game is over; press n for a new game"
		return
	}
	if ui.selected == chess.NoSquare {
		piece := position.PieceAt(ui.cursor)
		if piece.IsEmpty() || piece.Color != position.Turn() {
			ui.message = "Select a piece belonging to the side to move"
			return
		}
		ui.selected = ui.cursor
		ui.message = "Select a highlighted destination"
		return
	}
	if ui.cursor == ui.selected {
		ui.selected = chess.NoSquare
		ui.message = "Selection cleared"
		return
	}
	var chosen chess.Move
	found := false
	for _, move := range position.LegalMoves() {
		if move.From == ui.selected && move.To == ui.cursor && (!found || move.Promotion == chess.Queen) {
			chosen, found = move, true
		}
	}
	if found {
		san, _ := position.SAN(chosen)
		if err := s.game.Play(chosen); err != nil {
			ui.message = err.Error()
			return
		}
		if !s.clock.completeMove(position.Turn()) {
			s.flag(position.Turn())
		}
		ui.selected = chess.NoSquare
		ui.message = "Played " + san
		return
	}
	piece := position.PieceAt(ui.cursor)
	if !piece.IsEmpty() && piece.Color == position.Turn() {
		ui.selected = ui.cursor
		ui.message = "Selection changed"
		return
	}
	ui.message = "That is not a legal destination"
}

func renderInteractive(output io.Writer, game *chess.Game, ui boardUI, flipped bool, clocks string) {
	position := game.Position()
	files, ranks := []int{0, 1, 2, 3, 4, 5, 6, 7}, []int{7, 6, 5, 4, 3, 2, 1, 0}
	if flipped {
		files, ranks = []int{7, 6, 5, 4, 3, 2, 1, 0}, []int{0, 1, 2, 3, 4, 5, 6, 7}
	}
	legal := make(map[chess.Square]bool)
	if ui.selected != chess.NoSquare {
		for _, move := range position.LegalMoves() {
			if move.From == ui.selected {
				legal[move.To] = true
			}
		}
	}
	last := map[chess.Square]bool{}
	moves := game.Moves()
	if len(moves) > 0 {
		last[moves[len(moves)-1].From], last[moves[len(moves)-1].To] = true, true
	}
	checkSquare := chess.NoSquare
	if position.InCheck() {
		for square := chess.Square(0); square < 64; square++ {
			piece := position.PieceAt(square)
			if piece.Type == chess.King && piece.Color == position.Turn() {
				checkSquare = square
			}
		}
	}
	fmt.Fprint(output, "\x1b[H\x1b[2JChess\n\n")
	for _, rank := range ranks {
		fmt.Fprintf(output, "%d ", rank+1)
		for _, file := range files {
			square := chess.Square(rank*8 + file)
			style := ""
			switch {
			case square == ui.cursor:
				style = "\x1b[7m"
			case square == checkSquare:
				style = "\x1b[41m"
			case square == ui.selected:
				style = "\x1b[46m"
			case legal[square]:
				style = "\x1b[42m"
			case last[square]:
				style = "\x1b[43m"
			}
			fmt.Fprintf(output, "%s%c \x1b[0m", style, pieceSymbol(position.PieceAt(square)))
		}
		fmt.Fprintln(output)
	}
	fmt.Fprint(output, "  ")
	for _, file := range files {
		fmt.Fprintf(output, "%c  ", 'a'+file)
	}
	fmt.Fprint(output, "\n")
	if clocks != "" {
		fmt.Fprintln(output, clocks)
	}
	fmt.Fprintf(output, "%s\n%s to move", capturedSummary(game), colorName(position.Turn()))
	if position.InCheck() {
		fmt.Fprint(output, " — Check")
	}
	fmt.Fprintln(output)
	if ui.message != "" {
		fmt.Fprintln(output, ui.message)
	}
	fmt.Fprintln(output, "Arrows/hjkl move · Enter selects · u undo · r redo · n new · : command · ? help · q quit")
}
