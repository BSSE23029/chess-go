package main

import (
	"fmt"
	"log"

	"chess-go/lan"
)

func main() {
	advertiser, err := lan.NewAdvertiser(lan.Service{
		Instance: "demo-chess", Host: "localhost", Port: 8080,
		Metadata: map[string]string{"protocol": "1"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(advertiser.Service.Instance, advertiser.Service.Port)
}
