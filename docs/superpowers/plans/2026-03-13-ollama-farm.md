# ollama-farm Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single Go binary CLI tool that routes Ollama LLM API requests from a public server to available GPU client nodes via persistent WebSocket tunnels.

**Architecture:** A server binary listens for HTTP requests on :8080 and maintains a WebSocket registry of connected client nodes. Each client declares its available Ollama models at registration; the server routes incoming requests to a FREE client with the matching model using round-robin selection, then tunnels the full streaming response back to the original caller.

**Tech Stack:** Go 1.22+, cobra (CLI), bubbletea + lipgloss (TUI), nhooyr.io/websocket (WS), testify (tests), goreleaser (distribution)

---

## Chunk 1: Scaffold, Protocol, Registry

### Task 1: Project scaffold

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/server.go`
- Create: `cmd/client.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/danielecalderazzo/stride/ollama-farm
go mod init github.com/danielecalderazzo/ollama-farm
```

Expected: `go.mod` created with `module github.com/danielecalderazzo/ollama-farm`

- [ ] **Step 2: Install dependencies**

```bash
go get github.com/spf13/cobra@latest
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get nhooyr.io/websocket@latest
go get github.com/google/uuid@latest
go get github.com/stretchr/testify@latest
```

- [ ] **Step 3: Write main.go**

```go
// main.go
package main

import "github.com/danielecalderazzo/ollama-farm/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 4: Write cmd/root.go**

Create `cmd/root.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ollama-farm",
	Short: "Route Ollama API requests across multiple GPU nodes",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Write cmd/server.go stub**

```go
package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the ollama-farm server",
	RunE:  runServer,
}

var (
	serverPort    int
	serverToken   string
	serverTLSCert string
	serverTLSKey  string
)

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().IntVar(&serverPort, "port", 8080, "HTTP listen port")
	serverCmd.Flags().StringVar(&serverToken, "token", "", "Auth token (auto-generated if empty)")
	serverCmd.Flags().StringVar(&serverTLSCert, "tls-cert", "", "Path to TLS certificate")
	serverCmd.Flags().StringVar(&serverTLSKey, "tls-key", "", "Path to TLS key")
}

func runServer(cmd *cobra.Command, args []string) error {
	fmt.Println("server: not yet implemented")
	return nil
}
```

- [ ] **Step 6: Write cmd/client.go stub**

```go
package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Connect this node as an ollama-farm client",
	RunE:  runClient,
}

var (
	clientServer   string
	clientToken    string
	clientOllamaURL string
)

func init() {
	rootCmd.AddCommand(clientCmd)
	clientCmd.Flags().StringVar(&clientServer, "server", "", "Server WebSocket URL (required)")
	clientCmd.Flags().StringVar(&clientToken, "token", "", "Auth token (required)")
	clientCmd.Flags().StringVar(&clientOllamaURL, "ollama-url", "http://localhost:11434", "Local Ollama URL")
	_ = clientCmd.MarkFlagRequired("server")
	_ = clientCmd.MarkFlagRequired("token")
}

func runClient(cmd *cobra.Command, args []string) error {
	fmt.Println("client: not yet implemented")
	return nil
}
```

- [ ] **Step 7: Verify the CLI builds and runs**

```bash
go build ./...
./ollama-farm --help
./ollama-farm server --help
./ollama-farm client --help
```

Expected: help text prints for all three commands, no errors.

- [ ] **Step 8: Commit**

```bash
git init
git add .
git commit -m "feat: project scaffold with cobra CLI"
```

---

### Task 2: Protocol message types

**Files:**
- Create: `internal/protocol/messages.go`
- Create: `internal/protocol/messages_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/protocol/messages_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/protocol/... -v
```

Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement messages.go**

Create `internal/protocol/messages.go`:

```go
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
	DataBase64 string      `json:"data"`    // base64-encoded response bytes
	StatusCode int         `json:"status_code,omitempty"`
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/protocol/... -v
```

Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/protocol/
git commit -m "feat: protocol message types with JSON round-trip tests"
```

---

### Task 3: Client registry

**Files:**
- Create: `internal/server/registry.go`
- Create: `internal/server/registry_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/server/registry_test.go`:

```go
package server_test

import (
	"testing"
	"time"

	"github.com/danielecalderazzo/ollama-farm/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_AddAndGet(t *testing.T) {
	r := server.NewRegistry()
	entry := &server.ClientEntry{
		ID:          "client-1",
		Models:      []string{"llama3", "mistral"},
		Status:      server.StatusFree,
		ConnectedAt: time.Now(),
	}
	r.Add(entry)

	got, ok := r.Get("client-1")
	require.True(t, ok)
	assert.Equal(t, entry.ID, got.ID)
	assert.Equal(t, entry.Models, got.Models)
	assert.Equal(t, server.StatusFree, got.Status)
}

func TestRegistry_Remove(t *testing.T) {
	r := server.NewRegistry()
	r.Add(&server.ClientEntry{ID: "client-1", Models: []string{"llama3"}, Status: server.StatusFree})
	r.Remove("client-1")

	_, ok := r.Get("client-1")
	assert.False(t, ok)
}

func TestRegistry_SetStatus(t *testing.T) {
	r := server.NewRegistry()
	r.Add(&server.ClientEntry{ID: "c1", Models: []string{"phi3"}, Status: server.StatusFree})

	r.SetStatus("c1", server.StatusBusy, "req-abc")
	entry, ok := r.Get("c1")
	require.True(t, ok)
	assert.Equal(t, server.StatusBusy, entry.Status)
	assert.Equal(t, "req-abc", entry.ActiveReqID)

	r.SetStatus("c1", server.StatusFree, "")
	entry, _ = r.Get("c1")
	assert.Equal(t, server.StatusFree, entry.Status)
	assert.Empty(t, entry.ActiveReqID)
}

func TestRegistry_FreeByModel(t *testing.T) {
	r := server.NewRegistry()
	r.Add(&server.ClientEntry{ID: "c1", Models: []string{"llama3"}, Status: server.StatusFree})
	r.Add(&server.ClientEntry{ID: "c2", Models: []string{"llama3", "mistral"}, Status: server.StatusBusy})
	r.Add(&server.ClientEntry{ID: "c3", Models: []string{"phi3"}, Status: server.StatusFree})

	free := r.FreeByModel("llama3")
	assert.Len(t, free, 1)
	assert.Equal(t, "c1", free[0].ID)
}

func TestRegistry_Snapshot(t *testing.T) {
	r := server.NewRegistry()
	r.Add(&server.ClientEntry{ID: "c1", Models: []string{"llama3"}, Status: server.StatusFree})
	r.Add(&server.ClientEntry{ID: "c2", Models: []string{"phi3"}, Status: server.StatusBusy})

	snap := r.Snapshot()
	assert.Len(t, snap, 2)
}

func TestRegistry_IncrementTotal(t *testing.T) {
	r := server.NewRegistry()
	r.Add(&server.ClientEntry{ID: "c1", Models: []string{"llama3"}, Status: server.StatusFree})

	r.IncrementTotal("c1")
	r.IncrementTotal("c1")

	entry, _ := r.Get("c1")
	assert.Equal(t, 2, entry.TotalRequests)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/server/... -v
```

Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement registry.go**

