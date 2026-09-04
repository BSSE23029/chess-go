package main

import (
	"fmt"
	"log"

	"chess-go"
)

func main() {
	square, err := chess.ParseSquare("e4")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(square, square.String())
}
