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
