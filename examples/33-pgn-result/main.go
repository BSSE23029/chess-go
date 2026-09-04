package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	game := chess.NewGame()
	if err := game.SetResult("1/2-1/2"); err != nil {
		log.Fatal(err)
	}
	fmt.Println(game.Result())
}
