// Package oauth implements the OAuth 2.0 PKCE flow for Anthropic Claude.
// It uses the official Anthropic OAuth endpoints and Client ID,
// matching the same flow that Claude CLI uses.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ZackO2o/all-relay-service/cliproxy-tls/utls"
)

const (
	AuthURL     = "https://claude.ai/oauth/authorize"
	TokenURL    = "https://api.anthropic.com/v1/oauth/token"
	ClientID    = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	RedirectURI = "http://localhost:54545/callback"

	Scope = "user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"

	MinBackoff = 5 * time.Second
	MaxBackoff = 5 * time.Minute
)

// PKCECodes holds the PKCE code verifier and challenge pair.
type PKCECodes struct {
	CodeVerifier  string `json:"code_verifier"`
	CodeChallenge string `json:"code_challenge"`
}

// GeneratePKCE generates a PKCE code verifier and challenge (S256 method).
func GeneratePKCE() (*PKCECodes, error) {
	bytes := make([]byte, 96)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}
	verifier := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(bytes)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
	return &PKCECodes{
		CodeVerifier:  verifier,
		CodeChallenge: challenge,
	}, nil
}

// TokenResponse represents the response from Anthropic's OAuth token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Organization struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	} `json:"organization"`
	Account struct {
		UUID         string `json:"uuid"`
		EmailAddress string `json:"email_address"`
	} `json:"account"`
}

// TokenData holds the parsed OAuth token information.
type TokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Email        string `json:"email"`
	Expire       string `json:"expire"`
	LastRefresh  string `json:"last_refresh"`
}

// Client handles Anthropic OAuth2 authentication with uTLS transport.
type Client struct {
	httpClient *http.Client
	mu         sync.Mutex
	blocked    map[string]time.Time
}

// NewClient creates an OAuth client using uTLS transport.
func NewClient() *Client {
	tr := utls.NewTransport()
	return &Client{
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   30 * time.Second,
		},
		blocked: make(map[string]time.Time),
	}
}

// GenerateAuthURL creates the OAuth authorization URL with PKCE.
func (c *Client) GenerateAuthURL(state string, pkce *PKCECodes) string {
	params := url.Values{
		"code":                  {"true"},
		"client_id":             {ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {RedirectURI},
		"scope":                 {Scope},
		"code_challenge":        {pkce.CodeChallenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}
	return fmt.Sprintf("%s?%s", AuthURL, params.Encode())
}

// ExchangeCode exchanges an authorization code for tokens.
func (c *Client) ExchangeCode(ctx context.Context, code string, pkce *PKCECodes) (*TokenData, error) {
	body := map[string]interface{}{
		"grant_type":    "authorization_code",
		"code":          code,
		"client_id":     ClientID,
		"redirect_uri":  RedirectURI,
		"code_verifier": pkce.CodeVerifier,
	}
	return c.tokenRequest(ctx, body)
}

// RefreshTokens refreshes an access token using the refresh token.
func (c *Client) RefreshTokens(ctx context.Context, refreshToken string) (*TokenData, error) {
	// Check if blocked
	c.mu.Lock()
	if until, ok := c.blocked[refreshToken]; ok && until.After(time.Now()) {
		c.mu.Unlock()
		return nil, fmt.Errorf("refresh blocked until %s", until.Format(time.RFC3339))
	}
	c.mu.Unlock()

	body := map[string]interface{}{
		"grant_type":    "refresh_token",
		"client_id":     ClientID,
		"refresh_token": refreshToken,
	}
	return c.tokenRequest(ctx, body)
}

func (c *Client) tokenRequest(ctx context.Context, reqBody map[string]interface{}) (*TokenData, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", TokenURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := string(respBody)
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp)
			// Store block if we have refresh token
			if rt, ok := reqBody["refresh_token"].(string); ok {
				c.mu.Lock()
				c.blocked[rt] = time.Now().Add(retryAfter)
				c.mu.Unlock()
			}
		}
		return nil, fmt.Errorf("token request failed (HTTP %d): %s", resp.StatusCode, msg)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	return &TokenData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		Email:        tokenResp.Account.EmailAddress,
		Expire:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339),
		LastRefresh:  time.Now().Format(time.RFC3339),
	}, nil
}

func parseRetryAfter(resp *http.Response) time.Duration {
	if raw := strings.TrimSpace(resp.Header.Get("Retry-After-Ms")); raw != "" {
		var ms int
		if _, err := fmt.Sscanf(raw, "%d", &ms); err == nil && ms > 0 {
			d := time.Duration(ms) * time.Millisecond
			if d > MaxBackoff {
				return MaxBackoff
			}
			if d < MinBackoff {
				return MinBackoff
			}
			return d
		}
	}
	if raw := strings.TrimSpace(resp.Header.Get("Retry-After")); raw != "" {
		var seconds int
		if _, err := fmt.Sscanf(raw, "%d", &seconds); err == nil && seconds > 0 {
			d := time.Duration(seconds) * time.Second
			if d > MaxBackoff {
				return MaxBackoff
			}
			if d < MinBackoff {
				return MinBackoff
			}
			return d
		}
	}
	return MinBackoff
}

// ClearBlock removes a refresh token from the block list (on successful refresh).
func (c *Client) ClearBlock(refreshToken string) {
	c.mu.Lock()
	delete(c.blocked, refreshToken)
	c.mu.Unlock()
}
