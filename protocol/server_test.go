package protocol

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestServerSessionAndMatchLifecycle(t *testing.T) {
	server := NewServer()
	if _, err := server.Connect(""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("empty connect error = %v", err)
	}
	if session, err := server.Create(CreateMatchRequest{MatchID: "game", PlayerID: "alice"}); err != nil || session.Result != "*" {
		t.Fatalf("create = %#v, %v", session, err)
	}
	if _, err := server.Create(CreateMatchRequest{MatchID: "game"}); !errors.Is(err, ErrMatchExists) {
		t.Fatalf("duplicate error = %v", err)
	}
	if _, err := server.Join(JoinMatchRequest{MatchID: "game", PlayerID: "bob", Color: "black"}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.Join(JoinMatchRequest{MatchID: "game", PlayerID: "mallory", Color: "white"}); !errors.Is(err, ErrSeatTaken) {
		t.Fatalf("seat error = %v", err)
	}
	snapshot, err := server.Join(JoinMatchRequest{MatchID: "game", PlayerID: "spectator", Color: "spectator"})
	if err != nil || snapshot.Spectators != 1 {
		t.Fatalf("spectator join = %#v, %v", snapshot, err)
	}
	if matches := server.ListMatches(); len(matches) != 1 || matches[0].MatchID != "game" {
		t.Fatalf("list = %#v", matches)
	}
	if err := server.Disconnect("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := server.ApplyMove(MoveRequest{MatchID: "game", PlayerID: "alice", UCI: "e2e4"}); !errors.Is(err, ErrSessionNotConnected) {
		t.Fatalf("disconnected move error = %v", err)
	}
	if session, err := server.Connect("alice"); err != nil || !session.Connected || session.MatchID != "game" {
		t.Fatalf("reconnect = %#v, %v", session, err)
	}
	current, err := server.Snapshot(SnapshotRequest{MatchID: "game", PlayerID: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := server.ApplyMove(MoveRequest{MatchID: "game", PlayerID: "alice", Sequence: current.Sequence, PositionHash: current.PositionHash, UCI: "e2e4"})
	if err != nil || accepted.Sequence != 1 || accepted.Result != "*" {
		t.Fatalf("move = %#v, %v", accepted, err)
	}
}

func TestServerHandleDispatchAndErrors(t *testing.T) {
	server := NewServer()
	create, err := Encode(CreateMatch, "create-1", CreateMatchRequest{MatchID: "wire", PlayerID: "alice", Color: "white"})
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Handle(create)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := Decode(response)
	if err != nil || envelope.Type != Snapshot || envelope.RequestID != "create-1" {
		t.Fatalf("create response = %#v, %v", envelope, err)
	}
	var snapshot MatchSnapshot
	if err := envelope.UnmarshalPayload(&snapshot); err != nil || snapshot.MatchID != "wire" {
		t.Fatalf("snapshot = %#v, %v", snapshot, err)
	}

	join, err := Encode(JoinMatch, "join-1", JoinMatchRequest{MatchID: "wire", PlayerID: "bob", Color: "black"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.Handle(join); err != nil {
		t.Fatal(err)
	}
	move, err := Encode(Move, "move-1", MoveRequest{MatchID: "wire", PlayerID: "alice", Sequence: snapshot.Sequence, PositionHash: snapshot.PositionHash, UCI: "e2e4"})
	if err != nil {
		t.Fatal(err)
	}
	response, err = server.Handle(move)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err = Decode(response)
	if err != nil || envelope.Type != MoveAcceptedType || envelope.RequestID != "move-1" {
		t.Fatalf("move response = %#v, %v", envelope, err)
	}
	var accepted MoveAccepted
	if err := envelope.UnmarshalPayload(&accepted); err != nil || accepted.Sequence != 1 {
		t.Fatalf("accepted = %#v, %v", accepted, err)
	}

	stale, err := Encode(Move, "stale-1", MoveRequest{MatchID: "wire", PlayerID: "bob", Sequence: 0, PositionHash: accepted.PositionHash, UCI: "e7e5"})
	if err != nil {
		t.Fatal(err)
	}
	response, err = server.Handle(stale)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err = Decode(response)
	if err != nil || envelope.Type != ProtocolError {
		t.Fatalf("stale response = %#v, %v", envelope, err)
	}
	var failure ProtocolErrorBody
	if err := envelope.UnmarshalPayload(&failure); err != nil || failure.Code != "sequence_conflict" {
		t.Fatalf("failure = %#v, %v", failure, err)
	}

	badPayload, err := Encode(Move, "bad-1", map[string]any{"match_id": "wire", "player_id": "bob", "sequence": 1, "position_hash": accepted.PositionHash, "uci": "e7e5", "unknown": true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.Handle(badPayload); err == nil {
		t.Fatal("unknown payload field accepted")
	}
	response, err = server.Handle([]byte(`{"version":1,"type":"match.move","payload":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	envelope, err = Decode(response)
	if err != nil || envelope.Type != ProtocolError {
		t.Fatalf("missing-field response = %#v, %v", envelope, err)
	}
}

func TestServerDrawAndResignLifecycle(t *testing.T) {
	server := NewServer()
	if _, err := server.Create(CreateMatchRequest{MatchID: "draw", PlayerID: "alice", Color: "white"}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.Join(JoinMatchRequest{MatchID: "draw", PlayerID: "bob", Color: "black"}); err != nil {
		t.Fatal(err)
	}
	if snapshot, err := server.OfferDraw(DrawOfferRequest{MatchID: "draw", PlayerID: "alice"}); err != nil || snapshot.DrawOfferedBy != "alice" || snapshot.Result != "*" {
		t.Fatalf("draw offer = %#v, %v", snapshot, err)
	}
	if snapshot, err := server.OfferDraw(DrawOfferRequest{MatchID: "draw", PlayerID: "bob"}); err != nil || snapshot.Result != "1/2-1/2" || snapshot.DrawOfferedBy != "alice" {
		t.Fatalf("draw accept = %#v, %v", snapshot, err)
	}
	if _, err := server.Resign(ResignRequest{MatchID: "draw", PlayerID: "alice"}); !errors.Is(err, ErrMatchOver) {
		t.Fatalf("post-draw resign = %v", err)
	}

	if _, err := server.Create(CreateMatchRequest{MatchID: "resign", PlayerID: "carol", Color: "white"}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.Resign(ResignRequest{MatchID: "resign", PlayerID: "carol"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := server.Snapshot(SnapshotRequest{MatchID: "resign"})
	if err != nil || snapshot.Result != "0-1" {
		t.Fatalf("resignation result = %#v, %v", snapshot, err)
	}
	if data, err := json.Marshal(snapshot); err != nil || len(data) == 0 {
		t.Fatalf("snapshot json = %s, %v", data, err)
	}
}
