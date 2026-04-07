// SPDX-License-Identifier: Apache-2.0

package anthropic_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
)

// successBody returns a minimal valid Anthropic Messages API response body.
func successBody(t *testing.T, content string) []byte {
	t.Helper()
	resp := map[string]any{
		"id":           "msg_01test",
		"type":         "message",
		"role":         "assistant",
		"model":        "claude-sonnet-4-20250514",
		"stop_reason":  "end_turn",
		"stop_sequence": nil,
		"content": []map[string]any{
			{"type": "text", "text": content},
		},
		"usage": map[string]any{
			"input_tokens":  10,
			"output_tokens": 5,
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("successBody: marshal: %v", err)
	}
	return b
}

// toolUseBody returns a response body with a single tool_use block.
func toolUseBody(t *testing.T) []byte {
	t.Helper()
	resp := map[string]any{
		"id":           "msg_02test",
		"type":         "message",
		"role":         "assistant",
		"model":        "claude-sonnet-4-20250514",
		"stop_reason":  "tool_use",
		"stop_sequence": nil,
		"content": []map[string]any{
			{
				"type":  "tool_use",
				"id":    "toolu_01",
				"name":  "get_weather",
				"input": map[string]any{"city": "London"},
			},
		},
		"usage": map[string]any{
			"input_tokens":  20,
			"output_tokens": 8,
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("toolUseBody: marshal: %v", err)
	}
	return b
}

// newTestServer returns a test HTTP server that writes the given status code
// and body for every request. The server is closed when t ends.
func newTestServer(t *testing.T, status int, body []byte, extraHeaders map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify required headers.
		if r.Header.Get("x-api-key") == "" {
			t.Error("x-api-key header missing")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("anthropic-version header missing")
		}
		if r.Header.Get("content-type") != "application/json" {
			t.Errorf("content-type = %q; want application/json", r.Header.Get("content-type"))
		}
		for k, v := range extraHeaders {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// captureServer returns a test server that captures the decoded request body
// and responds with the given body.
type captureServer struct {
	*httptest.Server
	LastRequest *map[string]any
}

func newCaptureServer(t *testing.T, body []byte) *captureServer {
	t.Helper()
	cs := &captureServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
		var m map[string]any
		if jsonErr := json.Unmarshal(raw, &m); jsonErr == nil {
			cs.LastRequest = &m
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	cs.Server = srv
	return cs
}

func TestProvider_Name(t *testing.T) {
	p := anthropic.New("test-key")
	if got := p.Name(); got != "anthropic" {
		t.Errorf("Name() = %q; want %q", got, "anthropic")
	}
}

func TestProvider_SupportsParallelToolCalls(t *testing.T) {
	p := anthropic.New("test-key")
	if !p.SupportsParallelToolCalls() {
		t.Error("SupportsParallelToolCalls() = false; want true")
	}
}

func TestProvider_Capabilities(t *testing.T) {
	p := anthropic.New("test-key")
	caps := p.Capabilities()
	if !caps.SupportsSystemPrompt {
		t.Error("SupportsSystemPrompt = false; want true")
	}
	if !caps.SupportsParallelToolCalls {
		t.Error("SupportsParallelToolCalls = false; want true")
	}
	if len(caps.SupportedStopReasons) == 0 {
		t.Error("SupportedStopReasons is empty")
	}
}

func TestProvider_Complete_Success(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, successBody(t, "Hello, world!"), nil)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v; want nil", err)
	}
	if resp.StopReason != llm.StopReasonEndTurn {
		t.Errorf("StopReason = %q; want %q", resp.StopReason, llm.StopReasonEndTurn)
	}
	if len(resp.Message.Parts) != 1 {
		t.Fatalf("len(Parts) = %d; want 1", len(resp.Message.Parts))
	}
	if resp.Message.Parts[0].Text != "Hello, world!" {
		t.Errorf("Parts[0].Text = %q; want %q", resp.Message.Parts[0].Text, "Hello, world!")
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("InputTokens = %d; want 10", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d; want 5", resp.Usage.OutputTokens)
	}
}

func TestProvider_Complete_ToolUseResponse(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, toolUseBody(t), nil)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("what's the weather?")}},
		},
		Tools: []llm.ToolDefinition{
			{
				Name:        "get_weather",
				Description: "Returns weather for a city",
				InputSchema: []byte(`{"type":"object","properties":{"city":{"type":"string"}}}`),
			},
		},
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v; want nil", err)
	}
	if resp.StopReason != llm.StopReasonToolUse {
		t.Errorf("StopReason = %q; want %q", resp.StopReason, llm.StopReasonToolUse)
	}
	if len(resp.Message.Parts) != 1 {
		t.Fatalf("len(Parts) = %d; want 1", len(resp.Message.Parts))
	}
	part := resp.Message.Parts[0]
	if part.Type != llm.PartTypeToolCall {
		t.Fatalf("Parts[0].Type = %q; want %q", part.Type, llm.PartTypeToolCall)
	}
	if part.ToolCall == nil {
		t.Fatal("Parts[0].ToolCall is nil")
	}
	if part.ToolCall.Name != "get_weather" {
		t.Errorf("ToolCall.Name = %q; want %q", part.ToolCall.Name, "get_weather")
	}
	if part.ToolCall.CallID != "toolu_01" {
		t.Errorf("ToolCall.CallID = %q; want %q", part.ToolCall.CallID, "toolu_01")
	}
}

