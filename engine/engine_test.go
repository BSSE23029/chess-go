package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"chess-go"
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

func TestPositionalEvaluationAddsStructureTerms(t *testing.T) {
	evaluator := PositionalEvaluator{}
	initial := evaluator.Evaluate(chess.NewPosition())
	central, _ := chess.ParseFEN("4k3/8/8/3pp3/3PP3/8/8/4K3 w - - 0 1")
	if got := evaluator.Evaluate(central); got == 0 || initial == got {
		t.Fatalf("positional evaluator did not add terms: initial %d central %d", initial, got)
	}
	passed, _ := chess.ParseFEN("4k3/8/8/4P3/8/8/8/4K3 w - - 0 1")
	blocked, _ := chess.ParseFEN("4k3/8/4p3/4P3/8/8/8/4K3 w - - 0 1")
	if evaluator.Evaluate(passed) <= evaluator.Evaluate(blocked) {
		t.Fatal("passed pawn was not rewarded")
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

func TestIterativeSearchAndNodeLimit(t *testing.T) {
	position := chess.NewPosition()
	move, stats, err := New(3).Search(context.Background(), position, SearchLimits{MaxDepth: 3})
	if err != nil || stats.Depth != 3 || stats.Nodes == 0 {
		t.Fatalf("iterative search = %s, %#v, %v", move.UCI(), stats, err)
	}
	if _, err := position.Apply(move); err != nil {
		t.Fatalf("iterative search returned illegal move: %v", err)
	}
	limitedMove, limited, err := New(4).Search(context.Background(), position, SearchLimits{MaxDepth: 4, MaxNodes: 45})
	if err != nil || limited.Depth != 1 || limited.Nodes > 45 {
		t.Fatalf("limited search = %s, %#v, %v", limitedMove.UCI(), limited, err)
	}
	if _, err := position.Apply(limitedMove); err != nil {
		t.Fatalf("limited search returned illegal move: %v", err)
	}
}

func TestSearchTimeAndContextLimits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := New(3).Search(ctx, chess.NewPosition(), SearchLimits{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled search error = %v", err)
	}
	if _, _, err := New(5).Search(context.Background(), chess.NewPosition(), SearchLimits{MaxDepth: 5, Time: time.Nanosecond}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed search error = %v", err)
	}
}

func TestStrengthProfiles(t *testing.T) {
	want := []struct {
		name  string
		level StrengthProfile
		depth int
		loss  Score
	}{
		{"Learner", Learner, 1, 300}, {"Beginner", Beginner, 2, 200},
		{"Casual", Casual, 2, 100}, {"Club", Club, 3, 50},
		{"Advanced", Advanced, 3, 25}, {"Expert", Expert, 4, 10}, {"Maximum", Maximum, 4, 0},
	}
	for _, test := range want {
		profile, err := ParseStrengthProfile(strings.ToLower(test.name))
		if err != nil || profile != test.level || profile.String() != test.name {
			t.Fatalf("profile %q = %v, %v", test.name, profile, err)
		}
		bot := NewProfile(profile)
		if bot.Depth != test.depth || bot.MaxLoss != test.loss || bot.Strength != test.level {
			t.Fatalf("config %q = depth %d loss %d level %v", test.name, bot.Depth, bot.MaxLoss, bot.Strength)
		}
	}
	if _, err := ParseStrengthProfile("unrated"); err == nil {
		t.Fatal("unknown profile accepted")
	}
	if got := Profiles(); len(got) != 7 || got[0] != Learner || got[6] != Maximum {
		t.Fatalf("profiles = %#v", got)
	}
}
