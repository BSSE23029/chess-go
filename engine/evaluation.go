package engine

import "chess-go"

// PositionalEvaluator combines material with lightweight positional features.
// It is deterministic and returns a score from White's perspective.
type PositionalEvaluator struct{}

// EndgameEvaluator adds king-centralization and king-pawn proximity to the
// positional evaluator for sparse positions.
type EndgameEvaluator struct{}

type pawnStructureCache struct {
	entries [1 << 8]pawnStructureEntry
	hits    uint64
}

type pawnStructureEntry struct {
	key   uint64
	score Score
	valid bool
}

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
	return evaluatePositional(position, nil)
}

func evaluatePositional(position chess.Position, pawnCache *pawnStructureCache) Score {
	material := MaterialEvaluator{}.Evaluate(position)
	var score Score = material
	var pawns [2][8]int
	var pawnRanks [2][8]uint16
	kings := [2]chess.Square{chess.NoSquare, chess.NoSquare}
	phase := maxGamePhase
	var bishops [2]int
	for square := chess.Square(0); square < 64; square++ {
		piece := position.PieceAt(square)
		if piece.IsEmpty() {
			continue
		}
		color := int(piece.Color)
		if piece.Type == chess.King {
			kings[color] = square
		}
		switch piece.Type {
		case chess.Knight, chess.Bishop:
			phase--
		case chess.Rook:
			phase -= 2
		case chess.Queen:
			phase -= 4
		}
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
			file, rank := int(square)%8, int(square)/8
			pawns[color][file]++
			pawnRanks[color][file] |= 1 << rank
		}
		if piece.Type == chess.Bishop {
			bishops[color]++
		}
	}
	structure := pawnStructure(pawns)
	if pawnCache != nil {
		key := pawnStructureKey(pawnRanks)
		entry := &pawnCache.entries[key&(uint64(len(pawnCache.entries))-1)]
		if entry.valid && entry.key == key {
			structure = entry.score
			pawnCache.hits++
		} else {
			*entry = pawnStructureEntry{key: key, score: structure, valid: true}
		}
	}
	score += structure
	if bishops[0] >= 2 {
		score += 30
	}
	if bishops[1] >= 2 {
		score -= 30
	}
	if phase < 0 {
		phase = 0
	}
	score += scaleEndgameTerm(kingSafety(position, pawnRanks, kings), maxGamePhase-phase)
	score += rookActivity(position, pawns)
	if position.InCheck() {
		if position.Turn() == chess.White {
			score -= 35
		} else {
			score += 35
		}
	}
	var legalMoves [64]chess.Move
	mobility := Score(len(position.LegalMovesInto(legalMoves[:0])) * 2)
	if position.Turn() == chess.White {
		score += mobility
	} else {
		score -= mobility
	}
	return score + passedPawns(position, pawnRanks)
}

// Evaluate returns positional evaluation with endgame-specific terms.
func (EndgameEvaluator) Evaluate(position chess.Position) Score {
	return evaluateEndgame(position, nil)
}

func evaluateEndgame(position chess.Position, pawnCache *pawnStructureCache) Score {
	score := evaluatePositional(position, pawnCache)
	weight := endgameWeight(position)
	score += scaleEndgameTerm(kingCentralization(position), weight)
	score += scaleEndgameTerm(kingPawnProximity(position), weight)
	return score
}

const maxGamePhase = 20

// endgameWeight returns a 0..maxGamePhase taper based on non-pawn material.
// Queens and rooks keep king-safety terms in the middlegame, while sparse
// positions receive the full king-activity and pawn-race signal.
func endgameWeight(position chess.Position) int {
	phase := maxGamePhase
	for square := chess.Square(0); square < 64; square++ {
		switch position.PieceAt(square).Type {
		case chess.Knight, chess.Bishop:
			phase--
		case chess.Rook:
			phase -= 2
		case chess.Queen:
			phase -= 4
		}
	}
	if phase < 0 {
		phase = 0
	}
	return phase
}

