// SPDX-License-Identifier: Apache-2.0

package gemini_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/gemini"
)

func TestProvider_ImplementsInterface(_ *testing.T) {
	var _ llm.Provider = (*gemini.Provider)(nil)
}

func TestProvider_Name(t *testing.T) {
	p := gemini.New("test-key")
	if p.Name() != "gemini" {
		t.Errorf("Name() = %q, want %q", p.Name(), "gemini")
	}
}

func TestProvider_Capabilities(t *testing.T) {
	p := gemini.New("test-key")
	caps := p.Capabilities()
	if !caps.SupportsParallelToolCalls {
		t.Error("SupportsParallelToolCalls should be true")
	}
	if !caps.SupportsSystemPrompt {
		t.Error("SupportsSystemPrompt should be true")
	}
	if caps.MaxContextTokens != 1_048_576 {
		t.Errorf("MaxContextTokens: want 1048576, got %d", caps.MaxContextTokens)
	}
}

func TestProvider_SupportsParallelToolCalls(t *testing.T) {
	p := gemini.New("test-key")
	if !p.SupportsParallelToolCalls() {
		t.Error("SupportsParallelToolCalls: want true")
	}
}

func TestProvider_Complete_SimpleText(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, geminiSuccessBody("Hello, world!"))
	defer srv.Close()

	p := gemini.New("test-key", gemini.WithBaseURL(srv.URL))

	resp, err := p.Complete(context.Background(), llm.LLMRequest{
		Model: "gemini-2.0-flash",
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
	body := geminiToolUseBody()
	srv := newTestServer(t, http.StatusOK, body)
	defer srv.Close()

	p := gemini.New("test-key", gemini.WithBaseURL(srv.URL))

	resp, err := p.Complete(context.Background(), llm.LLMRequest{
		Model: "gemini-2.0-flash",
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.MessagePart{llm.TextPart("Weather?")},
		}},
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
			if part.ToolCall.Name != "get_weather" {
				t.Errorf("Name: want get_weather, got %q", part.ToolCall.Name)
			}
			if !strings.Contains(string(part.ToolCall.ArgumentsJSON), "Berlin") {
				t.Errorf("Args should contain Berlin, got %s", part.ToolCall.ArgumentsJSON)
			}
		}
	}
	if !found {
		t.Error("expected tool call part in response")
	}
}

func TestProvider_Complete_SystemPrompt(t *testing.T) {
	var receivedBody map[string]json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(geminiSuccessBody("ok"))
	}))
	defer srv.Close()

	p := gemini.New("test-key", gemini.WithBaseURL(srv.URL))

	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:        "gemini-2.0-flash",
		SystemPrompt: "You are helpful.",
		Messages:     []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if receivedBody["systemInstruction"] == nil {
		t.Fatal("expected systemInstruction in request")
	}
	if !strings.Contains(string(receivedBody["systemInstruction"]), "You are helpful.") {
		t.Errorf("systemInstruction should contain prompt, got %s", receivedBody["systemInstruction"])
	}
}

func TestProvider_Complete_AuthViaQueryParam(t *testing.T) {
	var receivedQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.Query().Get("key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(geminiSuccessBody("ok"))
	}))
	defer srv.Close()

	p := gemini.New("my-api-key", gemini.WithBaseURL(srv.URL))

	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:    "gemini-2.0-flash",
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if receivedQuery != "my-api-key" {
		t.Errorf("API key query param: want my-api-key, got %q", receivedQuery)
	}
}

func TestProvider_Complete_DefaultModel(t *testing.T) {
	var receivedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(geminiSuccessBody("ok"))
	}))
	defer srv.Close()

	p := gemini.New("key", gemini.WithBaseURL(srv.URL), gemini.WithDefaultModel("gemini-1.5-pro"))

	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(receivedPath, "gemini-1.5-pro") {
		t.Errorf("URL path should contain model name, got %q", receivedPath)
	}
}

func TestProvider_Complete_HTTP400_PermanentError(t *testing.T) {
	srv := newTestServer(t, 400, geminiErrorBody("bad request"))
	defer srv.Close()

	p := gemini.New("key", gemini.WithBaseURL(srv.URL))
	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:    "gemini-2.0-flash",
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

func TestProvider_Complete_HTTP401_PermanentError(t *testing.T) {
	srv := newTestServer(t, 401, geminiErrorBody("invalid api key"))
	defer srv.Close()

	p := gemini.New("bad-key", gemini.WithBaseURL(srv.URL))
	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:    "gemini-2.0-flash",
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

func TestProvider_Complete_HTTP429_TransientError(t *testing.T) {
	srv := newTestServer(t, 429, geminiErrorBody("rate limited"))
	defer srv.Close()

	p := gemini.New("key", gemini.WithBaseURL(srv.URL))
	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:    "gemini-2.0-flash",
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

func TestProvider_Complete_HTTP500_TransientError(t *testing.T) {
	srv := newTestServer(t, 500, geminiErrorBody("internal error"))
	defer srv.Close()

	p := gemini.New("key", gemini.WithBaseURL(srv.URL))
	_, err := p.Complete(context.Background(), llm.LLMRequest{
		Model:    "gemini-2.0-flash",
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

func TestProvider_Stream_DelegatesToComplete(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, geminiSuccessBody("streamed"))
	defer srv.Close()

	p := gemini.New("key", gemini.WithBaseURL(srv.URL))

	ch, err := p.Stream(context.Background(), llm.LLMRequest{
		Model:    "gemini-2.0-flash",
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

func TestProvider_Stream_ErrorFromComplete(t *testing.T) {
	srv := newTestServer(t, 500, geminiErrorBody("server error"))
	defer srv.Close()

	p := gemini.New("key", gemini.WithBaseURL(srv.URL))
	ch, err := p.Stream(context.Background(), llm.LLMRequest{
		Model:    "gemini-2.0-flash",
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Stream setup: %v", err)
	}
	var chunks []llm.LLMStreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 error chunk, got %d", len(chunks))
	}
	if chunks[0].Err == nil {
		t.Error("expected error in stream chunk")
	}
}

func TestProvider_Complete_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// Block until test ends.
		select {}
	}))
	defer srv.Close()

	p := gemini.New("key", gemini.WithBaseURL(srv.URL))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := p.Complete(ctx, llm.LLMRequest{
		Model:    "gemini-2.0-flash",
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// --- Test helpers ---

func newTestServer(t *testing.T, status int, body []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
}

func geminiSuccessBody(content string) []byte {
	resp := map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"role": "model",
				"parts": []map[string]any{
					{"text": content},
				},
			},
			"finishReason": "STOP",
		}},
		"usageMetadata": map[string]any{
			"promptTokenCount":     10,
			"candidatesTokenCount": 5,
			"totalTokenCount":      15,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func geminiToolUseBody() []byte {
	resp := map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"role": "model",
				"parts": []map[string]any{
					{
						"functionCall": map[string]any{
							"name": "get_weather",
							"args": map[string]any{"city": "Berlin"},
						},
					},
				},
			},
			"finishReason": "STOP",
		}},
		"usageMetadata": map[string]any{
			"promptTokenCount":     50,
			"candidatesTokenCount": 20,
			"totalTokenCount":      70,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func geminiErrorBody(msg string) []byte {
	resp := map[string]any{
		"error": map[string]any{
			"code":    400,
			"message": msg,
			"status":  "INVALID_ARGUMENT",
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func isErrorType[T any](err error, _ *T) bool {
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
