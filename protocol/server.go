package protocol

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"chess-go"
)

var (
	// ErrInvalidRequest indicates missing or unsupported request fields.
	ErrInvalidRequest = errors.New("invalid match request")
	// ErrMatchExists indicates that a match ID is already registered.
	ErrMatchExists = errors.New("match already exists")
	// ErrMatchNotFound indicates that a match ID is not registered.
	ErrMatchNotFound = errors.New("match not found")
	// ErrSessionNotConnected indicates that a player must connect or reconnect.
	ErrSessionNotConnected = errors.New("player session is not connected")
	// ErrInvalidColor indicates that a requested seat is not white, black, or spectator.
	ErrInvalidColor = errors.New("invalid match color")
)

// Session identifies a connected player and the match they last joined.
// Disconnecting preserves MatchID, allowing the same player to reconnect
// without releasing their seat.
type Session struct {
	ID        string
	Connected bool
	MatchID   string
}

// Server owns authoritative matches and lightweight player sessions. It is
// transport-independent: HTTP, WebSocket, and tests can all call Handle with
// the same versioned JSON envelopes.
type Server struct {
	mu          sync.RWMutex
	matches     map[string]*Match
	sessions    map[string]*Session
	persistence func([]MatchState) error
}

// NewServer creates an empty authoritative match server.
func NewServer() *Server {
	return &Server{matches: make(map[string]*Match), sessions: make(map[string]*Session)}
}

