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
	"chess-go/engine"
	"golang.org/x/term"
)

type boardUI struct {
	cursor         chess.Square
	selected       chess.Square
	message        string
	whiteName      string
	blackName      string
	mode           string
	showHelp       bool
	thinking       bool
	botDetail      string
	promotion      []chess.Move
	promotionIndex int
	confirmNew     bool
	cache          tuiCache
	rendered       tuiRenderState
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
	defer fmt.Fprint(output, "\x1b[0m\x1b[?25h\x1b[?1049l")

	ui := boardUI{
		cursor:    initialCursor(s.human),
		selected:  chess.NoSquare,
		whiteName: "White",
		blackName: "Black",
		mode:      "LOCAL MATCH",
	}
	if s.bot != nil {
		if s.human == chess.White {
			ui.whiteName, ui.blackName = s.humanName, s.botLabel()
		} else {
			ui.whiteName, ui.blackName = s.botLabel(), s.humanName
		}
	}
	reader := bufio.NewReader(input)
	for {
		if s.timeout != "" {
			ui.message = "Game over: " + s.timeout + " — n: new game, q: quit"
		} else if result := s.game.Result(); result != "*" {
			ui.message = "Game over: " + result + " — n: new game, q: quit"
		} else if s.bot != nil && s.game.Position().Turn() != s.human {
			mover := s.game.Position().Turn()
			ui.thinking = true
			renderInteractive(output, s.game, &ui, (s.human == chess.Black) != s.flip, s.clockSummary(), s.theme)
			moveCtx, cancel := s.clock.context(ctx, mover)
			move, stats, err := s.chooseInteractiveMove(moveCtx, s.game.Position())
			cancel()
			ui.thinking = false
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
			ui.invalidate()
			if !s.clock.completeMove(mover) {
				s.flag(mover)
				continue
			}
			ui.message = fmt.Sprintf("%s played %s (%s)", s.botLabel(), san, move.UCI())
			if stats.Depth > 0 {
				ui.botDetail = fmt.Sprintf("Depth %d | %d nodes | %+d cp", stats.Depth, stats.Nodes, stats.Score)
			}
		}
		renderInteractive(output, s.game, &ui, (s.human == chess.Black) != s.flip, s.clockSummary(), s.theme)
		if s.timeout != "" {
			return nil
		}
		mover := s.game.Position().Turn()
		gameActive := s.timeout == "" && s.game.Result() == "*"
		if gameActive {
			s.clock.start(mover)
		}
		var pressed key
		var err error
		if s.clock == nil || !gameActive {
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
			ticker := time.NewTicker(500 * time.Millisecond)
			timedOut, inputReceived := false, false
			for !inputReceived && !timedOut {
				select {
				case pressed = <-keys:
					inputReceived = true
				case err = <-keyErrors:
					inputReceived = true
				case <-timer.C:
					s.flag(mover)
					timedOut = true
				case <-ticker.C:
					renderInteractive(output, s.game, &ui, (s.human == chess.Black) != s.flip, s.clockSummary(), s.theme)
				case <-ctx.Done():
					timer.Stop()
					ticker.Stop()
					return ctx.Err()
				}
			}
			timer.Stop()
			ticker.Stop()
			if timedOut {
				continue
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if pressed == keyCommand {
			fmt.Fprint(output, "\r\n:")
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
				ui.invalidate()
				ui.selected = chess.NoSquare
				ui.promotion, ui.promotionIndex = nil, 0
				ui.confirmNew = false
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
	if len(ui.promotion) > 0 {
		return s.handlePromotionKey(ui, pressed)
	}
	if ui.confirmNew {
		switch pressed {
		case keyQuit:
			return true
		case keyNew:
			s.resetInteractiveGame(ui)
			ui.message = "New game"
		case keyEscape:
			ui.confirmNew = false
			ui.message = "New game cancelled"
		default:
			ui.message = "Press n again to start a new game, or Esc to cancel"
		}
		return false
	}
	flipped := (s.human == chess.Black) != s.flip
	switch pressed {
	case keyQuit:
		return true
	case keyUnknown:
		ui.message = "Unknown key — press ? for the keyboard guide"
	case keyUp, keyDown, keyLeft, keyRight:
		ui.cursor = moveCursor(ui.cursor, pressed, flipped)
		ui.message = ""
	case keyEscape:
		wasSelected := ui.selected != chess.NoSquare
		ui.selected = chess.NoSquare
		ui.showHelp = false
		if wasSelected {
			ui.message = "Selection cleared"
		} else {
			ui.message = "Keyboard guide closed"
		}
	case keySelect:
		s.selectSquare(ui)
	case keyUndo:
		if err := s.travel(false); err != nil {
			ui.message = err.Error()
		} else {
			ui.invalidate()
			ui.selected = chess.NoSquare
			ui.message = "Move undone"
		}
	case keyRedo:
		if err := s.travel(true); err != nil {
			ui.message = err.Error()
		} else {
			ui.invalidate()
			ui.selected = chess.NoSquare
			ui.message = "Move restored"
		}
	case keyNew:
		if s.game.MoveCount() > 0 && s.timeout == "" && s.game.Result() == "*" {
			ui.confirmNew = true
			ui.message = "Start a new game? Press n again to confirm, or Esc to cancel"
		} else {
			s.resetInteractiveGame(ui)
			ui.message = "New game"
		}
	case keyHelp:
		ui.showHelp = !ui.showHelp
		if ui.showHelp {
			ui.message = "Keyboard guide opened — press ? or Esc to close"
		} else {
			ui.message = "Keyboard guide closed"
		}
	}
	return false
}

func (s *session) resetInteractiveGame(ui *boardUI) {
	s.game = chess.NewGame()
	ui.invalidate()
	s.timeout = ""
	s.clock.reset()
	ui.cursor, ui.selected = initialCursor(s.human), chess.NoSquare
	ui.promotion, ui.promotionIndex = nil, 0
	ui.confirmNew = false
	ui.botDetail = ""
}

func (s *session) chooseInteractiveMove(ctx context.Context, position chess.Position) (chess.Move, engine.SearchStats, error) {
	if bot, ok := s.bot.(*engine.Bot); ok {
		return bot.Search(ctx, position, engine.SearchLimits{})
	}
	move, err := s.bot.ChooseMove(ctx, position)
	return move, engine.SearchStats{}, err
}

func (s *session) handlePromotionKey(ui *boardUI, pressed key) bool {
	switch pressed {
	case keyQuit:
		return true
	case keyEscape:
		ui.promotion, ui.promotionIndex = nil, 0
		ui.selected = chess.NoSquare
		ui.message = "Promotion cancelled"
	case keyLeft:
		ui.promotionIndex = (ui.promotionIndex + len(ui.promotion) - 1) % len(ui.promotion)
		ui.message = "Promote to " + pieceTypeName(ui.promotion[ui.promotionIndex].Promotion) + " — Enter confirms"
	case keyRight:
		ui.promotionIndex = (ui.promotionIndex + 1) % len(ui.promotion)
		ui.message = "Promote to " + pieceTypeName(ui.promotion[ui.promotionIndex].Promotion) + " — Enter confirms"
	case keySelect:
		move := ui.promotion[ui.promotionIndex]
		s.playInteractiveMove(ui, move)
	default:
		ui.message = "Choose promotion with ←/→, then press Enter"
	}
	return false
}

func (s *session) selectSquare(ui *boardUI) {
	position := s.game.Position()
	model := ui.model(s.game, position, s.theme)
	if model.result != "*" {
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
	choices := make([]chess.Move, 0, 4)
	for _, move := range model.legalMoves {
		if move.From == ui.selected && move.To == ui.cursor {
			choices = append(choices, move)
		}
	}
	if len(choices) > 1 {
		ui.promotion, ui.promotionIndex = choices, 0
		ui.message = "Promote to Queen — ←/→ choose, Enter confirms"
		return
	}
	if len(choices) == 1 {
		s.playInteractiveMove(ui, choices[0])
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

func (s *session) playInteractiveMove(ui *boardUI, move chess.Move) {
	position := s.game.Position()
	san, _ := position.SAN(move)
	if err := s.game.Play(move); err != nil {
		ui.message = err.Error()
		return
	}
	ui.invalidate()
	if !s.clock.completeMove(position.Turn()) {
		s.flag(position.Turn())
	}
	ui.promotion, ui.promotionIndex = nil, 0
	ui.selected = chess.NoSquare
	ui.message = "Played " + san
}
