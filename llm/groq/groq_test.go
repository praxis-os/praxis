// SPDX-License-Identifier: Apache-2.0

package groq_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/groq"
	"github.com/praxis-os/praxis/llm/openai"
)

func TestProvider_ImplementsInterface(t *testing.T) {
	var _ llm.Provider = groq.New("test-key")
	_ = t
}

func TestProvider_Name(t *testing.T) {
	p := groq.New("test-key")
	if p.Name() != "groq" {
		t.Errorf("Name() = %q, want %q", p.Name(), "groq")
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
		openai.WithDefaultModel("llama-3.3-70b-versatile"),
		openai.WithName("groq"),
	)

	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if receivedModel != "llama-3.3-70b-versatile" {
		t.Errorf("Model: want llama-3.3-70b-versatile, got %q", receivedModel)
	}
}

func TestWithModel(t *testing.T) {
	p := groq.New("key", groq.WithModel("mixtral-8x7b-32768"))
	if p.Name() != "groq" {
		t.Errorf("Name() = %q, want %q", p.Name(), "groq")
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
