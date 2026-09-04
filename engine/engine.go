// Package engine provides computer players and position evaluation.
package engine

import (
	"errors"
	"slices"

	"chess-go"
)

// Score is an evaluation in centipawn-like units from white's perspective.
type Score int

const (
	// MateScore is the base magnitude used for checkmate scores.
	MateScore Score = 100_000
	infinity  Score = 1_000_000
)

// ErrNoLegalMoves indicates that a bot was asked to move in a terminal position.
var ErrNoLegalMoves = errors.New("position has no legal moves")

// Evaluator assigns a score to a position from white's perspective.
type Evaluator interface {
	Evaluate(chess.Position) Score
}

// MaterialEvaluator scores conventional piece material only.
type MaterialEvaluator struct{}

// Evaluate returns the material balance from white's perspective.
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

// Bot chooses moves with fixed-depth alpha-beta search.
type Bot struct {
	// Depth is the fixed search depth in plies.
	Depth int
	// Evaluator scores leaf positions; nil selects MaterialEvaluator.
	Evaluator Evaluator
	// Strength identifies the preset used to construct the bot.
	Strength StrengthProfile
	// MaxLoss is the candidate-move tolerance in evaluation units.
	MaxLoss Score
	// Book supplies an optional deterministic opening move before search.
	Book OpeningBook
}

// New returns a bot with at least depth one and material evaluation.
func New(depth int) *Bot {
	if depth < 1 {
		depth = 1
	}
	return &Bot{Depth: depth, Evaluator: MaterialEvaluator{}, Strength: Maximum}
}

func orderedMoves(position *chess.Position) []chess.Move {
	moves := position.LegalMoves()
	slices.SortStableFunc(moves, func(a, b chess.Move) int {
		return movePriority(position, b) - movePriority(position, a)
	})
	return moves
}

func movePriority(position *chess.Position, move chess.Move) int {
	priority := int(move.Promotion) * 10
	if move.Flags&chess.Capture != 0 {
		priority += int(position.PieceAt(move.To).Type)*10 - int(position.PieceAt(move.From).Type)
	}
	return priority
}
