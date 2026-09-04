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
	game.Undo()
	fmt.Println("can redo:", game.CanRedo())
	game.Redo()
	fmt.Println("moves:", game.MoveCount())
}
