package protocol

import (
	"encoding/json"
	"errors"
	"testing"

	"chess-go"
)

func TestVersionedEnvelopeLifecycle(t *testing.T) {
	payload := MoveRequest{MatchID: "m1", PlayerID: "alice", Sequence: 2, PositionHash: 7, UCI: "e2e4"}
	data, err := Encode(Move, "request-1", payload)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := Decode(data)
	if err != nil || envelope.Version != Version || envelope.Type != Move || envelope.RequestID != "request-1" {
		t.Fatalf("decoded envelope = %#v, %v", envelope, err)
	}
	var decoded MoveRequest
	if err := envelope.UnmarshalPayload(&decoded); err != nil || decoded != payload {
		t.Fatalf("decoded payload = %#v, %v", decoded, err)
	}
	for _, invalid := range []string{
		`{"version":2,"type":"match.move","payload":{}}`,
		`{"version":1,"type":"unknown","payload":{}}`,
		`{"version":1,"type":"match.move","payload":{},"extra":true}`,
		`{"version":1,"type":"match.move","payload":{}} {}`,
		`{"version":1,"type":"match.move"}`,
	} {
		if _, err := Decode([]byte(invalid)); err == nil {
			t.Errorf("Decode(%s) succeeded", invalid)
		}
	}
	if _, err := Encode("", "", nil); err == nil {
		t.Fatal("empty message type accepted")
	}
}

func TestAuthoritativeMatchSynchronizationAndValidation(t *testing.T) {
	match := NewMatch("m1", chess.NewPosition())
	if err := match.Join("alice", chess.White); err != nil {
		t.Fatal(err)
	}
	if err := match.Join("bob", chess.Black); err != nil {
		t.Fatal(err)
	}
	if err := match.Join("mallory", chess.White); !errors.Is(err, ErrSeatTaken) {
		t.Fatalf("seat error = %v", err)
	}
	if err := match.Join("bad", chess.Color(2)); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("invalid color error = %v", err)
	}
	if err := match.Join("alice", chess.Black); !errors.Is(err, ErrSeatTaken) {
		t.Fatalf("duplicate player seat error = %v", err)
	}
	if err := match.JoinSpectator("alice"); !errors.Is(err, ErrSeatTaken) {
		t.Fatalf("player spectator error = %v", err)
	}
	initial := match.Snapshot()
	accepted, err := match.ApplyMove(MoveRequest{MatchID: "m1", PlayerID: "alice", Sequence: initial.Sequence, PositionHash: initial.PositionHash, UCI: "e2e4"})
	if err != nil || accepted.Sequence != 1 || accepted.PositionHash == initial.PositionHash {
		t.Fatalf("accepted move = %#v, %v", accepted, err)
	}
	if _, err := match.ApplyMove(MoveRequest{MatchID: "m1", PlayerID: "bob", Sequence: 0, PositionHash: accepted.PositionHash, UCI: "e7e5"}); !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("sequence error = %v", err)
	}
	current := match.Snapshot()
	if _, err := match.ApplyMove(MoveRequest{MatchID: "m1", PlayerID: "mallory", Sequence: current.Sequence, PositionHash: current.PositionHash, UCI: "e7e5"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("authorization error = %v", err)
	}
	if _, err := match.ApplyMove(MoveRequest{MatchID: "m1", PlayerID: "bob", Sequence: current.Sequence, PositionHash: current.PositionHash + 1, UCI: "e7e5"}); !errors.Is(err, ErrPositionMismatch) {
		t.Fatalf("hash error = %v", err)
	}
	accepted, err = match.ApplyMove(MoveRequest{MatchID: "m1", PlayerID: "bob", Sequence: current.Sequence, PositionHash: current.PositionHash, UCI: "e7e5"})
	if err != nil || accepted.UCI != "e7e5" || match.Snapshot().Turn != "white" {
		t.Fatalf("black move = %#v, %v", accepted, err)
	}
	if _, err := json.Marshal(match.Snapshot()); err != nil {
		t.Fatal(err)
	}
}

func TestAuthoritativeMatchRejectsIllegalMove(t *testing.T) {
	match := NewMatch("m1", chess.NewPosition())
	snapshot := match.Snapshot()
	if _, err := match.ApplyMove(MoveRequest{MatchID: "m1", Sequence: snapshot.Sequence, PositionHash: snapshot.PositionHash, UCI: "e2e5"}); err == nil {
		t.Fatal("illegal move accepted")
	}
	if _, err := match.ApplyMove(MoveRequest{MatchID: "wrong", Sequence: snapshot.Sequence, PositionHash: snapshot.PositionHash, UCI: "e2e4"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong match error = %v", err)
	}
	if _, err := match.ApplyMove(MoveRequest{MatchID: "m1", Sequence: snapshot.Sequence, PositionHash: snapshot.PositionHash, UCI: "e2e4"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("empty player error = %v", err)
	}
}
