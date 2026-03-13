package protocol_test

import (
	"encoding/json"
	"testing"

	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterMessage_RoundTrip(t *testing.T) {
	msg := protocol.RegisterMessage{
		Type:   protocol.TypeRegister,
		Token:  "secret",
		Models: []string{"llama3", "mistral"},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var out protocol.RegisterMessage
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, msg, out)
}

func TestRequestMessage_RoundTrip(t *testing.T) {
	msg := protocol.RequestMessage{
		Type:       protocol.TypeRequest,
		ReqID:      "req-123",
		Method:     "POST",
		Path:       "/api/generate",
		Headers:    map[string]string{"content-type": "application/json"},
		BodyBase64: "eyJtb2RlbCI6ImxsYW1hMyJ9",
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var out protocol.RequestMessage
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, msg, out)
}

func TestChunkMessage_RoundTrip(t *testing.T) {
	msg := protocol.ChunkMessage{
		Type:       protocol.TypeChunk,
		ReqID:      "req-123",
		DataBase64: "dG9rZW4x",
		StatusCode: 200,
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var out protocol.ChunkMessage
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, msg, out)
}

func TestEndMessage_RoundTrip(t *testing.T) {
	msg := protocol.EndMessage{Type: protocol.TypeEnd, ReqID: "req-123"}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	var out protocol.EndMessage
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, msg, out)
}

func TestErrorMessage_RoundTrip(t *testing.T) {
	msg := protocol.ErrorMessage{
		Type:    protocol.TypeError,
		ReqID:   "req-123",
		Message: "connection refused",
		Code:    502,
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	var out protocol.ErrorMessage
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, msg, out)
}

func TestParseType(t *testing.T) {
	raw := `{"type":"register","token":"x","models":["llama3"]}`
	msgType, err := protocol.ParseType([]byte(raw))
	require.NoError(t, err)
	assert.Equal(t, protocol.TypeRegister, msgType)
}
