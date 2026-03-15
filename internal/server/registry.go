package server

import (
	"context"
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
	mu       sync.RWMutex
	clients  map[string]*ClientEntry
	pickWait sync.Mutex
	pickCond *sync.Cond // Broadcast when a client becomes FREE or is added (queue waiters)
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	reg := &Registry{clients: make(map[string]*ClientEntry)}
	reg.pickCond = sync.NewCond(&reg.pickWait)
	return reg
}

func (r *Registry) signalPickWaiters() {
	r.pickWait.Lock()
	r.pickCond.Broadcast()
	r.pickWait.Unlock()
}

// Add inserts a new client entry. Overwrites if the ID already exists.
func (r *Registry) Add(e *ClientEntry) {
	r.mu.Lock()
	r.clients[e.ID] = e
	r.mu.Unlock()
	r.signalPickWaiters()
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
	cp := *e
	return &cp, true
}

// SetStatus updates the status and active request ID for a client.
func (r *Registry) SetStatus(id string, status ClientStatus, reqID string) {
	r.mu.Lock()
	if e, ok := r.clients[id]; ok {
		e.Status = status
		e.ActiveReqID = reqID
	}
	r.mu.Unlock()
	if status == StatusFree {
		r.signalPickWaiters()
	}
}

// WaitForPickCapacity blocks until signalPickWaiters (FREE client or new client) or ctx cancelled.
// Caller should retry Pick after wake. Used to queue HTTP requests when all workers are busy.
func (r *Registry) WaitForPickCapacity(ctx context.Context) error {
	r.pickWait.Lock()
	defer r.pickWait.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			r.pickWait.Lock()
			r.pickCond.Broadcast()
			r.pickWait.Unlock()
		case <-stop:
		}
	}()
	r.pickCond.Wait()
	return ctx.Err()
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

// FreeAny returns all FREE clients (for requests that don't require a specific model, e.g. GET /api/tags).
func (r *Registry) FreeAny() []*ClientEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*ClientEntry
	for _, e := range r.clients {
		if e.Status == StatusFree {
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
func (r *Registry) GetConn(id string) *websocket.Conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.clients[id]; ok {
		return e.Conn
	}
	return nil
}
