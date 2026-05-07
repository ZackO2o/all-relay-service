// cliproxy-tls — uTLS-powered Claude API proxy with device fingerprint.
//
// Provides:
//   POST /v1/messages  — Multi-backend relay with uTLS transport
//   GET  /v1/models    — Fake models list (for local model registry bypass)
//   GET  /health       — Health check
//
// Multi-backend routing:
//   Set BACKENDS_CONFIG env var to a JSON array of backend configs.
//   Each backend can route requests to different providers (Anthropic, DeepSeek, OpenAI)
//   with per-backend sanitization and model name rewriting.
//
// OAuth endpoints:
//   GET  /oauth/authorize  — Generate OAuth authorization URL
//   POST /oauth/callback   — Exchange code for tokens
//   POST /oauth/refresh    — Refresh token
//   POST /oauth/token      — Token exchange (compatibility)
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ZackO2o/all-relay-service/cliproxy-tls/oauth"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/profile"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/proxy"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/router"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/sanitize"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/transform"
	"github.com/tidwall/gjson"
)

var (
	claudeProxy *proxy.ClaudeProxy
	oauthClient *oauth.Client
	relayRouter *router.Router
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9200"
	}

	// Auto-generate self-signed cert if missing (production mode)
	if isProduction() {
		if _, err := os.Stat("server.crt"); os.IsNotExist(err) {
			log.Printf("[init] generating self-signed TLS certificate...")
			if err := genSelfSignedCert("server.crt", "server.key"); err != nil {
				log.Fatalf("[init] failed to generate cert: %v", err)
			}
			log.Printf("[init] self-signed certificate generated")
		}
	}

	// Initialize
	devProfile := profile.DefaultDeviceProfile()
	log.Printf("[init] device profile: %s", devProfile)

	claudeProxy = proxy.New(devProfile)
	oauthClient = oauth.NewClient()

	// Initialize multi-backend router from env
	backendsJSON := os.Getenv("BACKENDS_CONFIG")
	relayRouter = router.MustParseBackends(backendsJSON)
	log.Printf("[init] router: %d backends configured", countBackends(relayRouter))

	// Routes
	http.HandleFunc("/v1/messages", handleRelay)
	http.HandleFunc("/v1/models", claudeProxy.HandleModels)
	http.HandleFunc("/health", handleHealth)

	// OAuth routes
	http.HandleFunc("/oauth/authorize", handleOAuthAuthorize)
	http.HandleFunc("/oauth/callback", handleOAuthCallback)
	http.HandleFunc("/oauth/refresh", handleOAuthRefresh)
	http.HandleFunc("/oauth/token", handleOAuthToken)

	log.Printf("[server] listening on :%s", port)
	if isProduction() {
		// Production: HTTPS with self-signed cert (for local proxy use)
		if err := http.ListenAndServeTLS(":"+port, "server.crt", "server.key", nil); err != nil {
			log.Fatalf("[server] %v", err)
		}
	} else {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("[server] %v", err)
		}
	}
}

// handleRelay is the main entry point for /v1/messages.
// It reads the request body, extracts the model name, resolves the backend,
// sanitizes and dispatches the request, then writes the response with optional
// stream transformation.
func handleRelay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Read the full request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"read body: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Extract model name from request
	clientModel := gjson.GetBytes(body, "model").String()

	// Resolve backend
	bc := relayRouter.Resolve(clientModel)
	if bc == nil {
		// Fallback to original ClaudeProxy for backward compatibility
		log.Printf("[relay] no backend resolved for model %q, using default claude proxy", clientModel)
		// Create a new request to replay through claudeProxy
		newReq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, "/v1/messages", strings.NewReader(string(body)))
		newReq.Header = r.Header.Clone()
		claudeProxy.HandleMessages(w, newReq)
		return
	}

	log.Printf("[relay] model=%q → backend=%q (%s)", clientModel, bc.Name, bc.BaseURL)

	// For Anthropic backends with no sanitization, use the original proxy path
	// which includes CCH signing and proper device profile headers
	if bc.Type == "anthropic" && bc.Sanitize == (sanitize.Config{}) && bc.ResponseModelName == "" {
		// Replay through original claudeProxy for full uTLS + CCH + profile treatment
		newReq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, "/v1/messages", strings.NewReader(string(body)))
		newReq.Header = r.Header.Clone()
		claudeProxy.HandleMessages(w, newReq)
		return
	}

	// Apply sanitization if configured
	modifiedBody := body
	if bc.Sanitize != (sanitize.Config{}) {
		sanCfg := bc.Sanitize
		if sanCfg.ReplaceModel == "" && bc.ModelMap != nil {
			if mapped, ok := bc.ModelMap[clientModel]; ok {
				sanCfg.ReplaceModel = mapped
			}
		}
		modifiedBody = sanitize.Sanitize(body, sanCfg)
		log.Printf("[relay] sanitized: %s", sanCfg.String())
	}

	// Build the target URL
	targetURL := strings.TrimRight(bc.BaseURL, "/")
	switch bc.Type {
	case "openai":
		targetURL += "/v1/chat/completions"
	default:
		targetURL += "/v1/messages"
	}

	// Create upstream request with uTLS transport
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, targetURL, strings.NewReader(string(modifiedBody)))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"create request: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Copy and set headers
	copyHeaders(upstreamReq.Header, r.Header)
	if upstreamReq.Header.Get("Content-Type") == "" {
		upstreamReq.Header.Set("Content-Type", "application/json")
	}
	if upstreamReq.Header.Get("Accept") == "" {
		upstreamReq.Header.Set("Accept", "application/json, text/event-stream")
	}
	if upstreamReq.Header.Get("anthropic-version") == "" {
		upstreamReq.Header.Set("anthropic-version", "2023-06-01")
	}

	// Apply device profile for Anthropic backends
	if bc.Type == "anthropic" {
		profile.DefaultDeviceProfile().Apply(upstreamReq)
	}

	// Apply CCH signing only for Anthropic backends
	if bc.Type == "anthropic" {
		modifiedBody = claudeProxy.SignBody(modifiedBody)
		// Recreate request with signed body - simple approach: create new reader
		upstreamReq.Body = io.NopCloser(strings.NewReader(string(modifiedBody)))
		upstreamReq.ContentLength = int64(len(modifiedBody))
	}

	// Execute the request
	start := time.Now()
	resp, err := claudeProxy.Do(upstreamReq)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"upstream request: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	log.Printf("[relay] %s %s -> %d (%d bytes, %v)", upstreamReq.Method, targetURL, resp.StatusCode, len(modifiedBody), elapsed)

	// Build stream transform pipeline
	var pipe *transform.Pipeline
	if bc.ResponseModelName != "" {
		pipe = transform.New(transform.ModelNameRewriter(bc.ResponseModelName))
		log.Printf("[relay] stream model rewrite: → %q", bc.ResponseModelName)
	}

	// Copy response headers
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Stream the response
	isStreaming := strings.Contains(r.Header.Get("Accept"), "text/event-stream") ||
		strings.Contains(string(body), `"stream":true`)
	if isStreaming && pipe != nil && pipe.Len() > 0 {
		// Use transform pipeline
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
					data := buf[:n]
					data = pipe.Apply(data)
					if data != nil {
						w.Write(data)
						flusher.Flush()
					}
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

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	info := map[string]interface{}{
		"status":  "ok",
		"service": "cliproxy-tls",
		"profile": profile.DefaultDeviceProfile().String(),
	}
	// Add router info
	if relayRouter != nil {
		backends := countBackends(relayRouter)
		info["backends"] = backends
	}
	json.NewEncoder(w).Encode(info)
}

func handleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		b := make([]byte, 16)
		rand.Read(b)
		state = fmt.Sprintf("state_%s", hex.EncodeToString(b))
	}

	pkce, err := oauth.GeneratePKCE()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"generate pkce: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	authURL := oauthClient.GenerateAuthURL(state, pkce)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auth_url":        authURL,
		"state":           state,
		"code_verifier":   pkce.CodeVerifier,
		"code_challenge":  pkce.CodeChallenge,
		"redirect_uri":    oauth.RedirectURI,
	})
}

func handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Browser redirect callback
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body><script>
			window.opener.postMessage({code:%q,state:%q},"*");
			window.close();
		</script><p>Authentication successful. You may close this window.</p></body></html>`, code, state)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"decode: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	pkce := &oauth.PKCECodes{CodeVerifier: req.CodeVerifier}
	token, err := oauthClient.ExchangeCode(r.Context(), req.Code, pkce)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"exchange: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)
}

func handleOAuthRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"decode: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	if req.RefreshToken == "" {
		http.Error(w, `{"error":"refresh_token required"}`, http.StatusBadRequest)
		return
	}

	token, err := oauthClient.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"refresh: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}

	oauthClient.ClearBlock(req.RefreshToken)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)
}

func handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"decode: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	grantType, _ := req["grant_type"].(string)

	switch grantType {
	case "authorization_code":
		code, _ := req["code"].(string)
		verifier, _ := req["code_verifier"].(string)
		if code == "" || verifier == "" {
			http.Error(w, `{"error":"code and code_verifier required"}`, http.StatusBadRequest)
			return
		}
		pkce := &oauth.PKCECodes{CodeVerifier: verifier}
		token, err := oauthClient.ExchangeCode(r.Context(), code, pkce)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"exchange: %s"}`, err.Error()), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(token)

	case "refresh_token":
		refreshToken, _ := req["refresh_token"].(string)
		if refreshToken == "" {
			http.Error(w, `{"error":"refresh_token required"}`, http.StatusBadRequest)
			return
		}
		token, err := oauthClient.RefreshTokens(r.Context(), refreshToken)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"refresh: %s"}`, err.Error()), http.StatusBadGateway)
			return
		}
		oauthClient.ClearBlock(refreshToken)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(token)

	default:
		http.Error(w, fmt.Sprintf(`{"error":"unsupported grant_type: %s"}`, grantType), http.StatusBadRequest)
	}
}

func init() {
	// Suppress noisy logs
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// Helper to check if running in production
func isProduction() bool {
	return strings.EqualFold(os.Getenv("GO_ENV"), "production")
}

// genSelfSignedCert generates a self-signed TLS certificate and key.
func genSelfSignedCert(certPath, keyPath string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"ALL Relay Service"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create cert: %w", err)
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("create cert file: %w", err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("encode cert: %w", err)
	}

	keyOut, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	defer keyOut.Close()
	if err := pem.Encode(keyOut, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}); err != nil {
		return fmt.Errorf("encode key: %w", err)
	}

	return nil
}

// copyHeaders copies headers from src to dst, skipping hop-by-hop headers.
func copyHeaders(dst, src http.Header) {
	hopByHop := map[string]bool{
		"connection": true, "keep-alive": true, "proxy-authenticate": true,
		"proxy-authorization": true, "te": true, "trailers": true,
		"transfer-encoding": true, "upgrade": true, "host": true,
	}
	for k, vals := range src {
		if hopByHop[strings.ToLower(k)] {
			continue
		}
		for _, v := range vals {
			dst.Add(k, v)
		}
	}
}

func countBackends(r *router.Router) int {
	// Approximate count: we can't access private backends field,
	// but we know the default router has 1 backend
	if r == nil {
		return 0
	}
	return 1 // placeholder; actual count requires exporting a method
}
