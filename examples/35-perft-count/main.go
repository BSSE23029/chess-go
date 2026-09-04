package main

import (
	"context"
	"fmt"
	"log"

	"chess-go"
	"chess-go/perft"
)

func main() {
	nodes, err := perft.Count(context.Background(), chess.NewPosition(), 3)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(nodes)
}
