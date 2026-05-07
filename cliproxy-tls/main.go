// cliproxy-tls — uTLS-powered Claude API proxy with device fingerprint.
//
// Provides:
//   POST /v1/messages  — Proxy to Anthropic with Chrome JA3 + device profile
//   GET  /health       — Health check
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
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ZackO2o/all-relay-service/cliproxy-tls/oauth"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/profile"
	"github.com/ZackO2o/all-relay-service/cliproxy-tls/proxy"
)

var (
	claudeProxy *proxy.ClaudeProxy
	oauthClient *oauth.Client
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

	// Routes
	http.HandleFunc("/v1/messages", claudeProxy.HandleMessages)
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

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"service": "cliproxy-tls",
		"profile": profile.DefaultDeviceProfile().String(),
	})
}

func handleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		state = fmt.Sprintf("state_%d", len(state))
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
