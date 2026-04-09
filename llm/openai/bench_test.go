// SPDX-License-Identifier: Apache-2.0

package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/openai"
)

// successResponseJSON is a precomputed minimal OpenAI success response body.
var successResponseJSON = func() []byte {
	resp := map[string]any{
		"id":     "chatcmpl-bench",
		"object": "chat.completion",
		"model":  "gpt-4o",
		"choices": []map[string]any{{
			"index":         0,
			"finish_reason": "stop",
			"message": map[string]any{
				"role":    "assistant",
				"content": "Hello, world!",
			},
		}},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 5,
			"total_tokens":      15,
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
//	go test -run '^$' -bench BenchmarkComplete_Success -benchmem -count=6 ./llm/openai/
func BenchmarkComplete_Success(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(successResponseJSON)
	}))
	defer srv.Close()

	p := openai.New("bench-key", openai.WithBaseURL(srv.URL))
	req := llm.LLMRequest{
		Model: "gpt-4o",
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
//	go test -run '^$' -bench BenchmarkComplete_Error429 -benchmem -count=6 ./llm/openai/
func BenchmarkComplete_Error429(b *testing.B) {
	errBody := []byte(`{"error":{"message":"rate limited","type":"rate_limit_error"}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write(errBody)
	}))
	defer srv.Close()

	p := openai.New("bench-key", openai.WithBaseURL(srv.URL))
	req := llm.LLMRequest{
		Model: "gpt-4o",
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
