package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielecalderazzo/ollama-farm/internal/client"
	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
	"github.com/stretchr/testify/assert"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func startEchoServer(t *testing.T, token string) (*httptest.Server, chan protocol.RegisterMessage) {
	registered := make(chan protocol.RegisterMessage, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		var reg protocol.RegisterMessage
		if err := wsjson.Read(r.Context(), conn, &reg); err != nil {
			return
		}
		registered <- reg
		time.Sleep(500 * time.Millisecond)
	}))
	t.Cleanup(srv.Close)
	return srv, registered
}

func TestWSClient_RegistersOnConnect(t *testing.T) {
	srv, registered := startEchoServer(t, "tok")
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proxy := client.NewOllamaProxy("http://localhost:11434")
	c := client.NewWSClient(wsURL, "tok", proxy)

	go c.Run(ctx)

	select {
	case reg := <-registered:
		assert.Equal(t, "tok", reg.Token)
		assert.NotEmpty(t, reg.Models)
	case <-ctx.Done():
		t.Fatal("timeout: client did not register")
	}
}

func TestWSClient_BackoffOnFailure(t *testing.T) {
	proxy := client.NewOllamaProxy("http://localhost:11434")
	c := client.NewWSClient("ws://127.0.0.1:19998", "tok", proxy)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	go c.Run(ctx)
	<-ctx.Done()

	attempts := c.Attempts()
	assert.Greater(t, attempts, 1, "should retry more than once")
	_ = start
}

// Ensure json import is used in test helpers.
var _ = json.Marshal