func TestProvider_Complete_SystemPromptIsSentAsTopLevel(t *testing.T) {
	cs := newCaptureServer(t, successBody(t, "ok"))
	p := anthropic.New("test-key", anthropic.WithBaseURL(cs.URL))

	req := llm.LLMRequest{
		SystemPrompt: "You are a helpful assistant.",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if cs.LastRequest == nil {
		t.Fatal("no request captured")
	}
	sys, ok := (*cs.LastRequest)["system"].(string)
	if !ok {
		t.Fatal("system field missing or not a string")
	}
	if sys != "You are a helpful assistant." {
		t.Errorf("system = %q; want %q", sys, "You are a helpful assistant.")
	}
}

func TestProvider_Complete_SystemMessageExcludedFromMessages(t *testing.T) {
	cs := newCaptureServer(t, successBody(t, "ok"))
	p := anthropic.New("test-key", anthropic.WithBaseURL(cs.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Parts: []llm.MessagePart{llm.TextPart("system")}},
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("user")}},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if cs.LastRequest == nil {
		t.Fatal("no request captured")
	}
	msgs, ok := (*cs.LastRequest)["messages"].([]any)
	if !ok {
		t.Fatal("messages field missing or wrong type")
	}
	if len(msgs) != 1 {
		t.Errorf("len(messages) = %d; want 1 (system should be excluded)", len(msgs))
	}
}

