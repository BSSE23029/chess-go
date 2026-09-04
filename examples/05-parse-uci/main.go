package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	move, err := chess.ParseUCI("e2e4")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("from=%s to=%s promotion=%d flags=%d\\n", move.From, move.To, move.Promotion, move.Flags)
}