func scaleEndgameTerm(term Score, weight int) Score {
	return term * Score(weight) / maxGamePhase
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
	kings := [2]chess.Square{chess.NoSquare, chess.NoSquare}
	for square := chess.Square(0); square < 64; square++ {
		piece := position.PieceAt(square)
		if piece.Type == chess.King {
			kings[piece.Color] = square
		}
	}
	for square := chess.Square(0); square < 64; square++ {
		pawn := position.PieceAt(square)
		if pawn.Type != chess.Pawn || kings[pawn.Color] == chess.NoSquare {
			continue
		}
		bonus := Score(14 - squareDistance(kings[pawn.Color], square))
		if pawn.Color == chess.White {
			score += bonus
		} else {
			score -= bonus
		}
	}
	return score
}

// kingSafety rewards a pawn shelter in front of each king. It is intentionally
// small and phase-tapered by the caller: material and tactical search remain
// decisive, while exposed kings are less attractive in middlegame positions.
func kingSafety(position chess.Position, pawnRanks [2][8]uint16, kings [2]chess.Square) Score {
	var score Score
	for color, king := range kings {
		if king == chess.NoSquare {
			continue
		}
		file, rank := int(king)%8, int(king)/8
		direction := 1
		if color == int(chess.Black) {
			direction = -1
		}
		shieldRank := rank + direction
		if shieldRank < 0 || shieldRank > 7 {
			continue
		}
		var shelter Score
		for adjacent := maxInt(file-1, 0); adjacent <= minInt(file+1, 7); adjacent++ {
			bit := uint16(1 << shieldRank)
			switch {
			case pawnRanks[color][adjacent]&bit != 0:
				shelter += 10
			case pawnRanks[1-color][adjacent]&bit != 0:
				shelter -= 8
			default:
				shelter -= 5
			}
		}
		if pawnRanks[color][file]&(1<<shieldRank) == 0 {
			shelter -= 4
		}
		pressure := kingZonePressure(position, king, chess.Color(1-color))
		shelter -= Score(pressure * 4)
		if color == int(chess.White) {
			score += shelter
		} else {
			score -= shelter
		}
	}
	return score
}

func kingZonePressure(position chess.Position, king chess.Square, attacker chess.Color) int {
	file, rank := int(king)%8, int(king)/8
	pressure := 0
	for deltaRank := -1; deltaRank <= 1; deltaRank++ {
		for deltaFile := -1; deltaFile <= 1; deltaFile++ {
			if deltaFile == 0 && deltaRank == 0 {
				continue
			}
			targetFile, targetRank := file+deltaFile, rank+deltaRank
			if targetFile < 0 || targetFile > 7 || targetRank < 0 || targetRank > 7 {
				continue
			}
			if squareAttackedBy(position, chess.Square(targetRank*8+targetFile), attacker) {
				pressure++
			}
		}
	}
	return pressure
}

func squareAttackedBy(position chess.Position, target chess.Square, attacker chess.Color) bool {
	for from := chess.Square(0); from < 64; from++ {
		piece := position.PieceAt(from)
		if !piece.IsEmpty() && piece.Color == attacker && pieceAttacksSquare(position, from, target, piece.Type) {
			return true
		}
	}
	return false
}

