package client

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/danielecalderazzo/ollama-farm/internal/protocol"
)

// OllamaProxy forwards requests to a local Ollama instance and streams chunks back.
type OllamaProxy struct {
	baseURL string
	http    *http.Client
}

// NewOllamaProxy creates an OllamaProxy pointing at the given Ollama URL.
func NewOllamaProxy(ollamaURL string) *OllamaProxy {
	return &OllamaProxy{
		baseURL: strings.TrimRight(ollamaURL, "/"),
		http:    &http.Client{},
	}
}

// Execute forwards the REQUEST to Ollama and calls onChunk for each response chunk.
// onEnd is called on successful completion; onError on failure.
func (p *OllamaProxy) Execute(
	req *protocol.RequestMessage,
	onChunk func(*protocol.ChunkMessage),
	onEnd func(),
	onError func(code int, msg string),
) error {
	var bodyReader io.Reader
	if req.BodyBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.BodyBase64)
		if err != nil {
			onError(400, fmt.Sprintf("invalid body encoding: %v", err))
			return nil
		}
		bodyReader = strings.NewReader(string(decoded))
	}

	httpReq, err := http.NewRequest(req.Method, p.baseURL+req.Path, bodyReader)
	if err != nil {
		onError(500, fmt.Sprintf("failed to build request: %v", err))
		return nil
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := p.http.Do(httpReq)
	if err != nil {
		onError(502, fmt.Sprintf("ollama unreachable: %v", err))
		return nil
	}
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	first := true
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			chunk := &protocol.ChunkMessage{
				Type:       protocol.TypeChunk,
				ReqID:      req.ReqID,
				DataBase64: base64.StdEncoding.EncodeToString(buf[:n]),
			}
			if first {
				chunk.StatusCode = resp.StatusCode
				first = false
			}
			onChunk(chunk)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			onError(502, fmt.Sprintf("stream read error: %v", readErr))
			return nil
		}
	}

	onEnd()
	return nil
}