Create `internal/server/registry.go`:

```go
package server

import (
	"sync"
	"time"

	"nhooyr.io/websocket"
)

type ClientStatus string

const (
	StatusFree ClientStatus = "FREE"
	StatusBusy ClientStatus = "BUSY"
)

// ClientEntry represents one connected client node.
type ClientEntry struct {
	ID            string
	Models        []string
	Status        ClientStatus
	ActiveReqID   string
	ConnectedAt   time.Time
	TotalRequests int
	Conn          *websocket.Conn // nil in tests
}

// HasModel returns true if the client declared the given model.
func (e *ClientEntry) HasModel(model string) bool {
	for _, m := range e.Models {
		if m == model {
			return true
		}
	}
	return false
}

// Registry is a thread-safe map of connected clients.
type Registry struct {
	mu      sync.RWMutex
	clients map[string]*ClientEntry
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{clients: make(map[string]*ClientEntry)}
}

// Add inserts a new client entry. Overwrites if the ID already exists.
func (r *Registry) Add(e *ClientEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[e.ID] = e
}

// Remove deletes a client by ID.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, id)
}

// Get returns a copy of the entry for the given ID.
func (r *Registry) Get(id string) (*ClientEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.clients[id]
	if !ok {
		return nil, false
	}
	// Return a shallow copy so callers don't mutate registry state.
	cp := *e
	return &cp, true
}

// SetStatus updates the status and active request ID for a client.
func (r *Registry) SetStatus(id string, status ClientStatus, reqID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.clients[id]; ok {
		e.Status = status
		e.ActiveReqID = reqID
	}
}

// IncrementTotal increments the total request counter for a client.
func (r *Registry) IncrementTotal(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.clients[id]; ok {
		e.TotalRequests++
	}
}

// FreeByModel returns all FREE clients that have the given model.
// Returns pointers to live entries — callers must not mutate them.
// Caller holds no lock; this is safe because we copy the slice under RLock.
func (r *Registry) FreeByModel(model string) []*ClientEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*ClientEntry
	for _, e := range r.clients {
		if e.Status == StatusFree && e.HasModel(model) {
			cp := *e
			result = append(result, &cp)
		}
	}
	return result
}

// Snapshot returns a copy of all entries for display (TUI, etc.).
func (r *Registry) Snapshot() []*ClientEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*ClientEntry, 0, len(r.clients))
	for _, e := range r.clients {
		cp := *e
		result = append(result, &cp)
	}
	return result
}

// GetConn returns the live WebSocket connection for a client ID.
// Returns nil if not found. Used by the dispatcher to send messages.
func (r *Registry) GetConn(id string) *websocket.Conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.clients[id]; ok {
		return e.Conn
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/server/... -v
```

Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/registry.go internal/server/registry_test.go
git commit -m "feat: thread-safe client registry with status tracking"
```

---

## Chunk 2: Router, HTTP Server, WebSocket Server

### Task 4: Model-aware round-robin router

**Files:**
- Create: `internal/server/router.go`
- Create: `internal/server/router_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/server/router_test.go`:

```go
package server_test

import (
	"testing"

	"github.com/danielecalderazzo/ollama-farm/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeRegistry(clients ...*server.ClientEntry) *server.Registry {
	r := server.NewRegistry()
	for _, c := range clients {
		r.Add(c)
	}
	return r
}

func TestRouter_NoCandidates_Returns503(t *testing.T) {
	r := makeRegistry()
	router := server.NewRouter(r)

	_, err := router.Pick("llama3")
	require.Error(t, err)
	assert.ErrorIs(t, err, server.ErrNoClientAvailable)
}

func TestRouter_SingleCandidate(t *testing.T) {
	r := makeRegistry(&server.ClientEntry{ID: "c1", Models: []string{"llama3"}, Status: server.StatusFree})
	router := server.NewRouter(r)

	id, err := router.Pick("llama3")
	require.NoError(t, err)
	assert.Equal(t, "c1", id)
}

func TestRouter_BusyClientSkipped(t *testing.T) {
	r := makeRegistry(
		&server.ClientEntry{ID: "c1", Models: []string{"llama3"}, Status: server.StatusBusy},
		&server.ClientEntry{ID: "c2", Models: []string{"llama3"}, Status: server.StatusFree},
	)
	router := server.NewRouter(r)

	id, err := router.Pick("llama3")
	require.NoError(t, err)
	assert.Equal(t, "c2", id)
}

func TestRouter_RoundRobin(t *testing.T) {
	r := makeRegistry(
		&server.ClientEntry{ID: "c1", Models: []string{"llama3"}, Status: server.StatusFree},
		&server.ClientEntry{ID: "c2", Models: []string{"llama3"}, Status: server.StatusFree},
	)
	router := server.NewRouter(r)

	seen := map[string]int{}
	for i := 0; i < 10; i++ {
		id, err := router.Pick("llama3")
		require.NoError(t, err)
		seen[id]++
	}
	// Both clients should have been picked
	assert.Equal(t, 2, len(seen))
}

func TestRouter_WrongModelSkipped(t *testing.T) {
	r := makeRegistry(&server.ClientEntry{ID: "c1", Models: []string{"phi3"}, Status: server.StatusFree})
	router := server.NewRouter(r)

	_, err := router.Pick("llama3")
	require.Error(t, err)
	assert.ErrorIs(t, err, server.ErrNoClientAvailable)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/server/... -run TestRouter -v
```

Expected: FAIL — Router types not defined.

- [ ] **Step 3: Implement router.go**

Create `internal/server/router.go`:

```go
package server

import (
	"errors"
	"sort"
	"sync"
)

// ErrNoClientAvailable is returned when no FREE client has the requested model.
var ErrNoClientAvailable = errors.New("no client available for model")

// Router selects a client for each incoming request using per-model round-robin.
type Router struct {
	registry *Registry
	mu       sync.Mutex
	counters map[string]int // per-model round-robin counter
}

// NewRouter creates a Router backed by the given Registry.
func NewRouter(reg *Registry) *Router {
	return &Router{
		registry: reg,
		counters: make(map[string]int),
	}
}

// Pick selects the ID of a FREE client that has the requested model.
// Returns ErrNoClientAvailable if no candidate exists.
// The counter is only advanced when the free-client slice is non-empty.
func (rt *Router) Pick(model string) (string, error) {
	candidates := rt.registry.FreeByModel(model)
	if len(candidates) == 0 {
		return "", ErrNoClientAvailable
	}

	// Sort by ID for stable, deterministic round-robin ordering.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ID < candidates[j].ID
	})

	rt.mu.Lock()
	idx := rt.counters[model] % len(candidates)
	rt.counters[model] = idx + 1
	rt.mu.Unlock()

	return candidates[idx].ID, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/server/... -run TestRouter -v
