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
