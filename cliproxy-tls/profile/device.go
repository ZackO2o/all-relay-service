// Package profile provides device fingerprint for Claude API requests.
// It mimics the X-Stainless-* headers that Claude CLI sends,
// preventing Anthropic from fingerprinting the request as third-party.
package profile

import (
	"fmt"
	"net/http"
)

// DefaultDeviceProfile returns the baseline device fingerprint values
// matching real Claude CLI behavior on macOS ARM64.
func DefaultDeviceProfile() *DeviceProfile {
	return &DeviceProfile{
		UserAgent:      "claude-cli/2.1.63 (external, cli)",
		PackageVersion: "0.74.0",
		RuntimeVersion: "v24.3.0",
		OS:             "MacOS",
		Arch:           "arm64",
	}
}

// DeviceProfile holds the fingerprint headers that Claude CLI sends.
type DeviceProfile struct {
	UserAgent      string
	PackageVersion string
	RuntimeVersion string
	OS             string
	Arch           string
}

// Apply sets the device profile headers on the request.
// It clears any existing values first to ensure a clean fingerprint.
func (p *DeviceProfile) Apply(r *http.Request) {
	if r == nil {
		return
	}
	headers := []string{
		"User-Agent",
		"X-Stainless-Package-Version",
		"X-Stainless-Runtime-Version",
		"X-Stainless-Os",
		"X-Stainless-Arch",
	}
	for _, h := range headers {
		r.Header.Del(h)
	}
	r.Header.Set("User-Agent", p.UserAgent)
	r.Header.Set("X-Stainless-Package-Version", p.PackageVersion)
	r.Header.Set("X-Stainless-Runtime-Version", p.RuntimeVersion)
	r.Header.Set("X-Stainless-Os", p.OS)
	r.Header.Set("X-Stainless-Arch", p.Arch)
}

// String returns a human-readable description of the device profile.
func (p *DeviceProfile) String() string {
	return fmt.Sprintf("claude-cli/%s (%s/%s) pkg=%s runtime=%s",
		p.UserAgent, p.OS, p.Arch, p.PackageVersion, p.RuntimeVersion)
}
