package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	position, err := chess.ParseFEN(chess.InitialFEN)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(position.FEN() == chess.InitialFEN)
}
