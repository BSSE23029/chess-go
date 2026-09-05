package protocol

import (
	"encoding/json"
	"errors"
	"fmt"

	"chess-go/protocol/pb"
	"google.golang.org/protobuf/proto"
)

// EncodeProto creates the opt-in protobuf-framed form of a protocol envelope.
// Payloads remain the existing strict JSON objects during migration so old
// domain validation and newer binary transports can coexist.
func EncodeProto(messageType MessageType, requestID string, payload any) ([]byte, error) {
	if messageType == "" {
		return nil, errors.New("protocol message type is required")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return EncodeProtoEnvelope(Envelope{Version: Version, Type: messageType, RequestID: requestID, Payload: data})
}

// EncodeProtoEnvelope marshals an already validated envelope into protobuf.
func EncodeProtoEnvelope(envelope Envelope) ([]byte, error) {
	if envelope.Version != Version || !knownMessageType(envelope.Type) || len(envelope.Payload) == 0 || string(envelope.Payload) == "null" {
		return nil, errors.New("protocol envelope requires version, type, and payload")
	}
	return proto.Marshal(&pb.Envelope{
		Version:   uint32(envelope.Version),
		Type:      string(envelope.Type),
		RequestId: envelope.RequestID,
		Payload:   envelope.Payload,
	})
}

// DecodeProto unmarshals and validates a protobuf-framed envelope. Unknown
// protobuf fields are retained by the protobuf runtime's forward-compatible
// wire rules; the typed JSON payload remains strict when decoded by callers.
func DecodeProto(data []byte) (Envelope, error) {
	var message pb.Envelope
	if err := proto.Unmarshal(data, &message); err != nil {
		return Envelope{}, fmt.Errorf("invalid protobuf envelope: %w", err)
	}
	envelope := Envelope{Version: int(message.Version), Type: MessageType(message.Type), RequestID: message.RequestId, Payload: message.Payload}
	if envelope.Version != Version {
		return Envelope{}, fmt.Errorf("unsupported protocol version %d", envelope.Version)
	}
	if !knownMessageType(envelope.Type) || len(envelope.Payload) == 0 || string(envelope.Payload) == "null" {
		return Envelope{}, errors.New("protocol envelope requires type and payload")
	}
	return envelope, nil
}

// HandleProto adapts the existing authoritative JSON handler to a binary
// envelope without duplicating domain dispatch and validation logic.
func (s *Server) HandleProto(data []byte) ([]byte, error) {
	envelope, err := DecodeProto(data)
	if err != nil {
		return nil, err
	}
	jsonRequest, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	jsonResponse, err := s.Handle(jsonRequest)
	if err != nil {
		return nil, err
	}
	response, err := Decode(jsonResponse)
	if err != nil {
		return nil, err
	}
	return EncodeProtoEnvelope(response)
}
