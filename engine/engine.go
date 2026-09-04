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
	// Temperature controls weighted imperfect selection among eligible moves.
	Temperature float64
	// Seed is the deterministic random seed used for imperfect selection.
	Seed uint64
	// Personality identifies the independent move-selection style.
	Personality Personality
	// InaccuracyChance is the base chance of accepting a roughly 30-80 cp loss.
	InaccuracyChance float64
	// MistakeChance is the base chance of accepting a roughly 80-200 cp loss.
	MistakeChance float64
	// BlunderChance is the base chance of accepting a 200+ cp loss.
	BlunderChance float64
	// TacticalAwareness reduces mistake risk in positions with forcing moves.
	TacticalAwareness float64
}

// New returns a bot with at least depth one and material evaluation.
func New(depth int) *Bot {
	if depth < 1 {
		depth = 1
	}
	return &Bot{Depth: depth, Evaluator: MaterialEvaluator{}, Strength: Maximum, Personality: Materialist}
}

// NewRandom returns a depth-limited bot that samples among near-best moves.
// A non-zero seed makes the sequence reproducible; use New for deterministic
// best-move selection.
func NewRandom(depth int, seed uint64) *Bot {
	bot := New(depth)
	bot.MaxLoss = 20
	bot.Temperature = 8
	bot.Seed = seed
	return bot
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