// Connect creates or reactivates a player session. Existing match membership
// is retained across disconnect/reconnect cycles.
func (s *Server) Connect(playerID string) (Session, error) {
	if blank(playerID) {
		return Session{}, ErrInvalidRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureMapsLocked()
	session := s.sessions[playerID]
	if session == nil {
		session = &Session{ID: playerID}
		s.sessions[playerID] = session
	}
	session.Connected = true
	return *session, nil
}

// Disconnect deactivates a session without releasing its match seat.
func (s *Server) Disconnect(playerID string) error {
	if blank(playerID) {
		return ErrInvalidRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[playerID]
	if !ok {
		return ErrSessionNotConnected
	}
	session.Connected = false
	return nil
}

// Session returns a copy of the current session state.
func (s *Server) Session(playerID string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[playerID]
	if !ok {
		return Session{}, false
	}
	return *session, true
}

// Create registers a match. If PlayerID is provided, that player is connected
// and joined to Color (defaulting to white); Color may also be spectator.
func (s *Server) Create(request CreateMatchRequest) (MatchSnapshot, error) {
	if blank(request.MatchID) {
		return MatchSnapshot{}, ErrInvalidRequest
	}
	clockInitial, err := durationFromMillis(request.ClockMillis)
	if err != nil {
		return MatchSnapshot{}, ErrInvalidClockConfig
	}
	clockIncrement, err := durationFromMillis(request.IncrementMillis)
	if err != nil {
		return MatchSnapshot{}, ErrInvalidClockConfig
	}
	if clockInitial == 0 && clockIncrement != 0 {
		return MatchSnapshot{}, ErrInvalidClockConfig
	}
	position := chess.NewPosition()
	if request.InitialFEN != "" {
		parsed, err := chess.ParseFEN(request.InitialFEN)
		if err != nil {
			return MatchSnapshot{}, fmt.Errorf("invalid initial FEN: %w", err)
		}
		position = parsed
	}
	var color chess.Color
	var spectator bool
	if request.PlayerID != "" {
		value := request.Color
		if value == "" {
			value = "white"
		}
		parsed, isSpectator, ok := parseColor(value)
		if !ok {
			return MatchSnapshot{}, ErrInvalidColor
		}
		color, spectator = parsed, isSpectator
	}

	s.mu.Lock()
	s.ensureMapsLocked()
	if _, exists := s.matches[request.MatchID]; exists {
		s.mu.Unlock()
		return MatchSnapshot{}, ErrMatchExists
	}
	match, err := NewMatchWithClock(request.MatchID, position, ClockConfig{Initial: clockInitial, Increment: clockIncrement})
	if err != nil {
		s.mu.Unlock()
		return MatchSnapshot{}, err
	}
	s.matches[request.MatchID] = match
	if request.PlayerID != "" {
		session := s.ensureSessionLocked(request.PlayerID)
		var err error
		if spectator {
			err = match.JoinSpectator(request.PlayerID)
		} else {
			err = match.Join(request.PlayerID, color)
		}
		if err != nil {
			delete(s.matches, request.MatchID)
			s.mu.Unlock()
			return MatchSnapshot{}, err
		}
		session.MatchID = request.MatchID
	}
	s.mu.Unlock()
	snapshot := match.Snapshot()
	if err := s.persist(); err != nil {
		return MatchSnapshot{}, err
	}
	return snapshot, nil
}

// Join connects playerID and claims a white, black, or spectator role.
func (s *Server) Join(request JoinMatchRequest) (MatchSnapshot, error) {
	if blank(request.MatchID) || blank(request.PlayerID) {
		return MatchSnapshot{}, ErrInvalidRequest
	}
	color, spectator, ok := parseColor(request.Color)
	if !ok {
		return MatchSnapshot{}, ErrInvalidColor
	}
	s.mu.Lock()
	match, ok := s.matches[request.MatchID]
	if !ok {
		s.mu.Unlock()
		return MatchSnapshot{}, ErrMatchNotFound
	}
	var err error
	if spectator {
		err = match.JoinSpectator(request.PlayerID)
	} else {
		err = match.Join(request.PlayerID, color)
	}
	if err != nil {
		s.mu.Unlock()
		return MatchSnapshot{}, err
	}
	session := s.ensureSessionLocked(request.PlayerID)
	session.MatchID = request.MatchID
	s.mu.Unlock()
	snapshot := match.Snapshot()
	if err := s.persist(); err != nil {
		return MatchSnapshot{}, err
	}
	return snapshot, nil
}

// Snapshot returns the current state. A non-empty PlayerID must have an active
// session attached to the requested match; empty PlayerID permits public reads.
func (s *Server) Snapshot(request SnapshotRequest) (MatchSnapshot, error) {
	if blank(request.MatchID) {
		return MatchSnapshot{}, ErrInvalidRequest
	}
	match, err := s.matchFor(request.MatchID)
	if err != nil {
		return MatchSnapshot{}, err
	}
	if request.PlayerID != "" {
		if err := s.authorizeSession(request.PlayerID, request.MatchID); err != nil {
			return MatchSnapshot{}, err
		}
	}
	return match.Snapshot(), nil
}

// ApplyMove applies a synchronized legal move from an active player session.
func (s *Server) ApplyMove(request MoveRequest) (MoveAccepted, error) {
	if blank(request.MatchID) || blank(request.PlayerID) {
		return MoveAccepted{}, ErrInvalidRequest
	}
	match, err := s.matchFor(request.MatchID)
	if err != nil {
		return MoveAccepted{}, err
	}
	if err := s.authorizeSession(request.PlayerID, request.MatchID); err != nil {
		return MoveAccepted{}, err
	}
	accepted, err := match.ApplyMove(request)
	if err != nil {
		return MoveAccepted{}, err
	}
	if err := s.persist(); err != nil {
		return MoveAccepted{}, err
	}
	return accepted, nil
}

// Resign ends a match in favor of the opposing player.
func (s *Server) Resign(request ResignRequest) (MatchSnapshot, error) {
	if blank(request.MatchID) || blank(request.PlayerID) {
		return MatchSnapshot{}, ErrInvalidRequest
	}
	match, err := s.matchFor(request.MatchID)
	if err != nil {
		return MatchSnapshot{}, err
	}
	if err := s.authorizeSession(request.PlayerID, request.MatchID); err != nil {
		return MatchSnapshot{}, err
	}
	if err := match.Resign(request.PlayerID); err != nil {
		return MatchSnapshot{}, err
	}
	snapshot := match.Snapshot()
	if err := s.persist(); err != nil {
		return MatchSnapshot{}, err
	}
	return snapshot, nil
}

// OfferDraw offers a draw or accepts an offer from the other player.
func (s *Server) OfferDraw(request DrawOfferRequest) (MatchSnapshot, error) {
	if blank(request.MatchID) || blank(request.PlayerID) {
		return MatchSnapshot{}, ErrInvalidRequest
	}
	match, err := s.matchFor(request.MatchID)
	if err != nil {
		return MatchSnapshot{}, err
	}
	if err := s.authorizeSession(request.PlayerID, request.MatchID); err != nil {
		return MatchSnapshot{}, err
	}
	if err := match.OfferDraw(request.PlayerID); err != nil {
		return MatchSnapshot{}, err
	}
	snapshot := match.Snapshot()
	if err := s.persist(); err != nil {
		return MatchSnapshot{}, err
	}
	return snapshot, nil
}

// ListMatches returns all matches in stable ID order.
func (s *Server) ListMatches() []MatchSnapshot {
	s.mu.RLock()
	ids := make([]string, 0, len(s.matches))
	for id := range s.matches {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	matches := make([]*Match, 0, len(ids))
	for _, id := range ids {
		matches = append(matches, s.matches[id])
	}
	s.mu.RUnlock()
	result := make([]MatchSnapshot, 0, len(matches))
	for _, match := range matches {
		result = append(result, match.Snapshot())
	}
	return result
}

// Handle decodes one request envelope and encodes its authoritative response.
// Malformed envelopes or payloads return an error. Valid requests that fail
// domain validation return a ProtocolError envelope with the original ID.
func (s *Server) Handle(data []byte) ([]byte, error) {
	envelope, err := Decode(data)
	if err != nil {
		return nil, err
	}
	switch envelope.Type {
	case CreateMatch:
		var request CreateMatchRequest
		if err := envelope.UnmarshalPayload(&request); err != nil {
			return nil, fmt.Errorf("invalid create payload: %w", err)
		}
		snapshot, err := s.Create(request)
		if err != nil {
			return s.encodeError(envelope.RequestID, err, "invalid_request")
		}
		return Encode(Snapshot, envelope.RequestID, snapshot)
	case JoinMatch:
		var request JoinMatchRequest
		if err := envelope.UnmarshalPayload(&request); err != nil {
			return nil, fmt.Errorf("invalid join payload: %w", err)
		}
		snapshot, err := s.Join(request)
		if err != nil {
			return s.encodeError(envelope.RequestID, err, "invalid_request")
		}
		return Encode(Snapshot, envelope.RequestID, snapshot)
	case Snapshot:
		var request SnapshotRequest
		if err := envelope.UnmarshalPayload(&request); err != nil {
			return nil, fmt.Errorf("invalid snapshot payload: %w", err)
		}
		snapshot, err := s.Snapshot(request)
		if err != nil {
			return s.encodeError(envelope.RequestID, err, "invalid_request")
		}
		return Encode(Snapshot, envelope.RequestID, snapshot)
	case Move:
		var request MoveRequest
		if err := envelope.UnmarshalPayload(&request); err != nil {
			return nil, fmt.Errorf("invalid move payload: %w", err)
		}
		accepted, err := s.ApplyMove(request)
		if err != nil {
			return s.encodeError(envelope.RequestID, err, moveErrorCode(err))
		}
		return Encode(MoveAcceptedType, envelope.RequestID, accepted)
	case Resign:
		var request ResignRequest
		if err := envelope.UnmarshalPayload(&request); err != nil {
			return nil, fmt.Errorf("invalid resign payload: %w", err)
		}
		snapshot, err := s.Resign(request)
		if err != nil {
			return s.encodeError(envelope.RequestID, err, "invalid_request")
		}
		return Encode(Snapshot, envelope.RequestID, snapshot)
	case DrawOffer:
		var request DrawOfferRequest
		if err := envelope.UnmarshalPayload(&request); err != nil {
			return nil, fmt.Errorf("invalid draw payload: %w", err)
		}
		snapshot, err := s.OfferDraw(request)
		if err != nil {
			return s.encodeError(envelope.RequestID, err, "invalid_request")
		}
		return Encode(Snapshot, envelope.RequestID, snapshot)
	default:
		return s.encodeError(envelope.RequestID, ErrInvalidRequest, "invalid_request")
	}
}

func (s *Server) encodeError(requestID string, err error, fallback string) ([]byte, error) {
	code := fallback
	switch {
	case errors.Is(err, ErrInvalidRequest):
		code = "invalid_request"
	case errors.Is(err, ErrMatchExists):
		code = "match_exists"
	case errors.Is(err, ErrMatchNotFound):
		code = "match_not_found"
	case errors.Is(err, ErrSessionNotConnected):
		code = "session_not_connected"
	case errors.Is(err, ErrInvalidColor):
		code = "invalid_color"
	case errors.Is(err, ErrSequenceConflict):
		code = "sequence_conflict"
	case errors.Is(err, ErrPositionMismatch):
		code = "position_mismatch"
	case errors.Is(err, ErrUnauthorized):
		code = "unauthorized"
	case errors.Is(err, ErrSeatTaken):
		code = "seat_taken"
	case errors.Is(err, ErrMatchOver):
		code = "match_over"
	case errors.Is(err, ErrTimeExpired):
		code = "timeout"
	}
	return Encode(ProtocolError, requestID, ProtocolErrorBody{Code: code, Message: err.Error()})
}

func moveErrorCode(err error) string {
	if errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrMatchNotFound) || errors.Is(err, ErrSessionNotConnected) {
		return "invalid_request"
	}
	return "illegal_move"
}

func (s *Server) matchFor(id string) (*Match, error) {
	s.mu.RLock()
	match, ok := s.matches[id]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrMatchNotFound
	}
	return match, nil
}

func (s *Server) authorizeSession(playerID, matchID string) error {
	s.mu.RLock()
	session, ok := s.sessions[playerID]
	connected := ok && session.Connected
	joinedMatch := ""
	if ok {
		joinedMatch = session.MatchID
	}
	s.mu.RUnlock()
	if !connected {
		return ErrSessionNotConnected
	}
	if joinedMatch != matchID {
		return ErrUnauthorized
	}
	return nil
}

func (s *Server) ensureMapsLocked() {
	if s.matches == nil {
		s.matches = make(map[string]*Match)
	}
	if s.sessions == nil {
		s.sessions = make(map[string]*Session)
	}
}

func (s *Server) ensureSessionLocked(playerID string) *Session {
	session := s.sessions[playerID]
	if session == nil {
		session = &Session{ID: playerID}
		s.sessions[playerID] = session
	}
	session.Connected = true
	return session
}

func parseColor(value string) (chess.Color, bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "white":
		return chess.White, false, true
	case "black":
		return chess.Black, false, true
	case "spectator", "watcher":
		return chess.White, true, true
	default:
		return chess.White, false, false
	}
}

func blank(value string) bool { return strings.TrimSpace(value) == "" }
