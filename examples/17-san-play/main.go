package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	game := chess.NewGame()
	for _, san := range []string{"e4", "e5", "Nf3"} {
		if err := game.PlaySAN(san); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println(game.Moves()[2].UCI())
}
