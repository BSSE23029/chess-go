package main

import (
	"fmt"
	"log"

	"chess-go/protocol"
)

func main() {
	server := protocol.NewServer()
	snapshot, err := server.Create(protocol.CreateMatchRequest{MatchID: "demo", PlayerID: "alice", Color: "white"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(snapshot.MatchID, snapshot.Turn, snapshot.Result)
}
