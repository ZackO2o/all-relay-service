package transform

import (
	"strings"
	"testing"
)

func TestPipelineEmpty(t *testing.T) {
	pipe := New()
	if pipe.Len() != 0 {
		t.Errorf("empty pipeline should have 0 transformers")
	}
	data := []byte(`test data`)
	result := pipe.Apply(data)
	if string(result) != string(data) {
		t.Errorf("empty pipeline should pass through: %s", result)
	}
}

func TestPipelineSingle(t *testing.T) {
	pipe := New(func(data []byte) []byte {
		return []byte(strings.ToUpper(string(data)))
	})
	if pipe.Len() != 1 {
		t.Errorf("expected 1 transformer")
	}
	result := pipe.Apply([]byte(`hello`))
	if string(result) != "HELLO" {
		t.Errorf("expected HELLO, got %s", result)
	}
}

func TestPipelineChain(t *testing.T) {
	pipe := New(
		func(data []byte) []byte {
			return []byte(strings.ReplaceAll(string(data), "bad", "good"))
		},
		func(data []byte) []byte {
			return []byte(strings.ToUpper(string(data)))
		},
	)
	result := pipe.Apply([]byte(`bad world`))
	if string(result) != "GOOD WORLD" {
		t.Errorf("expected GOOD WORLD, got %s", result)
	}
}

func TestPipelineNilReturn(t *testing.T) {
	pipe := New(func(data []byte) []byte {
		return nil
	})
	result := pipe.Apply([]byte(`data`))
	if result != nil {
		t.Errorf("expected nil, got %s", result)
	}
}

func TestPipelineAdd(t *testing.T) {
	pipe := New()
	pipe.Add(func(data []byte) []byte { return []byte("added") })
	if pipe.Len() != 1 {
		t.Errorf("expected 1 transformer after Add")
	}
	result := pipe.Apply([]byte(`original`))
	if string(result) != "added" {
		t.Errorf("expected 'added', got %s", result)
	}
}

func TestModelNameRewriter(t *testing.T) {
	// Test with empty target - should passthrough
	pipe := New(ModelNameRewriter(""))
	data := []byte(`{"model":"deepseek-v4-flash"}`)
	result := pipe.Apply(data)
	if string(result) != string(data) {
		t.Errorf("empty target should passthrough: %s", result)
	}

	// Test with actual target
	pipe2 := New(ModelNameRewriter("claude-sonnet-4-6"))
	result2 := pipe2.Apply([]byte(`{"model":"deepseek-v4-flash"}`))
	if !strings.Contains(string(result2), "claude-sonnet-4-6") {
		t.Errorf("model name should be rewritten: %s", result2)
	}
	if strings.Contains(string(result2), "deepseek-v4-flash") {
		t.Errorf("original model name should be gone: %s", result2)
	}
}

func TestLoggingTransformer(t *testing.T) {
	logged := ""
	pipe := New(LoggingTransformer(func(msg string, args ...interface{}) {
		logged = msg
	}))
	result := pipe.Apply([]byte(`data`))
	if string(result) != "data" {
		t.Errorf("logging transformer should not modify data")
	}
	if logged == "" {
		t.Error("logging transformer should call the log function")
	}
}

func TestNilPipelineApply(t *testing.T) {
	var pipe *Pipeline
	data := []byte(`data`)
	result := pipe.Apply(data)
	if string(result) != "data" {
		t.Errorf("nil pipeline should passthrough")
	}
}

func TestNilPipelineLen(t *testing.T) {
	var pipe *Pipeline
	if pipe.Len() != 0 {
		t.Errorf("nil pipeline should have length 0")
	}
}
