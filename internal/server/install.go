package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const installScriptTemplate = `#!/bin/sh
set -e
# Injected by ollama-farm server: SERVER_URL and DOWNLOAD_BASE
SERVER_URL="{{.ServerURL}}"
DOWNLOAD_BASE="{{.DownloadBase}}"
BINARY="ollama-farm"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64)          ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

FILENAME="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="${DOWNLOAD_BASE}/${FILENAME}"

echo "Downloading ollama-farm for $OS/$ARCH from this server..."
curl -fsSL "$URL" | tar -xz "$BINARY"

INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  echo "Installing to $INSTALL_DIR (requires sudo)"
  sudo mv "$BINARY" "$INSTALL_DIR/$BINARY"
else
  mv "$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "ollama-farm installed to $INSTALL_DIR/$BINARY"

# Parse args: --token TOKEN
TOKEN=""
while [ $# -gt 0 ]; do
  case "$1" in
    --token) TOKEN="$2"; shift 2 ;;
    *) shift ;;
  esac
done

if [ -n "$TOKEN" ]; then
  echo "Starting client (server: $SERVER_URL)..."
  exec "$INSTALL_DIR/$BINARY" client --server "$SERVER_URL" --token "$TOKEN"
else
  echo "Run with your token to connect to this server:"
  echo "  ollama-farm client --server $SERVER_URL --token YOUR_TOKEN"
fi
`

const installPS1Template = `# Injected by ollama-farm server: SERVER_URL and DOWNLOAD_BASE
$SERVER_URL = "{{.ServerURL}}"
$DOWNLOAD_BASE = "{{.DownloadBase}}"

$arch = if ($env:PROCESSOR_ARCHITECTURE -match "ARM64") { "arm64" } else { "amd64" }
$filename = "ollama-farm_windows_$arch.zip"
$url = "$DOWNLOAD_BASE/$filename"

$tmp = "$env:TEMP\ollama-farm-install"
Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$zipPath = "$tmp\$filename"

Write-Host "Downloading ollama-farm for Windows/$arch..."
try {
  Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing
} catch {
  Write-Error "Download failed. If the server has no GitHub release yet, ask the admin to put $filename in the server's --releases-dir folder, or build from source."
  exit 1
}
if (-not (Test-Path $zipPath) -or (Get-Item $zipPath).Length -eq 0) {
  Write-Error "Download failed (empty or missing file). No release on GitHub? Use --releases-dir on the server with pre-built binaries."
  exit 1
}
Expand-Archive -Path $zipPath -DestinationPath $tmp -Force

$exe = Get-ChildItem -Path $tmp -Filter "ollama-farm.exe" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty FullName
if (-not $exe) { Write-Error "ollama-farm.exe not found in archive"; exit 1 }

$installDir = "$env:LOCALAPPDATA\ollama-farm"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Copy-Item -Path $exe -Destination "$installDir\ollama-farm.exe" -Force
$env:PATH = "$installDir;$env:PATH"

$token = $env:OLLAMA_FARM_TOKEN
if ($token) {
  Write-Host "Starting client (server: $SERVER_URL)..."
  & "$installDir\ollama-farm.exe" client --server $SERVER_URL --token $token
} else {
  Write-Host "ollama-farm installed to $installDir"
  Write-Host "Set OLLAMA_FARM_TOKEN and run: ollama-farm client --server $SERVER_URL --token YOUR_TOKEN"
}
`

var (
	githubReleasesLatest = "https://api.github.com/repos/danielecalderazzo/ollama-farm/releases/latest"
	downloadFilenameRe   = regexp.MustCompile(`^ollama-farm_(linux|darwin|windows)_(amd64|arm64)\.(tar\.gz|zip)$`)
)

// InstallScriptHandler returns a handler that serves the install script with ServerURL and DownloadBase
// set from the request (scheme + Host).
func InstallScriptHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		scheme := "https"
		if r.TLS == nil {
			scheme = "http"
		}
		host := r.Host
		if host == "" {
			host = "localhost:8080"
		}
		wsScheme := "wss"
		if r.TLS == nil {
			wsScheme = "ws"
		}
		serverURL := wsScheme + "://" + host
		downloadBase := scheme + "://" + host + "/download"

		script := installScriptTemplate
		script = strings.ReplaceAll(script, "{{.ServerURL}}", serverURL)
		script = strings.ReplaceAll(script, "{{.DownloadBase}}", downloadBase)

		w.Header().Set("Content-Type", "application/x-sh")
		w.Header().Set("Content-Disposition", `attachment; filename="install.sh"`)
		if r.Method == http.MethodGet {
		_, _ = io.Copy(w, bytes.NewReader([]byte(script)))
	}
	})
}

