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

func TestMatchmakingPairsOpenSeatsAndCreatesWaitingMatches(t *testing.T) {
	server := NewServer()
	first, err := server.Matchmake(MatchmakeRequest{PlayerID: "alice"})
	if err != nil || first.MatchID != "match-1" || first.Spectators != 0 {
		t.Fatalf("first matchmaking result = %#v, %v", first, err)
	}
	second, err := server.Matchmake(MatchmakeRequest{PlayerID: "bob"})
	if err != nil || second.MatchID != first.MatchID {
		t.Fatalf("second matchmaking result = %#v, %v", second, err)
	}
	third, err := server.Matchmake(MatchmakeRequest{PlayerID: "carol", Color: "black"})
	if err != nil || third.MatchID != "match-2" {
		t.Fatalf("third matchmaking result = %#v, %v", third, err)
	}
	reconnected, err := server.Matchmake(MatchmakeRequest{PlayerID: "alice"})
	if err != nil || reconnected.MatchID != first.MatchID {
		t.Fatalf("existing matchmaking session = %#v, %v", reconnected, err)
	}
	if _, err := server.Matchmake(MatchmakeRequest{PlayerID: "bad", Color: "spectator"}); !errors.Is(err, ErrInvalidColor) {
		t.Fatalf("invalid matchmaking color = %v", err)
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

func TestMatchStateRoundTripAndPersistenceHook(t *testing.T) {
	match := NewMatch("state", chess.NewPosition())
	if err := match.Join("alice", chess.White); err != nil {
		t.Fatal(err)
	}
	initial := match.Snapshot()
	if _, err := match.ApplyMove(MoveRequest{MatchID: "state", PlayerID: "alice", Sequence: initial.Sequence, PositionHash: initial.PositionHash, UCI: "e2e4"}); err != nil {
		t.Fatal(err)
	}
	state, err := match.State()
	if err != nil || state.Sequence != 1 || len(state.Moves) != 1 || state.Moves[0] != "e2e4" {
		t.Fatalf("state = %#v, %v", state, err)
	}
	restored, err := NewMatchFromState(state)
	if err != nil || restored.Snapshot().FEN != match.Snapshot().FEN {
		t.Fatalf("restored = %#v, %v", restored.Snapshot(), err)
	}
	if _, err := NewMatchFromState(MatchState{MatchID: "bad", InitialFEN: chess.InitialFEN, FEN: chess.InitialFEN, Sequence: 1}); err == nil {
		t.Fatal("inconsistent state accepted")
	}

	server := NewServer()
	saves := 0
	server.SetPersistenceHook(func(states []MatchState) error {
		saves++
		if len(states) != 1 || states[0].MatchID != "hook" {
			return errors.New("unexpected persisted states")
		}
		return nil
	})
	if _, err := server.Create(CreateMatchRequest{MatchID: "hook"}); err != nil {
		t.Fatal(err)
	}
	if saves != 1 {
		t.Fatalf("create persistence calls = %d", saves)
	}
}
