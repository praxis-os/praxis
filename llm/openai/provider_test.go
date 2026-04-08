// SPDX-License-Identifier: Apache-2.0

package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/openai"
)

func TestProvider_ImplementsInterface(_ *testing.T) {
	var _ llm.Provider = (*openai.Provider)(nil)
}

func TestProvider_Name(t *testing.T) {
	p := openai.New("test-key")
	if p.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", p.Name(), "openai")
	}
}

func TestProvider_Capabilities(t *testing.T) {
	p := openai.New("test-key")
	caps := p.Capabilities()
	if !caps.SupportsParallelToolCalls {
		t.Error("SupportsParallelToolCalls should be true")
	}
	if !caps.SupportsSystemPrompt {
		t.Error("SupportsSystemPrompt should be true")
	}
}

func TestProvider_Complete_SimpleText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header: want 'Bearer test-key', got %q", r.Header.Get("Authorization"))
		}

		resp := map[string]any{
			"id":     "chatcmpl-123",
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := openai.New("test-key", openai.WithBaseURL(srv.URL))

	resp, err := p.Complete(context.Background(), llm.LLMRequest{
		Model: "gpt-4o",
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.MessagePart{llm.TextPart("Hi")},
		}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.StopReason != llm.StopReasonEndTurn {
		t.Errorf("StopReason: want EndTurn, got %v", resp.StopReason)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("InputTokens: want 10, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 5 {
		t.Errorf("OutputTokens: want 5, got %d", resp.Usage.OutputTokens)
	}
	if len(resp.Message.Parts) != 1 || resp.Message.Parts[0].Text != "Hello, world!" {
		t.Errorf("unexpected response content: %+v", resp.Message.Parts)
	}
}

func TestProvider_Complete_ToolUse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id": "chatcmpl-456",
			"choices": []map[string]any{{
				"index":         0,
				"finish_reason": "tool_calls",
				"message": map[string]any{
					"role": "assistant",
					"tool_calls": []map[string]any{{
						"id":   "call_1",
						"type": "function",
						"function": map[string]any{
							"name":      "get_weather",
							"arguments": `{"city":"Berlin"}`,
						},
					}},
				},
			}},
			"usage": map[string]any{"prompt_tokens": 50, "completion_tokens": 20},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := openai.New("test-key", openai.WithBaseURL(srv.URL))

	resp, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("Weather?")}}},
		Tools: []llm.ToolDefinition{{
			Name:        "get_weather",
			Description: "Get weather",
			InputSchema: []byte(`{"type":"object","properties":{"city":{"type":"string"}}}`),
		}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.StopReason != llm.StopReasonToolUse {
		t.Errorf("StopReason: want ToolUse, got %v", resp.StopReason)
	}

	found := false
	for _, part := range resp.Message.Parts {
		if part.Type == llm.PartTypeToolCall && part.ToolCall != nil {
			found = true
			if part.ToolCall.CallID != "call_1" {
				t.Errorf("CallID: want call_1, got %q", part.ToolCall.CallID)
			}
			if part.ToolCall.Name != "get_weather" {
				t.Errorf("Name: want get_weather, got %q", part.ToolCall.Name)
			}
		}
	}
	if !found {
		t.Error("expected tool call part in response")
	}
}

func TestProvider_Complete_HTTP429_TransientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(429)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "rate limited", "type": "rate_limit_error"},
		})
	}))
	defer srv.Close()

	p := openai.New("test-key", openai.WithBaseURL(srv.URL))

	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var transient *praxiserrors.TransientLLMError
	if !isErrorType(err, &transient) {
		t.Errorf("expected TransientLLMError, got %T: %v", err, err)
	}
}

func TestProvider_Complete_HTTP400_PermanentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "bad request", "type": "invalid_request_error"},
		})
	}))
	defer srv.Close()

	p := openai.New("test-key", openai.WithBaseURL(srv.URL))

	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var permanent *praxiserrors.PermanentLLMError
	if !isErrorType(err, &permanent) {
		t.Errorf("expected PermanentLLMError, got %T: %v", err, err)
	}
}

func TestProvider_Complete_SystemPrompt(t *testing.T) {
	var receivedMessages []json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		json.NewDecoder(r.Body).Decode(&body)
		receivedMessages = nil
		json.Unmarshal(body["messages"], &receivedMessages)

		resp := map[string]any{
			"choices": []map[string]any{{
				"index": 0, "finish_reason": "stop",
				"message": map[string]any{"role": "assistant", "content": "ok"},
			}},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 1},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := openai.New("test-key", openai.WithBaseURL(srv.URL))

	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:        "gpt-4o",
		SystemPrompt: "You are helpful.",
		Messages:     []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// First message should be the system prompt.
	if len(receivedMessages) < 2 {
		t.Fatalf("expected at least 2 messages (system + user), got %d", len(receivedMessages))
	}

	var first map[string]string
	json.Unmarshal(receivedMessages[0], &first)
	if first["role"] != "system" {
		t.Errorf("first message role: want system, got %q", first["role"])
	}
	if first["content"] != "You are helpful." {
		t.Errorf("system content: want 'You are helpful.', got %q", first["content"])
	}
}

func TestProvider_Stream_DelegatesToComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{{
				"index": 0, "finish_reason": "stop",
				"message": map[string]any{"role": "assistant", "content": "streamed"},
			}},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 1},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := openai.New("test-key", openai.WithBaseURL(srv.URL))

	ch, err := p.Stream(context.Background(), llm.LLMRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var chunks []llm.LLMStreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !chunks[0].Final {
		t.Error("expected final chunk")
	}
}

// isErrorType is a helper for checking error types without importing errors package.
func isErrorType[T any](err error, target *T) bool {
	for err != nil {
		if _, ok := err.(T); ok {
			return true
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}
