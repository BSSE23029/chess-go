package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	position, err := chess.ParseFEN("7k/P7/8/8/8/8/8/7K w - - 0 1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(chess.NewGameFromPosition(position).PGN())
}
