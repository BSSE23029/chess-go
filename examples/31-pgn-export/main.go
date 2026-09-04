package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	game := chess.NewGame()
	if err := game.PlaySAN("e4"); err != nil {
		log.Fatal(err)
	}
	fmt.Println(game.PGN())
}
