package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"chess-go/protocol"
)

// Client sends protocol envelopes to an HTTPServer.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	Format     WireFormat
}

// WireFormat selects the envelope framing used by Do. Resource endpoints
// remain JSON because they are ordinary read-only HTTP representations.
type WireFormat string

const (
	WireJSON     WireFormat = "json"
	WireProtobuf WireFormat = "protobuf"
)

// NewClient creates an HTTP client for a server base URL.
func NewClient(baseURL, token string) (*Client, error) {
	client, err := newClientWithTLS(baseURL, token, DefaultTLSConfig())
	if err != nil {
		return nil, err
	}
	client.Format = WireJSON
	return client, nil
}

func newClient(baseURL, token string) (*Client, error) {
	normalizedURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(normalizedURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("network URL must include an http(s) scheme and host")
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return nil, errors.New("network URL must use http or https")
	}
	return &Client{BaseURL: normalizedURL, Token: token}, nil
}

// NewClientFromEnv reads CHESS_NETWORK_URL, CHESS_NETWORK_TOKEN, and the
// optional CHESS_TLS_* trust/client-certificate settings.
func NewClientFromEnv() (*Client, error) {
	config, err := TLSConfigFromEnv()
	if err != nil {
		return nil, err
	}
	client, err := newClientWithTLS(os.Getenv("CHESS_NETWORK_URL"), os.Getenv("CHESS_NETWORK_TOKEN"), config)
	if err != nil {
		return nil, err
	}
	client.Format = wireFormatFromEnv()
	return client, nil
}

// NewClientFromEnvTLS applies CHESS_TLS_* settings to an explicitly supplied
// network URL. It is useful for command-line clients whose URL is a flag but
// whose trust policy is deployment-specific.
func NewClientFromEnvTLS(baseURL, token string) (*Client, error) {
	config, err := TLSConfigFromEnv()
	if err != nil {
		return nil, err
	}
	client, err := newClientWithTLS(baseURL, token, config)
	if err != nil {
		return nil, err
	}
	client.Format = wireFormatFromEnv()
	return client, nil
}

// Do encodes a request envelope and returns the decoded response envelope.
func (c *Client) Do(ctx context.Context, messageType protocol.MessageType, requestID string, payload any) (protocol.Envelope, error) {
	if c == nil || c.HTTPClient == nil {
		return protocol.Envelope{}, errors.New("network client is not configured")
	}
	body, contentType, err := c.encode(messageType, requestID, payload)
	if err != nil {
		return protocol.Envelope{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return protocol.Envelope{}, err
	}
	request.Header.Set("Content-Type", contentType)
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return protocol.Envelope{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxMessageBytes+1))
	if err != nil {
		return protocol.Envelope{}, err
	}
	if len(responseBody) > maxMessageBytes {
		return protocol.Envelope{}, errors.New("network response body too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return protocol.Envelope{}, fmt.Errorf("network request failed with HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	if strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), protobufContentType) {
		return protocol.DecodeProto(responseBody)
	}
	return protocol.Decode(responseBody)
}

func (c *Client) encode(messageType protocol.MessageType, requestID string, payload any) ([]byte, string, error) {
	if c.Format == WireProtobuf {
		body, err := protocol.EncodeProto(messageType, requestID, payload)
		return body, protobufContentType, err
	}
	body, err := protocol.Encode(messageType, requestID, payload)
	return body, "application/json", err
}

func wireFormatFromEnv() WireFormat {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("CHESS_NETWORK_FORMAT")), string(WireProtobuf)) {
		return WireProtobuf
	}
	return WireJSON
}

// Create creates a match and decodes its initial snapshot.
func (c *Client) Create(ctx context.Context, requestID string, request protocol.CreateMatchRequest) (protocol.MatchSnapshot, error) {
	response, err := c.Do(ctx, protocol.CreateMatch, requestID, request)
	if err != nil {
		return protocol.MatchSnapshot{}, err
	}
	if response.Type == protocol.ProtocolError {
		return protocol.MatchSnapshot{}, decodeProtocolError(response)
	}
	if response.Type != protocol.Snapshot {
		return protocol.MatchSnapshot{}, fmt.Errorf("unexpected create response %q", response.Type)
	}
	var snapshot protocol.MatchSnapshot
	if err := response.UnmarshalPayload(&snapshot); err != nil {
		return protocol.MatchSnapshot{}, err
	}
	return snapshot, nil
}

// Matchmake joins a compatible open match or creates a waiting match.
func (c *Client) Matchmake(ctx context.Context, requestID string, request protocol.MatchmakeRequest) (protocol.MatchSnapshot, error) {
	response, err := c.Do(ctx, protocol.Matchmake, requestID, request)
	if err != nil {
		return protocol.MatchSnapshot{}, err
	}
	return snapshotResponse(response, "matchmake")
}

