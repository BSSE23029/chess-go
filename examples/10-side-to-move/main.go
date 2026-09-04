package main

import (
	"fmt"

	"chess-go"
)

func main() {
	position := chess.NewPosition()
	fmt.Println("turn:", position.Turn(), "opponent:", position.Turn().Opponent())
}
