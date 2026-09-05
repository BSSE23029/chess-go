package engine

import "chess-go"

func quiescenceMoves(position *chess.Position, ply int, control *searchControl) []chess.Move {
	var buffer []chess.Move
	if control != nil && ply >= 0 && ply < len(control.moveStorage) {
		buffer = control.moveStorage[ply][:0]
	}
	return orderedMovesInto(position, buffer)
}
