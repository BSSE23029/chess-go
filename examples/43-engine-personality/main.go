package main

import (
	"fmt"
	"log"

	"chess-go/engine"
)

func main() {
	personality, err := engine.ParsePersonality("aggressive")
	if err != nil {
		log.Fatal(err)
	}
	bot := engine.New(2)
	bot.SetPersonality(personality, 42)
	fmt.Println(bot.Personality.String(), bot.Seed)
}
