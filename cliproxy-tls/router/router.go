// Package router provides multi-backend routing for the relay proxy.
//
// Claude Code sends all requests to a single endpoint, but the proxy can
// route different models to different backends:
//
//   - claude-* models → Anthropic (uTLS + CCH signing, default)
//   - deepseek-* models → DeepSeek (Anthropic-compatible API)
//   - gpt-* models → OpenAI (format translation required)
//
// Each backend can have its own sanitizer config and model name map.
package router

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ZackO2o/all-relay-service/cliproxy-tls/profile"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/sanitize"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/transform"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/utls"
)

// BackendType defines the API format a backend expects.
type BackendType string

const (
	BackendAnthropic BackendType = "anthropic" // Native Anthropic Messages API
	BackendOpenAI    BackendType = "openai"    // OpenAI Chat Completions API
	BackendGeneric   BackendType = "generic"   // Any compatible API, pass through with sanitize
)

// BackendConfig defines a single backend routing target.
type BackendConfig struct {
	// Name is a human-readable identifier (e.g., "deepseek", "openai").
	Name string `json:"name"`

	// BaseURL is the target API base URL.
	// For Anthropic: "https://api.anthropic.com"
	// For DeepSeek Anthropic-compat: "https://api.deepseek.com/anthropic"
	// For OpenAI: "https://api.openai.com"
	BaseURL string `json:"base_url"`

	// Type indicates the API format this backend speaks.
	Type BackendType `json:"type"`

	// ModelPattern is a comma-separated list of model prefixes to match.
	// E.g., "deepseek-,ds-" matches deepseek-v4-flash, ds-coder, etc.
	ModelPattern string `json:"model_pattern"`

	// ModelMap maps client model names to backend model names.
	// If empty, the original model name is used as-is.
	ModelMap map[string]string `json:"model_map,omitempty"`

	// Sanitize configures request body sanitization for this backend.
	// For Anthropic backends, this is usually all false (passthrough).
	// For DeepSeek/OpenAI, remove output_config and thinking blocks.
	Sanitize sanitize.Config `json:"sanitize"`

	// ResponseModelName overrides the model name in streaming responses.
	// Set this to the name Claude Code expects (e.g., "claude-sonnet-4-6").
	// Empty means no rewriting.
	ResponseModelName string `json:"response_model_name,omitempty"`

	// Timeout in seconds for this backend's requests.
	Timeout int `json:"timeout,omitempty"`
}

// Router dispatches requests to the appropriate backend.
type Router struct {
	backends []BackendConfig
	defaultB *BackendConfig
}

// New creates a router with the given backends.
// The first backend with an empty ModelPattern is used as default.
func New(backends []BackendConfig) (*Router, error) {
	r := &Router{
		backends: make([]BackendConfig, len(backends)),
	}
	copy(r.backends, backends)

	for i := range r.backends {
		if r.backends[i].Timeout <= 0 {
			r.backends[i].Timeout = 60
		}
		if r.backends[i].Type == "" {
			r.backends[i].Type = BackendAnthropic
		}
		if r.backends[i].ModelPattern == "" && r.defaultB == nil {
			r.defaultB = &r.backends[i]
		}
	}

	if r.defaultB == nil && len(r.backends) > 0 {
		r.defaultB = &r.backends[0]
	}

	return r, nil
}

// DefaultRouter returns a router with a single Anthropic backend.
// This is the default behavior matching the original cliproxy-tls.
func DefaultRouter() *Router {
	r, _ := New([]BackendConfig{
		{
			Name:               "anthropic",
			BaseURL:            "https://api.anthropic.com",
			Type:               BackendAnthropic,
			ResponseModelName:  "",
			Timeout:            0,
			Sanitize:           sanitize.Config{}, // passthrough — no sanitization
		},
	})
	return r
}

