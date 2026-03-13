# ollama-farm

**Un unico endpoint HTTP per più nodi Ollama.** Un server pubblico riceve le richieste API Ollama e le instrada, via WebSocket, ai client con GPU collegati. I chiamanti vedono un solo "Ollama" mentre il carico viene distribuito in round-robin sui nodi liberi.

---

## Indice

- [Come funziona](#come-funziona)
- [Requisiti](#requisiti)
- [Installazione](#installazione)
- [Uso: Server](#uso-server)
- [Uso: Client](#uso-client)
- [Esempi completi](#esempi-completi)
- [Protocollo](#protocollo)
- [Limitazioni](#limitazioni)
- [Sviluppo](#sviluppo)

---

## Come funziona

```
CHIAMANTE ──HTTP──▶ SERVER ──WebSocket (persistente)──▶ CLIENT(1..N)
  (curl, app)         :8080                                   │
                       │                                      ▼
                       │                               Ollama :11434
                       │
                       └── TUI (dashboard in terminale)
```

- **Server** (`ollama-farm server`): ascolta su HTTP (es. `:8080`), espone le stesse API di Ollama (`/api/generate`, `/api/chat`, `/api/tags`, ecc.) e mantiene un registro dei client connessi via WebSocket. Per ogni richiesta estrae il campo `model` dal body JSON, sceglie un client **libero** che ha quel modello (round-robin se ce ne sono più di uno), invia la richiesta sul tunnel WebSocket e inoltra lo stream di risposta al chiamante.

- **Client** (`ollama-farm client`): si connette **in uscita** al server (nessuna porta da aprire, funziona dietro NAT/firewall). All'avvio interroga l'Ollama locale (`/api/tags`) per ottenere la lista dei modelli, si registra sul server con token + modelli e resta in attesa. Quando arriva una richiesta dal server, la inoltra a Ollama locale e rimanda indietro chunk, end o error.

- **TUI**: il server mostra una dashboard (BubbleTea) con porta, token e lista client (FREE/BUSY, modelli, richieste totali), più il comando per installare nuovi client.

**Flusso di una richiesta**

1. Il chiamante invia `POST /api/generate` (o altra API Ollama) al server.
2. Il server bufferizza il body (max 10 MB; oltre → 413), legge `model` dal JSON.
3. Il router sceglie un client FREE con quel modello (round-robin).
4. Il server invia un messaggio `REQUEST` (method, path, headers, body in base64) sul WebSocket del client e lo marca BUSY.
5. Il client decodifica il body, fa la richiesta a Ollama locale e invia ogni chunk di risposta come messaggio `CHUNK`.
6. Il server inoltra i chunk al chiamante HTTP (streaming).
7. A fine stream il client invia `END`; il server marca il client FREE. In caso di errore invia `ERROR` e il server risponde 502 al chiamante.

Ogni client gestisce **una richiesta alla volta** (nessun pipelining), in linea con il modello di Ollama.

---

## Requisiti

- **Server**: Go 1.22+ (o binario precompilato). Nessun Ollama sul server.
- **Client**: Go 1.22+ (o binario) e **Ollama in esecuzione** sulla stessa macchina (default `http://localhost:11434`).
- Rete: i client devono poter raggiungere il server (WebSocket); il server deve essere raggiungibile dai chiamanti (HTTP).

---

## Installazione

### Su un client: un comando (dal tuo server)

Se hai già un server ollama-farm in esecuzione, su ogni macchina client puoi usare **un solo comando** per scaricare il binario dal server e avviare il client (sostituisci `TUO_SERVER` con l'indirizzo del server, es. `farm.example.com:8080`, e `TUO_TOKEN` con il token mostrato dalla TUI):

```bash
curl -fsSL https://TUO_SERVER/install.sh | sh -s -- --token TUO_TOKEN
```

Lo script è servito dal server stesso: scarica il binario da `/download/` (proxy alle release GitHub), installa in `/usr/local/bin` e avvia subito `ollama-farm client` verso quel server. Con **http** se il server non usa TLS:

```bash
curl -fsSL http://TUO_SERVER/install.sh | sh -s -- --token TUO_TOKEN
```

### Da release (senza server)

Per installare solo il binario (es. sul server o per uso locale):

```bash
curl -fsSL https://get.ollama-farm.io | sh
```

(Reindirizza allo script su GitHub.) Oppure:

```bash
curl -fsSL https://raw.githubusercontent.com/danielecalderazzo/ollama-farm/main/scripts/install.sh | sh
```

Rileva OS (Linux/macOS) e arch (amd64/arm64), scarica l'ultima release e installa in `/usr/local/bin`.

### Da sorgente

```bash
git clone https://github.com/danielecalderazzo/ollama-farm.git
cd ollama-farm
go build -o ollama-farm .
# opzionale: mv ollama-farm /usr/local/bin/
```

---

## Uso: Server

```bash
ollama-farm server [flags]
```

| Flag        | Default | Descrizione |
|------------|---------|-------------|
| `--port`   | 8080    | Porta HTTP (es. `8080` → `:8080`) |
| `--token`  | (vuoto) | Token di autenticazione per i client. Se omesso, viene generato e salvato in `~/.ollama-farm/config.json` (riusato ai prossimi avvii). |
| `--host`   | (vuoto) | Host pubblico (es. `farm.example.com`) per la TUI: il comando di install mostrato userà questo indirizzo, pronto da copiare. |
| `--releases-dir` | (vuoto) | Cartella con binari precompilati (`ollama-farm_<os>_<arch>.tar.gz` o `.zip`). Se GitHub non ha release (404), il server serve i file da qui. Utile prima della prima release o in rete isolata. |
| `--no-tui` | false   | Avvia solo l'HTTP server senza dashboard (utile in background o senza TTY, es. `systemd` o script). |
| `--tls-cert` | (vuoto) | Path al certificato TLS (HTTPS). |
| `--tls-key`  | (vuoto) | Path alla chiave TLS. |

Il server espone **risorse di installazione**: `GET /install.sh` e `GET /install.ps1` servono script che scaricano il binario da `GET /download/<file>`. Di default il download è in proxy alle release GitHub; se non esiste ancora una release (404), puoi avviare il server con **`--releases-dir ./releases`** e mettere in `./releases` i file (es. `ollama-farm_windows_amd64.zip`). La TUI mostra il comando unico da eseguire sui client.

**Esempi**

```bash
# Porta 8080, token generato e persistito
ollama-farm server

# Porta 9000, token fissato
ollama-farm server --port 9000 --token "my-secret-token"

# In background (senza TUI, stampa token su stderr)
ollama-farm server --port 8080 --token "my-token" --no-tui

# Con host pubblico (la TUI mostrerà il comando di install già con questo indirizzo)
ollama-farm server --port 8080 --host farm.example.com

# Senza release su GitHub: servi i binari da una cartella locale (es. dopo go build + zip manuale)
mkdir -p releases && cp ollama-farm_windows_amd64.zip releases/  # poi:
ollama-farm server --port 8080 --releases-dir ./releases

# HTTPS
ollama-farm server --port 443 --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem
```

All'avvio parte la **TUI**: vedi porta, token e lista client. Il comando "INSTALL NEW CLIENT" usa l'indirizzo di questo server (`/install.sh` e `/download/` sono serviti dal server). Il token mostrato va passato a ogni client. Per uscire: un tasto qualsiasi (es. q) o SIGINT/SIGTERM.

---

## Uso: Client

```bash
ollama-farm client --server <URL_WS> --token <TOKEN> [flags]
```

| Flag          | Default              | Descrizione |
|---------------|----------------------|-------------|
| `--server`    | (obbligatorio)       | URL WebSocket del server (es. `ws://host:8080` o `wss://host:443`). Il client aggiunge `/ws` se manca. |
| `--token`     | (obbligatorio)       | Stesso token configurato sul server. |
| `--ollama-url`| http://localhost:11434 | URL dell'istanza Ollama locale. |

**Esempi**

```bash
# Server in locale, porta 8080
ollama-farm client --server ws://localhost:8080 --token "my-secret-token"

# Server remoto in HTTPS
ollama-farm client --server wss://farm.example.com --token "my-secret-token"

# Ollama su altra porta
ollama-farm client --server ws://localhost:8080 --token "my-secret-token" --ollama-url http://localhost:11435
```

Il client si connette, scopre i modelli da Ollama (`/api/tags`), si registra e resta in attesa. In caso di disconnessione riprova con backoff esponenziale (max 30 s). Se il server chiude con codice 4001 (token errato), il client non riprova. Per fermare: Ctrl+C (o SIGTERM).

---

## Esempi completi

### 1. Server + un client sulla stessa macchina

**Terminale 1 – Server**

```bash
ollama-farm server --port 8080
# Annota il token mostrato in TUI, es. abc123def456
```

**Terminale 2 – Client** (Ollama deve essere avviato, es. `ollama serve`)

```bash
ollama-farm client --server ws://localhost:8080 --token "abc123def456"
```

**Terminale 3 – Chiamata**

```bash
curl -s http://localhost:8080/api/generate \
  -H "Content-Type: application/json" \
  -d '{"model":"llama3","prompt":"Say hello in one word","stream":false}'
```

La TUI del server mostrerà il client prima FREE, poi BUSY durante la richiesta, poi di nuovo FREE.

### 2. Aggiungere un client con un solo comando

Sul server (es. `farm.example.com:8080`) è in esecuzione `ollama-farm server` e in TUI vedi il token. Su una **nuova macchina** (con Ollama installato) esegui:

```bash
curl -fsSL https://farm.example.com:8080/install.sh | sh -s -- --token IL_TUO_TOKEN
```

Scarica il binario da quel server, lo installa e avvia il client verso il server. Se il server non usa HTTPS, usa `http://` nell'URL.

### 3. Più client (round-robin)

Avvia il server una volta, poi più client (anche su macchine diverse) con lo stesso token. Le richieste per un dato modello verranno distribuite in round-robin tra i client liberi che hanno quel modello.

### 4. Verificare i modelli disponibili

```bash
curl -s http://localhost:8080/api/tags
```

Il server inoltra la richiesta a un client che ha risposto a `/api/tags`; i modelli sono l'unione di quelli dichiarati dai client connessi.

---

## Protocollo

I messaggi WebSocket sono JSON con un campo `type`. Direzione: client→server o server→client.

| Tipo       | Direzione      | Contenuto |
|------------|----------------|-----------|
| `register` | client → server | `token`, `models[]` — registrazione dopo l'upgrade WS. |
| `request`  | server → client | `req_id`, `method`, `path`, `headers`, `body_b64` — richiesta HTTP da inoltrare a Ollama. |
| `chunk`    | client → server | `req_id`, `data` (base64), `status_code` (solo nel primo chunk). |
| `end`      | client → server | `req_id` — fine stream; il server marca il client FREE. |
| `error`    | client → server | `req_id`, `message`, `code` — errore; il server risponde 502 e marca FREE. |

Dopo l'upgrade WebSocket il server attende un messaggio `register` entro **5 secondi**; se il token non corrisponde chiude con codice 4001 (unauthorized). Ping/pong mantengono la connessione viva.

---

## Limitazioni

- **Body massimo**: 10 MB per richiesta (oltre → 413).
- **Modelli per client**: la lista inviata alla registrazione è quella usata per tutta la connessione. Se aggiungi/rimuovi modelli su Ollama, riavvia il client per aggiornare la lista.
- **Un request per client**: un client alla volta gestisce una sola richiesta; non c'è pipelining.
- **Nessun persist**: il registro client è in memoria; allo stop del server tutti i client devono riconnettersi.

---

## Sviluppo

**Build e test**

```bash
go build -o ollama-farm .
go test ./... -v -timeout 60s
```

**Test di integrazione (fake Ollama)**

```bash
go test ./tests/... -v -tags integration -timeout 30s
```

**Release (GoReleaser)**

Il push di un tag `v*` attiva la workflow GitHub che costruisce gli artefatti per Linux, macOS e Windows (amd64/arm64). Configurazione in `.goreleaser.yaml`.

---

## Licenza

Vedi [LICENSE](LICENSE) nel repository.
