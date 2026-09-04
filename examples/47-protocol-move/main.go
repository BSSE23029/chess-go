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
	if _, err := server.Join(protocol.JoinMatchRequest{MatchID: "demo", PlayerID: "bob", Color: "black"}); err != nil {
		log.Fatal(err)
	}
	accepted, err := server.ApplyMove(protocol.MoveRequest{
		MatchID: "demo", PlayerID: "alice", Sequence: snapshot.Sequence,
		PositionHash: snapshot.PositionHash, UCI: "e2e4",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(accepted.Sequence, accepted.UCI)
}
