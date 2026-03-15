package server

import (
	"context"
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

// PickWait blocks until a FREE client with the model exists (or ctx done). Same round-robin as Pick.
func (rt *Router) PickWait(ctx context.Context, model string) (string, error) {
	for {
		id, err := rt.Pick(model)
		if err == nil {
			return id, nil
		}
		if err != ErrNoClientAvailable {
			return "", err
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := rt.registry.WaitForPickCapacity(ctx); err != nil {
			return "", err
		}
	}
}

// PickAny selects any FREE client (round-robin). Used for requests without a model (e.g. GET /api/tags).
func (rt *Router) PickAny() (string, error) {
	candidates := rt.registry.FreeAny()
	if len(candidates) == 0 {
		return "", ErrNoClientAvailable
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ID < candidates[j].ID
	})
	rt.mu.Lock()
	idx := rt.counters[""] % len(candidates)
	rt.counters[""] = idx + 1
	rt.mu.Unlock()
	return candidates[idx].ID, nil
}

// PickAnyWait blocks until any FREE client exists (or ctx done).
func (rt *Router) PickAnyWait(ctx context.Context) (string, error) {
	for {
		id, err := rt.PickAny()
		if err == nil {
			return id, nil
		}
		if err != ErrNoClientAvailable {
			return "", err
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := rt.registry.WaitForPickCapacity(ctx); err != nil {
			return "", err
		}
	}
}
