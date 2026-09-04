package main

import (
	"fmt"

	"chess-go"
)

func main() {
	position := chess.NewPosition()
	move := position.LegalMoves()[0]
	undo := position.MakeLegalMove(move)
	fmt.Println("after:", position.FEN())
	position.UnmakeMove(undo)
	fmt.Println("restored:", position.FEN())
}
