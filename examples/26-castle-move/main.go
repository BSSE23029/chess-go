package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	position, err := chess.ParseFEN("r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")
	if err != nil {
		log.Fatal(err)
	}
	next, err := position.ApplyUCI("e1g1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(next.FEN())
}