func TestProvider_Complete_DefaultModelAndMaxTokens(t *testing.T) {
	cs := newCaptureServer(t, successBody(t, "ok"))
	p := anthropic.New("test-key", anthropic.WithBaseURL(cs.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
		// Model and MaxTokens intentionally left zero.
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if cs.LastRequest == nil {
		t.Fatal("no request captured")
	}
	model, _ := (*cs.LastRequest)["model"].(string)
	if model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q; want %q", model, "claude-sonnet-4-20250514")
	}
	mt, _ := (*cs.LastRequest)["max_tokens"].(float64)
	if int(mt) != 4096 {
		t.Errorf("max_tokens = %v; want 4096", mt)
	}
}

func TestProvider_Complete_WithModelAndMaxTokensOptions(t *testing.T) {
	cs := newCaptureServer(t, successBody(t, "ok"))
	p := anthropic.New("test-key",
		anthropic.WithBaseURL(cs.URL),
		anthropic.WithModel("claude-3-opus-20240229"),
		anthropic.WithMaxTokens(1024),
	)

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if cs.LastRequest == nil {
		t.Fatal("no request captured")
	}
	model, _ := (*cs.LastRequest)["model"].(string)
	if model != "claude-3-opus-20240229" {
		t.Errorf("model = %q; want %q", model, "claude-3-opus-20240229")
	}
	mt, _ := (*cs.LastRequest)["max_tokens"].(float64)
	if int(mt) != 1024 {
		t.Errorf("max_tokens = %v; want 1024", mt)
	}
}

func TestProvider_Complete_RequestModelOverridesDefault(t *testing.T) {
	cs := newCaptureServer(t, successBody(t, "ok"))
	p := anthropic.New("test-key", anthropic.WithBaseURL(cs.URL))

	req := llm.LLMRequest{
		Model: "claude-3-haiku-20240307",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if cs.LastRequest == nil {
		t.Fatal("no request captured")
	}
	model, _ := (*cs.LastRequest)["model"].(string)
	if model != "claude-3-haiku-20240307" {
		t.Errorf("model = %q; want %q", model, "claude-3-haiku-20240307")
	}
}

func TestProvider_Complete_ErrorMapping(t *testing.T) {
	errorBody := []byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid api key"}}`)

	tests := []struct {
		name       string
		status     int
		body       []byte
		wantKind   praxiserrors.ErrorKind
		extraHdrs  map[string]string
	}{
		{
			name:     "401 → permanent",
			status:   http.StatusUnauthorized,
			body:     errorBody,
			wantKind: praxiserrors.ErrorKindPermanentLLM,
		},
		{
			name:     "400 → permanent",
			status:   http.StatusBadRequest,
			body:     errorBody,
			wantKind: praxiserrors.ErrorKindPermanentLLM,
		},
		{
			name:      "429 → transient",
			status:    http.StatusTooManyRequests,
			body:      errorBody,
			wantKind:  praxiserrors.ErrorKindTransientLLM,
			extraHdrs: map[string]string{"retry-after": "30"},
		},
		{
			name:     "500 → transient",
			status:   http.StatusInternalServerError,
			body:     errorBody,
			wantKind: praxiserrors.ErrorKindTransientLLM,
		},
		{
			name:     "502 → transient",
			status:   http.StatusBadGateway,
			body:     errorBody,
			wantKind: praxiserrors.ErrorKindTransientLLM,
		},
		{
			name:     "503 → transient",
			status:   http.StatusServiceUnavailable,
			body:     errorBody,
			wantKind: praxiserrors.ErrorKindTransientLLM,
		},
		{
			name:     "529 → transient",
			status:   529,
			body:     errorBody,
			wantKind: praxiserrors.ErrorKindTransientLLM,
		},
		{
			name:     "404 → permanent (other)",
			status:   http.StatusNotFound,
			body:     errorBody,
			wantKind: praxiserrors.ErrorKindPermanentLLM,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t, tc.status, tc.body, tc.extraHdrs)
			p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

			req := llm.LLMRequest{
				Messages: []llm.Message{
					{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
				},
			}

			_, err := p.Complete(context.Background(), req)
			if err == nil {
				t.Fatal("expected error; got nil")
			}

			var te praxiserrors.TypedError
			if !errors.As(err, &te) {
				t.Fatalf("error does not implement TypedError: %T", err)
			}
			if te.Kind() != tc.wantKind {
				t.Errorf("Kind() = %q; want %q", te.Kind(), tc.wantKind)
			}
		})
	}
}

func TestProvider_Complete_ContextCancellation(t *testing.T) {
	// Server that blocks until the request context is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	_, err := p.Complete(ctx, req)
	if err == nil {
		t.Fatal("expected error after cancellation; got nil")
	}

	var te praxiserrors.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("error does not implement TypedError: %T", err)
	}
	if te.Kind() != praxiserrors.ErrorKindCancellation {
		t.Errorf("Kind() = %q; want %q", te.Kind(), praxiserrors.ErrorKindCancellation)
	}
}