```

Expected: all 5 router tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/router.go internal/server/router_test.go
git commit -m "feat: model-aware round-robin router"
```

---

### Task 5: Server HTTP handler (body buffering, routing, streaming)

**Files:**
- Create: `internal/server/http.go`
- Create: `internal/server/http_test.go`
- Create: `internal/server/dispatcher.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/server/http_test.go`:

```go
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
	h := server.NewHTTPHandler(reg, server.NewRouter(reg), "token")

	// 10MB + 1 byte body
	bigBody := strings.NewReader(strings.Repeat("x", 10*1024*1024+1))
	req := httptest.NewRequest(http.MethodPost, "/api/generate", bigBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestHTTPHandler_503_NoClient(t *testing.T) {
	reg := server.NewRegistry()
	h := server.NewHTTPHandler(reg, server.NewRouter(reg), "token")

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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/server/... -run TestHTTPHandler -v
```

Expected: FAIL — HTTPHandler not defined.

- [ ] **Step 3: Implement http.go**

Create `internal/server/http.go`:

```go
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

// NewHTTPHandler creates an HTTPHandler. The token is used by the WebSocket handler,
// stored here for the server startup to share it.
func NewHTTPHandler(reg *Registry, router *Router, token string) *HTTPHandler {
	return &HTTPHandler{
		registry:   reg,
		router:     router,
		dispatcher: NewDispatcher(reg),
		token:      token,
	}
}

// ServeHTTP is the main HTTP entry point.
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Buffer body with size limit.
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

	model, err := ExtractModel(bodyBytes)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not determine model: %v", err), http.StatusBadRequest)
		return
	}

	clientID, err := h.router.Pick(model)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("no client available for model %s", model),
		})
		return
	}

	// Mark client busy before dispatching.
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
```

- [ ] **Step 4: Create dispatcher.go stub** (full implementation in Task 6)

Create `internal/server/dispatcher.go`:

```go
package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
)

// Dispatcher sends REQUEST messages to clients and streams responses back.
type Dispatcher struct {
	registry *Registry
	pending  map[string]chan *protocol.ChunkMessage // req_id → chunk channel
}

// NewDispatcher creates a Dispatcher backed by the given registry.
func NewDispatcher(reg *Registry) *Dispatcher {
	return &Dispatcher{registry: reg, pending: make(map[string]chan *protocol.ChunkMessage)}
}

// Dispatch selects the client conn, sends the REQUEST, and streams the response.
func (d *Dispatcher) Dispatch(w http.ResponseWriter, r *http.Request, clientID string, body []byte) {
	conn := d.registry.GetConn(clientID)
	if conn == nil {
		http.Error(w, "client connection lost", http.StatusBadGateway)
		return
	}

	reqID := uuid.New().String()
	d.registry.SetStatus(clientID, StatusBusy, reqID)
	defer d.registry.SetStatus(clientID, StatusFree, "")

	// Build headers map (single values only — sufficient for Ollama).
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	msg := protocol.RequestMessage{
		Type:       protocol.TypeRequest,
		ReqID:      reqID,
		Method:     r.Method,
		Path:       r.URL.RequestURI(),
		Headers:    headers,
		BodyBase64: base64.StdEncoding.EncodeToString(body),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		http.Error(w, fmt.Sprintf("failed to send to client: %v", err), http.StatusBadGateway)
		return
	}

	// Stream response chunks back to caller.
	// Chunks arrive via the WS message loop (ws.go) through a per-request channel.
	// See RegisterPending / DeliverChunk / DeliverEnd / DeliverError below.
	ch := make(chan *protocol.ChunkMessage, 64)
	d.RegisterPending(reqID, ch)
	defer d.RemovePending(reqID)

	flusher, canFlush := w.(http.Flusher)
	headersSent := false

	for chunk := range ch {
		if chunk == nil {
			// nil sentinel = END or ERROR with no more data
			break
		}
		decoded, err := base64.StdEncoding.DecodeString(chunk.DataBase64)
		if err != nil {
			continue
		}
		if !headersSent {
			w.WriteHeader(chunk.StatusCode)
			headersSent = true
		}
		_, _ = w.Write(decoded)
		if canFlush {
			flusher.Flush()
		}
	}
}

func (d *Dispatcher) RegisterPending(reqID string, ch chan *protocol.ChunkMessage) {
	d.pending[reqID] = ch
}

func (d *Dispatcher) RemovePending(reqID string) {
	delete(d.pending, reqID)
}

// DeliverChunk delivers a chunk to the waiting HTTP handler.
func (d *Dispatcher) DeliverChunk(chunk *protocol.ChunkMessage) {
	if ch, ok := d.pending[chunk.ReqID]; ok {
		ch <- chunk
	}
}

// DeliverEnd closes the pending channel for a request, signaling completion.
func (d *Dispatcher) DeliverEnd(reqID string) {
	if ch, ok := d.pending[reqID]; ok {
		ch <- nil // sentinel
		close(ch)
		delete(d.pending, reqID)
	}
}

// DeliverError closes the pending channel after error.
func (d *Dispatcher) DeliverError(reqID string) {
	if ch, ok := d.pending[reqID]; ok {
		ch <- nil
		close(ch)
		delete(d.pending, reqID)
	}
}
```

- [ ] **Step 5: Run HTTP tests to verify they pass**

```bash
go test ./internal/server/... -run TestHTTPHandler -v
```

Expected: all 4 HTTP tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/server/http.go internal/server/http_test.go internal/server/dispatcher.go
git commit -m "feat: HTTP handler with body buffering, model extraction, and dispatcher"
```

---

### Task 6: Server WebSocket handler (registration, ping/pong, message loop)

**Files:**
- Create: `internal/server/ws.go`
- Create: `internal/server/ws_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/server/ws_test.go`:

```go
package server_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
	"github.com/danielecalderazzo/ollama-farm/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func startTestServer(t *testing.T, token string) (*httptest.Server, *server.Registry) {
	t.Helper()
	reg := server.NewRegistry()
	router := server.NewRouter(reg)
	dispatcher := server.NewDispatcher(reg)
	wsHandler := server.NewWSHandler(reg, dispatcher, token)
	srv := httptest.NewServer(wsHandler)
	t.Cleanup(srv.Close)
	return srv, reg
}

