// SPDX-License-Identifier: Apache-2.0

package openrouter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/openai"
	"github.com/praxis-os/praxis/llm/openrouter"
)

func TestProvider_ImplementsInterface(t *testing.T) {
	var _ llm.Provider = openrouter.New("test-key")
	_ = t // use t
}

func TestProvider_Name(t *testing.T) {
	p := openrouter.New("test-key")
	if p.Name() != "openrouter" {
		t.Errorf("Name() = %q, want %q", p.Name(), "openrouter")
	}
}

func TestProvider_DefaultModel(t *testing.T) {
	var receivedModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Model string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		receivedModel = body.Model
		writeOKResponse(w)
	}))
	defer srv.Close()

	p := openai.New("test-key",
		openai.WithBaseURL(srv.URL),
		openai.WithDefaultModel("anthropic/claude-sonnet-4-20250514"),
		openai.WithName("openrouter"),
	)

	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if receivedModel != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("Model: want anthropic/claude-sonnet-4-20250514, got %q", receivedModel)
	}
}

func TestProvider_CustomHeaders(t *testing.T) {
	var gotReferer, gotTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-Title")
		writeOKResponse(w)
	}))
	defer srv.Close()

	// Use openai.Provider directly with extra headers to test the mechanism,
	// since openrouter.New hardcodes the base URL.
	p := openai.New("test-key",
		openai.WithBaseURL(srv.URL),
		openai.WithName("openrouter"),
		openai.WithExtraHeaders(map[string]string{
			"HTTP-Referer": "https://myapp.example.com",
			"X-Title":      "My App",
		}),
	)

	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:    "anthropic/claude-sonnet-4-20250514",
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if gotReferer != "https://myapp.example.com" {
		t.Errorf("HTTP-Referer: want https://myapp.example.com, got %q", gotReferer)
	}
	if gotTitle != "My App" {
		t.Errorf("X-Title: want 'My App', got %q", gotTitle)
	}
}

func TestWithModel(t *testing.T) {
	p := openrouter.New("key", openrouter.WithModel("meta-llama/llama-3.3-70b"))
	// Can't directly inspect model, but Name should still be openrouter.
	if p.Name() != "openrouter" {
		t.Errorf("Name() = %q, want %q", p.Name(), "openrouter")
	}
}

func writeOKResponse(w http.ResponseWriter) {
	resp := map[string]any{
		"choices": []map[string]any{{
			"index": 0, "finish_reason": "stop",
			"message": map[string]any{"role": "assistant", "content": "ok"},
		}},
		"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
