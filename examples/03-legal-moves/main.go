package main

import (
	"fmt"

	"chess-go"
)

func main() {
	fmt.Println("legal moves:", len(chess.NewPosition().LegalMoves()))
}
