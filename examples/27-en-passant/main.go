package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	position, err := chess.ParseFEN("4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 2")
	if err != nil {
		log.Fatal(err)
	}
	next, err := position.ApplyUCI("e5d6")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(next.FEN())
}