func TestProvider_Complete_ToolResultMessagesMapping(t *testing.T) {
	cs := newCaptureServer(t, successBody(t, "done"))
	p := anthropic.New("test-key", anthropic.WithBaseURL(cs.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("call tool")}},
			{
				Role: llm.RoleAssistant,
				Parts: []llm.MessagePart{
					llm.ToolCallPart(&llm.LLMToolCall{
						CallID:        "toolu_42",
						Name:          "my_tool",
						ArgumentsJSON: []byte(`{"x":1}`),
					}),
				},
			},
			{
				Role: llm.RoleTool,
				Parts: []llm.MessagePart{
					llm.ToolResultPart(&llm.LLMToolResult{
						CallID:  "toolu_42",
						Content: `{"result":"ok"}`,
					}),
				},
			},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if cs.LastRequest == nil {
		t.Fatal("no request captured")
	}

	msgs, _ := (*cs.LastRequest)["messages"].([]any)
	// Expect 3 messages: user, assistant (tool_use), user (tool_result).
	if len(msgs) != 3 {
		t.Errorf("len(messages) = %d; want 3", len(msgs))
	}
}

func TestProvider_Stream_DeliversSingleChunk(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, successBody(t, "streamed"), nil)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	ch, err := p.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream() error = %v; want nil", err)
	}

	var chunks []llm.LLMStreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}

	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d; want 1", len(chunks))
	}
	if !chunks[0].Final {
		t.Error("chunks[0].Final = false; want true")
	}
	if chunks[0].Response == nil {
		t.Fatal("chunks[0].Response is nil")
	}
	if chunks[0].Err != nil {
		t.Errorf("chunks[0].Err = %v; want nil", chunks[0].Err)
	}
}

func TestProvider_Stream_ErrorDeliveredAsChunk(t *testing.T) {
	srv := newTestServer(t, http.StatusUnauthorized, []byte(`{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`), nil)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	ch, err := p.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream() setup error = %v; want nil", err)
	}

	var chunks []llm.LLMStreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}

	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d; want 1", len(chunks))
	}
	if chunks[0].Err == nil {
		t.Fatal("chunks[0].Err is nil; want non-nil error")
	}
}

func TestProvider_WithHTTPClient(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, successBody(t, "ok"), nil)
	customClient := &http.Client{}
	p := anthropic.New("test-key",
		anthropic.WithBaseURL(srv.URL),
		anthropic.WithHTTPClient(customClient),
	)

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// fromAPIStopReason edge cases
// ---------------------------------------------------------------------------

func stopReasonBody(t *testing.T, stopReason string) []byte {
	t.Helper()
	resp := map[string]any{
		"id":            "msg_03test",
		"type":          "message",
		"role":          "assistant",
		"model":         "claude-sonnet-4-20250514",
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"content":       []map[string]any{{"type": "text", "text": "ok"}},
		"usage":         map[string]any{"input_tokens": 1, "output_tokens": 1},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("stopReasonBody: marshal: %v", err)
	}
	return b
}

