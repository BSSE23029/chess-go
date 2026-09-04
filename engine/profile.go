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

// Personality names a move-selection style independent of playing strength.
type Personality uint8

const (
	// Cautious prefers lower-variance candidate choices.
	Cautious Personality = iota
	// Aggressive favors forcing and capture candidates.
	Aggressive
	// Materialist favors material-winning candidates.
	Materialist
	// Tactician favors sharp tactical candidates.
	Tactician
	// Positional favors quiet positional candidates.
	Positional
	// Simplifier favors exchanges when candidates are close.
	Simplifier
	// Trickster accepts volatile candidates.
	Trickster
)

// String returns the canonical personality name.
func (p Personality) String() string {
	if p > Trickster {
		return "Unknown"
	}
	return [...]string{"Cautious", "Aggressive", "Materialist", "Tactician", "Positional", "Simplifier", "Trickster"}[p]
}

// ParsePersonality parses a personality name case-insensitively.
func ParsePersonality(value string) (Personality, error) {
	for personality := Cautious; personality <= Trickster; personality++ {
		if strings.EqualFold(strings.TrimSpace(value), personality.String()) {
			return personality, nil
		}
	}
	return 0, fmt.Errorf("unknown personality %q", value)
}

// PersonalityConfig controls deterministic imperfect move selection.
type PersonalityConfig struct {
	// Temperature controls weighted selection among eligible candidates.
	Temperature float64
}

// Config returns move-selection settings for p.
func (p Personality) Config() PersonalityConfig {
	configs := [...]PersonalityConfig{{0}, {20}, {10}, {30}, {0}, {5}, {45}}
	if p > Trickster {
		return configs[Positional]
	}
	return configs[p]
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
	// Temperature controls deterministic imperfect selection within MaxLoss.
	Temperature float64
	// InaccuracyChance is the base chance of accepting a small evaluation loss.
	InaccuracyChance float64
	// MistakeChance is the base chance of accepting a medium evaluation loss.
	MistakeChance float64
	// BlunderChance is the base chance of accepting a severe evaluation loss.
	BlunderChance float64
	// TacticalAwareness reduces error risk when forcing moves are available.
	TacticalAwareness float64
}

// Config returns immutable search settings for p.
func (p StrengthProfile) Config() StrengthConfig {
	configs := [...]StrengthConfig{
		{Depth: 1, MaxLoss: 300, Temperature: 80, InaccuracyChance: 0.35, MistakeChance: 0.12, BlunderChance: 0.03, TacticalAwareness: 0.35},
		{Depth: 2, MaxLoss: 200, Temperature: 40, InaccuracyChance: 0.25, MistakeChance: 0.08, BlunderChance: 0.015, TacticalAwareness: 0.50},
		{Depth: 2, MaxLoss: 100, Temperature: 20, InaccuracyChance: 0.15, MistakeChance: 0.04, BlunderChance: 0.008, TacticalAwareness: 0.65},
		{Depth: 3, MaxLoss: 50, Temperature: 10, InaccuracyChance: 0.08, MistakeChance: 0.02, BlunderChance: 0.003, TacticalAwareness: 0.75},
		{Depth: 3, MaxLoss: 25, Temperature: 5, InaccuracyChance: 0.04, MistakeChance: 0.01, BlunderChance: 0.001, TacticalAwareness: 0.85},
		{Depth: 4, MaxLoss: 10, Temperature: 2, InaccuracyChance: 0.01, MistakeChance: 0.002, BlunderChance: 0, TacticalAwareness: 0.95},
		{Depth: 4, MaxLoss: 0, Temperature: 0, InaccuracyChance: 0, MistakeChance: 0, BlunderChance: 0, TacticalAwareness: 1},
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
	evaluator := Evaluator(PositionalEvaluator{})
	if p >= Advanced {
		evaluator = EndgameEvaluator{}
	}
	return &Bot{Depth: config.Depth, Evaluator: evaluator, Strength: p, MaxLoss: config.MaxLoss, Temperature: config.Temperature, Seed: 0x9e3779b97f4a7c15 + uint64(p), Personality: Positional, InaccuracyChance: config.InaccuracyChance, MistakeChance: config.MistakeChance, BlunderChance: config.BlunderChance, TacticalAwareness: config.TacticalAwareness, Book: BuiltinOpeningBook()}
}

// SetPersonality applies a deterministic move-selection style to b.
func (b *Bot) SetPersonality(personality Personality, seed uint64) {
	b.Personality = personality
	b.Temperature = personality.Config().Temperature
	if seed == 0 {
		seed = 0x243f6a8885a308d3
	}
	b.Seed = seed
}
