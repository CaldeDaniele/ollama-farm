# ollama-farm — Design Spec
**Date:** 2026-03-12
**Status:** Approved

---

## Context

Running multiple machines with GPUs and Ollama installed creates a fragmentation problem: each node has its own endpoint, and callers must know which node is available and which models it has loaded. This tool solves that by placing a single public-facing server in front of all Ollama nodes. Callers interact with one endpoint as if it were a single Ollama instance; the server transparently routes requests to the right available node.

---

## System Overview

**ollama-farm** is a single Go binary with two modes:

- `ollama-farm server` — public-facing HTTP proxy with terminal dashboard
- `ollama-farm client` — connects outbound to the server, proxies requests to local Ollama

Clients always initiate the connection (outbound WebSocket), so they work behind NAT/firewalls with no port forwarding required.

---

## Architecture

```
CALLER ──HTTP──▶ SERVER ──WS (persistent)──▶ CLIENT(s)
                  :8080                         │
                  │                             ▼
                  │                       Ollama :11434
                  │
                  └── TUI (BubbleTea dashboard)
```

### Request Flow

1. Caller sends `POST /api/generate` (or any Ollama-compatible HTTP request)
2. Server buffers the full request body (max 10MB; returns 413 if exceeded) and reads the `model` field from the JSON body. The buffered body is what gets forwarded — the original `r.Body` is not re-read.
3. Router selects a FREE client that has the model loaded (round-robin if multiple)
4. Server serializes the HTTP request (method, path, headers, base64-encoded body) as a `REQUEST` message on the client's WebSocket channel and marks the client BUSY
5. Client decodes the body (base64), forwards the request to local Ollama (`--ollama-url`, default `http://localhost:11434`) and streams each response chunk back as `CHUNK` messages
6. Server streams chunks directly to the original HTTP caller
7. On stream end, client sends `END`; server marks the client FREE and updates the registry. On error, client sends `ERROR`; server returns 502 to caller and marks the client FREE.

**Explicit constraint:** Each client handles exactly one request at a time. No pipelining. This simplifies the protocol and is sufficient since Ollama itself is single-threaded per model on most hardware.

---

## WebSocket Protocol (JSON messages)

All messages share a `type` field. Direction noted per message.

| Type | Direction | Fields |
|------|-----------|--------|
| `REGISTER` | client→server | `token`, `models[]` |
| `REQUEST` | server→client | `req_id`, `method`, `path`, `headers{}`, `body_b64` (base64 string) |
| `CHUNK` | client→server | `req_id`, `data` (raw bytes as base64), `status_code` (first chunk only) |
| `END` | client→server | `req_id` — signals request complete; server marks client FREE |
| `ERROR` | client→server | `req_id`, `message`, `code` — server returns 502 and marks client FREE |

**No `STATUS` message.** Client availability is derived exclusively from `END`/`ERROR` receipt, not from a separate status message. This avoids race conditions between two "request done" signals.

### Registration Handshake

After the WebSocket upgrade, the server waits up to **5 seconds** for a `REGISTER` message:
- If none arrives within 5s → close with code 4002 (registration timeout)
- If token is invalid → close with code 4001 (unauthorized); client stops retrying (permanent error)
- If token is valid → client is added to registry as FREE with its declared models

**Model list is static per connection.** The `models[]` declared at `REGISTER` are trusted for the lifetime of that connection. If models change on disk (added/removed from Ollama), the client must reconnect to advertise the updated list. This is a known limitation; a future `MODELS_UPDATE` message could address it.

---

## Routing Logic

```
incoming request (model: X)
  → buffer body, extract "model" field
  → if body > 10MB → 413 Request Entity Too Large
  → filter registry: clients with model X
  → filter: status == FREE
  → if empty → 503 {"error": "no client available for model X"}
  → else → round-robin among free clients
  → mark selected client BUSY
  → dispatch REQUEST
```

**Round-robin state:** One `map[string]*int` counter per model name, stored in the router. The counter indexes into a sorted slice of FREE clients for that model. On each dispatch the counter increments mod len(free clients). When a client disconnects, the router rebuilds the free-client slice; the counter wraps naturally.

---

## Client Registry

In-memory map on the server, keyed by client ID (UUID assigned at REGISTER). Protected by a `sync.RWMutex`: read-lock for routing and TUI reads, write-lock for REGISTER/disconnect/status changes.

```go
type ClientEntry struct {
    ID           string
    Models       []string
    Status       ClientStatus  // FREE | BUSY
    ActiveReqID  string        // current req_id or ""
    ConnectedAt  time.Time
    TotalRequests int
    Conn         *websocket.Conn
}

type Registry struct {
    mu      sync.RWMutex
    clients map[string]*ClientEntry
}
```

The TUI reads the registry via `RLock` to render the dashboard; this never blocks request dispatch (which uses the same `RLock` for reads, `Lock` only for writes).

