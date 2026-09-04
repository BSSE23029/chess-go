package engine

import (
	"math"

	"chess-go"
)

const (
	inaccuracyLoss Score = 60
	mistakeLoss    Score = 140
	blunderLoss    Score = 300
)

// selectCandidate applies the profile's bounded imperfection and personality
// style after search has scored the root candidates.
func (b *Bot) selectCandidate(position chess.Position, candidates []scoredMove, bestScore Score, control *searchControl) chess.Move {
	if len(candidates) == 0 {
		return chess.Move{}
	}
	allowedLoss := b.MaxLoss
	if control != nil && control.random != 0 {
		allowedLoss = maxScore(allowedLoss, b.positionLoss(position, control))
	}
	threshold := bestScore - allowedLoss
	eligible := candidates[:0]
	for _, candidate := range candidates {
		if candidate.score >= threshold {
			eligible = append(eligible, candidate)
		}
	}
	if len(eligible) == 0 {
		eligible = candidates[:1]
	}
	return chooseCandidateStyled(position, eligible, b.Temperature, b.Personality, control)
}

func (b *Bot) positionLoss(position chess.Position, control *searchControl) Score {
	inaccuracy, mistake, blunder := b.InaccuracyChance, b.MistakeChance, b.BlunderChance
	if inaccuracy <= 0 && mistake <= 0 && blunder <= 0 {
		return b.MaxLoss
	}
	if tacticalPosition(position) {
		awareness := math.Max(0, math.Min(1, b.TacticalAwareness))
		multiplier := 1.5 - awareness*0.75
		inaccuracy *= multiplier
		mistake *= multiplier
		blunder *= multiplier
	} else {
		inaccuracy *= 0.7
		mistake *= 0.7
		blunder *= 0.7
	}
	roll := randomUnit(control)
	if roll < blunder {
		return blunderLoss
	}
	if roll < blunder+mistake {
		return mistakeLoss
	}
	if roll < blunder+mistake+inaccuracy {
		return inaccuracyLoss
	}
	return b.MaxLoss
}

func tacticalPosition(position chess.Position) bool {
	if position.InCheck() {
		return true
	}
	for _, move := range position.LegalMoves() {
		if move.Flags&chess.Capture != 0 || move.Promotion != chess.NoPiece {
			return true
		}
	}
	return false
}

func maxScore(left, right Score) Score {
	if right > left {
		return right
	}
	return left
}

func chooseCandidateStyled(position chess.Position, candidates []scoredMove, temperature float64, personality Personality, control *searchControl) chess.Move {
	if len(candidates) == 1 {
		return candidates[0].move
	}
	adjusted := make([]Score, len(candidates))
	bestAdjusted := Score(-infinity)
	for index, candidate := range candidates {
		adjusted[index] = candidate.score + styleBonus(position, candidate.move, personality)
		if adjusted[index] > bestAdjusted {
			bestAdjusted = adjusted[index]
		}
	}
	if temperature <= 0 || control == nil || control.random == 0 {
		topScore := candidates[0].score
		for _, candidate := range candidates[1:] {
			if candidate.score > topScore {
				topScore = candidate.score
			}
		}
		for index, candidate := range candidates {
			if candidate.score == topScore {
				return candidates[index].move
			}
		}
		return candidates[0].move
	}
	total := 0.0
	weights := make([]float64, len(candidates))
	for index, score := range adjusted {
		weight := math.Exp(float64(score-bestAdjusted) / temperature)
		weights[index], total = weight, total+weight
	}
	target := randomUnit(control) * total
	for index, weight := range weights {
		if target < weight {
			return candidates[index].move
		}
		target -= weight
	}
	return candidates[len(candidates)-1].move
}

func styleBonus(position chess.Position, move chess.Move, personality Personality) Score {
	target := position.PieceAt(move.To)
	captureValue := pieceValue(target.Type)
	bonus := Score(0)
	if move.Flags&chess.Capture != 0 {
		bonus += captureValue / 10
	}
	if move.Promotion != chess.NoPiece {
		bonus += 80
	}
	next, err := position.Apply(move)
	givesCheck := err == nil && next.InCheck()
	if givesCheck {
		bonus += 25
	}
	central := move.To%8 >= 2 && move.To%8 <= 5 && move.To/8 >= 2 && move.To/8 <= 5
	if central {
		bonus += 10
	}
	quiet := move.Flags&chess.Capture == 0 && move.Promotion == chess.NoPiece
	switch personality {
	case Cautious:
		if quiet {
			bonus += 18
		}
	case Aggressive:
		if move.Flags&chess.Capture != 0 {
			bonus += 35
		}
		if givesCheck {
			bonus += 25
		}
	case Materialist:
		bonus += captureValue / 2
	case Tactician:
		if move.Flags&chess.Capture != 0 {
			bonus += 30
		}
		if givesCheck {
			bonus += 45
		}
	case Positional:
		if quiet {
			bonus += 12
		}
	case Simplifier:
		if move.Flags&chess.Capture != 0 {
			bonus += 40
		}
	case Trickster:
		if givesCheck {
			bonus += 35
		}
		if quiet {
			bonus += 14
		}
	}
	return bonus
}

func pieceValue(piece chess.PieceType) Score {
	return [...]Score{0, 100, 320, 330, 500, 900, 0}[piece]
}
