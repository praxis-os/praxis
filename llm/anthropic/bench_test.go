// SPDX-License-Identifier: Apache-2.0

package anthropic_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
)

// successResponseJSON is a precomputed minimal Anthropic success response body.
var successResponseJSON = func() []byte {
	resp := map[string]any{
		"id":            "msg_bench",
		"type":          "message",
		"role":          "assistant",
		"model":         "claude-sonnet-4-20250514",
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"content": []map[string]any{
			{"type": "text", "text": "Hello, world!"},
		},
		"usage": map[string]any{
			"input_tokens":  10,
			"output_tokens": 5,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}()

// BenchmarkComplete_Success measures the provider's success-path overhead
// (HTTP round-trip through httptest, JSON decode, response mapping).
//
// Run:
//
//	go test -run '^$' -bench BenchmarkComplete_Success -benchmem -count=6 ./llm/anthropic/
func BenchmarkComplete_Success(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(successResponseJSON)
	}))
	defer srv.Close()

	p := anthropic.New("bench-key", anthropic.WithBaseURL(srv.URL))
	req := llm.LLMRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.MessagePart{llm.TextPart("Hi")},
		}},
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := p.Complete(ctx, req); err != nil {
			b.Fatalf("Complete: %v", err)
		}
	}
}

// BenchmarkComplete_Error429 measures the provider's error-path overhead
// (HTTP 429 with a small error body and Retry-After header).
//
// Run:
//
//	go test -run '^$' -bench BenchmarkComplete_Error429 -benchmem -count=6 ./llm/anthropic/
func BenchmarkComplete_Error429(b *testing.B) {
	errBody := []byte(`{"error":{"type":"rate_limit_error","message":"rate limited"}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write(errBody)
	}))
	defer srv.Close()

	p := anthropic.New("bench-key", anthropic.WithBaseURL(srv.URL))
	req := llm.LLMRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.MessagePart{llm.TextPart("Hi")},
		}},
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = p.Complete(ctx, req)
	}
}
