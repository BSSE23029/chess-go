package engine

import (
	"testing"

	"chess-go"
)

func tacticalMove(t *testing.T, position chess.Position, uci string) chess.Move {
	t.Helper()
	for _, move := range position.LegalMoves() {
		if move.UCI() == uci {
			return move
		}
	}
	t.Fatalf("move %s is not legal in %s", uci, position.FEN())
	return chess.Move{}
}

func TestForkPatternChecksKingAndAttacksQueen(t *testing.T) {
	position := mustPosition(t, "2q1k3/8/8/8/4N3/8/8/4K3 w - - 0 1")
	next, err := position.Apply(tacticalMove(t, position, "e4d6"))
	if err != nil || !next.InCheck() {
		t.Fatalf("fork did not check the king: %v", err)
	}
	queen, err := chess.ParseSquare("c8")
	if err != nil || !pieceAttacksSquare(next, chess.Square(43), queen, chess.Knight) {
		t.Fatalf("fork knight does not attack c8: %v", err)
	}
}

func TestPinnedKnightHasNoLegalExposingMoves(t *testing.T) {
	position := mustPosition(t, "4k3/4n3/8/8/8/8/8/4R1K1 b - - 0 1")
	for _, move := range position.LegalMoves() {
		if move.From == chess.Square(52) {
			t.Fatalf("pinned knight move remained legal: %s", move.UCI())
		}
	}
}

func TestSkewerUncoversQueenAfterKingMoves(t *testing.T) {
	position := mustPosition(t, "4q3/4k3/8/8/8/8/8/4R1K1 b - - 0 1")
	queen, err := chess.ParseSquare("e8")
	if err != nil {
		t.Fatal(err)
	}
	for _, move := range position.LegalMoves() {
		if move.From != chess.Square(52) {
			continue
		}
		next, applyErr := position.Apply(move)
		if applyErr == nil && pieceAttacksSquare(next, chess.Square(4), queen, chess.Rook) {
			return
		}
	}
	t.Fatal("no legal king move uncovered the skewered queen")
}

func TestProtectedQueenSacrificeGivesCheck(t *testing.T) {
	position := mustPosition(t, "6k1/5ppp/8/6NQ/8/8/6PP/6KR w - - 0 1")
	next, err := position.Apply(tacticalMove(t, position, "h5h7"))
	if err != nil || !next.InCheck() {
		t.Fatalf("sacrifice candidate did not give check: %v", err)
	}
	target, err := chess.ParseSquare("h7")
	if err != nil || !pieceAttacksSquare(next, chess.Square(38), target, chess.Knight) {
		t.Fatalf("sacrifice candidate was not protected by the knight: %v", err)
	}
}
