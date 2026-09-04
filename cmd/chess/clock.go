package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"chess-go"
)

type gameClock struct {
	initial   time.Duration
	increment time.Duration
	remaining [2]time.Duration
	active    chess.Color
	started   time.Time
	running   bool
	now       func() time.Time
}

func clockFlags(options *flag.FlagSet) (*string, *string) {
	limit := options.String("clock", os.Getenv("CHESS_CLOCK"), "time per side, such as 10m")
	increment := options.String("increment", firstSet(os.Getenv("CHESS_INCREMENT"), "0s"), "increment per move")
	return limit, increment
}

func parseClock(limitValue, incrementValue string) (*gameClock, error) {
	if limitValue == "" {
		if incrementValue != "" && incrementValue != "0" && incrementValue != "0s" {
			return nil, errors.New("increment requires a clock")
		}
		return nil, nil
	}
	limit, err := time.ParseDuration(limitValue)
	if err != nil || limit <= 0 {
		return nil, fmt.Errorf("clock must be a positive duration")
	}
	increment, err := time.ParseDuration(incrementValue)
	if err != nil || increment < 0 {
		return nil, fmt.Errorf("increment must be a non-negative duration")
	}
	clock := &gameClock{initial: limit, increment: increment, now: time.Now}
	clock.reset()
	return clock, nil
}

func (c *gameClock) reset() {
	if c == nil {
		return
	}
	c.remaining = [2]time.Duration{c.initial, c.initial}
	c.running = false
}

func (c *gameClock) start(color chess.Color) {
	if c == nil || c.running {
		return
	}
	c.active, c.started, c.running = color, c.now(), true
}

func (c *gameClock) values() [2]time.Duration {
	if c == nil {
		return [2]time.Duration{}
	}
	values := c.remaining
	if c.running {
		values[c.active] -= c.now().Sub(c.started)
		if values[c.active] < 0 {
			values[c.active] = 0
		}
	}
	return values
}

func (c *gameClock) completeMove(color chess.Color) bool {
	if c == nil {
		return true
	}
	c.start(color)
	values := c.values()
	c.remaining = values
	if c.remaining[color] <= 0 {
		c.running = false
		return false
	}
	c.remaining[color] += c.increment
	c.active, c.started = color.Opponent(), c.now()
	return true
}

func (c *gameClock) untilFlag(color chess.Color) time.Duration {
	if c == nil {
		return 0
	}
	c.start(color)
	return c.values()[color]
}

func (c *gameClock) sync(color chess.Color) {
	if c == nil {
		return
	}
	if c.running {
		c.remaining = c.values()
	}
	c.active, c.started, c.running = color, c.now(), true
}

func (c *gameClock) context(parent context.Context, color chess.Color) (context.Context, context.CancelFunc) {
	if c == nil {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, c.untilFlag(color))
}

func (s *session) flag(color chess.Color) {
	s.timeout = colorName(color.Opponent()) + " wins on time"
	if s.clock != nil {
		s.clock.remaining = s.clock.values()
		s.clock.running = false
	}
}

func (s *session) clockSummary() string {
	if s.clock == nil {
		return ""
	}
	values := s.clock.values()
	return fmt.Sprintf("White %s · Black %s · +%s", formatClock(values[chess.White]), formatClock(values[chess.Black]), formatClock(s.clock.increment))
}

func formatClock(value time.Duration) string {
	if value < 0 {
		value = 0
	}
	total := int64((value + time.Second - 1) / time.Second)
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}
