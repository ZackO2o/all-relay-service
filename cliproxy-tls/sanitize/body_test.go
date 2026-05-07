package sanitize

import (
	"strings"
	"testing"
)

func TestSanitizeNoConfig(t *testing.T) {
	input := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}]}`)
	result := Sanitize(input, Config{})
	if string(result) != string(input) {
		t.Errorf("expected no change, got %s", string(result))
	}
}

func TestSanitizeRemoveOutputConfig(t *testing.T) {
	input := []byte(`{"model":"claude-sonnet-4-6","output_config":{"effort":"max"},"messages":[{"role":"user","content":"hi"}]}`)
	cfg := Config{RemoveOutputConfig: true}
	result := Sanitize(input, cfg)
	if strings.Contains(string(result), "output_config") {
		t.Errorf("output_config should be removed: %s", string(result))
	}
	if !strings.Contains(string(result), "claude-sonnet-4-6") {
		t.Errorf("model should remain: %s", string(result))
	}
}

func TestSanitizeReplaceModel(t *testing.T) {
	input := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}]}`)
	cfg := Config{ReplaceModel: "deepseek-v4-flash"}
	result := Sanitize(input, cfg)
	if !strings.Contains(string(result), "deepseek-v4-flash") {
		t.Errorf("model should be replaced: %s", string(result))
	}
}

func TestSanitizeRemoveThinking(t *testing.T) {
	input := []byte(`{
		"model": "claude-sonnet-4-6",
		"messages": [
			{"role": "user", "content": "hello"},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "let me think..."},
				{"type": "text", "text": "the answer is 42"}
			]}
		]
	}`)
	cfg := Config{RemoveThinking: true}
	result := Sanitize(input, cfg)
	if strings.Contains(string(result), "let me think") {
		t.Errorf("thinking blocks should be removed: %s", string(result))
	}
	if !strings.Contains(string(result), "the answer is 42") {
		t.Errorf("text blocks should remain: %s", string(result))
	}
}

func TestSanitizeRemoveThinkingAllContentThinking(t *testing.T) {
	input := []byte(`{
		"model": "claude-sonnet-4-6",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "thinking..."}
			]}
		]
	}`)
	cfg := Config{RemoveThinking: true}
	result := Sanitize(input, cfg)
	if !strings.Contains(string(result), "...") {
		t.Errorf("placeholder text should be inserted: %s", string(result))
	}
}

func TestSanitizeMalformedJSON(t *testing.T) {
	input := []byte(`{invalid json}`)
	result := Sanitize(input, Config{RemoveOutputConfig: true, RemoveThinking: true})
	if string(result) != string(input) {
		t.Errorf("malformed JSON should pass through unchanged")
	}
}

func TestSanitizeStringPassthrough(t *testing.T) {
	cfg := Config{}
	if cfg.String() != "passthrough" {
		t.Errorf("empty config should be passthrough: %s", cfg.String())
	}
}

func TestSanitizeStringFull(t *testing.T) {
	cfg := Config{RemoveOutputConfig: true, RemoveThinking: true, ReplaceModel: "test"}
	s := cfg.String()
	if s != "no-output_config+no-thinking+model→test" {
		t.Errorf("unexpected string: %s", s)
	}
}