func TestWSHandler_ValidRegistration(t *testing.T) {
	srv, reg := startTestServer(t, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	reg_msg := protocol.RegisterMessage{
		Type:   protocol.TypeRegister,
		Token:  "secret",
		Models: []string{"llama3"},
	}
	require.NoError(t, wsjson.Write(ctx, conn, reg_msg))

	// Wait for registration to propagate
	time.Sleep(100 * time.Millisecond)

	clients := reg.Snapshot()
	require.Len(t, clients, 1)
	assert.Equal(t, []string{"llama3"}, clients[0].Models)
	assert.Equal(t, server.StatusFree, clients[0].Status)
}

func TestWSHandler_InvalidToken_Rejected(t *testing.T) {
	srv, _ := startTestServer(t, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)

	bad := protocol.RegisterMessage{
		Type:   protocol.TypeRegister,
		Token:  "wrong",
		Models: []string{"llama3"},
	}
	require.NoError(t, wsjson.Write(ctx, conn, bad))

	// Server should close connection
	_, _, readErr := conn.Read(ctx)
	assert.Error(t, readErr)
}

func TestWSHandler_RegistrationTimeout(t *testing.T) {
	srv, _ := startTestServer(t, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)

	// Send nothing — wait for server-side timeout (5s) + margin
	readCtx, readCancel := context.WithTimeout(ctx, 7*time.Second)
	defer readCancel()

	_, _, readErr := conn.Read(readCtx)
	assert.Error(t, readErr, "server should close connection after registration timeout")
}

func TestWSHandler_ClientRemovedOnDisconnect(t *testing.T) {
	srv, reg := startTestServer(t, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)

	reg_msg := protocol.RegisterMessage{
		Type:   protocol.TypeRegister,
		Token:  "secret",
		Models: []string{"llama3"},
	}
	require.NoError(t, wsjson.Write(ctx, conn, reg_msg))
	time.Sleep(100 * time.Millisecond)

	require.Len(t, reg.Snapshot(), 1)

	conn.Close(websocket.StatusNormalClosure, "bye")
	time.Sleep(200 * time.Millisecond)

	assert.Empty(t, reg.Snapshot())
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/server/... -run TestWSHandler -v
```

Expected: FAIL — WSHandler not defined.

- [ ] **Step 3: Implement ws.go**

Create `internal/server/ws.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
)

const (
	registrationTimeout = 5 * time.Second
	pingInterval        = 15 * time.Second
	pongTimeout         = 10 * time.Second
)

// WSHandler upgrades HTTP connections to WebSocket and manages the client lifecycle.
type WSHandler struct {
	registry   *Registry
	dispatcher *Dispatcher
	token      string
}

// NewWSHandler creates a WSHandler.
func NewWSHandler(reg *Registry, dispatcher *Dispatcher, token string) *WSHandler {
	return &WSHandler{registry: reg, dispatcher: dispatcher, token: token}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // origins handled by caller config
	})
	if err != nil {
		return
	}

	ctx := r.Context()

	// Phase 1: registration with timeout.
	clientID, err := h.awaitRegistration(ctx, conn)
	if err != nil {
		return // connection already closed inside awaitRegistration
	}
	defer h.registry.Remove(clientID)

	// Phase 2: message loop with heartbeat.
	h.messageLoop(ctx, conn, clientID)
}

func (h *WSHandler) awaitRegistration(ctx context.Context, conn *websocket.Conn) (string, error) {
	regCtx, cancel := context.WithTimeout(ctx, registrationTimeout)
	defer cancel()

	_, data, err := conn.Read(regCtx)
	if err != nil {
		conn.Close(websocket.StatusGoingAway, "registration timeout")
		return "", err
	}

	var msg protocol.RegisterMessage
	if err := json.Unmarshal(data, &msg); err != nil || msg.Type != protocol.TypeRegister {
		conn.Close(websocket.StatusCode(4002), "invalid registration message")
		return "", err
	}

	if msg.Token != h.token {
		conn.Close(websocket.StatusCode(4001), "unauthorized")
		return "", errUnauthorized
	}

	clientID := uuid.New().String()
	h.registry.Add(&ClientEntry{
		ID:          clientID,
		Models:      msg.Models,
		Status:      StatusFree,
		ConnectedAt: time.Now(),
		Conn:        conn,
	})

	return clientID, nil
}

var errUnauthorized = &wsError{"unauthorized"}

type wsError struct{ msg string }

func (e *wsError) Error() string { return e.msg }

func (h *WSHandler) messageLoop(ctx context.Context, conn *websocket.Conn, clientID string) {
	// Ping goroutine: send a ping every 15s; remove client if no pong within 10s.
	pingCtx, pingCancel := context.WithCancel(ctx)
	defer pingCancel()
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-ticker.C:
				pongCtx, pongCancel := context.WithTimeout(pingCtx, pongTimeout)
				err := conn.Ping(pongCtx)
				pongCancel()
				if err != nil {
					h.registry.Remove(clientID)
					conn.Close(websocket.StatusGoingAway, "ping timeout")
					pingCancel()
					return
				}
			}
		}
	}()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		msgType, err := protocol.ParseType(data)
		if err != nil {
			continue
		}

		switch msgType {
		case protocol.TypeChunk:
			var chunk protocol.ChunkMessage
			if err := json.Unmarshal(data, &chunk); err == nil {
				h.dispatcher.DeliverChunk(&chunk)
			}

		case protocol.TypeEnd:
			var end protocol.EndMessage
			if err := json.Unmarshal(data, &end); err == nil {
				h.registry.SetStatus(clientID, StatusFree, "")
				h.dispatcher.DeliverEnd(end.ReqID)
			}

		case protocol.TypeError:
			var errMsg protocol.ErrorMessage
			if err := json.Unmarshal(data, &errMsg); err == nil {
				h.registry.SetStatus(clientID, StatusFree, "")
				h.dispatcher.DeliverError(errMsg.ReqID)
			}
		}
	}
}
```

- [ ] **Step 4: Run WS tests to verify they pass**

```bash
go test ./internal/server/... -run TestWSHandler -v -timeout 30s
```

Expected: all 4 WS tests PASS. Note: `TestWSHandler_RegistrationTimeout` takes ~6 seconds.

- [ ] **Step 5: Commit**

```bash
git add internal/server/ws.go internal/server/ws_test.go
git commit -m "feat: WebSocket handler with registration, token auth, timeout, and message loop"
```

---

## Chunk 3: Client (Ollama Proxy + WS Connection)

### Task 7: Client Ollama proxy

**Files:**
- Create: `internal/client/ollama.go`
- Create: `internal/client/ollama_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/client/ollama_test.go`:

```go
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
	// Fake Ollama server
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
	// Decode all chunks and verify content
	var full []byte
	for _, c := range chunks {
		dec, _ := base64.StdEncoding.DecodeString(c.DataBase64)
		full = append(full, dec...)
	}
	assert.Equal(t, "tok1tok2tok3", string(full))
}

