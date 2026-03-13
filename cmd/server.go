package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	serverPort     int
	serverToken    string
	serverHost     string
	serverReleases string
	serverTLSCert  string
	serverTLSKey   string
	serverNoTUI    bool
)

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().IntVar(&serverPort, "port", 8080, "HTTP listen port")
	serverCmd.Flags().StringVar(&serverToken, "token", "", "Auth token (auto-generated if empty)")
	serverCmd.Flags().StringVar(&serverHost, "host", "", "Public host for install URL (es. farm.example.com); if set, TUI shows ready-to-copy one-liner")
	serverCmd.Flags().StringVar(&serverReleases, "releases-dir", "", "Optional folder with pre-built binaries (ollama-farm_<os>_<arch>.tar.gz or .zip); used when GitHub has no release")
	serverCmd.Flags().StringVar(&serverTLSCert, "tls-cert", "", "Path to TLS certificate")
	serverCmd.Flags().StringVar(&serverTLSKey, "tls-key", "", "Path to TLS key")
	serverCmd.Flags().BoolVar(&serverNoTUI, "no-tui", false, "Run without TUI (for background/headless)")
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
	httpHandler := server.NewHTTPHandler(reg, router, dispatcher, cfg.Token)

	mux := http.NewServeMux()
	mux.Handle("/ws", wsHandler)
	mux.Handle("/install.sh", server.InstallScriptHandler())
	mux.Handle("/install", server.InstallScriptHandler())
	mux.Handle("/install.ps1", server.InstallPS1Handler())
	mux.Handle("/download/", server.DownloadProxyHandler(serverReleases))
	mux.Handle("/", httpHandler)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", serverPort),
		Handler: mux,
	}

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	if serverNoTUI {
		host := serverHost
		if host == "" {
			host = fmt.Sprintf("localhost:%d", serverPort)
		}
		scheme := "http"
		if serverTLSCert != "" && serverTLSKey != "" {
			scheme = "https"
		}
		fmt.Fprintf(os.Stderr, "ollama-farm server listening on :%d\n", serverPort)
		fmt.Fprintf(os.Stderr, "token: %s\n", cfg.Token)
		fmt.Fprintf(os.Stderr, "install + run client (one command):\n  curl -fsSL %s://%s/install.sh | sh -s -- --token %q\n", scheme, host, cfg.Token)
		<-quit
		_ = srv.Shutdown(context.Background())
		return nil
	}

	snap := &tui.Snapshot{Port: serverPort, Token: cfg.Token, ServerHost: serverHost}
	dashboard := tui.NewDashboard(snap)
	p := tea.NewProgram(dashboard, tea.WithAltScreen())

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			clients := reg.Snapshot()
			dashboard.UpdateSnapshot(&tui.Snapshot{
				Port:       serverPort,
				Token:      cfg.Token,
				ServerHost: serverHost,
				Clients:    clients,
			})
			p.Send(tui.TickMsg{})
		}
	}()

	go func() {
		<-quit
		_ = srv.Shutdown(context.Background())
		p.Quit()
	}()

	_, err = p.Run()
	return err
}
