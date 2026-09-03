package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/BSSE23029/chess-go"
)

var _ chess.Player = (*Bot)(nil)

func TestMaterialEvaluation(t *testing.T) {
	evaluator := MaterialEvaluator{}
	if got := evaluator.Evaluate(chess.NewPosition()); got != 0 {
		t.Fatalf("initial evaluation = %d", got)
	}
	position, _ := chess.ParseFEN("4k3/8/8/8/8/8/3Q4/4K3 w - - 0 1")
	if got := evaluator.Evaluate(position); got != 900 {
		t.Fatalf("queen advantage = %d", got)
	}
}

func TestBotChoosesMaterialAndLeavesInputUnchanged(t *testing.T) {
	position, _ := chess.ParseFEN("4k3/8/8/8/3q4/8/3R4/4K3 w - - 0 1")
	hash := position.Hash()
	move, err := New(1).ChooseMove(context.Background(), position)
	if err != nil {
		t.Fatal(err)
	}
	if got := move.UCI(); got != "d2d4" {
		t.Fatalf("ChooseMove() = %s, want d2d4", got)
	}
	if position.Hash() != hash {
		t.Fatal("search mutated its input")
	}
	if _, err := position.Apply(move); err != nil {
		t.Fatalf("bot returned illegal move: %v", err)
	}
}

func TestBotFindsMate(t *testing.T) {
	position, _ := chess.ParseFEN("7k/5Q2/6K1/8/8/8/8/8 w - - 0 1")
	move, err := New(2).ChooseMove(context.Background(), position)
	if err != nil {
		t.Fatal(err)
	}
	next, err := position.Apply(move)
	if err != nil || !next.InCheck() || len(next.LegalMoves()) != 0 {
		t.Fatalf("ChooseMove() = %s, not mate", move.UCI())
	}
}

func TestBotCancellationAndTerminalPosition(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(3).ChooseMove(ctx, chess.NewPosition()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled search error = %v", err)
	}
	position, _ := chess.ParseFEN("7k/5Q2/6K1/8/8/8/8/8 b - - 0 1")
	if _, err := New(1).ChooseMove(context.Background(), position); !errors.Is(err, ErrNoLegalMoves) {
		t.Fatalf("terminal search error = %v", err)
	}
}
