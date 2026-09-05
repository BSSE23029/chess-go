package transport

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chess-go/protocol"
)

func TestDefaultTLSConfigRequiresTLS13(t *testing.T) {
	config := DefaultTLSConfig()
	if config.MinVersion != tls.VersionTLS13 {
		t.Fatalf("minimum TLS version = %d, want TLS 1.3", config.MinVersion)
	}
	client, err := NewClient("https://example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.HTTPClient.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion != tls.VersionTLS13 {
		t.Fatalf("client TLS policy = %#v", transport)
	}
}

func TestHTTPTransportEndpointsAndBearerAuth(t *testing.T) {
	server := protocol.NewServer()
	httpServer := NewHTTPServer(server, "secret")

	request := httptest.NewRequest(http.MethodGet, "http://example.test/v1/matches", nil)
	recorder := httptest.NewRecorder()
	httpServer.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", recorder.Code)
	}

	create, err := protocol.Encode(protocol.CreateMatch, "http-create", protocol.CreateMatchRequest{MatchID: "http-match", PlayerID: "alice", Color: "white"})
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "http://example.test/v1/messages", bytes.NewReader(create))
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	httpServer.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d", recorder.Code)
	}
	envelope, err := protocol.Decode(recorder.Body.Bytes())
	if err != nil || envelope.Type != protocol.Snapshot || envelope.RequestID != "http-create" {
		t.Fatalf("create response = %#v, %v", envelope, err)
	}

	request = httptest.NewRequest(http.MethodGet, "http://example.test/v1/matches", nil)
	request.Header.Set("Authorization", "Bearer secret")
	recorder = httptest.NewRecorder()
	httpServer.ServeHTTP(recorder, request)
	var matches []protocol.MatchSnapshot
	if err := json.NewDecoder(recorder.Body).Decode(&matches); err != nil || recorder.Code != http.StatusOK || len(matches) != 1 || matches[0].MatchID != "http-match" {
		t.Fatalf("list = %#v, status %d, %v", matches, recorder.Code, err)
	}

	request = httptest.NewRequest(http.MethodGet, "http://example.test/v1/matches/http-match?player_id=alice", nil)
	request.Header.Set("Authorization", "Bearer secret")
	recorder = httptest.NewRecorder()
	httpServer.ServeHTTP(recorder, request)
	var snapshot protocol.MatchSnapshot
	if err := json.NewDecoder(recorder.Body).Decode(&snapshot); err != nil || recorder.Code != http.StatusOK || snapshot.MatchID != "http-match" {
		t.Fatalf("snapshot = %#v, status %d, %v", snapshot, recorder.Code, err)
	}
}

func TestHTTPClientTypedLifecycle(t *testing.T) {
	authority := protocol.NewServer()
	adapter := NewHTTPServer(authority, "secret")
	client, err := NewClient("http://example.test/", "secret")
	if err != nil {
		t.Fatal(err)
	}
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		adapter.ServeHTTP(recorder, request)
		return &http.Response{StatusCode: recorder.Code, Header: recorder.Header(), Body: io.NopCloser(bytes.NewReader(recorder.Body.Bytes())), Request: request}, nil
	})}
	snapshot, err := client.Create(context.Background(), "client-create", protocol.CreateMatchRequest{MatchID: "client-match", PlayerID: "alice", Color: "white"})
	if err != nil || snapshot.MatchID != "client-match" {
		t.Fatalf("client create = %#v, %v", snapshot, err)
	}
	matches, err := client.List(context.Background())
	if err != nil || len(matches) != 1 {
		t.Fatalf("client list = %#v, %v", matches, err)
	}
	matched, err := client.Matchmake(context.Background(), "client-matchmake", protocol.MatchmakeRequest{PlayerID: "bob"})
	if err != nil || matched.MatchID != "client-match" {
		t.Fatalf("client matchmaking = %#v, %v", matched, err)
	}
	fetched, err := client.Snapshot(context.Background(), protocol.SnapshotRequest{MatchID: "client-match", PlayerID: "alice"})
	if err != nil || fetched.PositionHash != snapshot.PositionHash {
		t.Fatalf("client snapshot = %#v, %v", fetched, err)
	}
	if _, err := client.Create(context.Background(), "duplicate", protocol.CreateMatchRequest{MatchID: "client-match"}); err == nil || !strings.Contains(err.Error(), "match_exists") {
		t.Fatalf("client domain error = %v", err)
	}
}

func TestHTTPProtobufEnvelopeLifecycle(t *testing.T) {
	authority := protocol.NewServer()
	adapter := NewHTTPServer(authority, "secret")
	client, err := NewClient("https://example.test", "secret")
	if err != nil {
		t.Fatal(err)
	}
	client.Format = WireProtobuf
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		adapter.ServeHTTP(recorder, request)
		return &http.Response{StatusCode: recorder.Code, Header: recorder.Header(), Body: io.NopCloser(bytes.NewReader(recorder.Body.Bytes())), Request: request}, nil
	})}
	snapshot, err := client.Create(context.Background(), "binary-create", protocol.CreateMatchRequest{MatchID: "binary-match", PlayerID: "alice", Color: "white"})
	if err != nil || snapshot.MatchID != "binary-match" {
		t.Fatalf("protobuf create = %#v, %v", snapshot, err)
	}
}

