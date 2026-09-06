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