// Resolve finds the backend config that matches the given model name.
// Returns the default backend if no match is found.
func (r *Router) Resolve(modelName string) *BackendConfig {
	if r == nil {
		return nil
	}
	for i := range r.backends {
		bc := &r.backends[i]
		if bc.ModelPattern == "" {
			continue
		}
		patterns := strings.Split(bc.ModelPattern, ",")
		for _, pat := range patterns {
			pat = strings.TrimSpace(pat)
			if pat == "" {
				continue
			}
			if strings.HasPrefix(modelName, pat) {
				return bc
			}
		}
	}
	return r.defaultB
}

// ---------------------------------------------------------------------------
// Dispatch: send a request to the resolved backend
// ---------------------------------------------------------------------------

// DispatchResult holds the upstream response and optional stream pipeline.
type DispatchResult struct {
	StatusCode int
	Headers    http.Header
	Body       ioReadCloser
	Pipeline   *transform.Pipeline // stream transformer for SSE, nil means passthrough
}

// We need a ReadCloser interface for the body
type ioReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}

// Dispatch sends the modified request body to the target backend.
// Returns a DispatchResult with the upstream response and transform pipeline.
func Dispatch(upstreamReq *http.Request, body []byte, bc *BackendConfig, clientModel string) (*DispatchResult, error) {
	// 1. Sanitize the body
	if bc.Sanitize != (sanitize.Config{}) {
		if bc.Sanitize.ReplaceModel == "" {
			// Use resolved model mapping if configured
			mappedModel := clientModel
			if bc.ModelMap != nil {
				if m, ok := bc.ModelMap[clientModel]; ok {
					mappedModel = m
				}
			}
			cfg := bc.Sanitize
			cfg.ReplaceModel = mappedModel
			body = sanitize.Sanitize(body, cfg)
		} else {
			body = sanitize.Sanitize(body, bc.Sanitize)
		}
	}

	// 2. Build the target URL
	targetURL := strings.TrimRight(bc.BaseURL, "/")
	switch bc.Type {
	case BackendAnthropic:
		targetURL += "/v1/messages"
	case BackendOpenAI:
		targetURL += "/v1/chat/completions"
	default:
		targetURL += "/v1/messages"
	}

	// 3. Create the upstream request
	req, err := http.NewRequestWithContext(upstreamReq.Context(), http.MethodPost, targetURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 4. Copy relevant headers
	copyHeaders(req.Header, upstreamReq.Header)

	// 5. Ensure content type
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	// 6. Use uTLS transport for all backends (fingerprint uniformity)
	tr := utls.NewTransport()
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(bc.Timeout) * time.Second,
	}

	// 7. Send the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream request: %w", err)
	}

	// 8. Build stream pipeline if model name rewriting is needed
	var pipe *transform.Pipeline
	if bc.ResponseModelName != "" {
		pipe = transform.New(transform.ModelNameRewriter(bc.ResponseModelName))
	}

	// 9. Apply device profile headers on the outgoing request (for Anthropic)
	if bc.Type == BackendAnthropic {
		dp := profile.DefaultDeviceProfile()
		dp.Apply(req)
	}

	log.Printf("[router] %s -> %s (%s) [%s]", clientModel, bc.Name, targetURL, bc.Sanitize.String())

	return &DispatchResult{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       resp.Body,
		Pipeline:   pipe,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var hopByHopHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailers":            true,
	"transfer-encoding":   true,
	"upgrade":             true,
	"host":                true,
}

func copyHeaders(dst, src http.Header) {
	for k, vals := range src {
		if hopByHopHeaders[strings.ToLower(k)] {
			continue
		}
		for _, v := range vals {
			dst.Add(k, v)
		}
	}
}

// MustParseBackends parses a JSON-encoded backend list or falls back to default.
// Intended for parsing environment variable config.
func MustParseBackends(jsonData string) *Router {
	if jsonData == "" {
		return DefaultRouter()
	}
	var cfgs []BackendConfig
	if err := json.Unmarshal([]byte(jsonData), &cfgs); err != nil {
		log.Printf("[router] failed to parse backends config: %v, using default", err)
		return DefaultRouter()
	}
	if len(cfgs) == 0 {
		return DefaultRouter()
	}
	r, err := New(cfgs)
	if err != nil {
		log.Printf("[router] failed to create router: %v, using default", err)
		return DefaultRouter()
	}
	return r
}
