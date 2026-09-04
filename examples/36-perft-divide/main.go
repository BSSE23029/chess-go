package main

import (
	"context"
	"fmt"
	"log"

	"chess-go"
	"chess-go/perft"
)

func main() {
	results, err := perft.Divide(context.Background(), chess.NewPosition(), 2)
	if err != nil {
		log.Fatal(err)
	}
	for _, result := range results[:3] {
		fmt.Println(result.Move.UCI(), result.Nodes)
	}
}
