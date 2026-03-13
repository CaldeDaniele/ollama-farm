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
