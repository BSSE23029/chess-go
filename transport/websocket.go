package transport

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"chess-go/protocol"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

var (
	errWebSocketProtocol    = errors.New("invalid websocket frame")
	errWebSocketUnsupported = errors.New("unsupported websocket frame")
)

// WebSocketServer upgrades HTTP requests and forwards JSON text or protobuf
// binary messages to a protocol.Server. A player_id query parameter establishes
// a reconnectable session for the lifetime of the connection.
type WebSocketServer struct {
	MatchServer     *protocol.Server
	Token           string
	MaxMessageBytes int
}

// NewWebSocketServer creates a WebSocket adapter over server.
func NewWebSocketServer(server *protocol.Server, token string) *WebSocketServer {
	if server == nil {
		server = protocol.NewServer()
	}
	return &WebSocketServer{MatchServer: server, Token: token, MaxMessageBytes: maxMessageBytes}
}

// ServeHTTP performs an RFC 6455 HTTP upgrade and serves envelopes until the
// peer closes or sends a protocol error.
func (s *WebSocketServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !authorizeRequest(request, s.Token) {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	if request.Method != http.MethodGet || !headerHasToken(request.Header.Get("Connection"), "upgrade") || !strings.EqualFold(request.Header.Get("Upgrade"), "websocket") {
		http.Error(writer, "websocket upgrade required", http.StatusBadRequest)
		return
	}
	if request.Header.Get("Sec-WebSocket-Version") != "13" {
		writer.Header().Set("Sec-WebSocket-Version", "13")
		http.Error(writer, "unsupported websocket version", http.StatusBadRequest)
		return
	}
	key := strings.TrimSpace(request.Header.Get("Sec-WebSocket-Key"))
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(decoded) != 16 {
		http.Error(writer, "invalid websocket key", http.StatusBadRequest)
		return
	}
	playerID := request.URL.Query().Get("player_id")
	if playerID != "" {
		if _, err := s.MatchServer.Connect(playerID); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		defer s.MatchServer.Disconnect(playerID)
	}
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		http.Error(writer, "websocket hijacking is unavailable", http.StatusNotImplemented)
		return
	}
	connection, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer connection.Close()
	accept := sha1.Sum([]byte(key + websocketGUID))
	if _, err := buffered.WriteString("HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(accept[:]) + "\r\n\r\n"); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}

	peer := websocketPeer{reader: buffered.Reader, writer: buffered.Writer, max: s.maxMessageBytes()}
	for {
		opcode, payload, err := peer.readFrame()
		if err != nil {
			if errors.Is(err, errWebSocketProtocol) || errors.Is(err, errWebSocketUnsupported) {
				_ = peer.writeClose(1002, err.Error())
			}
			return
		}
		switch opcode {
		case 0x8:
			_ = peer.writeFrame(0x8, payload)
			return
		case 0x9:
			if err := peer.writeFrame(0xA, payload); err != nil {
				return
			}
		case 0xA:
			continue
		case 0x1:
			response, err := s.MatchServer.Handle(payload)
			if err != nil {
				response, _ = protocol.Encode(protocol.ProtocolError, "", protocol.ProtocolErrorBody{Code: "invalid_request", Message: err.Error()})
			}
			if err := peer.writeFrame(0x1, response); err != nil {
				return
			}
		case 0x2:
			response, err := s.MatchServer.HandleProto(payload)
			if err != nil {
				response, _ = protocol.EncodeProto(protocol.ProtocolError, "", protocol.ProtocolErrorBody{Code: "invalid_request", Message: err.Error()})
			}
			if err := peer.writeFrame(0x2, response); err != nil {
				return
			}
		}
	}
}

func (s *WebSocketServer) maxMessageBytes() int {
	if s.MaxMessageBytes <= 0 {
		return maxMessageBytes
	}
	return s.MaxMessageBytes
}

type websocketPeer struct {
	reader *bufio.Reader
	writer *bufio.Writer
	max    int
	mu     sync.Mutex
}

func (p *websocketPeer) readFrame() (byte, []byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(p.reader, header[:]); err != nil {
		return 0, nil, err
	}
	if header[0]&0x70 != 0 {
		return 0, nil, errWebSocketProtocol
	}
	fin := header[0]&0x80 != 0
	opcode := header[0] & 0x0f
	if !fin {
		return 0, nil, errWebSocketUnsupported
	}
	if opcode != 0x1 && opcode != 0x2 && opcode != 0x8 && opcode != 0x9 && opcode != 0xA {
		return 0, nil, errWebSocketUnsupported
	}
	masked := header[1]&0x80 != 0
	if !masked {
		return 0, nil, errWebSocketProtocol
	}
	length := uint64(header[1] & 0x7f)
	if length == 126 {
		var extended [2]byte
		if _, err := io.ReadFull(p.reader, extended[:]); err != nil {
			return 0, nil, err
		}
		length = uint64(extended[0])<<8 | uint64(extended[1])
	} else if length == 127 {
		var extended [8]byte
		if _, err := io.ReadFull(p.reader, extended[:]); err != nil {
			return 0, nil, err
		}
		if extended[0]&0x80 != 0 {
			return 0, nil, errWebSocketProtocol
		}
		for _, value := range extended {
			length = length<<8 | uint64(value)
		}
	}
	if (opcode == 0x8 || opcode == 0x9 || opcode == 0xA) && length > 125 {
		return 0, nil, errWebSocketProtocol
	}
	if length > uint64(p.max) {
		return 0, nil, fmt.Errorf("websocket message exceeds %d bytes", p.max)
	}
	var mask [4]byte
	if _, err := io.ReadFull(p.reader, mask[:]); err != nil {
		return 0, nil, err
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(p.reader, payload); err != nil {
		return 0, nil, err
	}
	for index := range payload {
		payload[index] ^= mask[index%4]
	}
	if opcode == 0x8 && len(payload) == 1 {
		return 0, nil, errWebSocketProtocol
	}
	return opcode, payload, nil
}

func (p *websocketPeer) writeFrame(opcode byte, payload []byte) error {
	if len(payload) > p.max {
		return fmt.Errorf("websocket message exceeds %d bytes", p.max)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	header := []byte{0x80 | opcode}
	switch {
	case len(payload) < 126:
		header = append(header, byte(len(payload)))
	case len(payload) <= 65535:
		header = append(header, 126, byte(len(payload)>>8), byte(len(payload)))
	default:
		header = append(header, 127, 0, 0, 0, 0, 0, 0, 0, 0)
		length := uint64(len(payload))
		for index := 0; index < 8; index++ {
			header[2+index] = byte(length >> uint(56-8*index))
		}
	}
	if _, err := p.writer.Write(header); err != nil {
		return err
	}
	if _, err := p.writer.Write(payload); err != nil {
		return err
	}
	return p.writer.Flush()
}

func (p *websocketPeer) writeClose(code uint16, reason string) error {
	if len(reason) > 123 {
		reason = reason[:123]
	}
	payload := append([]byte{byte(code >> 8), byte(code)}, []byte(reason)...)
	return p.writeFrame(0x8, payload)
}

func headerHasToken(value, wanted string) bool {
	for _, token := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(token), wanted) {
			return true
		}
	}
	return false
}
