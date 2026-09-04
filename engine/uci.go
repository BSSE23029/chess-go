package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"chess-go"
)

// ErrNoBestMove indicates that a UCI process did not return a usable move.
var ErrNoBestMove = errors.New("UCI engine returned no bestmove")

// UCIEngine adapts an external UCI-compatible executable to chess.Player.
// Command and Args are passed directly to os/exec; callers should configure
// them from their own environment or deployment settings.
type UCIEngine struct {
	// Command is the executable path or name, such as stockfish.
	Command string
	// Args are optional fixed command-line arguments.
	Args []string
	// Depth is the requested UCI search depth in plies. Values below one use 1.
	Depth int
}

var _ chess.Player = (*UCIEngine)(nil)

// NewUCIEngine returns an adapter for command. It does not start the process
// until ChooseMove, allowing callers to construct configuration before an
// executable is installed or selected.
func NewUCIEngine(command string, args ...string) (*UCIEngine, error) {
	if strings.TrimSpace(command) == "" {
		return nil, errors.New("UCI engine command is required")
	}
	return &UCIEngine{Command: command, Args: append([]string(nil), args...), Depth: 8}, nil
}

// ChooseMove asks the external engine for a move and validates it locally.
func (e *UCIEngine) ChooseMove(ctx context.Context, position chess.Position) (chess.Move, error) {
	if e == nil || strings.TrimSpace(e.Command) == "" {
		return chess.Move{}, errors.New("UCI engine command is required")
	}
	if ctx == nil {
		return chess.Move{}, errors.New("nil UCI context")
	}
	depth := e.Depth
	if depth < 1 {
		depth = 1
	}
	commands := strings.Join([]string{
		"uci",
		"isready",
		"ucinewgame",
		"isready",
		"position fen " + position.FEN(),
		fmt.Sprintf("go depth %d", depth),
		"quit",
		"",
	}, "\n")
	command := exec.CommandContext(ctx, e.Command, e.Args...)
	command.Stdin = strings.NewReader(commands)
	var output bytes.Buffer
	command.Stdout = &output
	if err := command.Run(); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return chess.Move{}, contextErr
		}
		return chess.Move{}, fmt.Errorf("run UCI engine %q: %w", e.Command, err)
	}
	value, err := parseBestMove(output.String())
	if err != nil {
		return chess.Move{}, err
	}
	return legalUCIMove(position, value)
}

func parseBestMove(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "bestmove" {
			continue
		}
		if len(fields) < 2 || fields[1] == "(none)" {
			return "", ErrNoBestMove
		}
		return fields[1], nil
	}
	return "", ErrNoBestMove
}

func legalUCIMove(position chess.Position, value string) (chess.Move, error) {
	requested, err := chess.ParseUCI(value)
	if err != nil {
		return chess.Move{}, fmt.Errorf("UCI engine returned %q: %w", value, err)
	}
	for _, move := range position.LegalMoves() {
		if move.From == requested.From && move.To == requested.To && move.Promotion == requested.Promotion {
			return move, nil
		}
	}
	return chess.Move{}, fmt.Errorf("UCI engine returned illegal move %q", value)
}
