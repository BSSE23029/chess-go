package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"chess-go/protocol"
)

func TestFileStoreRoundTripAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matches.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	authority := protocol.NewServer()
	if _, err := authority.Create(protocol.CreateMatchRequest{MatchID: "saved", PlayerID: "alice", Color: "white"}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.Join(protocol.JoinMatchRequest{MatchID: "saved", PlayerID: "bob", Color: "black"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := authority.Snapshot(protocol.SnapshotRequest{MatchID: "saved"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ApplyMove(protocol.MoveRequest{MatchID: "saved", PlayerID: "alice", Sequence: snapshot.Sequence, PositionHash: snapshot.PositionHash, UCI: "e2e4"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveServer(authority); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("store file mode = %v, %v", info.Mode().Perm(), err)
	}

	restored := protocol.NewServer()
	if err := store.Load(restored); err != nil {
		t.Fatal(err)
	}
	if session, err := restored.Connect("alice"); err != nil || session.MatchID != "saved" {
		t.Fatalf("restored session = %#v, %v", session, err)
	}
	restoredSnapshot, err := restored.Snapshot(protocol.SnapshotRequest{MatchID: "saved", PlayerID: "alice"})
	if err != nil || restoredSnapshot.Sequence != 1 || len(restoredSnapshot.Moves) != 1 || restoredSnapshot.Moves[0] != "e2e4" {
		t.Fatalf("restored snapshot = %#v, %v", restoredSnapshot, err)
	}
	if _, err := restored.ApplyMove(protocol.MoveRequest{MatchID: "saved", PlayerID: "alice", Sequence: restoredSnapshot.Sequence, PositionHash: restoredSnapshot.PositionHash, UCI: "e7e5"}); !errors.Is(err, protocol.ErrUnauthorized) {
		// Alice is white; the restored state must retain the side assignment.
		t.Fatalf("restored authorization = %v", err)
	}
	if _, err := restored.Connect("bob"); err != nil {
		t.Fatal(err)
	}
	if _, err := restored.ApplyMove(protocol.MoveRequest{MatchID: "saved", PlayerID: "bob", Sequence: restoredSnapshot.Sequence, PositionHash: restoredSnapshot.PositionHash, UCI: "e7e5"}); err != nil {
		t.Fatal(err)
	}
}

func TestFileStoreRejectsUnknownAndUnsupportedDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matches.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{
		`{"version":2,"saved_at":"2026-01-01T00:00:00Z","matches":[]}`,
		`{"version":1,"saved_at":"2026-01-01T00:00:00Z","matches":[],"unknown":true}`,
		`{"version":1,"saved_at":"2026-01-01T00:00:00Z","matches":[]} {}`,
	} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := store.Load(protocol.NewServer()); err == nil {
			t.Fatalf("document %s accepted", data)
		}
	}
	if _, err := NewFileStore(""); err == nil || !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("empty store path error = %v", err)
	}
}
