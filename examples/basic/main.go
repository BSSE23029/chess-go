// Command basic demonstrates the public chess, engine, and perft APIs.
package main

import (
	"context"
	"fmt"
	"log"

	"chess-go"
	"chess-go/engine"
	"chess-go/perft"
)

func main() {
	game, err := chess.FromSAN([]string{"e4", "e5", "Nf3", "Nc6"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("position:", game.Position().FEN())

	bot := engine.New(2)
	move, err := bot.ChooseMove(context.Background(), game.Position())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("engine move:", move.UCI())

	nodes, err := perft.Count(context.Background(), chess.NewPosition(), 3)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("perft(3):", nodes)
}
