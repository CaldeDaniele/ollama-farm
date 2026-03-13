package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielecalderazzo/ollama-farm/internal/server"
	"github.com/stretchr/testify/assert"
)

func TestHTTPHandler_413_BodyTooLarge(t *testing.T) {
	reg := server.NewRegistry()
	h := server.NewHTTPHandler(reg, server.NewRouter(reg), server.NewDispatcher(reg), "token")

	bigBody := strings.NewReader(strings.Repeat("x", 10*1024*1024+1))
	req := httptest.NewRequest(http.MethodPost, "/api/generate", bigBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestHTTPHandler_503_NoClient(t *testing.T) {
	reg := server.NewRegistry()
	h := server.NewHTTPHandler(reg, server.NewRouter(reg), server.NewDispatcher(reg), "token")

	body := `{"model":"llama3","prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHTTPHandler_ExtractModel(t *testing.T) {
	body := `{"model":"phi3","prompt":"hi","stream":true}`
	model, err := server.ExtractModel([]byte(body))
	assert.NoError(t, err)
	assert.Equal(t, "phi3", model)
}

func TestHTTPHandler_ExtractModel_Missing(t *testing.T) {
	body := `{"prompt":"hi"}`
	_, err := server.ExtractModel([]byte(body))
	assert.Error(t, err)
}

// GET /api/tags has no body; should not return "unexpected end of JSON input".
func TestHTTPHandler_ApiTags_EmptyBody(t *testing.T) {
	reg := server.NewRegistry()
	h := server.NewHTTPHandler(reg, server.NewRouter(reg), server.NewDispatcher(reg), "token")

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	// No client connected -> 503, not 400
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.NotContains(t, w.Body.String(), "unexpected end of JSON input")
}
