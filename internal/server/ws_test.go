package server_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
	"github.com/danielecalderazzo/ollama-farm/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func startTestServer(t *testing.T, token string) (*httptest.Server, *server.Registry) {
	t.Helper()
	reg := server.NewRegistry()
	router := server.NewRouter(reg)
	dispatcher := server.NewDispatcher(reg)
	wsHandler := server.NewWSHandler(reg, dispatcher, token)
	_ = router
	srv := httptest.NewServer(wsHandler)
	t.Cleanup(srv.Close)
	return srv, reg
}

func TestWSHandler_ValidRegistration(t *testing.T) {
	srv, reg := startTestServer(t, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	regMsg := protocol.RegisterMessage{
		Type:   protocol.TypeRegister,
		Token:  "secret",
		Models: []string{"llama3"},
	}
	require.NoError(t, wsjson.Write(ctx, conn, regMsg))

	time.Sleep(100 * time.Millisecond)

	clients := reg.Snapshot()
	require.Len(t, clients, 1)
	assert.Equal(t, []string{"llama3"}, clients[0].Models)
	assert.Equal(t, server.StatusFree, clients[0].Status)
}

func TestWSHandler_InvalidToken_Rejected(t *testing.T) {
	srv, _ := startTestServer(t, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)

	bad := protocol.RegisterMessage{
		Type:   protocol.TypeRegister,
		Token:  "wrong",
		Models: []string{"llama3"},
	}
	require.NoError(t, wsjson.Write(ctx, conn, bad))

	_, _, readErr := conn.Read(ctx)
	assert.Error(t, readErr)
}

func TestWSHandler_RegistrationTimeout(t *testing.T) {
	srv, _ := startTestServer(t, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)

	readCtx, readCancel := context.WithTimeout(ctx, 7*time.Second)
	defer readCancel()

	_, _, readErr := conn.Read(readCtx)
	assert.Error(t, readErr, "server should close connection after registration timeout")
}

func TestWSHandler_ClientRemovedOnDisconnect(t *testing.T) {
	srv, reg := startTestServer(t, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)

	regMsg := protocol.RegisterMessage{
		Type:   protocol.TypeRegister,
		Token:  "secret",
		Models: []string{"llama3"},
	}
	require.NoError(t, wsjson.Write(ctx, conn, regMsg))
	time.Sleep(100 * time.Millisecond)

	require.Len(t, reg.Snapshot(), 1)

	conn.Close(websocket.StatusNormalClosure, "bye")
	time.Sleep(200 * time.Millisecond)

	assert.Empty(t, reg.Snapshot())
}
