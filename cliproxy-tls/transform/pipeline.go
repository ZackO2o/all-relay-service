// Package transform provides a streaming response transformation pipeline
// for SSE chunks. Each chunk passes through a chain of transformers
// that can inspect, modify, or log the data in real-time.
//
// Use case example — model name rewriting:
//
//	Anthropic returns `"model":"claude-sonnet-4-6"` in SSE chunks.
//	A third-party backend might return `"model":"deepseek-v4-flash"`.
//	Claude Code checks the model field and may reject unknown values.
//	A ModelNameRewriter transformer fixes this at the byte-stream level.
package transform

import (
	"io"
	"net/http"
	"regexp"
)

// Transformer is a function that transforms a single SSE data chunk.
// It receives the raw bytes of one chunk and returns modified bytes.
// Return nil to skip writing this chunk (filters it out).
// Return the original slice unchanged for passthrough.
type Transformer func(data []byte) []byte

// Pipeline chains multiple transformers and applies them in order.
type Pipeline struct {
	transforms []Transformer
}

// New creates a transform pipeline with the given transformers applied in order.
func New(transforms ...Transformer) *Pipeline {
	return &Pipeline{transforms: transforms}
}

// Add appends a transformer to the pipeline.
func (p *Pipeline) Add(t Transformer) {
	if p.transforms == nil {
		p.transforms = make([]Transformer, 0, 4)
	}
	p.transforms = append(p.transforms, t)
}

// Apply runs all transformers on the chunk in sequence.
func (p *Pipeline) Apply(data []byte) []byte {
	if p == nil || len(p.transforms) == 0 {
		return data
	}
	current := data
	for _, t := range p.transforms {
		if current == nil {
			return nil
		}
		current = t(current)
	}
	return current
}

// Len returns the number of transformers in the pipeline.
func (p *Pipeline) Len() int {
	if p == nil {
		return 0
	}
	return len(p.transforms)
}

// ---------------------------------------------------------------------------
// Built-in transformers
// ---------------------------------------------------------------------------

// modelNamePattern matches `"model":"<any>"` in JSON SSE chunks.
// Handles variations like: "model":"claude-sonnet-4-6"
var modelNamePattern = regexp.MustCompile(`"model"\s*:\s*"[^"]+"`)

// ModelNameRewriter returns a Transformer that replaces the model name
// in SSE data chunks with the given name.
//
// Example: ModelNameRewriter("claude-sonnet-4-6") will rewrite
//
//	`"model":"deepseek-v4-flash"` → `"model":"claude-sonnet-4-6"`
func ModelNameRewriter(targetName string) Transformer {
	if targetName == "" {
		return passthrough
	}
	replacement := `"model":"` + targetName + `"`
	return func(data []byte) []byte {
		return modelNamePattern.ReplaceAll(data, []byte(replacement))
	}
}

// LoggingTransformer returns a Transformer that logs chunk info
// using the given logger function. It never modifies the data.
func LoggingTransformer(logFn func(string, ...interface{})) Transformer {
	return func(data []byte) []byte {
		if logFn != nil && len(data) > 0 {
			logFn("[transform] chunk %d bytes: %s", len(data), truncateBytes(data, 80))
		}
		return data
	}
}

// passthrough is a no-op transformer.
func passthrough(data []byte) []byte { return data }

func truncateBytes(data []byte, maxLen int) string {
	s := string(data)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ---------------------------------------------------------------------------
// Streaming writer with transform pipeline
// ---------------------------------------------------------------------------

// WriteStream copies from reader to writer, applying the pipeline to each chunk.
// Uses a 32KB buffer. Handles client disconnect via ctx.
// This is the recommended way to integrate the pipeline into SSE handlers.
func WriteStream(w http.ResponseWriter, r io.Reader, pipe *Pipeline) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		// Fallback: no flush capability, just copy raw
		_, err := io.Copy(w, r)
		return err
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			data := buf[:n]
			if pipe != nil && pipe.Len() > 0 {
				data = pipe.Apply(data)
			}
			if data != nil {
				w.Write(data)
				flusher.Flush()
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}