---

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| No client available for requested model | HTTP 503 + `{"error":"no client available for model X"}` |
| Body > 10MB | HTTP 413 before routing |
| Client disconnects mid-request | HTTP 502 to caller; client removed from registry; no retry |
| Client reconnects after disconnect | Exponential backoff (1s→2s→4s→8s→max 30s); re-registers with fresh REGISTER |
| Invalid token | WS close 4001; client stops (no retry — permanent error) |
| No REGISTER within 5s of WS upgrade | WS close 4002; client retries with backoff |
| Ollama unreachable on client | Client sends `ERROR`; server returns 502; client stays FREE |
| Zombie client (no pong) | Removed after 10s timeout on 15s ping cycle |

---

## TUI Dashboard (BubbleTea)

Compact single-screen layout, auto-refreshing every 500ms via BubbleTea's `tea.Tick`:

```
● ollama-farm server v1.0  │  :8080  │  token: abc123xx
┌─── CLIENTS (3) ──────────────────────────────────────────┐
│ ● gpu-server-01   llama3, mistral   BUSY   req/tot: 1/142 │
│ ● gpu-server-02   llama3, phi3      FREE   req/tot: 0/89  │
│ ● macbook-pro     phi3              FREE   req/tot: 0/23  │
└──────────────────────────────────────────────────────────┘
┌─── ACTIVE REQUESTS (1) ──────────────────────────────────┐
│ → POST /api/generate  model:llama3  gpu-01  1.2s          │
└──────────────────────────────────────────────────────────┘
┌─── INSTALL NEW CLIENT ────────────────────────────────────┐
│ curl -fsSL https://get.ollama-farm.io | sh                │
│ ollama-farm client --server wss://yourhost:8080 \         │
│   --token abc123xx                                        │
└──────────────────────────────────────────────────────────┘
```

---

## CLI Interface

```bash
# Server
ollama-farm server [--port 8080] [--token <string>] [--tls-cert <path>] [--tls-key <path>]

# Client
ollama-farm client --server wss://host:8080 --token <string> [--ollama-url http://localhost:11434]
```

Token is auto-generated on first `server` run and saved to `~/.ollama-farm/config.json`. Shown prominently in TUI and in the install command snippet.

---

## Project Structure

```
ollama-farm/
├── main.go                    # cobra CLI root
├── cmd/
│   ├── server.go              # `server` subcommand
│   └── client.go              # `client` subcommand
├── internal/
│   ├── server/
│   │   ├── http.go            # HTTP listener, body buffering, 10MB cap
│   │   ├── ws.go              # WebSocket upgrade, registration timeout, message loop
│   │   ├── registry.go        # ClientEntry map, sync.RWMutex protected
│   │   └── router.go          # model-aware round-robin selector + per-model counters
│   ├── client/
│   │   ├── ws.go              # outbound WS conn + exponential backoff reconnect loop
│   │   └── ollama.go          # local Ollama proxy + streaming, --ollama-url
│   ├── protocol/
│   │   └── messages.go        # shared JSON message types (REGISTER, REQUEST, CHUNK, END, ERROR)
│   └── tui/
│       └── dashboard.go       # BubbleTea model + view, 500ms tick
├── scripts/
│   └── install.sh             # OS/arch detection + binary download (Linux/macOS only)
├── .github/workflows/
│   └── release.yml            # goreleaser cross-compilation
├── go.mod
└── go.sum
```

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/lipgloss` | TUI styling |
| `nhooyr.io/websocket` | WebSocket client + server |
| `github.com/google/uuid` | Unique request IDs |

---

## Distribution

- **Binaries:** goreleaser builds `darwin/linux/windows × amd64/arm64` on GitHub tag push
- **Install script (Linux/macOS):** `scripts/install.sh` detects OS/arch via `uname`, downloads the right binary from GitHub Releases, places it in `/usr/local/bin`
- **One-liner (Linux/macOS):** `curl -fsSL https://get.ollama-farm.io | sh`
- **Windows:** Manual download of the `.exe` from GitHub Releases (the `curl | sh` one-liner does not work natively on Windows)

---

## Verification

```bash
# 1. Start server
ollama-farm server --port 8080
# → TUI appears, token printed, WS endpoint ready at ws://localhost:8080/ws

# 2. Connect a client (separate terminal)
ollama-farm client --server ws://localhost:8080 --token <token>
# → Client appears in TUI as FREE with its models listed

# 3. Send a request as if talking to Ollama directly
curl http://localhost:8080/api/generate \
  -d '{"model":"llama3","prompt":"say hello","stream":true}'
# → Streaming response arrives from the client node; tokens appear progressively

# 4. Test unavailable model
curl http://localhost:8080/api/generate \
  -d '{"model":"nonexistent","prompt":"test"}'
# → HTTP 503 {"error":"no client available for model nonexistent"}

# 5. Test client disconnect mid-stream
#    In terminal 2, while a streaming request is running:
kill -9 <client-pid>
# → Caller receives HTTP 502; TUI removes the client from the registry immediately

# 6. Test body size limit
curl http://localhost:8080/api/generate \
  -d "$(python3 -c "import json; print(json.dumps({'model':'llama3','prompt':'x'*10_000_001}))")"
# → HTTP 413 Request Entity Too Large
```
