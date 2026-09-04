package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"chess-go/engine"
	"chess-go/tournament"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "tournament:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	options := flag.NewFlagSet("tournament", flag.ContinueOnError)
	options.SetOutput(os.Stderr)
	profiles := options.String("profiles", firstSet(os.Getenv("CHESS_TOURNAMENT_PROFILES"), "Learner,Beginner,Casual"), "comma-separated strength profiles")
	games := options.Int("games", envPositive("CHESS_TOURNAMENT_GAMES", 2), "games per profile pair")
	plies := options.Int("plies", envPositive("CHESS_TOURNAMENT_PLIES", 100), "maximum plies per game")
	seed := options.String("seed", os.Getenv("CHESS_TOURNAMENT_SEED"), "deterministic seed")
	engineVersion := options.String("engine-version", firstSet(os.Getenv("CHESS_ENGINE_VERSION"), "dev"), "engine version metadata")
	nodeBudget := options.Uint64("node-budget", envUint64("CHESS_TOURNAMENT_NODE_BUDGET", 0), "node budget metadata")
	timeControl := options.String("time-control", os.Getenv("CHESS_TOURNAMENT_TIME_CONTROL"), "time-control metadata")
	hardwareClass := options.String("hardware-class", firstSet(os.Getenv("CHESS_TOURNAMENT_HARDWARE"), "unspecified"), "hardware metadata")
	pgnPath := options.String("pgn", "", "write tournament PGNs to a file")
	jsonPath := options.String("json", "", "write tournament report JSON to a file")
	if err := options.Parse(args); err != nil || options.NArg() != 0 {
		return errors.New("usage: tournament [--profiles Learner,Beginner] [--games N] [--plies N] [--pgn FILE] [--json FILE]")
	}
	parsedProfiles, err := parseProfiles(*profiles)
	if err != nil {
		return err
	}
	if *games < 1 || *plies < 1 {
		return errors.New("games and plies must be positive")
	}
	seedValue := uint64(0)
	if *seed != "" {
		seedValue, err = strconv.ParseUint(*seed, 0, 64)
		if err != nil {
			return errors.New("--seed must be an unsigned integer")
		}
	}
	report, err := tournament.Run(ctx, tournament.Config{Profiles: parsedProfiles, GamesPerPair: *games, MaxPlies: *plies, Seed: seedValue, EngineVersion: *engineVersion, NodeBudget: *nodeBudget, TimeControl: *timeControl, HardwareClass: *hardwareClass})
	if err != nil {
		return err
	}
	for _, profile := range parsedProfiles {
		name := profile.String()
		interval := report.Confidence95[name]
		fmt.Fprintf(output, "%s: %.1f (95%% %.1f–%.1f)\n", name, report.Ratings[name], interval[0], interval[1])
	}
	if *pgnPath != "" {
		if err := os.WriteFile(*pgnPath, []byte(report.PGN()+"\n"), 0o600); err != nil {
			return err
		}
	}
	if *jsonPath != "" {
		data, err := report.JSON()
		if err != nil {
			return err
		}
		if err := os.WriteFile(*jsonPath, append(data, '\n'), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func parseProfiles(value string) ([]engine.StrengthProfile, error) {
	parts := strings.Split(value, ",")
	profiles := make([]engine.StrengthProfile, 0, len(parts))
	for _, part := range parts {
		profile, err := engine.ParseStrengthProfile(part)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func firstSet(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func envPositive(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func envUint64(name string, fallback uint64) uint64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(value, 0, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
