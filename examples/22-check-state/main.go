package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	position, err := chess.ParseFEN("4k3/8/8/8/8/8/4r3/4K3 w - - 0 1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("in check:", position.InCheck())
}
