package engine

import "chess-go"

// PositionalEvaluator combines material with lightweight positional features.
// It is deterministic and returns a score from White's perspective.
type PositionalEvaluator struct{}

// EndgameEvaluator adds king-centralization and king-pawn proximity to the
// positional evaluator for sparse positions.
type EndgameEvaluator struct{}

var pieceSquare = map[chess.PieceType][64]Score{
	chess.Pawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		5, 10, 10, -20, -20, 10, 10, 5,
		5, -5, -10, 0, 0, -10, -5, 5,
		0, 0, 0, 20, 20, 0, 0, 0,
		5, 5, 10, 25, 25, 10, 5, 5,
		10, 10, 20, 30, 30, 20, 10, 10,
		50, 50, 50, 50, 50, 50, 50, 50,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	chess.Knight: {
		-50, -40, -30, -30, -30, -30, -40, -50,
		-40, -20, 0, 5, 5, 0, -20, -40,
		-30, 5, 10, 15, 15, 10, 5, -30,
		-30, 0, 15, 20, 20, 15, 0, -30,
		-30, 5, 15, 20, 20, 15, 5, -30,
		-30, 0, 10, 15, 15, 10, 0, -30,
		-40, -20, 0, 0, 0, 0, -20, -40,
		-50, -40, -30, -30, -30, -30, -40, -50,
	},
	chess.Bishop: {
		-20, -10, -10, -10, -10, -10, -10, -20,
		-10, 5, 0, 0, 0, 0, 5, -10,
		-10, 10, 10, 10, 10, 10, 10, -10,
		-10, 0, 10, 10, 10, 10, 0, -10,
		-10, 5, 5, 10, 10, 5, 5, -10,
		-10, 0, 5, 10, 10, 5, 0, -10,
		-10, 0, 0, 0, 0, 0, 0, -10,
		-20, -10, -10, -10, -10, -10, -10, -20,
	},
	chess.Rook: {
		0, 0, 0, 5, 5, 0, 0, 0,
		-5, 0, 0, 0, 0, 0, 0, -5,
		-5, 0, 0, 0, 0, 0, 0, -5,
		-5, 0, 0, 0, 0, 0, 0, -5,
		-5, 0, 0, 0, 0, 0, 0, -5,
		-5, 0, 0, 0, 0, 0, 0, -5,
		5, 10, 10, 10, 10, 10, 10, 5,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	chess.Queen: {
		-20, -10, -10, 0, 0, -10, -10, -20,
		-10, 0, 5, 0, 0, 0, 0, -10,
		-10, 5, 5, 5, 5, 5, 0, -10,
		0, 0, 5, 5, 5, 5, 0, -5,
		-5, 0, 5, 5, 5, 5, 0, -5,
		-10, 0, 5, 5, 5, 5, 0, -10,
		-10, 0, 0, 0, 0, 0, 0, -10,
		-20, -10, -10, 0, 0, -10, -10, -20,
	},
	chess.King: {
		20, 30, 10, 0, 0, 10, 30, 20,
		20, 20, 0, 0, 0, 0, 20, 20,
		-10, -20, -20, -20, -20, -20, -20, -10,
		-20, -30, -30, -40, -40, -30, -30, -20,
		-30, -40, -40, -50, -50, -40, -40, -30,
		-30, -40, -40, -50, -50, -40, -40, -30,
		-30, -40, -40, -50, -50, -40, -40, -30,
		-30, -40, -40, -50, -50, -40, -40, -30,
	},
}

// Evaluate returns material, piece-square, mobility, pawn-structure,
// bishop-pair, passed-pawn, and king-safety terms from White's perspective.
func (PositionalEvaluator) Evaluate(position chess.Position) Score {
	material := MaterialEvaluator{}.Evaluate(position)
	var score Score = material
	var pawns [2][8]int
	var bishops [2]int
	for square := chess.Square(0); square < 64; square++ {
		piece := position.PieceAt(square)
		if piece.IsEmpty() {
			continue
		}
		color := int(piece.Color)
		index := int(square)
		if piece.Color == chess.Black {
			index = 63 - index
		}
		if values, ok := pieceSquare[piece.Type]; ok {
			if piece.Color == chess.White {
				score += values[index]
			} else {
				score -= values[index]
			}
		}
		if piece.Type == chess.Pawn {
			pawns[color][int(square)%8]++
		}
		if piece.Type == chess.Bishop {
			bishops[color]++
		}
	}
	score += pawnStructure(pawns)
	if bishops[0] >= 2 {
		score += 30
	}
	if bishops[1] >= 2 {
		score -= 30
	}
	if position.InCheck() {
		if position.Turn() == chess.White {
			score -= 35
		} else {
			score += 35
		}
	}
	mobility := Score(len(position.LegalMoves()) * 2)
	if position.Turn() == chess.White {
		score += mobility
	} else {
		score -= mobility
	}
	return score + passedPawns(position)
}

// Evaluate returns positional evaluation with endgame-specific terms.
func (EndgameEvaluator) Evaluate(position chess.Position) Score {
	score := PositionalEvaluator{}.Evaluate(position)
	queens, rooks, pawns := 0, 0, 0
	for square := chess.Square(0); square < 64; square++ {
		switch position.PieceAt(square).Type {
		case chess.Queen:
			queens++
		case chess.Rook:
			rooks++
		case chess.Pawn:
			pawns++
		}
	}
	if queens == 0 && rooks == 0 {
		score += kingCentralization(position)
	}
	if pawns > 0 && queens == 0 && rooks == 0 {
		score += kingPawnProximity(position)
	}
	return score
}

func kingCentralization(position chess.Position) Score {
	white, black := chess.NoSquare, chess.NoSquare
	for square := chess.Square(0); square < 64; square++ {
		piece := position.PieceAt(square)
		if piece.Type != chess.King {
			continue
		}
		if piece.Color == chess.White {
			white = square
		} else {
			black = square
		}
	}
	if white == chess.NoSquare || black == chess.NoSquare {
		return 0
	}
	whiteDistance := centerDistance(white)
	blackDistance := centerDistance(black)
	return Score((blackDistance - whiteDistance) * 12)
}

func kingPawnProximity(position chess.Position) Score {
	var score Score
	for square := chess.Square(0); square < 64; square++ {
		pawn := position.PieceAt(square)
		if pawn.Type != chess.Pawn {
			continue
		}
		king := chess.NoSquare
		for kingSquare := chess.Square(0); kingSquare < 64; kingSquare++ {
			piece := position.PieceAt(kingSquare)
			if piece.Type == chess.King && piece.Color == pawn.Color {
				king = kingSquare
				break
			}
		}
		if king == chess.NoSquare {
			continue
		}
		bonus := Score(14 - squareDistance(king, square))
		if pawn.Color == chess.White {
			score += bonus
		} else {
			score -= bonus
		}
	}
	return score
}

func centerDistance(square chess.Square) int {
	file, rank := int(square)%8, int(square)/8
	return abs(file*2-7) + abs(rank*2-7)
}

func squareDistance(first, second chess.Square) int {
	file := abs(int(first)%8 - int(second)%8)
	rank := abs(int(first)/8 - int(second)/8)
	if file > rank {
		return file
	}
	return rank
}

func pawnStructure(pawns [2][8]int) Score {
	var score Score
	for color := 0; color < 2; color++ {
		for file, count := range pawns[color] {
			if count > 1 {
				penalty := Score((count - 1) * 15)
				if color == 0 {
					score -= penalty
				} else {
					score += penalty
				}
			}
			if count == 0 {
				continue
			}
			left, right := file == 0 || pawns[color][file-1] == 0, file == 7 || pawns[color][file+1] == 0
			if left && right {
				if color == 0 {
					score -= 12
				} else {
					score += 12
				}
			}
		}
	}
	return score
}

func passedPawns(position chess.Position) Score {
	var score Score
	for square := chess.Square(0); square < 64; square++ {
		pawn := position.PieceAt(square)
		if pawn.Type != chess.Pawn {
			continue
		}
		file, rank := int(square)%8, int(square)/8
		passed := true
		for other := chess.Square(0); other < 64; other++ {
			enemy := position.PieceAt(other)
			if enemy.Type != chess.Pawn || enemy.Color == pawn.Color || abs(int(other)%8-file) > 1 {
				continue
			}
			otherRank := int(other) / 8
			if (pawn.Color == chess.White && otherRank > rank) || (pawn.Color == chess.Black && otherRank < rank) {
				passed = false
				break
			}
		}
		if passed {
			bonus := Score(20 + 5*rank)
			if pawn.Color == chess.White {
				score += bonus
			} else {
				score -= bonus
			}
		}
	}
	return score
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
