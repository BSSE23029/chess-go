package main

import (
	"fmt"
	"log"

	"chess-go/protocol"
)

func main() {
	server := protocol.NewServer()
	if _, err := server.Create(protocol.CreateMatchRequest{MatchID: "demo", PlayerID: "alice", Color: "white"}); err != nil {
		log.Fatal(err)
	}
	snapshot, err := server.Resign(protocol.ResignRequest{MatchID: "demo", PlayerID: "alice"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(snapshot.Result)
}
