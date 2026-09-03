package chess

import (
	"strings"
	"testing"
)

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
	game := NewGameFromPosition(position)
	if err := game.PlayUCI("e1g1"); err != nil || game.Moves()[0].Flags&Castle == 0 {
		t.Fatalf("game did not retain castling move: %v", err)
	}
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

func TestGamePlayUndoRedoLifecycle(t *testing.T) {
	game := NewGame()
	for _, move := range []string{"e2e4", "e7e5", "g1f3"} {
		if err := game.PlayUCI(move); err != nil {
			t.Fatalf("PlayUCI(%q): %v", move, err)
		}
	}
	played := game.Moves()
	played[0] = Move{}
	if game.Moves()[0].UCI() != "e2e4" {
		t.Fatal("Moves exposed mutable game history")
	}
	after := game.Position().FEN()
	if !game.Undo() || game.Position().Turn() != White || !game.CanRedo() {
		t.Fatal("undo did not restore the prior turn")
	}
	if !game.Redo() || game.Position().FEN() != after {
		t.Fatal("redo did not restore the position")
	}
	game.Undo()
	if err := game.PlayUCI("d2d4"); err != nil {
		t.Fatal(err)
	}
	if game.CanRedo() || len(game.Moves()) != 3 {
		t.Fatal("new play did not replace the redo branch")
	}
	if err := game.PlayUCI("e1e8"); err == nil {
		t.Fatal("illegal game move accepted")
	}
}

func TestGameCheckmateAndStalemate(t *testing.T) {
	game := NewGame()
	for _, move := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
		if err := game.PlayUCI(move); err != nil {
			t.Fatal(err)
		}
	}
	if got := game.Status(); got != BlackCheckmates {
		t.Fatalf("Status() = %v, want BlackCheckmates", got)
	}
	if err := game.PlayUCI("a2a3"); err == nil {
		t.Fatal("move accepted after checkmate")
	}

	position, _ := ParseFEN("7k/5Q2/6K1/8/8/8/8/8 b - - 0 1")
	if got := NewGameFromPosition(position).Status(); got != Stalemate {
		t.Fatalf("Status() = %v, want Stalemate", got)
	}
}

func TestGameDrawRules(t *testing.T) {
	position, _ := ParseFEN("7k/8/8/8/8/8/8/R6K w - - 99 1")
	game := NewGameFromPosition(position)
	if err := game.PlayUCI("a1a2"); err != nil {
		t.Fatal(err)
	}
	if got := game.Status(); got != DrawFiftyMove {
		t.Fatalf("Status() = %v, want DrawFiftyMove", got)
	}

	game = NewGame()
	for _, move := range []string{"g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1", "f6g8"} {
		if err := game.PlayUCI(move); err != nil {
			t.Fatal(err)
		}
	}
	if got := game.Status(); got != DrawThreefoldRepetition {
		t.Fatalf("Status() = %v, want DrawThreefoldRepetition", got)
	}
}

func TestInsufficientMaterial(t *testing.T) {
	tests := []struct {
		fen  string
		draw bool
	}{
		{"7k/8/8/8/8/8/8/K7 w - - 0 1", true},
		{"7k/8/8/8/8/8/6N1/K7 w - - 0 1", true},
		{"5b1k/8/8/8/8/8/6B1/K7 w - - 0 1", false},
		{"6bk/8/8/8/8/8/6B1/K7 w - - 0 1", true},
		{"7k/8/8/8/8/8/6B1/KN6 w - - 0 1", false},
		{"7k/8/8/8/8/8/6P1/K7 w - - 0 1", false},
	}
	for _, test := range tests {
		position, err := ParseFEN(test.fen)
		if err != nil {
			t.Fatal(err)
		}
		got := NewGameFromPosition(position).Status() == DrawInsufficientMaterial
		if got != test.draw {
			t.Errorf("Status(%q) insufficient = %t, want %t", test.fen, got, test.draw)
		}
	}
}

