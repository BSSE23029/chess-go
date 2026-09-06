package engine

import "chess-go"

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