func TestFromAPIStopReason(t *testing.T) {
	tests := []struct {
		apiReason  string
		wantReason llm.StopReason
	}{
		{"end_turn", llm.StopReasonEndTurn},
		{"tool_use", llm.StopReasonToolUse},
		{"max_tokens", llm.StopReasonMaxTokens},
		{"stop_sequence", llm.StopReasonStopSequence},
		{"unknown_future_reason", llm.StopReasonEndTurn}, // default fallback
		{"", llm.StopReasonEndTurn},                      // empty string fallback
	}

	for _, tc := range tests {
		t.Run(tc.apiReason, func(t *testing.T) {
			srv := newTestServer(t, http.StatusOK, stopReasonBody(t, tc.apiReason), nil)
			p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

			req := llm.LLMRequest{
				Messages: []llm.Message{
					{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
				},
			}

			resp, err := p.Complete(context.Background(), req)
			if err != nil {
				t.Fatalf("Complete() error = %v", err)
			}
			if resp.StopReason != tc.wantReason {
				t.Errorf("StopReason = %q; want %q", resp.StopReason, tc.wantReason)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// toAPIContent error paths
// ---------------------------------------------------------------------------

func TestProvider_Complete_ToolCallNilToolCall(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, successBody(t, "ok"), nil)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{
				Role: llm.RoleAssistant,
				Parts: []llm.MessagePart{
					// PartTypeToolCall with nil ToolCall — should error.
					{Type: llm.PartTypeToolCall, ToolCall: nil},
				},
			},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nil ToolCall; got nil")
	}
}

func TestProvider_Complete_ToolResultNilToolResult(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, successBody(t, "ok"), nil)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{
				Role: llm.RoleTool,
				Parts: []llm.MessagePart{
					// PartTypeToolResult with nil ToolResult — should error.
					{Type: llm.PartTypeToolResult, ToolResult: nil},
				},
			},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nil ToolResult; got nil")
	}
}

func TestProvider_Complete_ImageURLNotSupported(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, successBody(t, "ok"), nil)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{
				Role: llm.RoleUser,
				Parts: []llm.MessagePart{
					{Type: llm.PartTypeImageURL, Text: "https://example.com/img.png"},
				},
			},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for PartTypeImageURL; got nil")
	}
}

func TestProvider_Complete_UnknownPartType(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, successBody(t, "ok"), nil)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{
				Role: llm.RoleUser,
				Parts: []llm.MessagePart{
					{Type: llm.PartType("unknown_future_type"), Text: "data"},
				},
			},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unknown part type; got nil")
	}
}

// ---------------------------------------------------------------------------
// toAPIRole error path
// ---------------------------------------------------------------------------

func TestProvider_Complete_UnsupportedRole(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, successBody(t, "ok"), nil)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			// llm.RoleSystem is filtered out before toAPIRole; use a truly
			// unknown role string to reach the default branch.
			{
				Role:  llm.Role("unsupported_role"),
				Parts: []llm.MessagePart{llm.TextPart("hi")},
			},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unsupported role; got nil")
	}
}

// ---------------------------------------------------------------------------
// provider.Complete error paths
// ---------------------------------------------------------------------------

// TestProvider_Complete_NetworkError exercises the transient error path taken
// when httpClient.Do returns a non-context error (e.g. connection refused).
func TestProvider_Complete_NetworkError(t *testing.T) {
	// Point the provider at a port that immediately refuses connections.
	p := anthropic.New("test-key", anthropic.WithBaseURL("http://127.0.0.1:1"))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected network error; got nil")
	}

	var te praxiserrors.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("error does not implement TypedError: %T", err)
	}
	if te.Kind() != praxiserrors.ErrorKindTransientLLM {
		t.Errorf("Kind() = %q; want %q", te.Kind(), praxiserrors.ErrorKindTransientLLM)
	}
}

// TestProvider_Complete_InvalidJSONResponse exercises the JSON decode error
// path when the server returns 200 but with an invalid body.
func TestProvider_Complete_InvalidJSONResponse(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, []byte(`not-valid-json`), nil)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected decode error; got nil")
	}

	var te praxiserrors.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("error does not implement TypedError: %T", err)
	}
	if te.Kind() != praxiserrors.ErrorKindTransientLLM {
		t.Errorf("Kind() = %q; want %q", te.Kind(), praxiserrors.ErrorKindTransientLLM)
	}
}

// ---------------------------------------------------------------------------
// withRetryAfter — non-numeric Retry-After (HTTP-date format)
// ---------------------------------------------------------------------------

