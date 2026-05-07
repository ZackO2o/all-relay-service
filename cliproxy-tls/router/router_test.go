package router

import (
	"testing"

	"github.com/ZackO2o/all-relay-service/cliproxy-tls/sanitize"
)

func TestDefaultRouter(t *testing.T) {
	r := DefaultRouter()
	if r == nil {
		t.Fatal("DefaultRouter should not return nil")
	}
	bc := r.Resolve("claude-sonnet-4-6")
	if bc == nil {
		t.Fatal("Resolve should return a backend")
	}
	if bc.Name != "anthropic" {
		t.Errorf("expected anthropic backend, got %s", bc.Name)
	}
}

func TestResolveModelPattern(t *testing.T) {
	backends := []BackendConfig{
		{
			Name:         "anthropic",
			BaseURL:      "https://api.anthropic.com",
			Type:         BackendAnthropic,
			ModelPattern: "claude-",
		},
		{
			Name:         "deepseek",
			BaseURL:      "https://api.deepseek.com/anthropic",
			Type:         BackendGeneric,
			ModelPattern: "deepseek-,ds-",
			Sanitize: sanitize.Config{
				RemoveOutputConfig: true,
				RemoveThinking:     true,
			},
			ResponseModelName: "claude-sonnet-4-6",
			Timeout:           120,
		},
	}

	r, err := New(backends)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Test Claude model → anthropic
	bc := r.Resolve("claude-sonnet-4-6")
	if bc == nil || bc.Name != "anthropic" {
		t.Errorf("claude- model should resolve to anthropic, got %v", bc)
	}

	// Test DeepSeek model → deepseek
	bc = r.Resolve("deepseek-v4-flash")
	if bc == nil || bc.Name != "deepseek" {
		t.Errorf("deepseek- model should resolve to deepseek, got %v", bc)
	}

	// Test DS model (alternative pattern)
	bc = r.Resolve("ds-coder")
	if bc == nil || bc.Name != "deepseek" {
		t.Errorf("ds- model should resolve to deepseek, got %v", bc)
	}

	// Test unknown model → default (first)
	bc = r.Resolve("unknown-model")
	if bc == nil || bc.Name != "anthropic" {
		t.Errorf("unknown model should resolve to default (anthropic), got %v", bc)
	}
}

func TestMustParseBackendsEmpty(t *testing.T) {
	r := MustParseBackends("")
	if r == nil {
		t.Fatal("MustParseBackends should not return nil for empty input")
	}
	bc := r.Resolve("any-model")
	if bc == nil || bc.Name != "anthropic" {
		t.Errorf("empty config should return default anthropic backend")
	}
}

func TestMustParseBackendsJSON(t *testing.T) {
	jsonData := `[{"name":"test","base_url":"https://test.com","type":"generic","model_pattern":"test-","response_model_name":"claude-sonnet-4-6","timeout":30}]`
	r := MustParseBackends(jsonData)
	if r == nil {
		t.Fatal("MustParseBackends should not return nil")
	}
	bc := r.Resolve("test-model")
	if bc == nil || bc.Name != "test" {
		t.Errorf("should resolve to test backend, got %v", bc)
	}
	if bc.ResponseModelName != "claude-sonnet-4-6" {
		t.Errorf("expected response_model_name, got %s", bc.ResponseModelName)
	}
	if bc.Timeout != 30 {
		t.Errorf("expected timeout 30, got %d", bc.Timeout)
	}
}

func TestMustParseBackendsInvalidJSON(t *testing.T) {
	r := MustParseBackends("{invalid}")
	if r == nil {
		t.Fatal("MustParseBackends should fallback to default on invalid JSON")
	}
}
