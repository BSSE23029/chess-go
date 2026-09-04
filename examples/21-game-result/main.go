package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	position, err := chess.ParseFEN("7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	if err != nil {
		log.Fatal(err)
	}
	game := chess.NewGameFromPosition(position)
	fmt.Println("status:", game.Status(), "result:", game.Result())
}
