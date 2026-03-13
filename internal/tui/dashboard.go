package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

// TickMsg is sent externally to trigger a TUI re-render.
type TickMsg struct{}

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
	case TickMsg:
		return d, nil
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

	header := fmt.Sprintf("● ollama-farm server  │  :%d  │  token: %s",
		d.snap.Port, styleBlue.Render(d.snap.Token))
	b.WriteString(styleBold.Render(header))
	b.WriteString("\n\n")

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
			idLen := minLen(20, len(c.ID))
			line := fmt.Sprintf("  ● %-20s %-30s %s   req/tot: %d",
				c.ID[:idLen],
				truncate(models, 30),
				status,
				c.TotalRequests,
			)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	host := d.snap.ServerHost
	if host == "" {
		host = fmt.Sprintf("YOUR_SERVER:%d", d.snap.Port)
	}

	b.WriteString(styleBold.Render("INSTALL NEW CLIENT (one command, from this server)"))
	b.WriteString("\n")
	b.WriteString(styleGray.Render("  Sostituisci YOUR_SERVER con l'indirizzo di questo server (es. farm.example.com)"))
	b.WriteString("\n")
	b.WriteString(styleGray.Render("  macOS:   "))
	b.WriteString(styleBlue.Render(fmt.Sprintf("curl -fsSL https://%s/install.sh | sh -s -- --token %s", host, d.snap.Token)))
	b.WriteString("\n")
	b.WriteString(styleGray.Render("  Linux:   "))
	b.WriteString(styleBlue.Render(fmt.Sprintf("curl -fsSL https://%s/install.sh | sh -s -- --token %s", host, d.snap.Token)))
	b.WriteString("\n")
	b.WriteString(styleGray.Render("  Windows: "))
	b.WriteString(styleBlue.Render(fmt.Sprintf("$env:OLLAMA_FARM_TOKEN='%s'; irm -useb https://%s/install.ps1 | iex", d.snap.Token, host)))
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
func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}
