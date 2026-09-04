package main

import (
	"context"
	"fmt"

	"chess-go"
	"chess-go/engine"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := engine.New(4).ChooseMove(ctx, chess.NewPosition())
	fmt.Println(err)
}
