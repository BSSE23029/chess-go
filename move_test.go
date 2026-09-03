	package chess

import "testing"

func TestPerft(t *testing.T) {
	tests := []struct {
		name   string
		fen    string
		counts []uint64
	}{
		{"initial", InitialFEN, []uint64{20, 400, 8902, 197281}},
		{"kiwipete", "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1", []uint64{48, 2039, 97862}},
		{"en-passant", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1", []uint64{14, 191, 2812, 43238}},
		{"promotions", "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1", []uint64{6, 264, 9467}},
		{"castling", "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8", []uint64{44, 1486, 62379}},
		{"middlegame", "r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10", []uint64{46, 2079, 89890}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			position, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatal(err)
			}
			for depth, want := range test.counts {
				if got := perft(position, depth+1); got != want {
					t.Fatalf("perft(%d) = %d, want %d", depth+1, got, want)
				}
			}
		})
	}
}

func TestApplyMoveLifecycle(t *testing.T) {
	position := NewPosition()
	for _, uci := range []string{"e2e4", "c7c5", "e4e5", "d7d5", "e5d6"} {
		var err error
		position, err = position.ApplyUCI(uci)
		if err != nil {
			t.Fatalf("ApplyUCI(%q): %v", uci, err)
		}
	}
	if got := position.FEN(); got != "rnbqkbnr/pp2pppp/3P4/2p5/8/8/PPPP1PPP/RNBQKBNR b KQkq - 0 3" {
		t.Fatalf("FEN() = %q", got)
	}
	if _, err := position.ApplyUCI("e8e7"); err == nil {
		t.Fatal("illegal move accepted")
	}
}

func TestCastlingAndPromotion(t *testing.T) {
	position, _ := ParseFEN("r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")
	next, err := position.ApplyUCI("e1g1")
	if err != nil {
		t.Fatal(err)
	}
	if got := next.FEN(); got != "r3k2r/8/8/8/8/8/8/R4RK1 b kq - 1 1" {
		t.Fatalf("castling FEN = %q", got)
	}

	position, _ = ParseFEN("7k/P7/8/8/8/8/8/7K w - - 0 1")
	next, err = position.ApplyUCI("a7a8q")
	if err != nil {
		t.Fatal(err)
	}
	if got := next.FEN(); got != "Q6k/8/8/8/8/8/8/7K b - - 0 1" {
		t.Fatalf("promotion FEN = %q", got)
	}
}

func TestApplyDoesNotMutatePosition(t *testing.T) {
	position := NewPosition()
	if _, err := position.ApplyUCI("e2e4"); err != nil {
		t.Fatal(err)
	}
	if got := position.FEN(); got != InitialFEN {
		t.Fatalf("original mutated: %q", got)
	}
}

func perft(position Position, depth int) uint64 {
	if depth == 0 {
		return 1
	}
	var nodes uint64
	for _, move := range position.LegalMoves() {
		nodes += perft(position.applyUnchecked(move), depth-1)
	}
	return nodes
}

func TestInitialPosition(t *testing.T) {
	position := NewPosition()
	if got := position.FEN(); got != InitialFEN {
		t.Fatalf("FEN() = %q, want %q", got, InitialFEN)
	}
	e1, _ := ParseSquare("e1")
	if got := position.PieceAt(e1); got != (Piece{Type: King, Color: White}) {
		t.Fatalf("PieceAt(e1) = %+v", got)
	}
	if position.Turn() != White || position.Castling() != WhiteKingSide|WhiteQueenSide|BlackKingSide|BlackQueenSide {
		t.Fatal("initial metadata is incorrect")
	}
}

func TestFENRoundTrip(t *testing.T) {
	const fen = "r3k2r/ppp2ppp/2n1bn2/3qp3/3P4/2N1PN2/PPP2PPP/R2QKB1R b KQkq d3 7 12"
	position, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	if got := position.FEN(); got != fen {
		t.Fatalf("FEN() = %q, want %q", got, fen)
	}
	if position.Turn() != Black || position.EnPassant().String() != "d3" || position.HalfmoveClock() != 7 || position.FullmoveNumber() != 12 {
		t.Fatal("parsed metadata is incorrect")
	}
}

func TestParseFENRejectsInvalidInput(t *testing.T) {
	tests := []string{
		"8/8/8/8/8/8/8/8 w - - 0",
		"8/8/8/8/8/8/8 w - - 0 1",
		"8/8/8/8/8/8/8/9 w - - 0 1",
		"8/8/8/8/8/8/8/8 x - - 0 1",
		"8/8/8/8/8/8/8/8 w KK - 0 1",
		"8/8/8/8/8/8/8/8 w - e4 0 1",
		"8/8/8/8/8/8/8/8 w - - -1 1",
		"8/8/8/8/8/8/8/8 w - - 0 0",
	}
	for _, fen := range tests {
		if _, err := ParseFEN(fen); err == nil {
			t.Errorf("ParseFEN(%q) succeeded", fen)
		}
	}
}

func TestSquareRoundTrip(t *testing.T) {
	for _, value := range []string{"a1", "e4", "h8"} {
		square, err := ParseSquare(value)
		if err != nil || square.String() != value {
			t.Fatalf("ParseSquare(%q) = %v, %v", value, square, err)
		}
	}
	for _, value := range []string{"", "a0", "i1", "a10"} {
		if _, err := ParseSquare(value); err == nil {
			t.Errorf("ParseSquare(%q) succeeded", value)
		}
	}
}
