package engine

import "chess-go"

var exchangePieceValues = [...]Score{0, 100, 320, 330, 500, 900, 0}

// staticExchange estimates the material result of a capture when both sides
// repeatedly recapture on the same square. Legal move generation excludes
// pinned and king-unsafe recaptures, keeping the tactical signal independent
// of search state and suitable for future ordering or conservative pruning.
func staticExchange(position chess.Position, move chess.Move) Score {
	if move.Flags&chess.Capture == 0 {
		return 0
	}
	gain := capturedPieceValue(position, move) + promotionGain(move)
	next := position
	undo := next.MakeLegalMove(move)
	gain -= bestExchangeRecapture(&next, move.To)
	next.UnmakeMove(undo)
	return gain
}

func bestExchangeRecapture(position *chess.Position, target chess.Square) Score {
	captured := position.PieceAt(target)
	if captured.IsEmpty() {
		return 0
	}
	var buffer [64]chess.Move
	moves := position.LegalMovesInto(buffer[:0])
	best := Score(0)
	for _, move := range moves {
		if move.To != target || move.Flags&chess.Capture == 0 {
			continue
		}
		undo := position.MakeLegalMove(move)
		gain := exchangePieceValues[captured.Type] + promotionGain(move) - bestExchangeRecapture(position, target)
		position.UnmakeMove(undo)
		if gain > best {
			best = gain
		}
	}
	return best
}

func capturedPieceValue(position chess.Position, move chess.Move) Score {
	square := move.To
	if move.Flags&chess.EnPassant != 0 {
		if position.Turn() == chess.White {
			square -= 8
		} else {
			square += 8
		}
	}
	return exchangePieceValues[position.PieceAt(square).Type]
}

func promotionGain(move chess.Move) Score {
	if move.Promotion == chess.NoPiece {
		return 0
	}
	return exchangePieceValues[move.Promotion] - exchangePieceValues[chess.Pawn]
}