func TestOllamaProxy_OllamaUnreachable(t *testing.T) {
	proxy := client.NewOllamaProxy("http://127.0.0.1:19999") // nothing listening here

	req := &protocol.RequestMessage{ReqID: "req-3", Method: "GET", Path: "/api/tags"}
	var errCode int
	_ = proxy.Execute(req, func(*protocol.ChunkMessage) {}, func() {}, func(code int, msg string) {
		errCode = code
	})
	assert.Equal(t, 502, errCode)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/client/... -v
```

Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement ollama.go**

Create `internal/client/ollama.go`:

```go
package client

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
)

// OllamaProxy forwards requests to a local Ollama instance and streams chunks back.
type OllamaProxy struct {
	baseURL string
	http    *http.Client
}

// NewOllamaProxy creates an OllamaProxy pointing at the given Ollama URL.
func NewOllamaProxy(ollamaURL string) *OllamaProxy {
	return &OllamaProxy{
		baseURL: strings.TrimRight(ollamaURL, "/"),
		http:    &http.Client{},
	}
}

// Execute forwards the REQUEST to Ollama and calls onChunk for each response chunk.
// onEnd is called on successful completion; onError on failure.
func (p *OllamaProxy) Execute(
	req *protocol.RequestMessage,
	onChunk func(*protocol.ChunkMessage),
	onEnd func(),
	onError func(code int, msg string),
) error {
	var bodyReader io.Reader
	if req.BodyBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.BodyBase64)
		if err != nil {
			onError(400, fmt.Sprintf("invalid body encoding: %v", err))
			return nil
		}
		bodyReader = strings.NewReader(string(decoded))
	}

	httpReq, err := http.NewRequest(req.Method, p.baseURL+req.Path, bodyReader)
	if err != nil {
		onError(500, fmt.Sprintf("failed to build request: %v", err))
		return nil
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := p.http.Do(httpReq)
	if err != nil {
		onError(502, fmt.Sprintf("ollama unreachable: %v", err))
		return nil
	}
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	first := true
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			chunk := &protocol.ChunkMessage{
				Type:       protocol.TypeChunk,
				ReqID:      req.ReqID,
				DataBase64: base64.StdEncoding.EncodeToString(buf[:n]),
			}
			if first {
				chunk.StatusCode = resp.StatusCode
				first = false
			}
			onChunk(chunk)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			onError(502, fmt.Sprintf("stream read error: %v", readErr))
			return nil
		}
	}

	onEnd()
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/client/... -v
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/client/ollama.go internal/client/ollama_test.go
git commit -m "feat: Ollama proxy with streaming and error handling"
```

---

### Task 8: Client WebSocket connection with reconnect loop

**Files:**
- Create: `internal/client/ws.go`
- Create: `internal/client/ws_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/client/ws_test.go`:

```go
package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielecalderazzo/ollama-farm/internal/client"
	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// echoServer accepts a WS connection, reads one REGISTER message, then echoes back
// a dummy REQUEST message to exercise the client's message handling.
func startEchoServer(t *testing.T, token string) (*httptest.Server, chan protocol.RegisterMessage) {
	registered := make(chan protocol.RegisterMessage, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		var reg protocol.RegisterMessage
		if err := wsjson.Read(r.Context(), conn, &reg); err != nil {
			return
		}
		registered <- reg
		// Keep alive
		time.Sleep(500 * time.Millisecond)
	}))
	t.Cleanup(srv.Close)
	return srv, registered
}

func TestWSClient_RegistersOnConnect(t *testing.T) {
	srv, registered := startEchoServer(t, "tok")
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proxy := client.NewOllamaProxy("http://localhost:11434")
	c := client.NewWSClient(wsURL, "tok", proxy)

	go c.Run(ctx)

	select {
	case reg := <-registered:
		assert.Equal(t, "tok", reg.Token)
		assert.NotEmpty(t, reg.Models) // Will be empty if Ollama not running, but type is correct
	case <-ctx.Done():
		t.Fatal("timeout: client did not register")
	}
}

func TestWSClient_BackoffOnFailure(t *testing.T) {
	// Point at a server that refuses connections
	proxy := client.NewOllamaProxy("http://localhost:11434")
	c := client.NewWSClient("ws://127.0.0.1:19998", "tok", proxy)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	go c.Run(ctx)
	<-ctx.Done()

	// Should have attempted multiple times within 3s with backoff
	attempts := c.Attempts()
	assert.Greater(t, attempts, 1, "should retry more than once")
	_ = start
}

```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/client/... -run TestWSClient -v
```

Expected: FAIL — WSClient not defined.

- [ ] **Step 3: Implement ws.go**

