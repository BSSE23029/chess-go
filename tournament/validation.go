package tournament

import (
	"errors"
	"fmt"
	"strings"

	"chess-go/engine"
)

func validateConfig(config Config) error {
	if err := validateCommon(config); err != nil {
		return err
	}
	if config.GamesPerPair < 1 || config.MaxPlies < 1 {
		return errors.New("tournament games and max plies must be positive")
	}
	seen := make(map[engine.StrengthProfile]struct{}, len(config.Profiles))
	for _, profile := range config.Profiles {
		if _, exists := seen[profile]; exists {
			return fmt.Errorf("duplicate tournament profile %s", profile)
		}
		seen[profile] = struct{}{}
	}
	return nil
}

func validateCommon(config Config) error {
	if len(config.Profiles) < 2 {
		return errors.New("tournament requires at least two profiles")
	}
	return nil
}

func validatePlayers(config Config, players []PlayerSpec) error {
	if len(players) < 2 {
		return errors.New("tournament requires at least two players")
	}
	if config.GamesPerPair < 1 || config.MaxPlies < 1 {
		return errors.New("tournament games and max plies must be positive")
	}
	seen := make(map[string]struct{}, len(players))
	for _, player := range players {
		if strings.TrimSpace(player.Name) == "" || player.New == nil {
			return errors.New("tournament players require names and constructors")
		}
		if _, exists := seen[player.Name]; exists {
			return fmt.Errorf("duplicate tournament player %s", player.Name)
		}
		seen[player.Name] = struct{}{}
	}
	return nil
}
