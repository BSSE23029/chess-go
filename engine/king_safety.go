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
			pressure += squareAttackWeight(position, chess.Square(targetRank*8+targetFile), attacker)
		}
	}
	return pressure
}

func squareAttackedBy(position chess.Position, target chess.Square, attacker chess.Color) bool {
	return squareAttackWeight(position, target, attacker) > 0
}

func squareAttackWeight(position chess.Position, target chess.Square, attacker chess.Color) int {
	weight := 0
	for from := chess.Square(0); from < 64; from++ {
		piece := position.PieceAt(from)
		if !piece.IsEmpty() && piece.Color == attacker && pieceAttacksSquare(position, from, target, piece.Type) {
			if value := attackWeight(piece.Type); value > weight {
				weight = value
			}
		}
	}
	return weight
}

func attackWeight(piece chess.PieceType) int {
	switch piece {
	case chess.Queen:
		return 4
	case chess.Rook:
		return 3
	case chess.Bishop, chess.Knight, chess.King:
		return 2
	case chess.Pawn:
		return 1
	default:
		return 0
	}
}

func castlingPotential(position chess.Position, color chess.Color, king chess.Square) Score {
	home := chess.Square(4)
	rights := chess.WhiteKingSide | chess.WhiteQueenSide
	if color == chess.Black {
		home = chess.Square(60)
		rights = chess.BlackKingSide | chess.BlackQueenSide
	}
	if king != home {
		return 0
	}
	if position.Castling()&rights != 0 {
		return 8
	}
	return -4
}

func kingOpposition(position chess.Position) Score {
	kings := [2]chess.Square{chess.NoSquare, chess.NoSquare}
	for square := chess.Square(0); square < 64; square++ {
		piece := position.PieceAt(square)
		if piece.Type == chess.King {
			kings[piece.Color] = square
		}
	}
	if kings[0] == chess.NoSquare || kings[1] == chess.NoSquare {
		return 0
	}
	whiteFile, whiteRank := int(kings[0])%8, int(kings[0])/8
	blackFile, blackRank := int(kings[1])%8, int(kings[1])/8
	direct := (whiteFile == blackFile && abs(whiteRank-blackRank) == 2) ||
		(whiteRank == blackRank && abs(whiteFile-blackFile) == 2)
	if !direct {
		return 0
	}
	if position.Turn() == chess.Black {
		return 12
	}
	return -12
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
