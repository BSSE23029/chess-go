package main

import (
	"context"
	"fmt"

	"chess-go"
	"chess-go/perft"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := perft.Count(ctx, chess.NewPosition(), 4)
	fmt.Println(err)
}