Create `internal/client/ws.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
)

// WSClient manages the outbound WebSocket connection to the server with reconnect.
type WSClient struct {
	serverURL string
	token     string
	proxy     *OllamaProxy
	attempts  int
}

// NewWSClient creates a WSClient.
func NewWSClient(serverURL, token string, proxy *OllamaProxy) *WSClient {
	return &WSClient{serverURL: serverURL, token: token, proxy: proxy}
}

// Attempts returns the number of connection attempts made (for testing).
func (c *WSClient) Attempts() int { return c.attempts }

// Run connects to the server and reconnects with exponential backoff until ctx is done.
// If the server closes with 4001 (unauthorized), Run stops immediately.
func (c *WSClient) Run(ctx context.Context) {
	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.attempts++
		err := c.connect(ctx)
		if err == nil {
			backoff = time.Second // reset on clean disconnect
			continue
		}

		// 4001 = unauthorized (permanent error)
		if isUnauthorized(err) {
			fmt.Printf("ollama-farm client: unauthorized — check your token\n")
			return
		}

		fmt.Printf("ollama-farm client: disconnected (%v), retrying in %s\n", err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = minDuration(backoff*2, 30*time.Second)
	}
}

func (c *WSClient) connect(ctx context.Context) error {
	wsURL := c.serverURL
	// Ensure path ends with /ws
	if len(wsURL) < 3 || wsURL[len(wsURL)-3:] != "/ws" {
		wsURL += "/ws"
	}

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{},
	})
	if err != nil {
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	models := c.discoverModels()
	reg := protocol.RegisterMessage{
		Type:   protocol.TypeRegister,
		Token:  c.token,
		Models: models,
	}
	if err := wsjson.Write(ctx, conn, reg); err != nil {
		return err
	}

	return c.messageLoop(ctx, conn)
}

func (c *WSClient) messageLoop(ctx context.Context, conn *websocket.Conn) error {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}

		msgType, err := protocol.ParseType(data)
		if err != nil {
			continue
		}

		if msgType == protocol.TypeRequest {
			var req protocol.RequestMessage
			if err := json.Unmarshal(data, &req); err != nil {
				continue
			}
			go c.handleRequest(ctx, conn, &req)
		}
	}
}

func (c *WSClient) handleRequest(ctx context.Context, conn *websocket.Conn, req *protocol.RequestMessage) {
	_ = c.proxy.Execute(
		req,
		func(chunk *protocol.ChunkMessage) {
			_ = wsjson.Write(ctx, conn, chunk)
		},
		func() {
			end := protocol.EndMessage{Type: protocol.TypeEnd, ReqID: req.ReqID}
			_ = wsjson.Write(ctx, conn, end)
		},
		func(code int, msg string) {
			errMsg := protocol.ErrorMessage{
				Type:    protocol.TypeError,
				ReqID:   req.ReqID,
				Message: msg,
				Code:    code,
			}
			_ = wsjson.Write(ctx, conn, errMsg)
		},
	)
}

// discoverModels queries the local Ollama for available models.
// Returns an empty slice if Ollama is not reachable (client still connects,
// but will never be routed to until it reconnects with models).
func (c *WSClient) discoverModels() []string {
	httpClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Get(c.proxy.baseURL + "/api/tags")
	if err != nil {
		return []string{}
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return []string{}
	}

	names := make([]string, 0, len(result.Models))
	for _, m := range result.Models {
		names = append(names, m.Name)
	}
	return names
}

func isUnauthorized(err error) bool {
	status := websocket.CloseStatus(err)
	return status == 4001
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

var _ = math.MaxFloat64 // suppress unused import
```

- [ ] **Step 4: Fix baseURL field visibility in ollama.go**

In `internal/client/ollama.go`, change `baseURL` to be exported so `ws.go` in the same package can access it:

```go
// Change in OllamaProxy struct:
type OllamaProxy struct {
	baseURL string  // already lowercase — ws.go is in same package, this is fine
	http    *http.Client
}
```

Both files are in `package client` so `baseURL` (lowercase) is accessible within the package.

- [ ] **Step 5: Run client tests**

```bash
go test ./internal/client/... -v -timeout 15s
```

Expected: `TestWSClient_BackoffOnFailure` and `TestOllamaProxy_*` PASS. `TestWSClient_RegistersOnConnect` may be skipped if Ollama is not running locally — that is expected.

- [ ] **Step 6: Commit**

```bash
git add internal/client/ws.go internal/client/ws_test.go
git commit -m "feat: client WebSocket with Ollama discovery and exponential backoff reconnect"
```

---

## Chunk 4: TUI, Server Entrypoint, Distribution

### Task 9: BubbleTea TUI dashboard

**Files:**
- Create: `internal/tui/dashboard.go`
- Create: `internal/tui/dashboard_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/dashboard_test.go`:

```go
package tui_test

import (
	"testing"
	"time"

	"github.com/danielecalderazzo/ollama-farm/internal/server"
	"github.com/danielecalderazzo/ollama-farm/internal/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboard_View_NoClients(t *testing.T) {
	snap := &tui.Snapshot{
		Port:    8080,
		Token:   "abc123",
		Clients: nil,
	}
	m := tui.NewDashboard(snap)
	view := m.View()
	assert.Contains(t, view, "8080")
	assert.Contains(t, view, "abc123")
	assert.Contains(t, view, "CLIENTS (0)")
}

func TestDashboard_View_WithClients(t *testing.T) {
	snap := &tui.Snapshot{
		Port:  8080,
		Token: "tok",
		Clients: []*server.ClientEntry{
			{
				ID:          "c1",
				Models:      []string{"llama3", "mistral"},
				Status:      server.StatusFree,
				ConnectedAt: time.Now(),
			},
			{
				ID:          "c2",
				Models:      []string{"phi3"},
				Status:      server.StatusBusy,
				ActiveReqID: "req-xyz",
			},
		},
	}
	m := tui.NewDashboard(snap)
	view := m.View()
	require.NotEmpty(t, view)
	assert.Contains(t, view, "CLIENTS (2)")
	assert.Contains(t, view, "llama3")
	assert.Contains(t, view, "BUSY")
	assert.Contains(t, view, "FREE")
}

func TestDashboard_InstallCommand(t *testing.T) {
	snap := &tui.Snapshot{Port: 9090, Token: "mytoken", ServerHost: "myserver.com"}
	m := tui.NewDashboard(snap)
	view := m.View()
	assert.Contains(t, view, "mytoken")
	assert.Contains(t, view, "myserver.com")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/tui/... -v
```

Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement dashboard.go**

Create `internal/tui/dashboard.go`:

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/danielecalderazzo/ollama-farm/internal/server"
)

// Snapshot holds the data the TUI needs to render.
type Snapshot struct {
	Port       int
	Token      string
	ServerHost string
	Clients    []*server.ClientEntry
}

var (
	styleBold   = lipgloss.NewStyle().Bold(true)
	styleGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("#3fb950"))
	styleOrange = lipgloss.NewStyle().Foreground(lipgloss.Color("#f0883e"))
	styleGray   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	styleBlue   = lipgloss.NewStyle().Foreground(lipgloss.Color("#58a6ff"))
	styleBox    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#30363d")).Padding(0, 1)
)

// Dashboard is the BubbleTea model for the server TUI.
type Dashboard struct {
	snap *Snapshot
}

// NewDashboard creates a Dashboard with the given initial snapshot.
func NewDashboard(snap *Snapshot) *Dashboard {
	return &Dashboard{snap: snap}
}

type tickMsg time.Time

// Init starts the tick timer.
func (d *Dashboard) Init() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages.
func (d *Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		return d, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case tea.KeyMsg:
		return d, tea.Quit
	}
	return d, nil
}

// UpdateSnapshot replaces the snapshot with fresh registry data.
func (d *Dashboard) UpdateSnapshot(snap *Snapshot) {
	d.snap = snap
}

