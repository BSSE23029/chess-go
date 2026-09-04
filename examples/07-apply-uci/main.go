package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	position, err := chess.NewPosition().ApplyUCI("g1f3")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(position.FEN())
}
