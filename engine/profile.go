package engine

import (
	"fmt"
	"strings"

	"chess-go"
)

// OpeningBook supplies a deterministic move for a position when one is known.
type OpeningBook interface {
	Lookup(chess.Position) (chess.Move, bool)
}

// HashBook maps incremental Zobrist keys to book moves.
type HashBook map[uint64]chess.Move

// Lookup returns the move stored for position's hash.
func (b HashBook) Lookup(position chess.Position) (chess.Move, bool) {
	move, ok := b[position.Hash()]
	return move, ok
}

// BuiltinOpeningBook returns a small deterministic classical opening line.
func BuiltinOpeningBook() HashBook {
	book := make(HashBook)
	position := chess.NewPosition()
	for _, value := range []string{"e2e4", "e7e5", "g1f3", "b8c6", "f1b5"} {
		move, err := chess.ParseUCI(value)
		if err != nil {
			continue
		}
		book[position.Hash()] = move
		position, err = position.Apply(move)
		if err != nil {
			break
		}
	}
	return book
}

// StrengthProfile names a deterministic bot strength preset.
type StrengthProfile uint8

const (
	// Learner favors speed and tolerates large evaluation losses.
	Learner StrengthProfile = iota
	// Beginner searches a little deeper than Learner.
	Beginner
	// Casual is a modest everyday playing strength.
	Casual
	// Club is a practical club-player baseline.
	Club
	// Advanced searches deeply with a narrow candidate window.
	Advanced
	// Expert is the strongest high-cost preset.
	Expert
	// Maximum uses the deepest built-in preset and only best moves.
	Maximum
)

// String returns the canonical profile name.
func (p StrengthProfile) String() string {
	if p > Maximum {
		return "Unknown"
	}
	return [...]string{"Learner", "Beginner", "Casual", "Club", "Advanced", "Expert", "Maximum"}[p]
}

// ParseStrengthProfile parses a profile name case-insensitively.
func ParseStrengthProfile(value string) (StrengthProfile, error) {
	for profile := Learner; profile <= Maximum; profile++ {
		if strings.EqualFold(strings.TrimSpace(value), profile.String()) {
			return profile, nil
		}
	}
	return 0, fmt.Errorf("unknown strength profile %q", value)
}

// StrengthConfig describes the search settings associated with a profile.
type StrengthConfig struct {
	// Depth is the fixed search depth in plies.
	Depth int
	// MaxLoss is the largest centipawn loss accepted when choosing among candidates.
	MaxLoss Score
}

// Config returns immutable search settings for p.
func (p StrengthProfile) Config() StrengthConfig {
	configs := [...]StrengthConfig{
		{Depth: 1, MaxLoss: 300},
		{Depth: 2, MaxLoss: 200},
		{Depth: 2, MaxLoss: 100},
		{Depth: 3, MaxLoss: 50},
		{Depth: 3, MaxLoss: 25},
		{Depth: 4, MaxLoss: 10},
		{Depth: 4, MaxLoss: 0},
	}
	if p > Maximum {
		return configs[Casual]
	}
	return configs[p]
}

// Profiles returns all supported profiles in increasing strength order.
func Profiles() []StrengthProfile {
	return []StrengthProfile{Learner, Beginner, Casual, Club, Advanced, Expert, Maximum}
}

// NewProfile returns a deterministic bot configured with p.
func NewProfile(p StrengthProfile) *Bot {
	config := p.Config()
	return &Bot{Depth: config.Depth, Evaluator: PositionalEvaluator{}, Strength: p, MaxLoss: config.MaxLoss, Book: BuiltinOpeningBook()}
}
