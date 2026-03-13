package client_test

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielecalderazzo/ollama-farm/internal/client"
	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaProxy_ForwardsRequest(t *testing.T) {
	fakeOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/generate", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "llama3")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response":"hello"}`))
	}))
	defer fakeOllama.Close()

	proxy := client.NewOllamaProxy(fakeOllama.URL)

	bodyJSON := `{"model":"llama3","prompt":"hi"}`
	req := &protocol.RequestMessage{
		ReqID:      "req-1",
		Method:     "POST",
		Path:       "/api/generate",
		Headers:    map[string]string{"content-type": "application/json"},
		BodyBase64: base64.StdEncoding.EncodeToString([]byte(bodyJSON)),
	}

	var chunks []*protocol.ChunkMessage
	var ended bool

	err := proxy.Execute(req, func(chunk *protocol.ChunkMessage) {
		chunks = append(chunks, chunk)
	}, func() {
		ended = true
	}, func(code int, msg string) {})

	require.NoError(t, err)
	assert.True(t, ended)
	require.NotEmpty(t, chunks)
	assert.Equal(t, 200, chunks[0].StatusCode)
}

func TestOllamaProxy_StreamsChunks(t *testing.T) {
	fakeOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
		for _, tok := range []string{"tok1", "tok2", "tok3"} {
			_, _ = w.Write([]byte(tok))
			flusher.Flush()
		}
	}))
	defer fakeOllama.Close()

	proxy := client.NewOllamaProxy(fakeOllama.URL)
	req := &protocol.RequestMessage{
		ReqID:  "req-2",
		Method: "GET",
		Path:   "/api/tags",
	}

	var chunks []*protocol.ChunkMessage
	_ = proxy.Execute(req, func(c *protocol.ChunkMessage) {
		chunks = append(chunks, c)
	}, func() {}, func(int, string) {})

	assert.NotEmpty(t, chunks)
	var full []byte
	for _, c := range chunks {
		dec, _ := base64.StdEncoding.DecodeString(c.DataBase64)
		full = append(full, dec...)
	}
	assert.Equal(t, "tok1tok2tok3", string(full))
}

func TestOllamaProxy_OllamaUnreachable(t *testing.T) {
	proxy := client.NewOllamaProxy("http://127.0.0.1:19999")

	req := &protocol.RequestMessage{ReqID: "req-3", Method: "GET", Path: "/api/tags"}
	var errCode int
	_ = proxy.Execute(req, func(*protocol.ChunkMessage) {}, func() {}, func(code int, msg string) {
		errCode = code
	})
	assert.Equal(t, 502, errCode)
}
