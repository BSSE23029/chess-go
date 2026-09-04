package main

import (
	"fmt"
	"log"

	"chess-go/protocol"
)

func main() {
	payload := protocol.MoveRequest{MatchID: "demo", PlayerID: "alice", Sequence: 0, PositionHash: 7, UCI: "e2e4"}
	data, err := protocol.Encode(protocol.Move, "request-1", payload)
	if err != nil {
		log.Fatal(err)
	}
	envelope, err := protocol.Decode(data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(envelope.Version, envelope.Type)
}
