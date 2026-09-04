package main

import (
	"fmt"

	"chess-go"
)

func main() {
	fmt.Printf("%016x\\n", chess.NewPosition().Hash())
}
