package main

import (
	"fmt"

	"chess-go"
)

func main() {
	fmt.Println(chess.NewPosition().EnPassant())
}
