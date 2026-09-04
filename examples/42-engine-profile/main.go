package main

import (
	"fmt"
	"log"

	"chess-go/engine"
)

func main() {
	profile, err := engine.ParseStrengthProfile("club")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(profile.String(), profile.Config().Depth)
}