// Join claims a seat or spectator role and decodes its snapshot.
func (c *Client) Join(ctx context.Context, requestID string, request protocol.JoinMatchRequest) (protocol.MatchSnapshot, error) {
	response, err := c.Do(ctx, protocol.JoinMatch, requestID, request)
	if err != nil {
		return protocol.MatchSnapshot{}, err
	}
	if response.Type == protocol.ProtocolError {
		return protocol.MatchSnapshot{}, decodeProtocolError(response)
	}
	if response.Type != protocol.Snapshot {
		return protocol.MatchSnapshot{}, fmt.Errorf("unexpected join response %q", response.Type)
	}
	var snapshot protocol.MatchSnapshot
	if err := response.UnmarshalPayload(&snapshot); err != nil {
		return protocol.MatchSnapshot{}, err
	}
	return snapshot, nil
}

// Move submits a synchronized move and decodes the authoritative acknowledgement.
func (c *Client) Move(ctx context.Context, requestID string, request protocol.MoveRequest) (protocol.MoveAccepted, error) {
	response, err := c.Do(ctx, protocol.Move, requestID, request)
	if err != nil {
		return protocol.MoveAccepted{}, err
	}
	if response.Type == protocol.ProtocolError {
		return protocol.MoveAccepted{}, decodeProtocolError(response)
	}
	if response.Type != protocol.MoveAcceptedType {
		return protocol.MoveAccepted{}, fmt.Errorf("unexpected move response %q", response.Type)
	}
	var accepted protocol.MoveAccepted
	if err := response.UnmarshalPayload(&accepted); err != nil {
		return protocol.MoveAccepted{}, err
	}
	return accepted, nil
}

// Resign resigns a player's seat and decodes the resulting snapshot.
func (c *Client) Resign(ctx context.Context, requestID string, request protocol.ResignRequest) (protocol.MatchSnapshot, error) {
	response, err := c.Do(ctx, protocol.Resign, requestID, request)
	if err != nil {
		return protocol.MatchSnapshot{}, err
	}
	return snapshotResponse(response, "resign")
}

// OfferDraw offers a draw or accepts the opponent's offer.
func (c *Client) OfferDraw(ctx context.Context, requestID string, request protocol.DrawOfferRequest) (protocol.MatchSnapshot, error) {
	response, err := c.Do(ctx, protocol.DrawOffer, requestID, request)
	if err != nil {
		return protocol.MatchSnapshot{}, err
	}
	return snapshotResponse(response, "draw")
}

// Snapshot fetches one match snapshot through the HTTP resource endpoint.
func (c *Client) Snapshot(ctx context.Context, request protocol.SnapshotRequest) (protocol.MatchSnapshot, error) {
	if c == nil || c.HTTPClient == nil {
		return protocol.MatchSnapshot{}, errors.New("network client is not configured")
	}
	if strings.TrimSpace(request.MatchID) == "" {
		return protocol.MatchSnapshot{}, protocol.ErrInvalidRequest
	}
	endpoint := c.BaseURL + "/v1/matches/" + url.PathEscape(request.MatchID)
	if request.PlayerID != "" {
		endpoint += "?player_id=" + url.QueryEscape(request.PlayerID)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return protocol.MatchSnapshot{}, err
	}
	if c.Token != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+c.Token)
	}
	response, err := c.HTTPClient.Do(httpRequest)
	if err != nil {
		return protocol.MatchSnapshot{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, maxMessageBytes))
		return protocol.MatchSnapshot{}, fmt.Errorf("snapshot request failed with HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	var snapshot protocol.MatchSnapshot
	if err := json.NewDecoder(io.LimitReader(response.Body, maxMessageBytes)).Decode(&snapshot); err != nil {
		return protocol.MatchSnapshot{}, err
	}
	return snapshot, nil
}

// List returns all currently registered match snapshots.
func (c *Client) List(ctx context.Context) ([]protocol.MatchSnapshot, error) {
	if c == nil || c.HTTPClient == nil {
		return nil, errors.New("network client is not configured")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/matches", nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, maxMessageBytes))
		return nil, fmt.Errorf("match listing failed with HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	var matches []protocol.MatchSnapshot
	if err := json.NewDecoder(io.LimitReader(response.Body, maxMessageBytes)).Decode(&matches); err != nil {
		return nil, err
	}
	return matches, nil
}

func decodeProtocolError(envelope protocol.Envelope) error {
	var body protocol.ProtocolErrorBody
	if err := envelope.UnmarshalPayload(&body); err != nil {
		return err
	}
	return fmt.Errorf("%s: %s", body.Code, body.Message)
}

func snapshotResponse(response protocol.Envelope, operation string) (protocol.MatchSnapshot, error) {
	if response.Type == protocol.ProtocolError {
		return protocol.MatchSnapshot{}, decodeProtocolError(response)
	}
	if response.Type != protocol.Snapshot {
		return protocol.MatchSnapshot{}, fmt.Errorf("unexpected %s response %q", operation, response.Type)
	}
	var snapshot protocol.MatchSnapshot
	if err := response.UnmarshalPayload(&snapshot); err != nil {
		return protocol.MatchSnapshot{}, err
	}
	return snapshot, nil
}
