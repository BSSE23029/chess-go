package main

import (
	"fmt"

	"chess-go"
	"chess-go/engine"
)

func main() {
	fmt.Println(engine.PositionalEvaluator{}.Evaluate(chess.NewPosition()))
}
