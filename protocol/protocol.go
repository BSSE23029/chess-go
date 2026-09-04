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
	case CreateMatch, JoinMatch, Snapshot, Move, MoveAcceptedType, Resign, DrawOffer, ProtocolError:
		return true
	default:
		return false
	}
}

// CreateMatchRequest asks the server to create a match from an optional FEN.
type CreateMatchRequest struct {
	MatchID    string `json:"match_id"`
	InitialFEN string `json:"initial_fen,omitempty"`
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
	MatchID      string `json:"match_id"`
	Sequence     uint64 `json:"sequence"`
	PositionHash uint64 `json:"position_hash"`
	FEN          string `json:"fen"`
	UCI          string `json:"uci"`
}

// MatchSnapshot is a synchronizable authoritative position snapshot.
type MatchSnapshot struct {
	MatchID      string `json:"match_id"`
	Sequence     uint64 `json:"sequence"`
	PositionHash uint64 `json:"position_hash"`
	FEN          string `json:"fen"`
	Turn         string `json:"turn"`
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
)

// Match is an authoritative in-memory chess match.
type Match struct {
	mu       sync.RWMutex
	id       string
	position chess.Position
	sequence uint64
	players  [2]string
}

// NewMatch creates a match from position with sequence zero.
func NewMatch(id string, position chess.Position) *Match {
	return &Match{id: id, position: position}
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
	m.players[color] = playerID
	return nil
}

// Snapshot returns the current authoritative state.
func (m *Match) Snapshot() MatchSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	turn := "white"
	if m.position.Turn() == chess.Black {
		turn = "black"
	}
	return MatchSnapshot{MatchID: m.id, Sequence: m.sequence, PositionHash: m.position.Hash(), FEN: m.position.FEN(), Turn: turn}
}

// ApplyMove validates synchronization, authorization, and chess legality before committing.
func (m *Match) ApplyMove(request MoveRequest) (MoveAccepted, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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
	return MoveAccepted{MatchID: m.id, Sequence: m.sequence, PositionHash: next.Hash(), FEN: next.FEN(), UCI: move.UCI()}, nil
}
