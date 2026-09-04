// Package protocol defines the versioned JSON contract and authoritative match
// state used by network transports.
package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"chess-go"
)

// Version is the current wire-protocol version.
const Version = 1

// MessageType identifies an envelope payload.
type MessageType string

const (
	// CreateMatch requests a new match.
	CreateMatch MessageType = "match.create"
	// Matchmake requests an open seat or a newly created waiting match.
	Matchmake MessageType = "match.matchmake"
	// JoinMatch requests a player seat.
	JoinMatch MessageType = "match.join"
	// Snapshot reports authoritative state.
	Snapshot MessageType = "match.snapshot"
	// Move submits a legal move request.
	Move MessageType = "match.move"
	// MoveAcceptedType acknowledges a committed move.
	MoveAcceptedType MessageType = "match.move.accepted"
	// Resign reports resignation.
	Resign MessageType = "match.resign"
	// DrawOffer reports a draw offer.
	DrawOffer MessageType = "match.draw.offer"
	// ProtocolError reports a request failure.
	ProtocolError MessageType = "error"
)

// Envelope is the versioned outer message shared by all transports.
type Envelope struct {
	Version   int             `json:"version"`
	Type      MessageType     `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

// Encode creates a version-one envelope containing payload.
func Encode(messageType MessageType, requestID string, payload any) ([]byte, error) {
	if messageType == "" {
		return nil, errors.New("protocol message type is required")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{Version: Version, Type: messageType, RequestID: requestID, Payload: data})
}

// Decode validates a version-one envelope and rejects unknown fields.
func Decode(data []byte) (Envelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope Envelope
	if err := decoder.Decode(&envelope); err != nil {
		return Envelope{}, fmt.Errorf("invalid protocol envelope: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Envelope{}, errors.New("invalid protocol envelope: trailing data")
	}
	if envelope.Version != Version {
		return Envelope{}, fmt.Errorf("unsupported protocol version %d", envelope.Version)
	}
	if !knownMessageType(envelope.Type) || len(envelope.Payload) == 0 || bytes.Equal(envelope.Payload, []byte("null")) {
		return Envelope{}, errors.New("protocol envelope requires type and payload")
	}
	return envelope, nil
}

// UnmarshalPayload decodes an envelope payload with strict field checking.
func (e Envelope) UnmarshalPayload(target any) error {
	decoder := json.NewDecoder(bytes.NewReader(e.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("invalid protocol payload: trailing data")
	}
	return nil
}

func knownMessageType(messageType MessageType) bool {
	switch messageType {
	case CreateMatch, Matchmake, JoinMatch, Snapshot, Move, MoveAcceptedType, Resign, DrawOffer, ProtocolError:
		return true
	default:
		return false
	}
}

// CreateMatchRequest asks the server to create a match from an optional FEN.
type CreateMatchRequest struct {
	MatchID         string `json:"match_id"`
	InitialFEN      string `json:"initial_fen,omitempty"`
	PlayerID        string `json:"player_id,omitempty"`
	Color           string `json:"color,omitempty"`
	ClockMillis     int64  `json:"clock_millis,omitempty"`
	IncrementMillis int64  `json:"increment_millis,omitempty"`
}

// MatchmakeRequest asks the server to pair a player with a compatible open
// match, creating a waiting match when none is available. Color may be empty,
// white, or black; an empty value prefers the first compatible seat.
type MatchmakeRequest struct {
	PlayerID        string `json:"player_id"`
	Color           string `json:"color,omitempty"`
	ClockMillis     int64  `json:"clock_millis,omitempty"`
	IncrementMillis int64  `json:"increment_millis,omitempty"`
}

// JoinMatchRequest claims a color seat in a match.
type JoinMatchRequest struct {
	MatchID  string `json:"match_id"`
	PlayerID string `json:"player_id"`
	Color    string `json:"color"`
}

// MoveRequest submits one move against an expected sequence and position hash.
type MoveRequest struct {
	MatchID      string `json:"match_id"`
	PlayerID     string `json:"player_id"`
	Sequence     uint64 `json:"sequence"`
	PositionHash uint64 `json:"position_hash"`
	UCI          string `json:"uci"`
}

// MoveAccepted is the authoritative state after a committed move.
type MoveAccepted struct {
	MatchID         string `json:"match_id"`
	Sequence        uint64 `json:"sequence"`
	PositionHash    uint64 `json:"position_hash"`
	FEN             string `json:"fen"`
	UCI             string `json:"uci"`
	Result          string `json:"result"`
	WhiteTimeMillis int64  `json:"white_time_millis,omitempty"`
	BlackTimeMillis int64  `json:"black_time_millis,omitempty"`
	IncrementMillis int64  `json:"increment_millis,omitempty"`
	ClockRunning    bool   `json:"clock_running,omitempty"`
}

// SnapshotRequest requests the latest authoritative state for a match.
type SnapshotRequest struct {
	MatchID  string `json:"match_id"`
	PlayerID string `json:"player_id,omitempty"`
}

// ResignRequest resigns the requesting player's seat.
type ResignRequest struct {
	MatchID  string `json:"match_id"`
	PlayerID string `json:"player_id"`
}

// DrawOfferRequest offers a draw or accepts an offer from the opposing player.
type DrawOfferRequest struct {
	MatchID  string `json:"match_id"`
	PlayerID string `json:"player_id"`
}

// MatchSnapshot is a synchronizable authoritative position snapshot.
type MatchSnapshot struct {
	MatchID         string   `json:"match_id"`
	Sequence        uint64   `json:"sequence"`
	PositionHash    uint64   `json:"position_hash"`
	FEN             string   `json:"fen"`
	Turn            string   `json:"turn"`
	Result          string   `json:"result"`
	DrawOfferedBy   string   `json:"draw_offered_by,omitempty"`
	Spectators      int      `json:"spectators"`
	WhiteTimeMillis int64    `json:"white_time_millis,omitempty"`
	BlackTimeMillis int64    `json:"black_time_millis,omitempty"`
	IncrementMillis int64    `json:"increment_millis,omitempty"`
	ClockRunning    bool     `json:"clock_running,omitempty"`
	Moves           []string `json:"moves,omitempty"`
}

// ProtocolErrorBody describes a stable machine-readable protocol failure.
type ProtocolErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var (
	// ErrSequenceConflict indicates a stale request sequence.
	ErrSequenceConflict = errors.New("match sequence conflict")
	// ErrPositionMismatch indicates a stale position hash.
	ErrPositionMismatch = errors.New("match position hash mismatch")
	// ErrUnauthorized indicates that a player does not own the side to move.
	ErrUnauthorized = errors.New("player is not authorized for this move")
	// ErrSeatTaken indicates that a requested color is already assigned.
	ErrSeatTaken = errors.New("match seat is already taken")
	// ErrMatchOver indicates that a match already has a terminal outcome.
	ErrMatchOver = errors.New("match is already over")
)

// Match is an authoritative in-memory chess match.
type Match struct {
	mu            sync.RWMutex
	id            string
	position      chess.Position
	sequence      uint64
	players       [2]string
	spectators    map[string]struct{}
	result        string
	drawOfferedBy string
	clock         *matchClock
	moves         []chess.Move
	initialFEN    string
}

// NewMatch creates a match from position with sequence zero.
func NewMatch(id string, position chess.Position) *Match {
	return newMatch(id, position, ClockConfig{})
}

// Join assigns playerID to color.
func (m *Match) Join(playerID string, color chess.Color) error {
	if playerID == "" || color > chess.Black {
		return ErrUnauthorized
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.players[color] != "" && m.players[color] != playerID {
		return ErrSeatTaken
	}
	for other, player := range m.players {
		if player == playerID && chess.Color(other) != color {
			return ErrSeatTaken
		}
	}
	m.players[color] = playerID
	delete(m.spectators, playerID)
	m.startClockLocked(m.clockNow())
	return nil
}

// JoinSpectator attaches playerID without claiming a playing seat.
func (m *Match) JoinSpectator(playerID string) error {
	if playerID == "" {
		return ErrUnauthorized
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.playerColor(playerID); ok {
		return ErrSeatTaken
	}
	if m.spectators == nil {
		m.spectators = make(map[string]struct{})
	}
	m.spectators[playerID] = struct{}{}
	return nil
}

func (m *Match) playerColor(playerID string) (chess.Color, bool) {
	for color, player := range m.players {
		if player == playerID {
			return chess.Color(color), true
		}
	}
	return chess.White, false
}

// Resign ends the match in favor of the opposing player.
func (m *Match) Resign(playerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncClockLocked(m.clockNow())
	if m.clock != nil && m.clock.expired {
		return ErrTimeExpired
	}
	if m.result != "" {
		return ErrMatchOver
	}
	color, ok := m.playerColor(playerID)
	if !ok {
		return ErrUnauthorized
	}
	if color == chess.White {
		m.result = "0-1"
	} else {
		m.result = "1-0"
	}
	return nil
}

// OfferDraw records a draw offer or accepts the existing offer.
func (m *Match) OfferDraw(playerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncClockLocked(m.clockNow())
	if m.clock != nil && m.clock.expired {
		return ErrTimeExpired
	}
	if m.result != "" {
		return ErrMatchOver
	}
	if _, ok := m.playerColor(playerID); !ok {
		return ErrUnauthorized
	}
	if m.drawOfferedBy != "" && m.drawOfferedBy != playerID {
		m.result = "1/2-1/2"
		return nil
	}
	m.drawOfferedBy = playerID
	return nil
}

// Snapshot returns the current authoritative state.
func (m *Match) Snapshot() MatchSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncClockLocked(m.clockNow())
	turn := "white"
	if m.position.Turn() == chess.Black {
		turn = "black"
	}
	result := m.result
	if result == "" {
		result = "*"
	}
	clock := m.clockSnapshotLocked()
	return MatchSnapshot{
		MatchID:         m.id,
		Sequence:        m.sequence,
		PositionHash:    m.position.Hash(),
		FEN:             m.position.FEN(),
		Turn:            turn,
		Result:          result,
		DrawOfferedBy:   m.drawOfferedBy,
		Spectators:      len(m.spectators),
		WhiteTimeMillis: clock.white,
		BlackTimeMillis: clock.black,
		IncrementMillis: clock.increment,
		ClockRunning:    clock.running,
		Moves:           moveStrings(m.moves),
	}
}

// ApplyMove validates synchronization, authorization, and chess legality before committing.
func (m *Match) ApplyMove(request MoveRequest) (MoveAccepted, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.clockNow()
	m.syncClockLocked(now)
	if m.clock != nil && m.clock.expired {
		return MoveAccepted{}, ErrTimeExpired
	}
	if request.PlayerID == "" {
		return MoveAccepted{}, ErrUnauthorized
	}
	if request.MatchID != m.id {
		return MoveAccepted{}, ErrUnauthorized
	}
	if request.Sequence != m.sequence {
		return MoveAccepted{}, ErrSequenceConflict
	}
	if request.PositionHash != m.position.Hash() {
		return MoveAccepted{}, ErrPositionMismatch
	}
	if m.result != "" {
		return MoveAccepted{}, ErrMatchOver
	}
	mover := m.position.Turn()
	if m.players[m.position.Turn()] != "" && m.players[m.position.Turn()] != request.PlayerID {
		return MoveAccepted{}, ErrUnauthorized
	}
	move, err := chess.ParseUCI(request.UCI)
	if err != nil {
		return MoveAccepted{}, err
	}
	next, err := m.position.Apply(move)
	if err != nil {
		return MoveAccepted{}, err
	}
	m.position, m.sequence = next, m.sequence+1
	m.moves = append(m.moves, move)
	m.drawOfferedBy = ""
	if actual := positionResult(next); actual != "" {
		m.result = actual
	}
	m.finishMoveClockLocked(mover, now)
	result := m.result
	if result == "" {
		result = "*"
	}
	clock := m.clockSnapshotLocked()
	return MoveAccepted{
		MatchID:         m.id,
		Sequence:        m.sequence,
		PositionHash:    next.Hash(),
		FEN:             next.FEN(),
		UCI:             move.UCI(),
		Result:          result,
		WhiteTimeMillis: clock.white,
		BlackTimeMillis: clock.black,
		IncrementMillis: clock.increment,
		ClockRunning:    clock.running,
	}, nil
}

func positionResult(position chess.Position) string {
	if len(position.LegalMoves()) != 0 {
		return ""
	}
	if position.InCheck() {
		if position.Turn() == chess.White {
			return "0-1"
		}
		return "1-0"
	}
	return "1/2-1/2"
}
