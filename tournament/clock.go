package tournament

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"chess-go"
)

type tournamentClock struct {
	remaining [2]time.Duration
	increment time.Duration
}

func newTournamentClock(value string) (*tournamentClock, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "fixed" || value == "unlimited" {
		return nil, nil
	}
	parts := strings.Split(value, "+")
	if len(parts) > 2 || strings.TrimSpace(parts[0]) == "" {
		return nil, errors.New("time control must be BASE[+INCREMENT], such as 5+3")
	}
	base, err := parseClockPart(parts[0], time.Minute)
	if err != nil || base <= 0 {
		return nil, errors.New("time control base must be positive")
	}
	increment := time.Duration(0)
	if len(parts) == 2 {
		increment, err = parseClockPart(parts[1], time.Second)
		if err != nil || increment < 0 {
			return nil, errors.New("time control increment must be non-negative")
		}
	}
	return &tournamentClock{remaining: [2]time.Duration{base, base}, increment: increment}, nil
}

func parseClockPart(value string, bareUnit time.Duration) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("empty time control part")
	}
	if strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		number, err := strconv.ParseFloat(value, 64)
		if err != nil || number < 0 {
			return 0, errors.New("invalid time control number")
		}
		return time.Duration(number * float64(bareUnit)), nil
	}
	return time.ParseDuration(value)
}

func (c *tournamentClock) moveContext(parent context.Context, color chess.Color) (context.Context, context.CancelFunc, bool) {
	if c == nil {
		return parent, func() {}, true
	}
	if c.remaining[color] <= 0 {
		return nil, func() {}, false
	}
	moveContext, cancel := context.WithTimeout(parent, c.remaining[color])
	return moveContext, cancel, true
}

func (c *tournamentClock) complete(color chess.Color, elapsed time.Duration) bool {
	if c == nil {
		return true
	}
	c.remaining[color] -= elapsed
	if c.remaining[color] < 0 {
		return false
	}
	c.remaining[color] += c.increment
	return true
}