// InstallPS1Handler returns a handler that serves the PowerShell install script with ServerURL and DownloadBase.
func InstallPS1Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		scheme := "https"
		if r.TLS == nil {
			scheme = "http"
		}
		host := r.Host
		if host == "" {
			host = "localhost:8080"
		}
		wsScheme := "wss"
		if r.TLS == nil {
			wsScheme = "ws"
		}
		serverURL := wsScheme + "://" + host
		downloadBase := scheme + "://" + host + "/download"

		script := installPS1Template
		script = strings.ReplaceAll(script, "{{.ServerURL}}", serverURL)
		script = strings.ReplaceAll(script, "{{.DownloadBase}}", downloadBase)

		w.Header().Set("Content-Type", "application/x-powershell")
		w.Header().Set("Content-Disposition", `attachment; filename="install.ps1"`)
		if r.Method == http.MethodGet {
			_, _ = io.Copy(w, bytes.NewReader([]byte(script)))
		}
	})
}

// DownloadProxyHandler proxies GET /download/<filename> to GitHub releases (latest).
// If releasesDir is non-empty and GitHub has no release (404), serves from that directory instead.
// Filename must match ollama-farm_<os>_<arch>.tar.gz or .zip.
func DownloadProxyHandler(releasesDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		filename := strings.TrimPrefix(r.URL.Path, "/download/")
		filename = strings.TrimPrefix(filename, "/")
		if filename == "" {
			http.Error(w, "missing filename", http.StatusBadRequest)
			return
		}
		if !downloadFilenameRe.MatchString(filename) {
			http.Error(w, "invalid filename", http.StatusBadRequest)
			return
		}

		// 1) Try GitHub releases
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, githubReleasesLatest, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			tryLocalOrError(w, releasesDir, filename, fmt.Sprintf("failed to get latest release: %v", err))
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			tryLocalOrError(w, releasesDir, filename, fmt.Sprintf("GitHub API returned %d (no release yet?)", resp.StatusCode))
			return
		}
		tag := extractTagName(body)
		if tag == "" {
			tryLocalOrError(w, releasesDir, filename, "could not determine latest version from GitHub")
			return
		}

		downloadURL := "https://github.com/danielecalderazzo/ollama-farm/releases/download/" + tag + "/" + filename
		proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, downloadURL, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		proxyResp, err := http.DefaultClient.Do(proxyReq)
		if err != nil {
			tryLocalOrError(w, releasesDir, filename, fmt.Sprintf("failed to download from GitHub: %v", err))
			return
		}
		defer proxyResp.Body.Close()
		if proxyResp.StatusCode != http.StatusOK {
			tryLocalOrError(w, releasesDir, filename, fmt.Sprintf("GitHub download returned %d", proxyResp.StatusCode))
			return
		}
		for k, v := range proxyResp.Header {
			if strings.ToLower(k) == "content-length" || strings.ToLower(k) == "content-type" {
				w.Header()[k] = v
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, proxyResp.Body)
	})
}

// tryLocalOrError tries to serve filename from releasesDir; on failure writes a clear error.
func tryLocalOrError(w http.ResponseWriter, releasesDir, filename, reason string) {
	if releasesDir != "" {
		localPath := filepath.Join(releasesDir, filename)
		absDir, _ := filepath.Abs(releasesDir)
		absPath, _ := filepath.Abs(localPath)
		if absDir != "" && absPath != "" && !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) && absPath != absDir {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		f, err := os.Open(localPath)
		if err == nil {
			defer f.Close()
			info, _ := f.Stat()
			if info.Mode().IsRegular() {
				w.Header().Set("Content-Type", "application/octet-stream")
				if info.Size() > 0 {
					w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
				}
				w.WriteHeader(http.StatusOK)
				_, _ = io.Copy(w, f)
				return
			}
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	msg := "Binary not available: " + reason + ". "
	if releasesDir != "" {
		msg += "Also checked --releases-dir, file not found. "
	}
	msg += "Create a GitHub release or run the server with --releases-dir pointing to a folder with " + filename + "."
	fmt.Fprint(w, msg)
}

func extractTagName(jsonBody []byte) string {
	// Minimal: find "tag_name":"v1.2.3"
	const prefix = `"tag_name":`
	idx := bytes.Index(jsonBody, []byte(prefix))
	if idx < 0 {
		return ""
	}
	start := idx + len(prefix)
	// Skip optional space and "
	for start < len(jsonBody) && (jsonBody[start] == ' ' || jsonBody[start] == '"') {
		start++
	}
	end := start
	for end < len(jsonBody) && jsonBody[end] != '"' {
		end++
	}
	return string(jsonBody[start:end])
}
