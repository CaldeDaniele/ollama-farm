package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
)

const (
	registrationTimeout = 5 * time.Second
	pingInterval        = 15 * time.Second
	pongTimeout         = 10 * time.Second
)

// WSHandler upgrades HTTP connections to WebSocket and manages the client lifecycle.
type WSHandler struct {
	registry   *Registry
	dispatcher *Dispatcher
	token      string
}

// NewWSHandler creates a WSHandler.
func NewWSHandler(reg *Registry, dispatcher *Dispatcher, token string) *WSHandler {
	return &WSHandler{registry: reg, dispatcher: dispatcher, token: token}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	ctx := r.Context()

	clientID, err := h.awaitRegistration(ctx, conn)
	if err != nil {
		return
	}
	defer h.registry.Remove(clientID)

	h.messageLoop(ctx, conn, clientID)
}

func (h *WSHandler) awaitRegistration(ctx context.Context, conn *websocket.Conn) (string, error) {
	regCtx, cancel := context.WithTimeout(ctx, registrationTimeout)
	defer cancel()

	_, data, err := conn.Read(regCtx)
	if err != nil {
		conn.Close(websocket.StatusGoingAway, "registration timeout")
		return "", err
	}

	var msg protocol.RegisterMessage
	if err := json.Unmarshal(data, &msg); err != nil || msg.Type != protocol.TypeRegister {
		conn.Close(websocket.StatusCode(4002), "invalid registration message")
		return "", err
	}

	if msg.Token != h.token {
		conn.Close(websocket.StatusCode(4001), "unauthorized")
		return "", errUnauthorized
	}

	clientID := uuid.New().String()
	h.registry.Add(&ClientEntry{
		ID:          clientID,
		Models:      msg.Models,
		Status:      StatusFree,
		ConnectedAt: time.Now(),
		Conn:        conn,
	})

	return clientID, nil
}

var errUnauthorized = &wsError{"unauthorized"}

type wsError struct{ msg string }

func (e *wsError) Error() string { return e.msg }

func (h *WSHandler) messageLoop(ctx context.Context, conn *websocket.Conn, clientID string) {
	pingCtx, pingCancel := context.WithCancel(ctx)
	defer pingCancel()
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-ticker.C:
				pongCtx, pongCancel := context.WithTimeout(pingCtx, pongTimeout)
				err := conn.Ping(pongCtx)
				pongCancel()
				if err != nil {
					h.registry.Remove(clientID)
					conn.Close(websocket.StatusGoingAway, "ping timeout")
					pingCancel()
					return
				}
			}
		}
	}()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		msgType, err := protocol.ParseType(data)
		if err != nil {
			continue
		}

		switch msgType {
		case protocol.TypeChunk:
			var chunk protocol.ChunkMessage
			if err := json.Unmarshal(data, &chunk); err == nil {
				h.dispatcher.DeliverChunk(&chunk)
			}

		case protocol.TypeEnd:
			var end protocol.EndMessage
			if err := json.Unmarshal(data, &end); err == nil {
				h.registry.SetStatus(clientID, StatusFree, "")
				h.dispatcher.DeliverEnd(end.ReqID)
			}

		case protocol.TypeError:
			var errMsg protocol.ErrorMessage
			if err := json.Unmarshal(data, &errMsg); err == nil {
				h.registry.SetStatus(clientID, StatusFree, "")
				h.dispatcher.DeliverError(errMsg.ReqID)
			}
		}
	}
}
