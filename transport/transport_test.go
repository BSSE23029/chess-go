package transport

import (
	"bufio"
	"bytes"
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
