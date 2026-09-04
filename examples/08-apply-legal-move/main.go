package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	position := chess.NewPosition()
	move := position.LegalMoves()[0]
	next, err := position.Apply(move)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(move.UCI(), next.FEN())
}
