package engine

import (
	"context"

	"chess-go"
)

type scoredMove struct {
	move  chess.Move
	score Score
}

func (b *Bot) iteration(ctx context.Context, evaluator Evaluator, position *chess.Position, moves []chess.Move, depth int, alpha, beta Score, control *searchControl) (chess.Move, Score, bool, bool, error) {
	moves = orderedSearchMoves(position, 0, control)
	if control.pvMove != (chess.Move{}) {
		moves = prioritizeMove(moves, control.pvMove)
	}
	originalAlpha := alpha
	best, bestScore := moves[0], -infinity
	var candidates []scoredMove
	if b.candidateSelectionVaries() {
		candidates = make([]scoredMove, 0, len(moves))
	}
	for index, move := range moves {
		undo := position.MakeLegalMove(move)
		searchAlpha, searchBeta := -beta, -alpha
		if index > 0 && b.principalVariationSearch() {
			// Most moves are expected to fail below alpha. A null-window probe
			// avoids a full subtree search; only a move that raises alpha is
			// re-searched with the exact window needed for ranking.
			searchAlpha, searchBeta = -alpha-1, -alpha
		}
		score, err := b.search(ctx, evaluator, position, depth-1, 1, searchAlpha, searchBeta, control)
		position.UnmakeMove(undo)
		if err != nil {
			return best, bestScore, false, false, err
		}
		score = -score
		if index > 0 && b.principalVariationSearch() && score > alpha && score < beta {
			undo = position.MakeLegalMove(move)
			score, err = b.search(ctx, evaluator, position, depth-1, 1, -beta, -alpha, control)
			position.UnmakeMove(undo)
			if err != nil {
				return best, bestScore, false, false, err
			}
			score = -score
		}
		if candidates != nil {
			candidates = append(candidates, scoredMove{move: move, score: score})
		}
		if score > bestScore {
			best, bestScore = move, score
		}
		if score > alpha {
			alpha = score
		}
		if score >= beta {
			break
		}
	}
	if candidates == nil {
		return best, bestScore, bestScore <= originalAlpha, bestScore >= beta, nil
	}
	return b.selectCandidate(*position, candidates, bestScore, control), bestScore, bestScore <= originalAlpha, bestScore >= beta, nil
}

func (b *Bot) principalVariationSearch() bool {
	// Imperfect bots need exact root scores to sample a bounded near-best set.
	// Deterministic best-move searches can safely use null-window probes.
	return b.MaxLoss == 0 && b.Temperature <= 0 && b.Seed == 0
}
