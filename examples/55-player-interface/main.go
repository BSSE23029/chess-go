package main

import (
	"context"
	"fmt"
	"log"

	"chess-go"
	"chess-go/engine"
)

func main() {
	var player chess.Player = engine.New(1)
	move, err := player.ChooseMove(context.Background(), chess.NewPosition())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(move.UCI())
}
