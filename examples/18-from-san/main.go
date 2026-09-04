package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	game, err := chess.FromSAN([]string{"e4", "e5", "Nf3", "Nc6"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(game.Position().FEN())
}
