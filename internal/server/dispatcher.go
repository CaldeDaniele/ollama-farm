package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
)

// Dispatcher sends REQUEST messages to clients and streams responses back.
type Dispatcher struct {
	registry *Registry
	mu       sync.Mutex
	pending  map[string]chan *protocol.ChunkMessage
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

	ch := make(chan *protocol.ChunkMessage, 64)
	d.RegisterPending(reqID, ch)
	defer d.RemovePending(reqID)

	flusher, canFlush := w.(http.Flusher)
	headersSent := false

	for chunk := range ch {
		if chunk == nil {
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
	d.mu.Lock()
	d.pending[reqID] = ch
	d.mu.Unlock()
}

func (d *Dispatcher) RemovePending(reqID string) {
	d.mu.Lock()
	delete(d.pending, reqID)
	d.mu.Unlock()
}

// DeliverChunk delivers a chunk to the waiting HTTP handler.
func (d *Dispatcher) DeliverChunk(chunk *protocol.ChunkMessage) {
	d.mu.Lock()
	ch, ok := d.pending[chunk.ReqID]
	d.mu.Unlock()
	if ok {
		ch <- chunk
	}
}

// DeliverEnd closes the pending channel for a request, signaling completion.
func (d *Dispatcher) DeliverEnd(reqID string) {
	d.mu.Lock()
	ch, ok := d.pending[reqID]
	if ok {
		delete(d.pending, reqID)
	}
	d.mu.Unlock()
	if ok {
		ch <- nil
		close(ch)
	}
}

// DeliverError closes the pending channel after error.
func (d *Dispatcher) DeliverError(reqID string) {
	d.mu.Lock()
	ch, ok := d.pending[reqID]
	if ok {
		delete(d.pending, reqID)
	}
	d.mu.Unlock()
	if ok {
		ch <- nil
		close(ch)
	}
}
