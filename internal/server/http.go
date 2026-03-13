package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const maxBodySize = 10 * 1024 * 1024 // 10MB

// HTTPHandler handles all incoming Ollama API requests and dispatches them to clients.
type HTTPHandler struct {
	registry   *Registry
	router     *Router
	dispatcher *Dispatcher
	token      string
}

// NewHTTPHandler creates an HTTPHandler. The dispatcher must be the same instance
// shared with the WSHandler so that response chunks can be routed back correctly.
func NewHTTPHandler(reg *Registry, router *Router, dispatcher *Dispatcher, token string) *HTTPHandler {
	return &HTTPHandler{
		registry:   reg,
		router:     router,
		dispatcher: dispatcher,
		token:      token,
	}
}

// Paths that can be handled by any client (no model in body).
var anyClientPaths = map[string]bool{
	"/api/tags":   true,
	"/api/version": true,
}

// ServeHTTP is the main HTTP entry point.
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	limited := io.LimitReader(r.Body, int64(maxBodySize)+1)
	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	if len(bodyBytes) > maxBodySize {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	path := r.URL.Path
	if path == "" {
		path = r.URL.RawPath
	}

	var clientID string
	model, extractErr := ExtractModel(bodyBytes)
	if extractErr != nil && len(bodyBytes) > 0 {
		http.Error(w, fmt.Sprintf("could not determine model: %v", extractErr), http.StatusBadRequest)
		return
	}
	if extractErr == nil && model != "" {
		var pickErr error
		clientID, pickErr = h.router.Pick(model)
		if pickErr != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("no client available for model %s", model),
			})
			return
		}
	} else if anyClientPaths[path] {
		var pickErr error
		clientID, pickErr = h.router.PickAny()
		if pickErr != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "no client available",
			})
			return
		}
	} else {
		http.Error(w, "could not determine model: missing body or model field (required for this path)", http.StatusBadRequest)
		return
	}

	h.registry.IncrementTotal(clientID)
	h.dispatcher.Dispatch(w, r, clientID, bodyBytes)
}

// ExtractModel reads the "model" field from a JSON body.
func ExtractModel(body []byte) (string, error) {
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Model == "" {
		return "", fmt.Errorf("missing required field: model")
	}
	return payload.Model, nil
}