func pieceAttacksSquare(position chess.Position, from, target chess.Square, pieceType chess.PieceType) bool {
	if from == target {
		return false
	}
	fromFile, fromRank := int(from)%8, int(from)/8
	targetFile, targetRank := int(target)%8, int(target)/8
	fileDelta, rankDelta := targetFile-fromFile, targetRank-fromRank
	fileDistance, rankDistance := abs(fileDelta), abs(rankDelta)
	switch pieceType {
	case chess.Pawn:
		piece := position.PieceAt(from)
		direction := 1
		if piece.Color == chess.Black {
			direction = -1
		}
		return rankDelta == direction && fileDistance == 1
	case chess.Knight:
		return (fileDistance == 1 && rankDistance == 2) || (fileDistance == 2 && rankDistance == 1)
	case chess.King:
		return fileDistance <= 1 && rankDistance <= 1
	case chess.Bishop:
		return fileDistance == rankDistance && rayClear(position, fromFile, fromRank, targetFile, targetRank)
	case chess.Rook:
		return (fileDelta == 0 || rankDelta == 0) && rayClear(position, fromFile, fromRank, targetFile, targetRank)
	case chess.Queen:
		straight := fileDelta == 0 || rankDelta == 0
		diagonal := fileDistance == rankDistance
		return (straight || diagonal) && rayClear(position, fromFile, fromRank, targetFile, targetRank)
	default:
		return false
	}
}

func rayClear(position chess.Position, fromFile, fromRank, targetFile, targetRank int) bool {
	fileStep, rankStep := sign(targetFile-fromFile), sign(targetRank-fromRank)
	for file, rank := fromFile+fileStep, fromRank+rankStep; file != targetFile || rank != targetRank; file, rank = file+fileStep, rank+rankStep {
		if !position.PieceAt(chess.Square(rank*8 + file)).IsEmpty() {
			return false
		}
	}
	return true
}

func sign(value int) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
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

func pawnStructureKey(pawnRanks [2][8]uint16) uint64 {
	key := uint64(0x9e3779b97f4a7c15)
	for color := range pawnRanks {
		for file, ranks := range pawnRanks[color] {
			key ^= uint64(ranks) + uint64(color*8+file+1)*0x9e3779b97f4a7c15
			key = (key ^ key>>30) * 0xbf58476d1ce4e5b9
			key = (key ^ key>>27) * 0x94d049bb133111eb
		}
	}
	return key ^ key>>31
}

func passedPawns(position chess.Position, pawnRanks [2][8]uint16) Score {
	var score Score
	for square := chess.Square(0); square < 64; square++ {
		pawn := position.PieceAt(square)
		if pawn.Type != chess.Pawn {
			continue
		}
		file, rank := int(square)%8, int(square)/8
		enemy := pawn.Color.Opponent()
		var enemyAhead uint16
		if pawn.Color == chess.White {
			enemyAhead = ^uint16((1 << (rank + 1)) - 1)
		} else {
			enemyAhead = (1 << rank) - 1
		}
		passed := true
		for adjacent := maxInt(file-1, 0); adjacent <= minInt(file+1, 7); adjacent++ {
			if pawnRanks[enemy][adjacent]&enemyAhead != 0 {
				passed = false
				break
			}
		}
		if passed {
			advance := rank
			if pawn.Color == chess.Black {
				advance = 7 - rank
			}
			bonus := Score(20 + 5*advance)
			if pawn.Color == chess.White {
				score += bonus
			} else {
				score -= bonus
			}
		}
	}
	return score
}

func minInt(first, second int) int {
	if first < second {
		return first
	}
	return second
}

func maxInt(first, second int) int {
	if first > second {
		return first
	}
	return second
}

// rookActivity rewards files that let rooks work and rooks that reach the
// opponent's second rank. The bonus is deliberately modest: material and
// tactical search remain more important than a purely geometric preference.
func rookActivity(position chess.Position, pawns [2][8]int) Score {
	var score Score
	for square := chess.Square(0); square < 64; square++ {
		rook := position.PieceAt(square)
		if rook.Type != chess.Rook {
			continue
		}
		file, rank := int(square)%8, int(square)/8
		color := int(rook.Color)
		bonus := Score(0)
		if pawns[0][file]+pawns[1][file] == 0 {
			bonus += 18
		} else if pawns[color][file] == 0 {
			bonus += 10
		}
		if (rook.Color == chess.White && rank == 6) || (rook.Color == chess.Black && rank == 1) {
			bonus += 20
		}
		if rook.Color == chess.White {
			score += bonus
		} else {
			score -= bonus
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
