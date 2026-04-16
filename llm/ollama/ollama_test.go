// SPDX-License-Identifier: Apache-2.0

package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/openai"
	"github.com/praxis-os/praxis/llm/ollama"
)

const expectedName = "ollama"

func TestProvider_ImplementsInterface(t *testing.T) {
	var _ llm.Provider = ollama.New()
	_ = t
}

func TestProvider_Name(t *testing.T) {
	p := ollama.New()
	if p.Name() != expectedName {
		t.Errorf("Name() = %q, want %q", p.Name(), expectedName)
	}
}

func TestProvider_Capabilities(t *testing.T) {
	p := ollama.New()
	caps := p.Capabilities()
	if caps.SupportsParallelToolCalls {
		t.Error("SupportsParallelToolCalls should be false for Ollama")
	}
	if p.SupportsParallelToolCalls() {
		t.Error("SupportsParallelToolCalls() should be false for Ollama")
	}
	if caps.MaxContextTokens != 8192 {
		t.Errorf("MaxContextTokens: want 8192, got %d", caps.MaxContextTokens)
	}
}

func TestProvider_NoAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeOKResponse(w)
	}))
	defer srv.Close()

	// Use openai.Provider directly to point at test server (ollama.New
	// hardcodes localhost).
	p := openai.New("",
		openai.WithBaseURL(srv.URL),
		openai.WithName("ollama"),
	)

	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:    "llama3.2",
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization header should be empty for Ollama, got %q", gotAuth)
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

	p := openai.New("",
		openai.WithBaseURL(srv.URL),
		openai.WithDefaultModel("llama3.2"),
		openai.WithName("ollama"),
	)

	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if receivedModel != "llama3.2" {
		t.Errorf("Model: want llama3.2, got %q", receivedModel)
	}
}

func TestWithBaseURL(t *testing.T) {
	p := ollama.New(ollama.WithBaseURL("http://192.168.1.100:11434"))
	if p.Name() != expectedName {
		t.Errorf("Name() = %q, want %q", p.Name(), expectedName)
	}
}

func TestWithModel(t *testing.T) {
	p := ollama.New(ollama.WithModel("mistral"))
	if p.Name() != expectedName {
		t.Errorf("Name() = %q, want %q", p.Name(), expectedName)
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
