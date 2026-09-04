package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	game := chess.NewGame()
	if err := game.SetTag("Event", "Demo"); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\\n", game.Tags())
}
