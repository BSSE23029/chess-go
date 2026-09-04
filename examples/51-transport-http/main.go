package main

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"

	"chess-go/protocol"
	"chess-go/transport"
)

func main() {
	server := httptest.NewServer(transport.NewHTTPServer(protocol.NewServer(), ""))
	defer server.Close()
	client, err := transport.NewClient(server.URL, "")
	if err != nil {
		log.Fatal(err)
	}
	snapshot, err := client.Create(context.Background(), "example-create", protocol.CreateMatchRequest{MatchID: "demo"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(snapshot.MatchID)
}