func TestProvider_Complete_RetryAfterHTTPDate(t *testing.T) {
	errorBody := []byte(`{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`)
	// Retry-After with an HTTP-date value (non-numeric).
	extraHdrs := map[string]string{"retry-after": "Wed, 21 Oct 2025 07:28:00 GMT"}
	srv := newTestServer(t, http.StatusTooManyRequests, errorBody, extraHdrs)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected rate-limit error; got nil")
	}

	var te praxiserrors.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("error does not implement TypedError: %T", err)
	}
	if te.Kind() != praxiserrors.ErrorKindTransientLLM {
		t.Errorf("Kind() = %q; want %q", te.Kind(), praxiserrors.ErrorKindTransientLLM)
	}
}

// ---------------------------------------------------------------------------
// mapHTTPError — 5xx catch-all branch (e.g. 504 Gateway Timeout)
// ---------------------------------------------------------------------------

func TestProvider_Complete_HTTP504IsTransient(t *testing.T) {
	errorBody := []byte(`{"type":"error","error":{"type":"server_error","message":"gateway timeout"}}`)
	srv := newTestServer(t, http.StatusGatewayTimeout, errorBody, nil)
	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error; got nil")
	}

	var te praxiserrors.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("error does not implement TypedError: %T", err)
	}
	if te.Kind() != praxiserrors.ErrorKindTransientLLM {
		t.Errorf("Kind() = %q; want %q", te.Kind(), praxiserrors.ErrorKindTransientLLM)
	}
}

// ---------------------------------------------------------------------------
// toAPIRequest — Temperature field and tool with empty schema
// ---------------------------------------------------------------------------

func TestProvider_Complete_TemperatureIsForwarded(t *testing.T) {
	cs := newCaptureServer(t, successBody(t, "ok"))
	p := anthropic.New("test-key", anthropic.WithBaseURL(cs.URL))

	req := llm.LLMRequest{
		Temperature: 0.7,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if cs.LastRequest == nil {
		t.Fatal("no request captured")
	}
	temp, ok := (*cs.LastRequest)["temperature"].(float64)
	if !ok {
		t.Fatal("temperature field missing or wrong type")
	}
	if temp != 0.7 {
		t.Errorf("temperature = %v; want 0.7", temp)
	}
}

func TestProvider_Complete_ToolWithEmptySchema(t *testing.T) {
	cs := newCaptureServer(t, successBody(t, "ok"))
	p := anthropic.New("test-key", anthropic.WithBaseURL(cs.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
		Tools: []llm.ToolDefinition{
			{
				Name:        "no_schema_tool",
				Description: "A tool with no input schema",
				InputSchema: nil, // empty schema — should be replaced with default
			},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if cs.LastRequest == nil {
		t.Fatal("no request captured")
	}
	tools, ok := (*cs.LastRequest)["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatal("tools field missing or empty")
	}
}

// ---------------------------------------------------------------------------
// toAPIContent — ToolCall with empty ArgumentsJSON uses default {}
// ---------------------------------------------------------------------------

func TestProvider_Complete_ToolCallEmptyArguments(t *testing.T) {
	cs := newCaptureServer(t, successBody(t, "ok"))
	p := anthropic.New("test-key", anthropic.WithBaseURL(cs.URL))

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{
				Role: llm.RoleAssistant,
				Parts: []llm.MessagePart{
					llm.ToolCallPart(&llm.LLMToolCall{
						CallID:        "toolu_empty",
						Name:          "noop",
						ArgumentsJSON: nil, // empty — should default to {}
					}),
				},
			},
			// Follow up with a tool result so the conversation is valid.
			{
				Role: llm.RoleTool,
				Parts: []llm.MessagePart{
					llm.ToolResultPart(&llm.LLMToolResult{
						CallID:  "toolu_empty",
						Content: "done",
					}),
				},
			},
		},
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
}
