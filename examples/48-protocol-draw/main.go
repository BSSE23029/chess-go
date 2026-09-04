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
	if _, err := server.Join(protocol.JoinMatchRequest{MatchID: "demo", PlayerID: "bob", Color: "black"}); err != nil {
		log.Fatal(err)
	}
	if _, err := server.OfferDraw(protocol.DrawOfferRequest{MatchID: "demo", PlayerID: "alice"}); err != nil {
		log.Fatal(err)
	}
	snapshot, err := server.OfferDraw(protocol.DrawOfferRequest{MatchID: "demo", PlayerID: "bob"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(snapshot.Result)
}
