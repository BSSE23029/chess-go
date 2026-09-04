package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	game := chess.NewGame()
	for _, move := range []string{"e2e4", "d7d5", "e4d5"} {
		if err := game.PlayUCI(move); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("captured: %+v\\n", game.Captured())
}
