package client

import (
	"context"
	"encoding/json"
	"fmt"
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
			backoff = time.Second
			continue
		}

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
