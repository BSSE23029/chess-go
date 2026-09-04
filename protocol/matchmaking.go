package protocol

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"chess-go"
)

// Matchmake joins the first compatible open seat in stable match-ID order or
// creates a deterministic waiting match when no seat is available.
func (s *Server) Matchmake(request MatchmakeRequest) (MatchSnapshot, error) {
	if blank(request.PlayerID) {
		return MatchSnapshot{}, ErrInvalidRequest
	}
	preferred, hasPreference, ok := matchmakingColor(request.Color)
	if !ok {
		return MatchSnapshot{}, ErrInvalidColor
	}
	if session, exists := s.Session(request.PlayerID); exists && session.MatchID != "" {
		if _, err := s.Connect(request.PlayerID); err != nil {
			return MatchSnapshot{}, err
		}
		return s.Snapshot(SnapshotRequest{MatchID: session.MatchID, PlayerID: request.PlayerID})
	}
	s.mu.RLock()
	ids := make([]string, 0, len(s.matches))
	for id := range s.matches {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	matches := make(map[string]*Match, len(ids))
	for _, id := range ids {
		matches[id] = s.matches[id]
	}
	s.mu.RUnlock()
	for _, id := range ids {
		match := matches[id]
		color, open := match.openSeat(preferred, hasPreference)
		if !open {
			continue
		}
		seat := "white"
		if color == chess.Black {
			seat = "black"
		}
		snapshot, err := s.Join(JoinMatchRequest{MatchID: id, PlayerID: request.PlayerID, Color: seat})
		if err == nil {
			return snapshot, nil
		}
		if !errors.Is(err, ErrSeatTaken) {
			return MatchSnapshot{}, err
		}
	}
	matchID := s.newMatchID()
	colorNameValue := "white"
	if hasPreference && preferred == chess.Black {
		colorNameValue = "black"
	}
	return s.Create(CreateMatchRequest{MatchID: matchID, PlayerID: request.PlayerID, Color: colorNameValue, ClockMillis: request.ClockMillis, IncrementMillis: request.IncrementMillis})
}

func (s *Server) newMatchID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureMapsLocked()
	for {
		s.nextMatchID++
		id := fmt.Sprintf("match-%d", s.nextMatchID)
		if _, exists := s.matches[id]; !exists {
			return id
		}
	}
}

func (m *Match) openSeat(preferred chess.Color, hasPreference bool) (chess.Color, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if hasPreference {
		return preferred, m.players[preferred] == ""
	}
	if m.players[chess.White] == "" {
		return chess.White, true
	}
	if m.players[chess.Black] == "" {
		return chess.Black, true
	}
	return chess.White, false
}

func matchmakingColor(value string) (chess.Color, bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "random":
		return chess.White, false, true
	case "white":
		return chess.White, true, true
	case "black":
		return chess.Black, true, true
	default:
		return chess.White, false, false
	}
}
