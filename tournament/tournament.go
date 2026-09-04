// Package tournament runs deterministic headless engine matches and reports
// reproducible results suitable for strength calibration.
package tournament

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"chess-go"
	"chess-go/engine"
)

// Config controls a round-robin tournament.
type Config struct {
	Profiles      []engine.StrengthProfile
	GamesPerPair  int
	MaxPlies      int
	Seed          uint64
	EngineVersion string
	NodeBudget    uint64
	TimeControl   string
	HardwareClass string
}

// GameRecord contains one completed game and its portable PGN.
type GameRecord struct {
	Number int    `json:"number"`
	White  string `json:"white"`
	Black  string `json:"black"`
	Result string `json:"result"`
	Plies  int    `json:"plies"`
	PGN    string `json:"pgn"`
}

// Report contains all games, metadata, Elo estimates, and approximate 95% CIs.
type Report struct {
	EngineVersion string                `json:"engine_version"`
	TimeControl   string                `json:"time_control"`
	NodeBudget    uint64                `json:"node_budget"`
	HardwareClass string                `json:"hardware_class"`
	Games         int                   `json:"games"`
	Records       []GameRecord          `json:"records"`
	Ratings       map[string]float64    `json:"ratings"`
	Confidence95  map[string][2]float64 `json:"confidence95"`
}

// Run executes every profile pair with colors alternated between games.
func Run(ctx context.Context, config Config) (Report, error) {
	if err := validateConfig(config); err != nil {
		return Report{}, err
	}
	report := Report{
		EngineVersion: config.EngineVersion,
		TimeControl:   config.TimeControl,
		NodeBudget:    config.NodeBudget,
		HardwareClass: config.HardwareClass,
		Ratings:       make(map[string]float64, len(config.Profiles)),
		Confidence95:  make(map[string][2]float64, len(config.Profiles)),
	}
	for _, profile := range config.Profiles {
		report.Ratings[profile.String()] = 1500
	}
	gameNumber := 0
	for first := 0; first < len(config.Profiles); first++ {
		for second := first + 1; second < len(config.Profiles); second++ {
			for gameIndex := 0; gameIndex < config.GamesPerPair; gameIndex++ {
				if err := ctx.Err(); err != nil {
					return Report{}, err
				}
				whiteProfile, blackProfile := config.Profiles[first], config.Profiles[second]
				if gameIndex%2 == 1 {
					whiteProfile, blackProfile = blackProfile, whiteProfile
				}
				record, err := playGame(ctx, gameNumber+1, whiteProfile, blackProfile, config)
				if err != nil {
					return Report{}, err
				}
				report.Records = append(report.Records, record)
				gameNumber++
				updateRatings(report.Ratings, record)
			}
		}
	}
	report.Games = len(report.Records)
	for _, profile := range config.Profiles {
		name := profile.String()
		games := 0
		for _, record := range report.Records {
			if record.White == name || record.Black == name {
				games++
			}
		}
		margin := 0.0
		if games > 0 {
			margin = 1.96 * 400 / math.Sqrt(float64(games))
		}
		report.Confidence95[name] = [2]float64{report.Ratings[name] - margin, report.Ratings[name] + margin}
	}
	return report, nil
}

// PGN concatenates all tournament games with a blank line separator.
func (r Report) PGN() string {
	values := make([]string, 0, len(r.Records))
	for _, record := range r.Records {
		values = append(values, record.PGN)
	}
	return strings.Join(values, "\n\n")
}

// JSON returns an indented metadata/results document for archival.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func validateConfig(config Config) error {
	if len(config.Profiles) < 2 {
		return errors.New("tournament requires at least two profiles")
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

func playGame(ctx context.Context, number int, whiteProfile, blackProfile engine.StrengthProfile, config Config) (GameRecord, error) {
	game := chess.NewGame()
	white := engine.NewProfile(whiteProfile)
	black := engine.NewProfile(blackProfile)
	white.Seed = config.Seed + uint64(number)*0x9e3779b97f4a7c15
	black.Seed = config.Seed + uint64(number)*0x243f6a8885a308d3
	for ply := 0; ply < config.MaxPlies && game.Status() == chess.Ongoing; ply++ {
		player := white
		if game.Position().Turn() == chess.Black {
			player = black
		}
		move, err := chooseMove(ctx, player, game.Position(), config.NodeBudget)
		if err != nil {
			return GameRecord{}, err
		}
		if err := game.Play(move); err != nil {
			return GameRecord{}, fmt.Errorf("profile move %s: %w", move.UCI(), err)
		}
	}
	if game.Result() == "*" {
		if err := game.SetResult("1/2-1/2"); err != nil {
			return GameRecord{}, err
		}
	}
	whiteName, blackName := whiteProfile.String(), blackProfile.String()
	for _, tag := range []chess.PGNTag{
		{Name: "Event", Value: "chess-go tournament"},
		{Name: "Round", Value: fmt.Sprint(number)},
		{Name: "White", Value: whiteName},
		{Name: "Black", Value: blackName},
		{Name: "EngineVersion", Value: config.EngineVersion},
		{Name: "TimeControl", Value: config.TimeControl},
	} {
		if tag.Value == "" {
			continue
		}
		if err := game.SetTag(tag.Name, tag.Value); err != nil {
			return GameRecord{}, err
		}
	}
	return GameRecord{Number: number, White: whiteName, Black: blackName, Result: game.Result(), Plies: len(game.Moves()), PGN: game.PGN()}, nil
}

func chooseMove(ctx context.Context, player *engine.Bot, position chess.Position, nodeBudget uint64) (chess.Move, error) {
	if nodeBudget == 0 {
		return player.ChooseMove(ctx, position)
	}
	move, _, err := player.Search(ctx, position, engine.SearchLimits{MaxDepth: player.Depth, MaxNodes: nodeBudget})
	return move, err
}

func updateRatings(ratings map[string]float64, record GameRecord) {
	score := 0.5
	if record.Result == "1-0" {
		score = 1
	} else if record.Result == "0-1" {
		score = 0
	}
	white, black := ratings[record.White], ratings[record.Black]
	expectedWhite := 1 / (1 + math.Pow(10, (black-white)/400))
	delta := 20 * (score - expectedWhite)
	ratings[record.White] = white + delta
	ratings[record.Black] = black - delta
}
