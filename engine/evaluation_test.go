package engine

import (
	"testing"

	"chess-go"
)

func mustPosition(t *testing.T, fen string) chess.Position {
	t.Helper()
	position, err := chess.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN(%q): %v", fen, err)
	}
	return position
}

func pawnEvaluationState(position chess.Position) ([2][8]uint16, [2]chess.Square) {
	var pawns [2][8]uint16
	kings := [2]chess.Square{chess.NoSquare, chess.NoSquare}
	for square := chess.Square(0); square < 64; square++ {
		piece := position.PieceAt(square)
		if piece.Type == chess.Pawn {
			pawns[piece.Color][int(square)%8] |= 1 << (int(square) / 8)
		}
		if piece.Type == chess.King {
			kings[piece.Color] = square
		}
	}
	return pawns, kings
}

func TestPassedPawnBonusScalesWithTheCorrectAdvanceDirection(t *testing.T) {
	evaluator := PositionalEvaluator{}
	whiteAdvanced := mustPosition(t, "4k3/8/8/4P3/8/8/8/4K3 w - - 0 1")
	whiteBack := mustPosition(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	if evaluator.Evaluate(whiteAdvanced) <= evaluator.Evaluate(whiteBack) {
		t.Fatalf("advanced white passer was not preferred: advanced %d back %d", evaluator.Evaluate(whiteAdvanced), evaluator.Evaluate(whiteBack))
	}
	blackAdvanced := mustPosition(t, "4k3/8/8/8/4p3/8/8/4K3 w - - 0 1")
	blackBack := mustPosition(t, "4k3/4p3/8/8/8/8/8/4K3 w - - 0 1")
	if evaluator.Evaluate(blackAdvanced) >= evaluator.Evaluate(blackBack) {
		t.Fatalf("advanced black passer was not preferred for Black: advanced %d back %d", evaluator.Evaluate(blackAdvanced), evaluator.Evaluate(blackBack))
	}
}

func TestRookActivityRewardsOpenFilesAndSeventhRank(t *testing.T) {
	evaluator := PositionalEvaluator{}
	open := mustPosition(t, "4k3/7p/8/8/8/8/1P5P/R3K3 w - - 0 1")
	blocked := mustPosition(t, "4k3/7p/8/8/8/8/P6P/R3K3 w - - 0 1")
	if evaluator.Evaluate(open) <= evaluator.Evaluate(blocked) {
		t.Fatalf("open-file rook was not preferred: open %d blocked %d", evaluator.Evaluate(open), evaluator.Evaluate(blocked))
	}
	seventh := mustPosition(t, "4k3/R7/8/8/8/8/7P/4K3 w - - 0 1")
	first := mustPosition(t, "4k3/8/8/8/8/8/7P/R3K3 w - - 0 1")
	if evaluator.Evaluate(seventh) <= evaluator.Evaluate(first) {
		t.Fatalf("seventh-rank rook was not preferred: seventh %d first %d", evaluator.Evaluate(seventh), evaluator.Evaluate(first))
	}
}

func TestKingSafetyRewardsPawnShelterInTheMiddlegame(t *testing.T) {
	sheltered := mustPosition(t, "4k3/8/8/8/8/8/5PPP/5K2 w - - 0 1")
	exposed := mustPosition(t, "4k3/8/8/8/8/8/PPP5/5K2 w - - 0 1")
	shelteredPawns, shelteredKings := pawnEvaluationState(sheltered)
	exposedPawns, exposedKings := pawnEvaluationState(exposed)
	if kingSafety(sheltered, shelteredPawns, shelteredKings) <= kingSafety(exposed, exposedPawns, exposedKings) {
		t.Fatalf("pawn shelter was not rewarded: sheltered %d exposed %d", kingSafety(sheltered, shelteredPawns, shelteredKings), kingSafety(exposed, exposedPawns, exposedKings))
	}
}

func TestKingSafetyCountsAttacksIntoTheKingZone(t *testing.T) {
	pressured := mustPosition(t, "6k1/6r1/8/8/8/8/5PPP/6K1 w - - 0 1")
	quiet := mustPosition(t, "6k1/r7/8/8/8/8/5PPP/6K1 w - - 0 1")
	king, err := chess.ParseSquare("g1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := kingZonePressure(pressured, king, chess.Black), kingZonePressure(quiet, king, chess.Black); got <= want {
		t.Fatalf("king-zone pressure = %d, quiet = %d", got, want)
	}
}

func TestKingSafetyRewardsAvailableCastlingRights(t *testing.T) {
	withRights := mustPosition(t, "r3k2r/8/8/8/8/8/8/R3K2R w KQ - 0 1")
	withoutRights := mustPosition(t, "r3k2r/8/8/8/8/8/8/R3K2R w - - 0 1")
	withPawns, withKings := pawnEvaluationState(withRights)
	withoutPawns, withoutKings := pawnEvaluationState(withoutRights)
	if got, want := kingSafety(withRights, withPawns, withKings), kingSafety(withoutRights, withoutPawns, withoutKings); got <= want {
		t.Fatalf("castling rights were not rewarded: with %d without %d", got, want)
	}
}

func TestSearchPawnCacheReusesPawnLayoutAcrossPositions(t *testing.T) {
	first := mustPosition(t, "4k3/8/8/8/8/8/4P3/R3K3 w - - 0 1")
	second := mustPosition(t, "4k3/8/8/8/8/8/4P3/4KR2 w - - 0 1")
	control := &searchControl{}
	control.evaluate(PositionalEvaluator{}, first)
	control.evaluate(PositionalEvaluator{}, second)
	if control.pawnCache.hits != 1 {
		t.Fatalf("pawn cache hits = %d, want 1", control.pawnCache.hits)
	}
}

func TestEndgameEvaluatorPreservesPositionalSignals(t *testing.T) {
	endgame := EndgameEvaluator{}
	openKing := mustPosition(t, "4k3/8/8/3P4/3K4/8/8/8 w - - 0 1")
	cornerKing := mustPosition(t, "4k3/8/8/3P4/8/8/8/7K w - - 0 1")
	if endgame.Evaluate(openKing) <= endgame.Evaluate(cornerKing) {
		t.Fatalf("active king was not rewarded in the endgame: active %d corner %d", endgame.Evaluate(openKing), endgame.Evaluate(cornerKing))
	}
	if _, ok := NewProfile(Advanced).Evaluator.(EndgameEvaluator); !ok {
		t.Fatal("advanced profile did not use the endgame evaluator")
	}
}

func TestEndgameTermsAreTaperedByRemainingMaterial(t *testing.T) {
	evaluator := EndgameEvaluator{}
	sparseActive := mustPosition(t, "4k3/8/8/3P4/3K4/8/8/8 w - - 0 1")
	sparseCorner := mustPosition(t, "4k3/8/8/3P4/8/8/8/7K w - - 0 1")
	richActive := mustPosition(t, "3qk3/8/8/3P4/3K4/8/8/3Q4 w - - 0 1")
	richCorner := mustPosition(t, "3qk3/8/8/3P4/8/8/8/3Q3K w - - 0 1")
	sparseDelta := evaluator.Evaluate(sparseActive) - evaluator.Evaluate(sparseCorner)
	richDelta := evaluator.Evaluate(richActive) - evaluator.Evaluate(richCorner)
	if sparseDelta <= richDelta {
		t.Fatalf("endgame taper did not increase sparse king activity: sparse %d rich %d", sparseDelta, richDelta)
	}
}

func TestEndgameRewardsTheSideWithKingOpposition(t *testing.T) {
	whiteToMove := mustPosition(t, "8/8/4k3/8/4K3/8/8/8 w - - 0 1")
	blackToMove := mustPosition(t, "8/8/4k3/8/4K3/8/8/8 b - - 0 1")
	if got, want := kingOpposition(whiteToMove), kingOpposition(blackToMove); got >= want {
		t.Fatalf("opposition did not favor the side not to move: white-to-move %d black-to-move %d", got, want)
	}
}
