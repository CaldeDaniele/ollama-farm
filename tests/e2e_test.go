//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielecalderazzo/ollama-farm/internal/client"
	"github.com/danielecalderazzo/ollama-farm/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOllamaServer simulates an Ollama server for integration tests.
func fakeOllamaServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]string{{"name": "llama3"}},
			})
		case "/api/generate":
			flusher := w.(http.Flusher)
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.WriteHeader(http.StatusOK)
			for _, tok := range []string{`{"response":"hello"}`, `{"response":" world"}`, `{"done":true}`} {
				_, _ = w.Write([]byte(tok + "\n"))
				flusher.Flush()
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestE2E_RequestRoutedToClient(t *testing.T) {
	fakeOllama := fakeOllamaServer()
	defer fakeOllama.Close()

	reg := server.NewRegistry()
	router := server.NewRouter(reg)
	dispatcher := server.NewDispatcher(reg)
	wsHandler := server.NewWSHandler(reg, dispatcher, "testtoken")
	httpHandler := server.NewHTTPHandler(reg, router, dispatcher, "testtoken")

	mux := http.NewServeMux()
	mux.Handle("/ws", wsHandler)
	mux.Handle("/", httpHandler)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	serverAddr := ln.Addr().String()

	proxy := client.NewOllamaProxy(fakeOllama.URL)
	wsClient := client.NewWSClient("ws://"+serverAddr, "testtoken", proxy)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go wsClient.Run(ctx)

	require.Eventually(t, func() bool {
		return len(reg.Snapshot()) > 0
	}, 3*time.Second, 100*time.Millisecond, "client should register within 3s")

	body := `{"model":"llama3","prompt":"say hello","stream":true}`
	resp, err := http.Post("http://"+serverAddr+"/api/generate", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(respBody), "hello")
}

func TestE2E_503_WhenNoClient(t *testing.T) {
	reg := server.NewRegistry()
	h := server.NewHTTPHandler(reg, server.NewRouter(reg), server.NewDispatcher(reg), "tok")

	body := `{"model":"llama3","prompt":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
