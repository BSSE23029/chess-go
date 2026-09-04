package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	game, err := chess.ParsePGN("[Result \"*\"]\n\n1. e4 e5 *")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(game.MoveCount())
}
