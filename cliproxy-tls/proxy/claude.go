// Package proxy handles forwarding requests to Claude API
// with uTLS transport and device profile fingerprint.
package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ZackO2o/all-relay-service/cliproxy-tls/profile"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/signing"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/utls"
)

// ClaudeProxy handles proxying requests to api.anthropic.com
// with Chrome TLS fingerprint and device profile headers.
type ClaudeProxy struct {
	transport *utls.Transport
	profile   *profile.DeviceProfile
	client    *http.Client
}

// New creates a new ClaudeProxy with uTLS transport.
func New(devProfile *profile.DeviceProfile) *ClaudeProxy {
	if devProfile == nil {
		devProfile = profile.DefaultDeviceProfile()
	}
	tr := utls.NewTransport()
	return &ClaudeProxy{
		transport: tr,
		profile:   devProfile,
		client: &http.Client{
			Transport: tr,
			Timeout:   0, // No timeout for streaming
		},
	}
}

// ClaudeBaseURL is the Anthropic API endpoint.
const ClaudeBaseURL = "https://api.anthropic.com"

// ---------------------------------------------------------------------------
// Exported methods for use by main.go's handleRelay
// ---------------------------------------------------------------------------

// SignBody applies CCH signing to the request body.
// It is exported so the router-based handler can use it for Anthropic backends.
func (p *ClaudeProxy) SignBody(body []byte) []byte {
	return signing.SignBody(body)
}

// Do executes an HTTP request through the uTLS transport and returns the response.
// It is exported so the router-based handler can reuse the same uTLS client pool.
func (p *ClaudeProxy) Do(req *http.Request) (*http.Response, error) {
	return p.client.Do(req)
}

// Profile returns the device profile used by this proxy.
func (p *ClaudeProxy) Profile() *profile.DeviceProfile {
	return p.profile
}

// HandleMessages proxies POST /v1/messages to Anthropic.
// Supports both streaming (SSE) and non-streaming responses.
func (p *ClaudeProxy) HandleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Read the original body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"read body: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Apply CCH signing if billing header contains cch placeholder
	// This prevents Anthropic from fingerprinting the request as non-Claude-CLI
	body = signing.SignBody(body)

	// Build upstream request
	upstreamURL := ClaudeBaseURL + "/v1/messages"
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"create request: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Copy relevant headers from original request
	copyHeaders(upstreamReq.Header, r.Header)

	// Apply device profile (overwrites UA and X-Stainless-*)
	p.profile.Apply(upstreamReq)

	// Ensure required headers
	if upstreamReq.Header.Get("Content-Type") == "" {
		upstreamReq.Header.Set("Content-Type", "application/json")
	}
	if upstreamReq.Header.Get("Accept") == "" {
		upstreamReq.Header.Set("Accept", "application/json")
	}
	if upstreamReq.Header.Get("anthropic-version") == "" {
		upstreamReq.Header.Set("anthropic-version", "2023-06-01")
	}

	// Determine if streaming
	isStreaming := strings.Contains(upstreamReq.Header.Get("Accept"), "text/event-stream")
	if !isStreaming {
		// Check request body for stream:true
		var bodyMap map[string]interface{}
		if err := json.Unmarshal(body, &bodyMap); err == nil {
			if stream, ok := bodyMap["stream"].(bool); ok && stream {
				isStreaming = true
			}
		}
	}

	// Execute request
	start := time.Now()
	resp, err := p.client.Do(upstreamReq)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"upstream request: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Log the request
	elapsed := time.Since(start)
	logRequest(upstreamReq, resp.StatusCode, len(body), elapsed)

	// Copy response headers
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Stream the response body
	if isStreaming {
		// SSE streaming: flush each chunk
		flusher, ok := w.(http.Flusher)
		if ok {
			buf := make([]byte, 32*1024)
			for {
				select {
				case <-r.Context().Done():
					return
				default:
				}
				n, err := resp.Body.Read(buf)
				if n > 0 {
					w.Write(buf[:n])
					flusher.Flush()
				}
				if err != nil {
					break
				}
			}
		} else {
			io.Copy(w, resp.Body)
		}
	} else {
		io.Copy(w, resp.Body)
	}
}

// HandleModels returns the list of available Claude models.
func (p *ClaudeProxy) HandleModels(w http.ResponseWriter, r *http.Request) {
	models := []map[string]interface{}{
		{"id": "claude-sonnet-4-20250514", "object": "model", "created": 1747000000, "owned_by": "anthropic"},
		{"id": "claude-sonnet-4", "object": "model", "created": 1747000000, "owned_by": "anthropic"},
		{"id": "claude-opus-4-20250514", "object": "model", "created": 1747000000, "owned_by": "anthropic"},
		{"id": "claude-opus-4", "object": "model", "created": 1747000000, "owned_by": "anthropic"},
		{"id": "claude-haiku-4-20250514", "object": "model", "created": 1747000000, "owned_by": "anthropic"},
		{"id": "claude-haiku-4", "object": "model", "created": 1747000000, "owned_by": "anthropic"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   models,
	})
}

// copyHeaders copies headers from src to dst.
func copyHeaders(dst, src http.Header) {
	for k, vals := range src {
		// Skip hop-by-hop headers
		if strings.EqualFold(k, "Connection") ||
			strings.EqualFold(k, "Keep-Alive") ||
			strings.EqualFold(k, "Proxy-Authenticate") ||
			strings.EqualFold(k, "Proxy-Authorization") ||
			strings.EqualFold(k, "Te") ||
			strings.EqualFold(k, "Trailers") ||
			strings.EqualFold(k, "Transfer-Encoding") ||
			strings.EqualFold(k, "Upgrade") ||
			strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range vals {
			dst.Add(k, v)
		}
	}
}

// logRequest prints a simple request log.
func logRequest(req *http.Request, statusCode int, bodySize int, elapsed time.Duration) {
	fmt.Printf("[proxy] %s %s -> %d (%d bytes, %v)\n",
		req.Method, req.URL.String(), statusCode, bodySize, elapsed)
}
