package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	position := chess.NewPosition()
	next, err := position.ApplyUCI("e2e4")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("original:", position.FEN())
	fmt.Println("next:", next.FEN())
}
