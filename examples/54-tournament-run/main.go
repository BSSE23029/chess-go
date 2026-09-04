package main

import (
	"context"
	"fmt"
	"log"

	"chess-go/engine"
	"chess-go/tournament"
)

func main() {
	report, err := tournament.Run(context.Background(), tournament.Config{
		Profiles:     []engine.StrengthProfile{engine.Learner, engine.Beginner},
		GamesPerPair: 1, MaxPlies: 4, Seed: 7,
		EngineVersion: "example", TimeControl: "fixed",
		HardwareClass: "example",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(report.Games, report.Ratings)
}
