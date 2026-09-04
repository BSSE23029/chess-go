package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	position := chess.NewPosition()
	move, err := position.ParseSAN("e4")
	if err != nil {
		log.Fatal(err)
	}
	san, err := position.SAN(move)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(san)
}
