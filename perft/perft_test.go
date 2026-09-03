package perft

import (
	"context"
	"errors"
	"testing"

	"chess-go"
)

func TestStandardPositions(t *testing.T) {
	tests := []struct {
		name   string
		fen    string
		counts []uint64
	}{
		{"initial", chess.InitialFEN, []uint64{20, 400, 8902, 197281}},
		{"kiwipete", "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1", []uint64{48, 2039, 97862}},
		{"en-passant", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1", []uint64{14, 191, 2812, 43238}},
		{"promotions", "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1", []uint64{6, 264, 9467}},
		{"castling", "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8", []uint64{44, 1486, 62379}},
		{"middlegame", "r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10", []uint64{46, 2079, 89890}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			position, err := chess.ParseFEN(test.fen)
			if err != nil {
				t.Fatal(err)
			}
			before := position.FEN()
			for depth, want := range test.counts {
				got, err := Count(context.Background(), position, depth+1)
				if err != nil || got != want {
					t.Fatalf("Count(%d) = %d, %v; want %d", depth+1, got, err, want)
				}
			}
			if position.FEN() != before {
				t.Fatal("Count mutated its input")
			}
		})
	}
}

func TestDivideAndValidation(t *testing.T) {
	position := chess.NewPosition()
	results, err := Divide(context.Background(), position, 2)
	if err != nil || len(results) != 20 {
		t.Fatalf("Divide() returned %d moves, %v", len(results), err)
	}
	var total uint64
	for _, result := range results {
		total += result.Nodes
	}
	if total != 400 || position.FEN() != chess.InitialFEN {
		t.Fatalf("divide total = %d or input mutated", total)
	}
	if _, err := Count(context.Background(), position, -1); err == nil {
		t.Fatal("negative depth accepted")
	}
	if _, err := Divide(context.Background(), position, 0); err == nil {
		t.Fatal("zero divide depth accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Count(ctx, position, 2); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Count error = %v", err)
	}
}
