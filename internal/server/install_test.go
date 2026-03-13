package server_test

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielecalderazzo/ollama-farm/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallScriptHandler_ServesScriptWithInjectedURLs(t *testing.T) {
	h := server.InstallScriptHandler()
	req := httptest.NewRequest(http.MethodGet, "http://farm.example.com:8080/install.sh", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/x-sh", w.Header().Get("Content-Type"))
	body, _ := io.ReadAll(w.Body)
	require.NotEmpty(t, body)
	// Injected from request Host (no TLS)
	assert.Contains(t, string(body), "ws://farm.example.com:8080")
	assert.Contains(t, string(body), "http://farm.example.com:8080/download")
	assert.NotContains(t, string(body), "{{.ServerURL}}")
	assert.NotContains(t, string(body), "{{.DownloadBase}}")
}

func TestDownloadProxyHandler_RejectsInvalidFilename(t *testing.T) {
	h := server.DownloadProxyHandler("")
	req := httptest.NewRequest(http.MethodGet, "http://localhost/download/../../../etc/passwd", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDownloadProxyHandler_RejectsWrongPattern(t *testing.T) {
	h := server.DownloadProxyHandler("")
	req := httptest.NewRequest(http.MethodGet, "http://localhost/download/evil.exe", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInstallPS1Handler_ServesScriptWithInjectedURLs(t *testing.T) {
	h := server.InstallPS1Handler()
	req := httptest.NewRequest(http.MethodGet, "https://farm.example.com/install.ps1", nil)
	req.TLS = &tls.ConnectionState{} // simulate TLS so we get wss/https
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/x-powershell", w.Header().Get("Content-Type"))
	body, _ := io.ReadAll(w.Body)
	require.NotEmpty(t, body)
	assert.Contains(t, string(body), "wss://farm.example.com")
	assert.Contains(t, string(body), "https://farm.example.com/download")
	assert.NotContains(t, string(body), "{{.ServerURL}}")
	assert.NotContains(t, string(body), "{{.DownloadBase}}")
}
