// Package sanitize provides request body cleansing for cross-model compatibility.
//
// Claude Code sends Anthropic-specific fields (output_config, thinking blocks)
// that cause 400 errors when forwarded to non-Anthropic backends.
// Sanitizer strips these while preserving the core request intent.
package sanitize

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Config controls what to strip from the request body.
// All fields default to false (no sanitization).
type Config struct {
	// RemoveOutputConfig strips the `output_config` top-level field.
	// This prevents triggering DeepSeek Reasoner mode which breaks tool calls.
	RemoveOutputConfig bool `json:"remove_output_config"`

	// RemoveThinking filters out `thinking` content blocks from assistant messages.
	// Reasoner mode requires thinking blocks in history, but tool-using mode rejects them.
	RemoveThinking bool `json:"remove_thinking"`

	// ReplaceModel overrides the `model` field in the request.
	// Empty string means no replacement.
	ReplaceModel string `json:"replace_model,omitempty"`
}

// DefaultConfig returns a sane default config suitable for non-Anthropic backends.
func DefaultConfig() Config {
	return Config{
		RemoveOutputConfig: true,
		RemoveThinking:     true,
		ReplaceModel:       "",
	}
}

// Sanitize applies all enabled sanitization rules to the request body.
// Returns the modified body, or the original if no changes were needed.
// Never returns an error for malformed JSON — passes through unchanged.
func Sanitize(body []byte, cfg Config) []byte {
	changed := false

	// Step 1: Remove top-level fields
	if cfg.RemoveOutputConfig {
		if gjson.GetBytes(body, "output_config").Exists() {
			var err error
			body, err = sjson.DeleteBytes(body, "output_config")
			if err == nil {
				changed = true
			}
		}
		// Also remove thinking at top level
		if gjson.GetBytes(body, "thinking").Exists() {
			var err error
			body, err = sjson.DeleteBytes(body, "thinking")
			if err == nil {
				changed = true
			}
		}
	}

	// Step 2: Replace model name
	if cfg.ReplaceModel != "" {
		if gjson.GetBytes(body, "model").Exists() {
			var err error
			body, err = sjson.SetBytes(body, "model", cfg.ReplaceModel)
			if err == nil {
				changed = true
			}
		}
	}

	// Step 3: Remove thinking blocks from assistant messages
	if cfg.RemoveThinking {
		body = cleanThinkingBlocks(body)
	}

	_ = changed // body is always a new slice if modified
	return body
}

// cleanThinkingBlocks removes `thinking` type content blocks from all assistant messages.
func cleanThinkingBlocks(body []byte) []byte {
	msgs := gjson.GetBytes(body, "messages")
	if !msgs.IsArray() || len(msgs.Array()) == 0 {
		return body
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return body
	}

	rawMsgs, ok := rawMap["messages"].([]interface{})
	if !ok {
		return body
	}

	anyFiltered := false
	newMsgs := make([]interface{}, 0, len(rawMsgs))

	for _, rawMsg := range rawMsgs {
		msg, ok := rawMsg.(map[string]interface{})
		if !ok {
			newMsgs = append(newMsgs, rawMsg)
			continue
		}

		role, _ := msg["role"].(string)
		if role != "assistant" {
			newMsgs = append(newMsgs, msg)
			continue
		}

		rawContent, ok := msg["content"].([]interface{})
		if !ok {
			newMsgs = append(newMsgs, msg)
			continue
		}

		filtered := make([]interface{}, 0, len(rawContent))
		for _, block := range rawContent {
			b, ok := block.(map[string]interface{})
			if !ok {
				filtered = append(filtered, block)
				continue
			}
			blockType, _ := b["type"].(string)
			if blockType == "thinking" {
				anyFiltered = true
				continue
			}
			filtered = append(filtered, block)
		}

		if anyFiltered && len(filtered) == 0 {
			filtered = append(filtered, map[string]interface{}{
				"type": "text",
				"text": "...",
			})
		}

		msg["content"] = filtered
		newMsgs = append(newMsgs, msg)
	}

	if !anyFiltered {
		return body
	}

	rawMap["messages"] = newMsgs
	result, err := json.Marshal(rawMap)
	if err != nil {
		return body
	}
	return result
}

// String returns a human-readable summary of the config.
func (c Config) String() string {
	var parts []string
	if c.RemoveOutputConfig {
		parts = append(parts, "no-output_config")
	}
	if c.RemoveThinking {
		parts = append(parts, "no-thinking")
	}
	if c.ReplaceModel != "" {
		parts = append(parts, "model→"+c.ReplaceModel)
	}
	if len(parts) == 0 {
		return "passthrough"
	}
	return strings.Join(parts, "+")
}
