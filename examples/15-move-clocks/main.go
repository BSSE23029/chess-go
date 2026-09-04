package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	game := chess.NewGame()
	if err := game.PlayUCI("e2e4"); err != nil {
		log.Fatal(err)
	}
	position := game.Position()
	fmt.Println("halfmove:", position.HalfmoveClock(), "fullmove:", position.FullmoveNumber())
}
