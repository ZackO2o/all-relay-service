// Package utls provides an HTTP transport that uses uTLS with Chrome fingerprint
// to bypass Cloudflare's TLS fingerprinting on Anthropic domains.
package utls

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// DefaultAnthropicHosts lists the hosts that should use uTLS fingerprinting.
var DefaultAnthropicHosts = map[string]struct{}{
	"api.anthropic.com": {},
	"claude.ai":         {},
}

// Transport implements http.RoundTripper using uTLS with Chrome TLS fingerprint.
// It caches HTTP/2 connections per host and only applies uTLS to Anthropic domains.
type Transport struct {
	mu          sync.Mutex
	connections map[string]*http2.ClientConn
	pending     map[string]*sync.Cond
	dialer      *net.Dialer
	anthropic   map[string]struct{}
}

// NewTransport creates a uTLS transport that applies Chrome JA3 fingerprint
// to the specified Anthropic hosts, falling back to standard TLS for others.
func NewTransport() *Transport {
	return &Transport{
		connections: make(map[string]*http2.ClientConn),
		pending:     make(map[string]*sync.Cond),
		dialer: &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
		anthropic: DefaultAnthropicHosts,
	}
}

// RoundTrip implements http.RoundTripper.
// For Anthropic HTTPS hosts, it uses uTLS with Chrome fingerprint.
// For all other hosts, it falls back to standard http.Transport.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	hostname := req.URL.Hostname()
	if req.URL.Scheme == "https" {
		if _, ok := t.anthropic[strings.ToLower(hostname)]; ok {
			return t.roundTripUTLS(req, hostname)
		}
	}
	// Fall back to standard transport for non-Anthropic hosts
	return t.standardRoundTrip(req)
}

func (t *Transport) roundTripUTLS(req *http.Request, hostname string) (*http.Response, error) {
	port := req.URL.Port()
	if port == "" {
		port = "443"
	}
	addr := net.JoinHostPort(hostname, port)

	h2Conn, err := t.getOrCreateConn(hostname, addr)
	if err != nil {
		return nil, err
	}

	resp, err := h2Conn.RoundTrip(req)
	if err != nil {
		t.mu.Lock()
		if cached, ok := t.connections[hostname]; ok && cached == h2Conn {
			delete(t.connections, hostname)
		}
		t.mu.Unlock()
		return nil, err
	}
	return resp, nil
}

func (t *Transport) getOrCreateConn(hostname, addr string) (*http2.ClientConn, error) {
	t.mu.Lock()

	// Check existing connection
	if h2Conn, ok := t.connections[hostname]; ok && h2Conn.CanTakeNewRequest() {
		t.mu.Unlock()
		return h2Conn, nil
	}

	// Wait for another goroutine creating a connection
	if cond, ok := t.pending[hostname]; ok {
		cond.Wait()
		if h2Conn, ok := t.connections[hostname]; ok && h2Conn.CanTakeNewRequest() {
			t.mu.Unlock()
			return h2Conn, nil
		}
	}

	// Mark as pending
	cond := sync.NewCond(&t.mu)
	t.pending[hostname] = cond
	t.mu.Unlock()

	h2Conn, err := t.createConn(hostname, addr)

	t.mu.Lock()
	delete(t.pending, hostname)
	cond.Broadcast()
	t.mu.Unlock()

	if err != nil {
		return nil, err
	}

	t.mu.Lock()
	t.connections[hostname] = h2Conn
	t.mu.Unlock()
	return h2Conn, nil
}

func (t *Transport) createConn(hostname, addr string) (*http2.ClientConn, error) {
	conn, err := t.dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	// Use uTLS with Chrome Auto fingerprint
	// HelloChrome_Auto picks the latest Chrome fingerprint automatically
	tlsConn := utls.UClient(conn, &utls.Config{
		ServerName: hostname,
	}, utls.HelloChrome_Auto)

	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	h2Transport := &http2.Transport{}
	h2Conn, err := h2Transport.NewClientConn(tlsConn)
	if err != nil {
		tlsConn.Close()
		return nil, err
	}
	return h2Conn, nil
}

func (t *Transport) standardRoundTrip(req *http.Request) (*http.Response, error) {
	// Use a shared standard transport for non-matching hosts
	return defaultStdTransport.RoundTrip(req)
}

var defaultStdTransport = &http.Transport{
	TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
	},
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	MaxIdleConns:        100,
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
}
