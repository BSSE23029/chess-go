package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	move, err := chess.ParseUCI("a7a8q")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(move.UCI())
}
