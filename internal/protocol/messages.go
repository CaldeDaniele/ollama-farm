package protocol

import "encoding/json"

type MessageType string

const (
	TypeRegister MessageType = "register"
	TypeRequest  MessageType = "request"
	TypeChunk    MessageType = "chunk"
	TypeEnd      MessageType = "end"
	TypeError    MessageType = "error"
)

// RegisterMessage is sent by the client immediately after WS upgrade.
type RegisterMessage struct {
	Type   MessageType `json:"type"`
	Token  string      `json:"token"`
	Models []string    `json:"models"`
}

// RequestMessage is sent by the server to dispatch an HTTP request to a client.
type RequestMessage struct {
	Type       MessageType       `json:"type"`
	ReqID      string            `json:"req_id"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	BodyBase64 string            `json:"body_b64"` // base64-encoded request body
}

// ChunkMessage carries one streaming response chunk from client to server.
// StatusCode is only set on the first chunk.
type ChunkMessage struct {
	Type       MessageType `json:"type"`
	ReqID      string      `json:"req_id"`
	DataBase64 string      `json:"data"`                  // base64-encoded response bytes
	StatusCode int         `json:"status_code,omitempty"` //nolint:tagliatelle
}

// EndMessage signals the end of a streaming response.
type EndMessage struct {
	Type  MessageType `json:"type"`
	ReqID string      `json:"req_id"`
}

// ErrorMessage signals a processing error on the client side.
type ErrorMessage struct {
	Type    MessageType `json:"type"`
	ReqID   string      `json:"req_id"`
	Message string      `json:"message"`
	Code    int         `json:"code"`
}

// typeOnly is used to peek at the type field without full deserialization.
type typeOnly struct {
	Type MessageType `json:"type"`
}

// ParseType extracts the "type" field from a raw JSON message.
func ParseType(data []byte) (MessageType, error) {
	var t typeOnly
	if err := json.Unmarshal(data, &t); err != nil {
		return "", err
	}
	return t.Type, nil
}