func TestWebSocketTransportUpgradeFramesAndDispatch(t *testing.T) {
	server := protocol.NewServer()
	httpServer := NewWebSocketServer(server, "secret")
	serverConnection, clientConnection := net.Pipe()
	defer clientConnection.Close()
	defer serverConnection.Close()
	serverBuffered := bufio.NewReadWriter(bufio.NewReader(serverConnection), bufio.NewWriter(serverConnection))
	responseWriter := &hijackResponseWriter{header: make(http.Header), connection: serverConnection, buffered: serverBuffered}
	request := httptest.NewRequest(http.MethodGet, "http://example.test/?player_id=alice", nil)
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Connection", "keep-alive, Upgrade")
	request.Header.Set("Sec-WebSocket-Version", "13")
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 16))
	request.Header.Set("Sec-WebSocket-Key", key)
	request.Header.Set("Authorization", "Bearer secret")
	done := make(chan struct{})
	go func() {
		httpServer.ServeHTTP(responseWriter, request)
		close(done)
	}()
	reader := bufio.NewReader(clientConnection)
	status, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(status, "101") {
		t.Fatalf("upgrade status = %q, %v", status, err)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}

	create, err := protocol.Encode(protocol.CreateMatch, "ws-create", protocol.CreateMatchRequest{MatchID: "ws-match", PlayerID: "alice", Color: "white"})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeClientFrame(clientConnection, 0x1, create); err != nil {
		t.Fatal(err)
	}
	opcode, payload, err := readServerFrame(reader)
	if err != nil || opcode != 0x1 {
		t.Fatalf("create frame = %d, %v", opcode, err)
	}
	envelope, err := protocol.Decode(payload)
	if err != nil || envelope.Type != protocol.Snapshot || envelope.RequestID != "ws-create" {
		t.Fatalf("create envelope = %#v, %v", envelope, err)
	}
	binaryCreate, err := protocol.EncodeProto(protocol.CreateMatch, "ws-binary-create", protocol.CreateMatchRequest{MatchID: "ws-binary-match", PlayerID: "alice", Color: "white"})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeClientFrame(clientConnection, 0x2, binaryCreate); err != nil {
		t.Fatal(err)
	}
	opcode, payload, err = readServerFrame(reader)
	if err != nil || opcode != 0x2 {
		t.Fatalf("binary create frame = %d, %v", opcode, err)
	}
	envelope, err = protocol.DecodeProto(payload)
	if err != nil || envelope.Type != protocol.Snapshot || envelope.RequestID != "ws-binary-create" {
		t.Fatalf("binary create envelope = %#v, %v", envelope, err)
	}

	if err := writeClientFrame(clientConnection, 0x9, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	opcode, payload, err = readServerFrame(reader)
	if err != nil || opcode != 0xA || string(payload) != "ping" {
		t.Fatalf("pong frame = %d %q, %v", opcode, payload, err)
	}
	if err := writeClientFrame(clientConnection, 0x8, []byte{0x03, 0xE8}); err != nil {
		t.Fatal(err)
	}
	opcode, _, err = readServerFrame(reader)
	if err != nil || opcode != 0x8 {
		t.Fatalf("close frame = %d, %v", opcode, err)
	}
	<-done
}

func writeClientFrame(writer io.Writer, opcode byte, payload []byte) error {
	header := []byte{0x80 | opcode}
	if len(payload) < 126 {
		header = append(header, 0x80|byte(len(payload)))
	} else if len(payload) <= 65535 {
		header = append(header, 0x80|126, byte(len(payload)>>8), byte(len(payload)))
	} else {
		return fmt.Errorf("test frame too large")
	}
	mask := [4]byte{1, 2, 3, 4}
	data := append([]byte(nil), payload...)
	for index := range data {
		data[index] ^= mask[index%4]
	}
	header = append(header, mask[:]...)
	_, err := writer.Write(append(header, data...))
	return err
}

type hijackResponseWriter struct {
	header     http.Header
	connection net.Conn
	buffered   *bufio.ReadWriter
	status     int
	body       bytes.Buffer
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func (w *hijackResponseWriter) Header() http.Header { return w.header }

func (w *hijackResponseWriter) WriteHeader(status int) { w.status = status }

func (w *hijackResponseWriter) Write(data []byte) (int, error) { return w.body.Write(data) }

func (w *hijackResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.connection, w.buffered, nil
}

func readServerFrame(reader *bufio.Reader) (byte, []byte, error) {
	first, err := reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	second, err := reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	if first&0x80 == 0 || second&0x80 != 0 {
		return 0, nil, fmt.Errorf("unexpected test frame")
	}
	length := int(second & 0x7f)
	if length == 126 {
		high, err := reader.ReadByte()
		if err != nil {
			return 0, nil, err
		}
		low, err := reader.ReadByte()
		if err != nil {
			return 0, nil, err
		}
		length = int(high)<<8 | int(low)
	} else if length == 127 {
		return 0, nil, fmt.Errorf("unexpected large test frame")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return 0, nil, err
	}
	return first & 0x0f, payload, nil
}
