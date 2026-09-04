package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"chess-go/protocol"
	"chess-go/storage"
)

func main() {
	server := protocol.NewServer()
	if _, err := server.Create(protocol.CreateMatchRequest{MatchID: "demo"}); err != nil {
		log.Fatal(err)
	}
	directory, err := os.MkdirTemp("", "chess-go-example-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(directory)
	store, err := storage.NewFileStore(filepath.Join(directory, "matches.json"))
	if err != nil {
		log.Fatal(err)
	}
	if err := store.SaveServer(server); err != nil {
		log.Fatal(err)
	}
	fmt.Println(store.Path)
}
