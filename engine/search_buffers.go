package engine

import "chess-go"

const quiescenceCheckLimit = 1

func quiescenceMoves(position *chess.Position, ply int, control *searchControl) []chess.Move {
	var buffer []chess.Move
	if control != nil && ply >= 0 && ply < len(control.moveStorage) {
		buffer = control.moveStorage[ply][:0]
	}
	moves := orderedMovesInto(position, buffer)
	if position.InCheck() {
		return moves
	}
	filtered := moves[:0]
	for _, move := range moves {
		if move.Flags&chess.Capture != 0 || move.Promotion != chess.NoPiece {
			filtered = append(filtered, move)
			continue
		}
		if ply >= quiescenceCheckLimit {
			continue
		}
		undo := position.MakeLegalMove(move)
		givesCheck := position.InCheck()
		position.UnmakeMove(undo)
		if givesCheck {
			filtered = append(filtered, move)
		}
	}
	return filtered
}
