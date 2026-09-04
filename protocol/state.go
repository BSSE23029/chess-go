package protocol

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"chess-go"
)

// MatchState is the durable representation of one authoritative match. It
// contains enough information to restore the position, seats, move history,
// terminal outcome, spectators, and a running time control after restart.
type MatchState struct {
	MatchID        string    `json:"match_id"`
	InitialFEN     string    `json:"initial_fen"`
	FEN            string    `json:"fen"`
	Sequence       uint64    `json:"sequence"`
	Moves          []string  `json:"moves,omitempty"`
	WhitePlayer    string    `json:"white_player,omitempty"`
	BlackPlayer    string    `json:"black_player,omitempty"`
	Spectators     []string  `json:"spectators,omitempty"`
	Result         string    `json:"result"`
	DrawOfferedBy  string    `json:"draw_offered_by,omitempty"`
	ClockInitial   int64     `json:"clock_initial_millis,omitempty"`
	ClockIncrement int64     `json:"clock_increment_millis,omitempty"`
	WhiteTime      int64     `json:"white_time_millis,omitempty"`
	BlackTime      int64     `json:"black_time_millis,omitempty"`
	ClockRunning   bool      `json:"clock_running,omitempty"`
	ClockStartedAt time.Time `json:"clock_started_at,omitempty"`
}

// State returns a consistent durable snapshot of a match.
func (m *Match) State() (MatchState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncClockLocked(m.clockNow())
	if m.id == "" || m.initialFEN == "" {
		return MatchState{}, errors.New("match has no durable identity")
	}
	clock := m.clockSnapshotLocked()
	result := m.result
	if result == "" {
		result = "*"
	}
	spectators := make([]string, 0, len(m.spectators))
	for playerID := range m.spectators {
		spectators = append(spectators, playerID)
	}
	sort.Strings(spectators)
	state := MatchState{
		MatchID:        m.id,
		InitialFEN:     m.initialFEN,
		FEN:            m.position.FEN(),
		Sequence:       m.sequence,
		Moves:          moveStrings(m.moves),
		WhitePlayer:    m.players[chess.White],
		BlackPlayer:    m.players[chess.Black],
		Spectators:     spectators,
		Result:         result,
		DrawOfferedBy:  m.drawOfferedBy,
		WhiteTime:      clock.white,
		BlackTime:      clock.black,
		ClockIncrement: clock.increment,
		ClockRunning:   clock.running,
	}
	if m.clock != nil && m.clock.initial > 0 {
		state.ClockInitial = durationMillis(m.clock.initial)
		if m.clock.running {
			state.ClockStartedAt = m.clock.startedAt
		}
	}
	return state, nil
}

