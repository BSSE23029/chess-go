package engine

import (
	"context"
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
