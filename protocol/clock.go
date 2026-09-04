package protocol

import (
	"errors"
	"time"

	"chess-go"
)

var (
	// ErrInvalidClockConfig indicates a negative or incomplete time control.
	ErrInvalidClockConfig = errors.New("invalid clock configuration")
	// ErrTimeExpired indicates that a player's clock reached zero.
	ErrTimeExpired = errors.New("player clock expired")
)

// ClockConfig defines the server-authoritative time control. An initial value
// of zero disables clocks; increments are applied after each committed move.
type ClockConfig struct {
	Initial   time.Duration
	Increment time.Duration
}

type matchClock struct {
	initial   time.Duration
	increment time.Duration
	remaining [2]time.Duration
	startedAt time.Time
	now       func() time.Time
	running   bool
	expired   bool
}

type clockSnapshot struct {
	white     int64
	black     int64
	increment int64
	running   bool
}

// NewMatchWithClock creates an authoritative match with a validated clock.
func NewMatchWithClock(id string, position chess.Position, config ClockConfig) (*Match, error) {
	if config.Initial < 0 || config.Increment < 0 || (config.Initial == 0 && config.Increment != 0) {
		return nil, ErrInvalidClockConfig
	}
	return newMatch(id, position, config), nil
}

func newMatch(id string, position chess.Position, config ClockConfig) *Match {
	return &Match{
		id:         id,
		position:   position,
		spectators: make(map[string]struct{}),
		result:     positionResult(position),
		clock:      newMatchClock(config),
	}
}

func newMatchClock(config ClockConfig) *matchClock {
	return &matchClock{
		initial:   config.Initial,
		increment: config.Increment,
		remaining: [2]time.Duration{config.Initial, config.Initial},
		now:       time.Now,
	}
}

func (m *Match) clockNow() time.Time {
	if m.clock == nil || m.clock.now == nil {
		return time.Now()
	}
	return m.clock.now()
}

func (m *Match) startClockLocked(now time.Time) {
	if m.clock == nil || m.clock.initial <= 0 || m.clock.running || m.result != "" {
		return
	}
	if m.players[chess.White] == "" || m.players[chess.Black] == "" {
		return
	}
	m.clock.running = true
	m.clock.startedAt = now
}

func (m *Match) syncClockLocked(now time.Time) {
	if m.clock == nil || !m.clock.running || m.result != "" {
		return
	}
	turn := m.position.Turn()
	elapsed := now.Sub(m.clock.startedAt)
	if elapsed <= 0 {
		return
	}
	m.clock.remaining[turn] -= elapsed
	m.clock.startedAt = now
	if m.clock.remaining[turn] > 0 {
		return
	}
	m.clock.remaining[turn] = 0
	m.clock.running = false
	m.clock.expired = true
	if turn == chess.White {
		m.result = "0-1"
	} else {
		m.result = "1-0"
	}
}

func (m *Match) finishMoveClockLocked(mover chess.Color, now time.Time) {
	if m.clock == nil || m.clock.initial <= 0 {
		return
	}
	if m.clock.increment > 0 {
		m.clock.remaining[mover] += m.clock.increment
	}
	if m.result == "" {
		m.clock.running = true
		m.clock.startedAt = now
	} else {
		m.clock.running = false
	}
}

func (m *Match) clockSnapshotLocked() clockSnapshot {
	if m.clock == nil || m.clock.initial <= 0 {
		return clockSnapshot{}
	}
	return clockSnapshot{
		white:     durationMillis(m.clock.remaining[chess.White]),
		black:     durationMillis(m.clock.remaining[chess.Black]),
		increment: durationMillis(m.clock.increment),
		running:   m.clock.running && m.result == "",
	}
}

func durationMillis(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}
	return int64(value / time.Millisecond)
}

func durationFromMillis(value int64) (time.Duration, error) {
	if value < 0 || value > int64((time.Duration(1<<63-1))/time.Millisecond) {
		return 0, ErrInvalidClockConfig
	}
	return time.Duration(value) * time.Millisecond, nil
}
