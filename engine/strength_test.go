package engine

import (
	"context"
	"slices"
	"testing"

	"chess-go"
)

func TestTacticalStrengthSuite(t *testing.T) {
	tests := []struct {
		name  string
		fen   string
		depth int
		check func(t *testing.T, position chess.Position, move chess.Move)
	}{
		{
			name:  "mate in one",
			fen:   "7k/5Q2/6K1/8/8/8/8/8 w - - 0 1",
			depth: 2,
			check: func(t *testing.T, position chess.Position, move chess.Move) {
				next, err := position.Apply(move)
				if err != nil || !next.InCheck() || len(next.LegalMoves()) != 0 {
					t.Fatalf("move %s did not deliver mate: %v", move.UCI(), err)
				}
			},
		},
		{
			name:  "promotion",
			fen:   "7k/P7/8/8/8/8/8/7K w - - 0 1",
			depth: 3,
			check: func(t *testing.T, _ chess.Position, move chess.Move) {
				if move.UCI() != "a7a8q" {
					t.Fatalf("promotion move = %s, want a7a8q", move.UCI())
				}
			},
		},
		{
			name:  "forced check evasion",
			fen:   "4k3/8/8/8/8/8/4r3/4K3 w - - 0 1",
			depth: 3,
			check: func(t *testing.T, _ chess.Position, move chess.Move) {
				if move.UCI() != "e1e2" {
					t.Fatalf("check evasion = %s, want e1e2", move.UCI())
				}
			},
		},
		{
			name:  "forcing check",
			fen:   "6k1/5ppp/8/8/6Q1/8/5PPP/6K1 w - - 0 1",
			depth: 2,
			check: func(t *testing.T, position chess.Position, move chess.Move) {
				next, err := position.Apply(move)
				if err != nil || !next.InCheck() {
					t.Fatalf("forcing move %s did not give check: %v", move.UCI(), err)
				}
			},
		},
		{
			name:  "defensive block or king move",
			fen:   "4r1k1/8/8/8/8/8/3Q4/4K3 w - - 0 1",
			depth: 2,
			check: func(t *testing.T, position chess.Position, move chess.Move) {
				if !position.InCheck() {
					t.Fatal("defensive test position is not in check")
				}
				next, err := position.Apply(move)
				if err != nil || next.InCheck() {
					t.Fatalf("defensive move %s left the king in check: %v", move.UCI(), err)
				}
			},
		},
		{
			name:  "winning capture",
			fen:   "4k3/8/8/8/3q4/8/3R4/4K3 w - - 0 1",
			depth: 1,
			check: func(t *testing.T, _ chess.Position, move chess.Move) {
				if move.UCI() != "d2d4" {
					t.Fatalf("winning capture = %s, want d2d4", move.UCI())
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			position, err := chess.ParseFEN(test.fen)
			if err != nil {
				t.Fatal(err)
			}
			hash := position.Hash()
			move, err := New(test.depth).ChooseMove(context.Background(), position)
			if err != nil {
				t.Fatal(err)
			}
			if position.Hash() != hash {
				t.Fatal("tactical search mutated its input")
			}
			test.check(t, position, move)
		})
	}
}

func selfPlayLine(t *testing.T, white, black *Bot, plies int) []string {
	t.Helper()
	game := chess.NewGame()
	line := make([]string, 0, plies)
	for range plies {
		bot := white
		if game.Position().Turn() == chess.Black {
			bot = black
		}
		move, err := bot.ChooseMove(context.Background(), game.Position())
		if err != nil {
			t.Fatal(err)
		}
		line = append(line, move.UCI())
		if err := game.Play(move); err != nil {
			t.Fatalf("self-play move %s: %v", move.UCI(), err)
		}
		if game.Result() != "*" {
			break
		}
	}
	return line
}

func TestSelfPlayRegressionSeparatesDeterministicAndSeededRandomModes(t *testing.T) {
	deterministic := selfPlayLine(t, New(2), New(2), 12)
	repeat := selfPlayLine(t, New(2), New(2), 12)
	if !slices.Equal(deterministic, repeat) {
		t.Fatalf("deterministic self-play changed: %v then %v", deterministic, repeat)
	}
	randomA := selfPlayLine(t, NewRandom(2, 42), NewRandom(2, 99), 12)
	randomRepeat := selfPlayLine(t, NewRandom(2, 42), NewRandom(2, 99), 12)
	if !slices.Equal(randomA, randomRepeat) {
		t.Fatalf("seeded random self-play changed: %v then %v", randomA, randomRepeat)
	}
	randomDifferent := selfPlayLine(t, NewRandom(2, 43), NewRandom(2, 100), 12)
	if slices.Equal(randomA, randomDifferent) {
		t.Fatalf("different seeds produced the same self-play line: %v", randomA)
	}
	if slices.Equal(deterministic, randomA) {
		t.Fatalf("randomized self-play did not diverge from deterministic line: %v", deterministic)
	}
}