func TestSANLifecycle(t *testing.T) {
	game, err := FromSAN([]string{"e4", "d5", "exd5", "Nf6"})
	if err != nil {
		t.Fatal(err)
	}
	if got := game.Position().FEN(); got != "rnbqkb1r/ppp1pppp/5n2/3P4/8/8/PPPP1PPP/RNBQKBNR w KQkq - 1 3" {
		t.Fatalf("SAN sequence FEN = %q", got)
	}
	if err := game.PlaySAN("not-a-move"); err == nil {
		t.Fatal("invalid SAN accepted")
	}

	position, _ := ParseFEN("4k3/8/8/8/8/5N2/8/1N2K3 w - - 0 1")
	move, _ := ParseUCI("b1d2")
	if got, err := position.SAN(move); err != nil || got != "Nbd2" {
		t.Fatalf("SAN(b1d2) = %q, %v", got, err)
	}
	parsed, err := position.ParseSAN("Nbd2")
	if err != nil || parsed.UCI() != "b1d2" {
		t.Fatalf("ParseSAN(Nbd2) = %s, %v", parsed.UCI(), err)
	}

	position, _ = ParseFEN("4k3/8/8/8/8/R7/8/R3K3 w - - 0 1")
	move, _ = ParseUCI("a1a2")
	if got, _ := position.SAN(move); got != "R1a2" {
		t.Fatalf("SAN(a1a2) = %q", got)
	}
}

func TestSANSpecialMovesAndMate(t *testing.T) {
	tests := []struct {
		fen, uci, san string
	}{
		{"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1g1", "O-O"},
		{"7k/P7/8/8/8/8/8/7K w - - 0 1", "a7a8q", "a8=Q+"},
	}
	for _, test := range tests {
		position, _ := ParseFEN(test.fen)
		move, _ := ParseUCI(test.uci)
		if got, err := position.SAN(move); err != nil || got != test.san {
			t.Errorf("SAN(%s) = %q, %v; want %q", test.uci, got, err, test.san)
		}
		if parsed, err := position.ParseSAN(test.san); err != nil || parsed.UCI() != test.uci {
			t.Errorf("ParseSAN(%q) = %s, %v", test.san, parsed.UCI(), err)
		}
	}

	game, _ := FromSAN([]string{"f3", "e5", "g4"})
	move, _ := ParseUCI("d8h4")
	if got, err := game.Position().SAN(move); err != nil || got != "Qh4#" {
		t.Fatalf("mate SAN = %q, %v", got, err)
	}
}

func TestPGNRoundTrip(t *testing.T) {
	input := `[Event "Test"]
[Result "0-1"]

1. f3 {weak move} e5
2. g4 (2. e4) Qh4# $1 0-1`
	game, err := ParsePGN(input)
	if err != nil {
		t.Fatal(err)
	}
	if game.Status() != BlackCheckmates || game.Result() != "0-1" || len(game.Moves()) != 4 {
		t.Fatal("parsed PGN lifecycle is incorrect")
	}
	exported := game.PGN()
	reparsed, err := ParsePGN(exported)
	if err != nil {
		t.Fatal(err)
	}
	if reparsed.Position().FEN() != game.Position().FEN() || reparsed.Result() != game.Result() {
		t.Fatal("PGN round-trip changed the game")
	}
}

func TestPGNCustomPositionAndValidation(t *testing.T) {
	const input = `[SetUp "1"]
[FEN "7k/7p/8/8/8/8/P7/K7 b - - 0 1"]
[Result "*"]

1... Kg8 *`
	game, err := ParsePGN(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := game.Position().FEN(); got != "6k1/7p/8/8/8/8/P7/K7 w - - 1 2" {
		t.Fatalf("custom PGN FEN = %q", got)
	}
	if output := game.PGN(); !strings.Contains(output, `[SetUp "1"]`) || !strings.Contains(output, "1... Kg8 *") {
		t.Fatalf("custom PGN not preserved:\n%s", output)
	}
	for _, invalid := range []string{
		`[SetUp "1"]` + "\n\n*",
		`[Result "1-0"]` + "\n\n1. f3 e5 2. g4 Qh4# 0-1",
		`[Result "bad"]` + "\n\n",
		`1. e4 1-0 e5`,
	} {
		if _, err := ParsePGN(invalid); err == nil {
			t.Errorf("ParsePGN(%q) succeeded", invalid)
		}
	}
	resigned, err := ParsePGN("[Result \"1-0\"]\n\n1. e4 1-0")
	if err != nil || resigned.Result() != "1-0" {
		t.Fatalf("resignation result was not retained: %v", err)
	}
	if err := resigned.PlaySAN("e5"); err == nil {
		t.Fatal("move accepted after declared PGN result")
	}
}
