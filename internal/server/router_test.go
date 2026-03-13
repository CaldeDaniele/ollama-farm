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
	assert.Equal(t, 2, len(seen))
}

func TestRouter_WrongModelSkipped(t *testing.T) {
	r := makeRegistry(&server.ClientEntry{ID: "c1", Models: []string{"phi3"}, Status: server.StatusFree})
	router := server.NewRouter(r)

	_, err := router.Pick("llama3")
	require.Error(t, err)
	assert.ErrorIs(t, err, server.ErrNoClientAvailable)
}
