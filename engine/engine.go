package engine

import (
	"context"
	"errors"
	"slices"

	chess "github.com/BSSE23029/chess-go"
)

type Score int

const (
	MateScore Score = 100_000
	infinity  Score = 1_000_000
)

var ErrNoLegalMoves = errors.New("position has no legal moves")

type Evaluator interface {
	Evaluate(chess.Position) Score
}

type MaterialEvaluator struct{}

func (MaterialEvaluator) Evaluate(position chess.Position) Score {
	values := [...]Score{0, 100, 320, 330, 500, 900, 0}
	var score Score
	for square := chess.Square(0); square < 64; square++ {
		piece := position.PieceAt(square)
		value := values[piece.Type]
		if piece.Color == chess.White {
			score += value
		} else {
			score -= value
		}
	}
	return score
}

type Bot struct {
	Depth     int
	Evaluator Evaluator
}

func New(depth int) *Bot {
	if depth < 1 {
		depth = 1
	}
	return &Bot{Depth: depth, Evaluator: MaterialEvaluator{}}
}

func (b *Bot) ChooseMove(ctx context.Context, position chess.Position) (chess.Move, error) {
	if err := ctx.Err(); err != nil {
		return chess.Move{}, err
	}
	moves := orderedMoves(position)
	if len(moves) == 0 {
		return chess.Move{}, ErrNoLegalMoves
	}
	depth := b.Depth
	if depth < 1 {
		depth = 1
	}
	evaluator := b.Evaluator
	if evaluator == nil {
		evaluator = MaterialEvaluator{}
	}
	best, bestScore := moves[0], -infinity
	for _, move := range moves {
		next, _ := position.Apply(move)
		score, err := b.search(ctx, evaluator, next, depth-1, 1, -infinity, infinity)
		if err != nil {
			return chess.Move{}, err
		}
		score = -score
		if score > bestScore {
			best, bestScore = move, score
		}
	}
	return best, nil
}

func (b *Bot) search(ctx context.Context, evaluator Evaluator, position chess.Position, depth, ply int, alpha, beta Score) (Score, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	moves := orderedMoves(position)
	if len(moves) == 0 {
		if position.InCheck() {
			return -MateScore + Score(ply), nil
		}
		return 0, nil
	}
	if depth == 0 {
		score := evaluator.Evaluate(position)
		if position.Turn() == chess.Black {
			score = -score
		}
		return score, nil
	}
	best := -infinity
	for _, move := range moves {
		next, _ := position.Apply(move)
		score, err := b.search(ctx, evaluator, next, depth-1, ply+1, -beta, -alpha)
		if err != nil {
			return 0, err
		}
		score = -score
		if score > best {
			best = score
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			break
		}
	}
	return best, nil
}

func orderedMoves(position chess.Position) []chess.Move {
	moves := position.LegalMoves()
	slices.SortStableFunc(moves, func(a, b chess.Move) int {
		return movePriority(position, b) - movePriority(position, a)
	})
	return moves
}

func movePriority(position chess.Position, move chess.Move) int {
	priority := int(move.Promotion) * 10
	if move.Flags&chess.Capture != 0 {
		priority += int(position.PieceAt(move.To).Type)*10 - int(position.PieceAt(move.From).Type)
	}
	return priority
}
