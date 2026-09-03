package chess

import "testing"

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
