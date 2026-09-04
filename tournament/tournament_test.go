package tournament

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"chess-go"
	"chess-go/engine"
)

func TestRoundRobinReportIsReproducibleAndPortable(t *testing.T) {
	config := Config{Profiles: []engine.StrengthProfile{engine.Learner, engine.Beginner}, GamesPerPair: 2, MaxPlies: 6, Seed: 42, EngineVersion: "test", NodeBudget: 123, TimeControl: "fixed", HardwareClass: "test-cpu"}
	first, err := Run(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if first.PGN() != second.PGN() || len(first.Records) != 2 || first.Games != 2 {
		t.Fatalf("non-deterministic report: %#v vs %#v", first, second)
	}
	if !strings.Contains(first.PGN(), `[EngineVersion "test"]`) || !strings.Contains(first.PGN(), `[Result "1/2-1/2"]`) {
		t.Fatalf("PGN metadata missing:\n%s", first.PGN())
	}
	parsed, err := chess.ParsePGN(first.Records[0].PGN)
	if err != nil || parsed.Result() != first.Records[0].Result {
		t.Fatalf("record PGN = %v, %q", err, first.Records[0].Result)
	}
	data, err := first.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(data, &decoded); err != nil || decoded.NodeBudget != 123 || decoded.HardwareClass != "test-cpu" {
		t.Fatalf("report JSON = %s, %v", data, err)
	}
}

func TestTournamentRejectsInvalidConfigAndCancellation(t *testing.T) {
	if _, err := Run(context.Background(), Config{Profiles: []engine.StrengthProfile{engine.Learner}, GamesPerPair: 1, MaxPlies: 1}); err == nil {
		t.Fatal("single-profile tournament accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, Config{Profiles: []engine.StrengthProfile{engine.Learner, engine.Beginner}, GamesPerPair: 1, MaxPlies: 1}); err == nil {
		t.Fatal("cancelled tournament completed")
	}
}
