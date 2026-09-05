// Package transport provides standard-library HTTP and WebSocket adapters for
// the transport-independent protocol server.
package transport

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"chess-go/protocol"
)

const maxMessageBytes = 1 << 20

const protobufContentType = "application/x-protobuf"

// HTTPServer exposes protocol messages and read-only match resources over HTTP.
// Token is optional; when non-empty requests must include Authorization:
// Bearer <Token>.
type HTTPServer struct {
	MatchServer *protocol.Server
	Token       string
}

// NewHTTPServer creates an HTTP adapter over server.
func NewHTTPServer(server *protocol.Server, token string) *HTTPServer {
	if server == nil {
		server = protocol.NewServer()
	}
	return &HTTPServer{MatchServer: server, Token: token}
}

// NewHTTPServerFromEnv creates an HTTP adapter using CHESS_NETWORK_TOKEN when
// configured. Empty token values intentionally leave local development open.
func NewHTTPServerFromEnv(server *protocol.Server) *HTTPServer {
	return NewHTTPServer(server, os.Getenv("CHESS_NETWORK_TOKEN"))
}

// ServeHTTP routes /v1/messages, /v1/matches, and individual match snapshots.
func (h *HTTPServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !authorizeRequest(request, h.Token) {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch {
	case request.Method == http.MethodPost && request.URL.Path == "/v1/messages":
		h.serveMessage(writer, request)
	case request.Method == http.MethodGet && request.URL.Path == "/v1/matches":
		h.writeJSON(writer, http.StatusOK, h.MatchServer.ListMatches())
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/v1/matches/"):
		h.serveSnapshot(writer, request)
	default:
		http.NotFound(writer, request)
	}
}

func (h *HTTPServer) serveMessage(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()
	body, err := io.ReadAll(io.LimitReader(request.Body, maxMessageBytes+1))
	if err != nil {
		http.Error(writer, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(body) > maxMessageBytes {
		http.Error(writer, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	protobuf := strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), protobufContentType)
	var response []byte
	if protobuf {
		response, err = h.MatchServer.HandleProto(body)
	} else {
		response, err = h.MatchServer.Handle(body)
	}
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if protobuf {
		writer.Header().Set("Content-Type", protobufContentType)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(response)
		return
	}
	h.writeJSON(writer, http.StatusOK, json.RawMessage(response))
}

func (h *HTTPServer) serveSnapshot(writer http.ResponseWriter, request *http.Request) {
	id := strings.TrimPrefix(request.URL.Path, "/v1/matches/")
	if strings.Contains(id, "/") || id == "" {
		http.NotFound(writer, request)
		return
	}
	snapshot, err := h.MatchServer.Snapshot(protocol.SnapshotRequest{MatchID: id, PlayerID: request.URL.Query().Get("player_id")})
	if err != nil {
		http.Error(writer, err.Error(), statusForError(err))
		return
	}
	h.writeJSON(writer, http.StatusOK, snapshot)
}

func (h *HTTPServer) writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func authorizeRequest(request *http.Request, token string) bool {
	if token == "" {
		return true
	}
	const prefix = "Bearer "
	header := request.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	provided, expected := []byte(strings.TrimSpace(strings.TrimPrefix(header, prefix))), []byte(token)
	return len(provided) == len(expected) && subtle.ConstantTimeCompare(provided, expected) == 1
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, protocol.ErrMatchNotFound):
		return http.StatusNotFound
	case errors.Is(err, protocol.ErrSessionNotConnected), errors.Is(err, protocol.ErrUnauthorized):
		return http.StatusForbidden
	default:
		return http.StatusBadRequest
	}
}