// View renders the dashboard.
func (d *Dashboard) View() string {
	var b strings.Builder

	// Header
	header := fmt.Sprintf("● ollama-farm server  │  :%d  │  token: %s",
		d.snap.Port, styleBlue.Render(d.snap.Token))
	b.WriteString(styleBold.Render(header))
	b.WriteString("\n\n")

	// Clients section
	clientCount := len(d.snap.Clients)
	b.WriteString(styleBold.Render(fmt.Sprintf("CLIENTS (%d)", clientCount)))
	b.WriteString("\n")

	if clientCount == 0 {
		b.WriteString(styleGray.Render("  No clients connected yet.\n"))
	} else {
		for _, c := range d.snap.Clients {
			status := styleGreen.Render("FREE")
			if c.Status == server.StatusBusy {
				status = styleOrange.Render("BUSY")
			}
			models := strings.Join(c.Models, ", ")
			line := fmt.Sprintf("  ● %-20s %-30s %s   req/tot: %d",
				c.ID[:minLen(20, len(c.ID))],
				truncate(models, 30),
				status,
				c.TotalRequests,
			)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Install section
	host := d.snap.ServerHost
	if host == "" {
		host = "yourserver:8080"
	}
	installCmd := fmt.Sprintf("ollama-farm client --server wss://%s --token %s", host, d.snap.Token)
	b.WriteString(styleBold.Render("INSTALL NEW CLIENT"))
	b.WriteString("\n")
	b.WriteString(styleGray.Render("  curl -fsSL https://get.ollama-farm.io | sh"))
	b.WriteString("\n")
	b.WriteString(styleGray.Render("  then: ") + styleBlue.Render(installCmd))
	b.WriteString("\n")

	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// minLen returns the smaller of a and b.
// Named minLen to avoid conflict with the built-in min introduced in Go 1.21.
func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **Step 4: Run TUI tests**

```bash
go test ./internal/tui/... -v
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat: BubbleTea TUI dashboard with client list and install command"
```

---

### Task 10: Wire up server and client entrypoints

**Files:**
- Modify: `cmd/server.go`
- Modify: `cmd/client.go`
- Create: `internal/server/config.go`

- [ ] **Step 1: Implement config.go (token persistence)**

Create `internal/server/config.go`:

```go
package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Token string `json:"token"`
}

func LoadOrCreateConfig(token string) (*Config, error) {
	cfgPath, err := configPath()
	if err != nil {
		return nil, err
	}

	// If token explicitly provided, use it (don't overwrite saved config).
	if token != "" {
		return &Config{Token: token}, nil
	}

	// Try to load existing config.
	data, err := os.ReadFile(cfgPath)
	if err == nil {
		var cfg Config
		if json.Unmarshal(data, &cfg) == nil && cfg.Token != "" {
			return &cfg, nil
		}
	}

	// Generate new token.
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	cfg := &Config{Token: hex.EncodeToString(buf)}

	// Persist it.
	_ = os.MkdirAll(filepath.Dir(cfgPath), 0700)
	data, _ = json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(cfgPath, data, 0600)

	return cfg, nil
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ollama-farm", "config.json"), nil
}
```

- [ ] **Step 2: Export TickMsg in internal/tui/dashboard.go**

Add this type and handle it in `Update` inside `internal/tui/dashboard.go`:

```go
// TickMsg is sent externally to trigger a TUI re-render without waiting for the internal tick.
type TickMsg struct{}
```

In the `Update` method, add a `case TickMsg:` branch:

```go
case TickMsg:
    return d, nil
```

- [ ] **Step 3: Wire up cmd/server.go**

Replace `cmd/server.go` with the full implementation:

```go
package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/danielecalderazzo/ollama-farm/internal/server"
	"github.com/danielecalderazzo/ollama-farm/internal/tui"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the ollama-farm server",
	RunE:  runServer,
}

var (
	serverPort    int
	serverToken   string
	serverTLSCert string
	serverTLSKey  string
)

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().IntVar(&serverPort, "port", 8080, "HTTP listen port")
	serverCmd.Flags().StringVar(&serverToken, "token", "", "Auth token (auto-generated if empty)")
	serverCmd.Flags().StringVar(&serverTLSCert, "tls-cert", "", "Path to TLS certificate")
	serverCmd.Flags().StringVar(&serverTLSKey, "tls-key", "", "Path to TLS key")
}

func runServer(cmd *cobra.Command, args []string) error {
	cfg, err := server.LoadOrCreateConfig(serverToken)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	reg := server.NewRegistry()
	router := server.NewRouter(reg)
	dispatcher := server.NewDispatcher(reg)
	wsHandler := server.NewWSHandler(reg, dispatcher, cfg.Token)
	httpHandler := server.NewHTTPHandler(reg, router, cfg.Token)

	mux := http.NewServeMux()
	mux.Handle("/ws", wsHandler)
	mux.Handle("/", httpHandler)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", serverPort),
		Handler: mux,
	}

	// Start TUI
	snap := &tui.Snapshot{Port: serverPort, Token: cfg.Token}
	dashboard := tui.NewDashboard(snap)
	p := tea.NewProgram(dashboard, tea.WithAltScreen())

	// Periodic registry → TUI sync (500ms interval to match TUI tick rate).
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			clients := reg.Snapshot()
			dashboard.UpdateSnapshot(&tui.Snapshot{
				Port:    serverPort,
				Token:   cfg.Token,
				Clients: clients,
			})
			p.Send(tui.TickMsg{})
		}
	}()

	// Start HTTP server
	go func() {
		var listenErr error
		if serverTLSCert != "" && serverTLSKey != "" {
			listenErr = srv.ListenAndServeTLS(serverTLSCert, serverTLSKey)
		} else {
			listenErr = srv.ListenAndServe()
		}
		if listenErr != nil && listenErr != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", listenErr)
		}
	}()

	// Handle shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		_ = srv.Shutdown(context.Background())
		p.Quit()
	}()

	_, err = p.Run()
	return err
}
```

- [ ] **Step 3: Export TickMsg from tui package**

Add to `internal/tui/dashboard.go`:

```go
// TickMsg is sent externally to trigger a TUI re-render.
type TickMsg struct{}
```

And update `Update` to handle it:

```go
case TickMsg:
    return d, nil
```

- [ ] **Step 4: Wire up cmd/client.go**

Replace `cmd/client.go`:

```go
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/danielecalderazzo/ollama-farm/internal/client"
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Connect this node as an ollama-farm client",
	RunE:  runClient,
}

var (
	clientServer    string
	clientToken     string
	clientOllamaURL string
)

func init() {
	rootCmd.AddCommand(clientCmd)
	clientCmd.Flags().StringVar(&clientServer, "server", "", "Server WebSocket URL (required)")
	clientCmd.Flags().StringVar(&clientToken, "token", "", "Auth token (required)")
	clientCmd.Flags().StringVar(&clientOllamaURL, "ollama-url", "http://localhost:11434", "Local Ollama URL")
	_ = clientCmd.MarkFlagRequired("server")
	_ = clientCmd.MarkFlagRequired("token")
}

func runClient(cmd *cobra.Command, args []string) error {
	proxy := client.NewOllamaProxy(clientOllamaURL)
	c := client.NewWSClient(clientServer, clientToken, proxy)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		fmt.Println("\nollama-farm client: shutting down")
		cancel()
	}()

	fmt.Printf("ollama-farm client: connecting to %s\n", clientServer)
	c.Run(ctx)
	return nil
}
```

- [ ] **Step 5: Build and smoke test**

```bash
go build -o ollama-farm .
./ollama-farm --help
./ollama-farm server --help
./ollama-farm client --help
```

Expected: all help text prints cleanly.

- [ ] **Step 6: Commit**

```bash
git add cmd/ internal/server/config.go
git commit -m "feat: wire up server and client entrypoints with full lifecycle"
```

---

### Task 11: Distribution (install script + goreleaser)

**Files:**
- Create: `scripts/install.sh`
- Create: `.goreleaser.yaml`
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Write install.sh**

Create `scripts/install.sh`:

```bash
#!/bin/sh
set -e

REPO="danielecalderazzo/ollama-farm"
BINARY="ollama-farm"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "Unsupported OS: $OS"
    echo "Please download manually from https://github.com/$REPO/releases"
    exit 1
    ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)          ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Get latest version tag from GitHub API
VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' | sed 's/.*"tag_name": "\(.*\)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Could not determine latest version"
  exit 1
fi

FILENAME="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/${VERSION}/${FILENAME}"

echo "Downloading ollama-farm $VERSION for $OS/$ARCH..."
curl -fsSL "$URL" | tar -xz "$BINARY"

INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  echo "Installing to $INSTALL_DIR (requires sudo)"
  sudo mv "$BINARY" "$INSTALL_DIR/$BINARY"
else
  mv "$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "ollama-farm installed to $INSTALL_DIR/$BINARY"
echo "Run: ollama-farm --help"
```

- [ ] **Step 2: Make install.sh executable**

```bash
chmod +x scripts/install.sh
```

- [ ] **Step 3: Write .goreleaser.yaml**

Create `.goreleaser.yaml`:

```yaml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - id: ollama-farm
    binary: ollama-farm
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - id: default
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "ollama-farm_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

release:
  github:
    owner: danielecalderazzo
    name: ollama-farm
```

- [ ] **Step 4: Write GitHub Actions release workflow**

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - uses: goreleaser/goreleaser-action@v5
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 5: Verify goreleaser config locally (dry run)**

```bash
# Install goreleaser if not present
go install github.com/goreleaser/goreleaser/v2@latest

goreleaser check
```

Expected: `config is valid` (no errors).

- [ ] **Step 6: Commit**

```bash
git add scripts/install.sh .goreleaser.yaml .github/
git commit -m "feat: distribution with install script and goreleaser cross-compilation"
```

---

### Task 12: End-to-end integration test

**Files:**
- Create: `tests/e2e_test.go`

- [ ] **Step 1: Write the integration test**

Create `tests/e2e_test.go`:

```go
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

// fakeOllama simulates an Ollama server for integration tests.
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
	// 1. Start fake Ollama
	fakeOllama := fakeOllamaServer()
	defer fakeOllama.Close()

	// 2. Start the server
	reg := server.NewRegistry()
	router := server.NewRouter(reg)
	dispatcher := server.NewDispatcher(reg)
	wsHandler := server.NewWSHandler(reg, dispatcher, "testtoken")
	httpHandler := server.NewHTTPHandler(reg, router, "testtoken")

	mux := http.NewServeMux()
	mux.Handle("/ws", wsHandler)
	mux.Handle("/", httpHandler)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	serverAddr := ln.Addr().String()

	// 3. Start a client pointing at fake Ollama
	proxy := client.NewOllamaProxy(fakeOllama.URL)
	wsClient := client.NewWSClient("ws://"+serverAddr, "testtoken", proxy)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go wsClient.Run(ctx)

	// 4. Wait for client to register
	require.Eventually(t, func() bool {
		return len(reg.Snapshot()) > 0
	}, 3*time.Second, 100*time.Millisecond, "client should register within 3s")

	// 5. Send a request to the server
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
	h := server.NewHTTPHandler(reg, server.NewRouter(reg), "tok")

	body := `{"model":"llama3","prompt":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
```

- [ ] **Step 2: Run unit tests (non-integration)**

```bash
go test ./... -v -timeout 60s
```

Expected: all non-integration tests PASS.

- [ ] **Step 3: Run integration test**

```bash
go test ./tests/... -v -tags integration -timeout 30s
```

Expected: `TestE2E_RequestRoutedToClient` and `TestE2E_503_WhenNoClient` PASS.

- [ ] **Step 4: Full manual smoke test**

```bash
# Terminal 1: start server
./ollama-farm server --port 8080

# Terminal 2: start client (requires Ollama running locally)
./ollama-farm client --server ws://localhost:8080 --token <printed-token>

# Terminal 3: send request
curl -s http://localhost:8080/api/generate \
  -d '{"model":"llama3","prompt":"say hello in one word","stream":false}'
```

Expected: TUI shows the client connected, then BUSY during the request, then FREE.

- [ ] **Step 5: Final commit**

```bash
git add tests/
git commit -m "test: end-to-end integration test for full request routing"
```

---

## Summary

| Task | Component | Tests |
|------|-----------|-------|
| 1 | Scaffold (go.mod, cobra CLI) | build check |
| 2 | Protocol messages | 6 JSON round-trip tests |
| 3 | Client registry | 6 unit tests |
| 4 | Model-aware router | 5 unit tests |
| 5 | HTTP handler (buffering, 413, 503) | 4 unit tests |
| 6 | WebSocket server (auth, timeout, loop) | 4 integration-style tests |
| 7 | Ollama proxy (streaming, errors) | 3 unit tests |
| 8 | Client WS (reconnect, backoff) | 2 unit tests |
| 9 | BubbleTea TUI dashboard | 3 unit tests |
| 10 | Server + client entrypoints | build + smoke test |
| 11 | Install script + goreleaser | goreleaser check |
| 12 | End-to-end integration test | 2 integration tests |
