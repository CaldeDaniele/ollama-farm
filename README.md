# ollama-farm

**A single HTTP endpoint for multiple Ollama nodes.** A public server receives Ollama API requests and routes them over WebSocket to connected GPU clients. Callers see one “Ollama” while load is distributed round-robin across free nodes.

---

## Table of contents

- [How it works](#how-it-works)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage: Server](#usage-server)
- [Usage: Client](#usage-client)
- [Full examples](#full-examples)
- [Protocol](#protocol)
- [Limitations](#limitations)
- [Development](#development)

---

## How it works

```
CALLER ──HTTP──▶ SERVER ──WebSocket (persistent)──▶ CLIENT(1..N)
  (curl, app)      :8080                                  │
                       │                                  ▼
                       │                           Ollama :11434
                       │
                       └── TUI (terminal dashboard)
```

- **Server** (`ollama-farm server`): listens on HTTP (e.g. `:8080`), exposes the same APIs as Ollama (`/api/generate`, `/api/chat`, `/api/tags`, etc.) and keeps a registry of clients connected via WebSocket. For each request it reads the `model` field from the JSON body, picks a **free** client that has that model (round-robin when several are available), sends the request over the WebSocket tunnel and streams the response back to the caller.

- **Client** (`ollama-farm client`): connects **outbound** to the server (no open ports, works behind NAT/firewalls). On startup it queries local Ollama (`/api/tags`) for the model list, registers with the server (token + models) and waits. When a request arrives from the server, it forwards it to local Ollama and sends back chunk, end or error messages.

- **TUI**: the server shows a dashboard (BubbleTea) with port, token and client list (FREE/BUSY, models, total requests), plus the command to install new clients.

**Request flow**

1. The caller sends `POST /api/generate` (or another Ollama API) to the server.
2. The server buffers the body (max 10 MB; 413 if larger), reads `model` from the JSON.
3. The router selects a FREE client with that model (round-robin).
4. The server sends a `REQUEST` message (method, path, headers, body in base64) on the client’s WebSocket and marks the client BUSY.
5. The client decodes the body, issues the request to local Ollama and sends each response chunk as a `CHUNK` message.
6. The server forwards chunks to the HTTP caller (streaming).
7. When the stream ends the client sends `END`; the server marks the client FREE. On error it sends `ERROR` and the server returns 502 to the caller.

Each client handles **one request at a time** (no pipelining), consistent with Ollama’s model.

---

## Requirements

- **Server**: Go 1.22+ (or a prebuilt binary). No Ollama on the server.
- **Client**: Go 1.22+ (or binary) and **Ollama running** on the same machine (default `http://localhost:11434`).
- Network: clients must be able to reach the server (WebSocket); the server must be reachable by callers (HTTP).

---

## Installation

### On a client: one command (from your server)

If you already have an ollama-farm server running, on each client machine you can use **one command** to download the binary from the server and start the client (replace `YOUR_SERVER` with the server address, e.g. `farm.example.com:8080`, and `YOUR_TOKEN` with the token shown in the TUI):

```bash
curl -fsSL https://YOUR_SERVER/install.sh | sh -s -- --token YOUR_TOKEN
```

The script is served by the server: it downloads the binary from `/download/` (proxy to GitHub releases), installs to `/usr/local/bin` and immediately runs `ollama-farm client` against that server. Use **http** if the server does not use TLS:

```bash
curl -fsSL http://YOUR_SERVER/install.sh | sh -s -- --token YOUR_TOKEN
```

### From release (without a server)

To install only the binary (e.g. on the server or for local use):

```bash
curl -fsSL https://get.ollama-farm.io | sh
```

(Redirects to the script on GitHub.) Or:

```bash
curl -fsSL https://raw.githubusercontent.com/danielecalderazzo/ollama-farm/main/scripts/install.sh | sh
```

Detects OS (Linux/macOS) and arch (amd64/arm64), downloads the latest release and installs to `/usr/local/bin`.

### From source

```bash
git clone https://github.com/danielecalderazzo/ollama-farm.git
cd ollama-farm
go build -o ollama-farm .
# optional: mv ollama-farm /usr/local/bin/
```

---

## Usage: Server

```bash
ollama-farm server [flags]
```

| Flag        | Default | Description |
|------------|---------|-------------|
| `--port`   | 8080    | HTTP listen port (e.g. `8080` → `:8080`) |
| `--token`  | (empty) | Auth token for clients. If omitted, one is generated and saved to `~/.ollama-farm/config.json` (reused on next runs). |
| `--host`   | (empty) | Public host (e.g. `farm.example.com`) for the TUI: the install command shown will use this address, ready to copy. |
| `--releases-dir` | (empty) | Folder with prebuilt binaries (`ollama-farm_<os>_<arch>.tar.gz` or `.zip`). If GitHub has no release (404), the server serves files from here. Useful before the first release or on an isolated network. |
| `--no-tui` | false   | Run HTTP server only, no dashboard (for background or headless, e.g. systemd or scripts). |
| `--tls-cert` | (empty) | Path to TLS certificate (HTTPS). |
| `--tls-key`  | (empty) | Path to TLS key. |

The server exposes **install assets**: `GET /install.sh` and `GET /install.ps1` serve scripts that download the binary from `GET /download/<file>`. By default the download is proxied to GitHub releases; if there is no release yet (404), you can start the server with **`--releases-dir ./releases`** and put files there (e.g. `ollama-farm_windows_amd64.zip`). The TUI shows the one-command install for clients.

**Examples**

```bash
# Port 8080, token generated and persisted
ollama-farm server

# Port 9000, fixed token
ollama-farm server --port 9000 --token "my-secret-token"

# Background (no TUI, prints token to stderr)
ollama-farm server --port 8080 --token "my-token" --no-tui

# With public host (TUI will show the install command with this address)
ollama-farm server --port 8080 --host farm.example.com

# No GitHub release yet: serve binaries from a local folder (e.g. after go build + manual zip)
mkdir -p releases && cp ollama-farm_windows_amd64.zip releases/  # then:
ollama-farm server --port 8080 --releases-dir ./releases

# HTTPS
ollama-farm server --port 443 --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem
```

On startup the **TUI** runs: you see port, token and client list. The “INSTALL NEW CLIENT” command uses this server’s address (`/install.sh` and `/download/` are served by the server). Pass the shown token to every client. To exit: any key (e.g. q) or SIGINT/SIGTERM.

---

## Usage: Client

```bash
ollama-farm client --server <WS_URL> --token <TOKEN> [flags]
```

| Flag          | Default              | Description |
|---------------|----------------------|-------------|
| `--server`    | (required)           | WebSocket URL of the server (e.g. `ws://host:8080` or `wss://host:443`). The client adds `/ws` if missing. |
| `--token`     | (required)           | Same token configured on the server. |
| `--ollama-url`| http://localhost:11434 | URL of the local Ollama instance. |

**Examples**

```bash
# Local server, port 8080
ollama-farm client --server ws://localhost:8080 --token "my-secret-token"

# Remote server over HTTPS
ollama-farm client --server wss://farm.example.com --token "my-secret-token"

# Ollama on a different port
ollama-farm client --server ws://localhost:8080 --token "my-secret-token" --ollama-url http://localhost:11435
```

The client connects, discovers models from Ollama (`/api/tags`), registers and waits. On disconnect it retries with exponential backoff (max 30 s). If the server closes with code 4001 (wrong token), the client does not retry. To stop: Ctrl+C (or SIGTERM).

---

## Full examples

### 1. Server and one client on the same machine

**Terminal 1 – Server**

```bash
ollama-farm server --port 8080
# Note the token shown in the TUI, e.g. abc123def456
```

**Terminal 2 – Client** (Ollama must be running, e.g. `ollama serve`)

```bash
ollama-farm client --server ws://localhost:8080 --token "abc123def456"
```

**Terminal 3 – Request**

```bash
curl -s http://localhost:8080/api/generate \
  -H "Content-Type: application/json" \
  -d '{"model":"llama3","prompt":"Say hello in one word","stream":false}'
```

The server TUI will show the client as FREE, then BUSY during the request, then FREE again.

### 2. Add a client with one command

The server (e.g. `farm.example.com:8080`) is running `ollama-farm server` and you see the token in the TUI. On a **new machine** (with Ollama installed) run:

```bash
curl -fsSL https://farm.example.com:8080/install.sh | sh -s -- --token YOUR_TOKEN
```

This downloads the binary from that server, installs it and starts the client. If the server does not use HTTPS, use `http://` in the URL.

### 3. Multiple clients (round-robin)

Start the server once, then run multiple clients (on different machines if you like) with the same token. Requests for a given model are distributed round-robin across free clients that have that model.

### 4. List available models

```bash
curl -s http://localhost:8080/api/tags
```

The server forwards the request to a client that responded to `/api/tags`; models are the union of those declared by connected clients.

---

## Protocol

WebSocket messages are JSON with a `type` field. Direction: client→server or server→client.

| Type       | Direction      | Content |
|------------|----------------|--------|
| `register` | client → server | `token`, `models[]` — registration after WS upgrade. |
| `request`  | server → client | `req_id`, `method`, `path`, `headers`, `body_b64` — HTTP request to forward to Ollama. |
| `chunk`    | client → server | `req_id`, `data` (base64), `status_code` (first chunk only). |
| `end`      | client → server | `req_id` — end of stream; server marks client FREE. |
| `error`    | client → server | `req_id`, `message`, `code` — error; server returns 502 and marks FREE. |

After the WebSocket upgrade the server waits for a `register` message within **5 seconds**; if the token does not match it closes with code 4001 (unauthorized). Ping/pong keep the connection alive.

---

## Limitations

- **Max body size**: 10 MB per request (413 if larger).
- **Models per client**: the list sent at registration is used for the whole connection. If you add/remove models in Ollama, restart the client to refresh the list.
- **One request per client**: each client handles a single request at a time; no pipelining.
- **No persistence**: the client registry is in memory; when the server stops, all clients must reconnect.

---

## Development

**Build and test**

```bash
go build -o ollama-farm .
go test ./... -v -timeout 60s
```

**Integration tests (fake Ollama)**

```bash
go test ./tests/... -v -tags integration -timeout 30s
```

**Release (GoReleaser)**

Pushing a `v*` tag triggers the GitHub workflow that builds artifacts for Linux, macOS and Windows (amd64/arm64). Config in `.goreleaser.yaml`.

---

## License

See [LICENSE](LICENSE) in the repository.
