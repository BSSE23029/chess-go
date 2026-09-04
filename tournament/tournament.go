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
	"time"

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
	Seed          uint64                `json:"seed"`
	NodeBudget    uint64                `json:"node_budget"`
	HardwareClass string                `json:"hardware_class"`
	Games         int                   `json:"games"`
	Records       []GameRecord          `json:"records"`
	Ratings       map[string]float64    `json:"ratings"`
	Confidence95  map[string][2]float64 `json:"confidence95"`
}

// PlayerSpec names a tournament participant and constructs a fresh player for
// each game. It can represent a built-in profile or an external UCI engine.
type PlayerSpec struct {
	Name string
	New  func() chess.Player
}

// Run executes every profile pair with colors alternated between games.
func Run(ctx context.Context, config Config) (Report, error) {
	if ctx == nil {
		return Report{}, errors.New("nil tournament context")
	}
	if err := validateConfig(config); err != nil {
		return Report{}, err
	}
	players := make([]PlayerSpec, 0, len(config.Profiles))
	for _, profile := range config.Profiles {
		profile := profile
		players = append(players, PlayerSpec{Name: profile.String(), New: func() chess.Player {
			return engine.NewProfile(profile)
		}})
	}
	return RunPlayers(ctx, config, players)
}

// RunPlayers executes a deterministic round-robin tournament for arbitrary
// chess.Player implementations, including UCI engines.
func RunPlayers(ctx context.Context, config Config, players []PlayerSpec) (Report, error) {
	if ctx == nil {
		return Report{}, errors.New("nil tournament context")
	}
	if err := validatePlayers(config, players); err != nil {
		return Report{}, err
	}
	if _, err := newTournamentClock(config.TimeControl); err != nil {
		return Report{}, err
	}
	report := Report{
		EngineVersion: config.EngineVersion,
		TimeControl:   config.TimeControl,
		Seed:          config.Seed,
		NodeBudget:    config.NodeBudget,
		HardwareClass: config.HardwareClass,
		Ratings:       make(map[string]float64, len(players)),
		Confidence95:  make(map[string][2]float64, len(players)),
	}
	for _, player := range players {
		report.Ratings[player.Name] = 1500
	}
	gameNumber := 0
	for first := 0; first < len(players); first++ {
		for second := first + 1; second < len(players); second++ {
			for gameIndex := 0; gameIndex < config.GamesPerPair; gameIndex++ {
				if err := ctx.Err(); err != nil {
					return Report{}, err
				}
				whitePlayer, blackPlayer := players[first], players[second]
				if gameIndex%2 == 1 {
					whitePlayer, blackPlayer = blackPlayer, whitePlayer
				}
				record, err := playGame(ctx, gameNumber+1, whitePlayer, blackPlayer, config)
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
	for _, player := range players {
		name := player.Name
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

func playGame(ctx context.Context, number int, whitePlayer, blackPlayer PlayerSpec, config Config) (GameRecord, error) {
	game := chess.NewGame()
	clock, err := newTournamentClock(config.TimeControl)
	if err != nil {
		return GameRecord{}, err
	}
	white := whitePlayer.New()
	black := blackPlayer.New()
	if white == nil || black == nil {
		return GameRecord{}, errors.New("tournament player constructor returned nil")
	}
	if bot, ok := white.(*engine.Bot); ok {
		bot.Seed = config.Seed + uint64(number)*0x9e3779b97f4a7c15
	}
	if bot, ok := black.(*engine.Bot); ok {
		bot.Seed = config.Seed + uint64(number)*0x243f6a8885a308d3
	}
	for ply := 0; ply < config.MaxPlies && game.Status() == chess.Ongoing; ply++ {
		player := white
		if game.Position().Turn() == chess.Black {
			player = black
		}
		mover := game.Position().Turn()
		moveContext, cancel, available := clock.moveContext(ctx, mover)
		if !available {
			if err := game.SetResult(timeoutResult(mover)); err != nil {
				return GameRecord{}, err
			}
			break
		}
		started := time.Now()
		move, err := chooseMove(moveContext, player, game.Position(), config.NodeBudget)
		cancel()
		if clock != nil && !clock.complete(mover, time.Since(started)) {
			if err := game.SetResult(timeoutResult(mover)); err != nil {
				return GameRecord{}, err
			}
			break
		}
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) && clock != nil {
				if setErr := game.SetResult(timeoutResult(mover)); setErr != nil {
					return GameRecord{}, setErr
				}
				break
			}
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
	whiteName, blackName := whitePlayer.Name, blackPlayer.Name
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

func timeoutResult(color chess.Color) string {
	if color == chess.White {
		return "0-1"
	}
	return "1-0"
}

func chooseMove(ctx context.Context, player chess.Player, position chess.Position, nodeBudget uint64) (chess.Move, error) {
	if bot, ok := player.(*engine.Bot); ok && nodeBudget != 0 {
		move, _, err := bot.Search(ctx, position, engine.SearchLimits{MaxDepth: bot.Depth, MaxNodes: nodeBudget})
		return move, err
	}
	return player.ChooseMove(ctx, position)
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
