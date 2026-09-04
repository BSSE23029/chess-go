// Package storage persists authoritative chess match state as versioned JSON.
package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chess-go/protocol"
)

const version = 1

// FileStore atomically persists match state at Path.
type FileStore struct {
	Path string
}

type document struct {
	Version int                   `json:"version"`
	SavedAt time.Time             `json:"saved_at"`
	Matches []protocol.MatchState `json:"matches"`
}

// NewFileStore validates a file path and creates a store descriptor.
func NewFileStore(path string) (*FileStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("match store path is required")
	}
	return &FileStore{Path: path}, nil
}

// NewFileStoreFromEnv reads CHESS_MATCH_STORE; a missing value disables
// persistence for local development.
func NewFileStoreFromEnv() (*FileStore, error) {
	path := os.Getenv("CHESS_MATCH_STORE")
	if path == "" {
		return nil, nil
	}
	return NewFileStore(path)
}

// SaveServer exports and atomically writes all server matches.
func (f *FileStore) SaveServer(server *protocol.Server) error {
	if f == nil || server == nil {
		return errors.New("match store and server are required")
	}
	states, err := server.ExportState()
	if err != nil {
		return err
	}
	return f.SaveStates(states)
}

// SaveStates atomically writes the supplied durable states.
func (f *FileStore) SaveStates(states []protocol.MatchState) error {
	if f == nil || strings.TrimSpace(f.Path) == "" {
		return errors.New("match store path is required")
	}
	data, err := json.MarshalIndent(document{Version: version, SavedAt: time.Now().UTC(), Matches: states}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	directory := filepath.Dir(f.Path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".chess-match-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, f.Path); err != nil {
		return err
	}
	removeTemporary = false
	return nil
}

// Load reads, validates, and restores all matches into server.
func (f *FileStore) Load(server *protocol.Server) error {
	if f == nil || server == nil || strings.TrimSpace(f.Path) == "" {
		return errors.New("match store and server are required")
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var saved document
	if err := decoder.Decode(&saved); err != nil {
		return fmt.Errorf("invalid match store: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("invalid match store: trailing data")
	}
	if saved.Version != version {
		return fmt.Errorf("unsupported match store version %d", saved.Version)
	}
	return server.RestoreState(saved.Matches)
}
