package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	game := chess.NewGame()
	for _, value := range []string{"e2e4", "d7d5", "e4d5"} {
		if err := game.PlayUCI(value); err != nil {
			log.Fatal(err)
		}
	}
	move := game.Moves()[2]
	fmt.Println("capture:", move.Flags&chess.Capture != 0)
}
