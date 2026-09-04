package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	square, err := chess.ParseSquare("e1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("e1: %+v\\n", chess.NewPosition().PieceAt(square))
}