// NewMatchFromState validates and restores a durable match state.
func NewMatchFromState(state MatchState) (*Match, error) {
	if strings.TrimSpace(state.MatchID) == "" || state.InitialFEN == "" || state.FEN == "" {
		return nil, errors.New("match state requires ID and FEN values")
	}
	initial, err := chess.ParseFEN(state.InitialFEN)
	if err != nil {
		return nil, fmt.Errorf("invalid initial match FEN: %w", err)
	}
	position, err := chess.ParseFEN(state.FEN)
	if err != nil {
		return nil, fmt.Errorf("invalid match FEN: %w", err)
	}
	if state.Result == "" {
		state.Result = "*"
	}
	if !validResult(state.Result) {
		return nil, errors.New("invalid match result")
	}
	if state.Sequence != uint64(len(state.Moves)) {
		return nil, errors.New("match sequence does not match move history")
	}
	if state.WhitePlayer != "" && state.WhitePlayer == state.BlackPlayer {
		return nil, errors.New("match players must be distinct")
	}
	seen := make(map[string]struct{}, len(state.Spectators))
	for _, playerID := range state.Spectators {
		if playerID == "" || playerID == state.WhitePlayer || playerID == state.BlackPlayer {
			return nil, errors.New("invalid match spectator")
		}
		if _, exists := seen[playerID]; exists {
			return nil, errors.New("duplicate match spectator")
		}
		seen[playerID] = struct{}{}
	}
	clockInitial, err := durationFromMillis(state.ClockInitial)
	if err != nil {
		return nil, err
	}
	clockIncrement, err := durationFromMillis(state.ClockIncrement)
	if err != nil {
		return nil, err
	}
	whiteTime, err := durationFromMillis(state.WhiteTime)
	if err != nil {
		return nil, err
	}
	blackTime, err := durationFromMillis(state.BlackTime)
	if err != nil {
		return nil, err
	}
	if clockInitial == 0 {
		if clockIncrement != 0 || whiteTime != 0 || blackTime != 0 || state.ClockRunning {
			return nil, ErrInvalidClockConfig
		}
	} else if whiteTime > clockInitial || blackTime > clockInitial || clockIncrement > clockInitial {
		return nil, ErrInvalidClockConfig
	}
	match := newMatch(state.MatchID, initial, ClockConfig{Initial: clockInitial, Increment: clockIncrement})
	match.sequence = state.Sequence
	match.moves = make([]chess.Move, len(state.Moves))
	replayed := initial
	for index, value := range state.Moves {
		move, err := chess.ParseUCI(value)
		if err != nil {
			return nil, fmt.Errorf("invalid match move %q: %w", value, err)
		}
		match.moves[index] = move
		replayed, err = replayed.Apply(move)
		if err != nil {
			return nil, fmt.Errorf("invalid match move %q: %w", value, err)
		}
	}
	if replayed.FEN() != position.FEN() {
		return nil, errors.New("match move history does not produce FEN")
	}
	match.position = replayed
	match.players[chess.White] = state.WhitePlayer
	match.players[chess.Black] = state.BlackPlayer
	for _, playerID := range state.Spectators {
		match.spectators[playerID] = struct{}{}
	}
	if state.Result != "*" {
		match.result = state.Result
	}
	match.drawOfferedBy = state.DrawOfferedBy
	if match.clock != nil && match.clock.initial > 0 {
		match.clock.remaining = [2]time.Duration{whiteTime, blackTime}
		match.clock.running = state.ClockRunning && match.result == ""
		match.clock.startedAt = state.ClockStartedAt
		if match.clock.running && match.clock.startedAt.IsZero() {
			match.clock.startedAt = match.clockNow()
		}
	}
	if position.FEN() != state.FEN {
		return nil, errors.New("match position does not match FEN")
	}
	return match, nil
}

// ExportState returns all matches in deterministic ID order.
func (s *Server) ExportState() ([]MatchState, error) {
	s.mu.RLock()
	matches := make([]*Match, 0, len(s.matches))
	for _, match := range s.matches {
		matches = append(matches, match)
	}
	s.mu.RUnlock()
	states := make([]MatchState, 0, len(matches))
	for _, match := range matches {
		state, err := match.State()
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool { return states[i].MatchID < states[j].MatchID })
	return states, nil
}

// RestoreState atomically replaces registered matches from durable states.
// Restored player sessions start disconnected and retain their seats.
func (s *Server) RestoreState(states []MatchState) error {
	matches := make(map[string]*Match, len(states))
	for _, state := range states {
		if _, exists := matches[state.MatchID]; exists {
			return errors.New("duplicate match state ID")
		}
		match, err := NewMatchFromState(state)
		if err != nil {
			return err
		}
		matches[state.MatchID] = match
	}
	s.mu.Lock()
	s.ensureMapsLocked()
	s.matches = matches
	for _, state := range states {
		for _, playerID := range []string{state.WhitePlayer, state.BlackPlayer} {
			if playerID == "" {
				continue
			}
			session := s.sessions[playerID]
			if session == nil {
				session = &Session{ID: playerID}
				s.sessions[playerID] = session
			}
			session.MatchID = state.MatchID
		}
	}
	for playerID, session := range s.sessions {
		if session.MatchID == "" {
			continue
		}
		if _, ok := matches[session.MatchID]; !ok {
			session.MatchID = ""
			delete(s.sessions, playerID)
		}
	}
	s.mu.Unlock()
	return nil
}

// SetPersistenceHook registers an optional callback invoked after every
// server-side match mutation. Passing nil disables automatic persistence.
func (s *Server) SetPersistenceHook(hook func([]MatchState) error) {
	s.mu.Lock()
	s.persistence = hook
	s.mu.Unlock()
}

func (s *Server) persist() error {
	s.mu.RLock()
	hook := s.persistence
	s.mu.RUnlock()
	if hook == nil {
		return nil
	}
	states, err := s.ExportState()
	if err != nil {
		return err
	}
	return hook(states)
}

func moveStrings(moves []chess.Move) []string {
	if len(moves) == 0 {
		return nil
	}
	result := make([]string, len(moves))
	for index, move := range moves {
		result[index] = move.UCI()
	}
	return result
}

func validResult(result string) bool {
	switch result {
	case "*", "1-0", "0-1", "1/2-1/2":
		return true
	default:
		return false
	}
}
