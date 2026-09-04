package main

import (
	"context"
	"fmt"
	"log"

	"chess-go"
	"chess-go/engine"
)

func main() {
	bot := engine.New(3)
	move, stats, err := bot.Search(context.Background(), chess.NewPosition(), engine.SearchLimits{MaxDepth: 3})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(move.UCI(), stats.Depth, stats.Nodes, stats.Score)
}
