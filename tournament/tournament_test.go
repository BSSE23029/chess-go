package tournament

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

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

type firstMovePlayer struct{}

func (firstMovePlayer) ChooseMove(_ context.Context, position chess.Position) (chess.Move, error) {
	moves := position.LegalMoves()
	if len(moves) == 0 {
		return chess.Move{}, engine.ErrNoLegalMoves
	}
	return moves[0], nil
}

func TestRunPlayersAcceptsExternalPlayer(t *testing.T) {
	players := []PlayerSpec{
		{Name: "local", New: func() chess.Player { return firstMovePlayer{} }},
		{Name: "external", New: func() chess.Player { return firstMovePlayer{} }},
	}
	report, err := RunPlayers(context.Background(), Config{GamesPerPair: 1, MaxPlies: 2}, players)
	if err != nil || report.Games != 1 || len(report.Records) != 1 {
		t.Fatalf("external-player tournament = %#v, %v", report, err)
	}
	if report.Records[0].White != "local" || report.Records[0].Black != "external" {
		t.Fatalf("external-player names = %#v", report.Records[0])
	}
}

func TestTournamentTimeControlParsesAndFlags(t *testing.T) {
	clock, err := newTournamentClock("5+3")
	if err != nil || clock.remaining[chess.White] != 5*time.Minute || clock.increment != 3*time.Second {
		t.Fatalf("parsed clock = %#v, %v", clock, err)
	}
	for _, value := range []string{"", "fixed", "unlimited", "5m+3s"} {
		if _, err := newTournamentClock(value); err != nil {
			t.Fatalf("time control %q rejected: %v", value, err)
		}
	}
	for _, value := range []string{"0+3", "bad", "5+-1"} {
		if _, err := newTournamentClock(value); err == nil {
			t.Fatalf("invalid time control %q accepted", value)
		}
	}
	if !clock.complete(chess.White, 2*time.Minute) || clock.remaining[chess.White] != 3*time.Minute+3*time.Second {
		t.Fatalf("clock completion = %s", clock.remaining[chess.White])
	}
}

type slowPlayer struct{}

func (slowPlayer) ChooseMove(_ context.Context, position chess.Position) (chess.Move, error) {
	time.Sleep(5 * time.Millisecond)
	return position.LegalMoves()[0], nil
}

func TestTournamentTimeControlProducesTimeoutResult(t *testing.T) {
	players := []PlayerSpec{
		{Name: "slow-white", New: func() chess.Player { return slowPlayer{} }},
		{Name: "fast-black", New: func() chess.Player { return firstMovePlayer{} }},
	}
	report, err := RunPlayers(context.Background(), Config{GamesPerPair: 1, MaxPlies: 1, TimeControl: "1ms"}, players)
	if err != nil || len(report.Records) != 1 || report.Records[0].Result != "0-1" {
		t.Fatalf("timeout report = %#v, %v", report, err)
	}
}
