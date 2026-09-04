package main

import (
	"fmt"
	"os"

	"chess-go/engine"
)

func main() {
	command := os.Getenv("CHESS_UCI_ENGINE")
	if command == "" {
		command = "stockfish"
	}
	adapter, err := engine.NewUCIEngine(command)
	if err != nil {
		panic(err)
	}
	fmt.Println(adapter.Command, adapter.Depth)
}
