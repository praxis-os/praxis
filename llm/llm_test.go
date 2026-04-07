// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"testing"
)

// Compile-time interface conformance check.
var _ Provider = (*stubProvider)(nil)

// stubProvider is a minimal Provider for interface verification.
type stubProvider struct{}

func (p *stubProvider) Complete(_ context.Context, _ LLMRequest) (LLMResponse, error) {
	return LLMResponse{
		Message: Message{
			Role:  RoleAssistant,
			Parts: []MessagePart{TextPart("hello")},
		},
		StopReason: StopReasonEndTurn,
		Usage:      TokenUsage{InputTokens: 10, OutputTokens: 5},
	}, nil
}

func (p *stubProvider) Stream(_ context.Context, _ LLMRequest) (<-chan LLMStreamChunk, error) {
	ch := make(chan LLMStreamChunk, 1)
	ch <- LLMStreamChunk{
		Delta: "hello",
		Final: true,
		Response: &LLMResponse{
			Message:    Message{Role: RoleAssistant, Parts: []MessagePart{TextPart("hello")}},
			StopReason: StopReasonEndTurn,
			Usage:      TokenUsage{InputTokens: 10, OutputTokens: 5},
		},
	}
	close(ch)
	return ch, nil
}

func (p *stubProvider) Name() string                    { return "stub" }
func (p *stubProvider) SupportsParallelToolCalls() bool { return false }
func (p *stubProvider) Capabilities() Capabilities {
	return Capabilities{
		SupportsStreaming:         true,
		SupportsParallelToolCalls: false,
		SupportsSystemPrompt:      true,
		SupportedStopReasons:      []StopReason{StopReasonEndTurn, StopReasonToolUse},
		MaxContextTokens:          200000,
	}
}

func TestProviderComplete(t *testing.T) {
	p := &stubProvider{}
	resp, err := p.Complete(context.Background(), LLMRequest{
		Messages:     []Message{{Role: RoleUser, Parts: []MessagePart{TextPart("hi")}}},
		Model:        "test-model",
		SystemPrompt: "You are a test assistant.",
		MaxTokens:    100,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if resp.Message.Role != RoleAssistant {
		t.Errorf("Role = %s, want %s", resp.Message.Role, RoleAssistant)
	}
	if resp.StopReason != StopReasonEndTurn {
		t.Errorf("StopReason = %s, want %s", resp.StopReason, StopReasonEndTurn)
	}
	if resp.Usage.TotalTokens() != 15 {
		t.Errorf("TotalTokens() = %d, want 15", resp.Usage.TotalTokens())
	}
}

func TestProviderStream(t *testing.T) {
	p := &stubProvider{}
	ch, err := p.Stream(context.Background(), LLMRequest{
		Messages: []Message{{Role: RoleUser, Parts: []MessagePart{TextPart("hi")}}},
		Model:    "test-model",
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var chunks int
	var lastChunk LLMStreamChunk
	for chunk := range ch {
		chunks++
		lastChunk = chunk
	}
	if chunks != 1 {
		t.Errorf("received %d chunks, want 1", chunks)
	}
	if !lastChunk.Final {
		t.Error("last chunk should have Final=true")
	}
	if lastChunk.Response == nil {
		t.Fatal("Final chunk should have non-nil Response")
	}
	if lastChunk.Response.Usage.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", lastChunk.Response.Usage.InputTokens)
	}
}

func TestProviderName(t *testing.T) {
	p := &stubProvider{}
	if got := p.Name(); got != "stub" {
		t.Errorf("Name() = %q, want %q", got, "stub")
	}
}

func TestProviderCapabilities(t *testing.T) {
	p := &stubProvider{}
	caps := p.Capabilities()
	if !caps.SupportsStreaming {
		t.Error("SupportsStreaming should be true")
	}
	if caps.SupportsParallelToolCalls {
		t.Error("SupportsParallelToolCalls should be false")
	}
	if !caps.SupportsSystemPrompt {
		t.Error("SupportsSystemPrompt should be true")
	}
	if caps.MaxContextTokens != 200000 {
		t.Errorf("MaxContextTokens = %d, want 200000", caps.MaxContextTokens)
	}
}

func TestRoleValues(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleUser, "user"},
		{RoleAssistant, "assistant"},
		{RoleSystem, "system"},
		{RoleTool, "tool"},
	}
	for _, tt := range tests {
		if got := tt.role.String(); got != tt.want {
			t.Errorf("Role(%q).String() = %q, want %q", tt.role, got, tt.want)
		}
	}
}

func TestStopReasonValues(t *testing.T) {
	tests := []struct {
		reason StopReason
		want   string
	}{
		{StopReasonEndTurn, "end_turn"},
		{StopReasonToolUse, "tool_use"},
		{StopReasonMaxTokens, "max_tokens"},
		{StopReasonStopSequence, "stop_sequence"},
	}
	for _, tt := range tests {
		if got := tt.reason.String(); got != tt.want {
			t.Errorf("StopReason(%q).String() = %q, want %q", tt.reason, got, tt.want)
		}
	}
}

func TestPartTypeValues(t *testing.T) {
	if PartTypeText != "text" {
		t.Errorf("PartTypeText = %q, want %q", PartTypeText, "text")
	}
	if PartTypeToolCall != "tool_call" {
		t.Errorf("PartTypeToolCall = %q, want %q", PartTypeToolCall, "tool_call")
	}
	if PartTypeToolResult != "tool_result" {
		t.Errorf("PartTypeToolResult = %q, want %q", PartTypeToolResult, "tool_result")
	}
	if PartTypeImageURL != "image_url" {
		t.Errorf("PartTypeImageURL = %q, want %q", PartTypeImageURL, "image_url")
	}
}

func TestTextPart(t *testing.T) {
	p := TextPart("hello")
	if p.Type != PartTypeText {
		t.Errorf("Type = %q, want %q", p.Type, PartTypeText)
	}
	if p.Text != "hello" {
		t.Errorf("Text = %q, want %q", p.Text, "hello")
	}
}

func TestToolCallPart(t *testing.T) {
	tc := &LLMToolCall{CallID: "c1", Name: "search", ArgumentsJSON: []byte(`{"q":"test"}`)}
	p := ToolCallPart(tc)
	if p.Type != PartTypeToolCall {
		t.Errorf("Type = %q, want %q", p.Type, PartTypeToolCall)
	}
	if p.ToolCall != tc {
		t.Error("ToolCall pointer mismatch")
	}
}

func TestToolResultPart(t *testing.T) {
	tr := &LLMToolResult{CallID: "c1", Content: "result", IsError: false}
	p := ToolResultPart(tr)
	if p.Type != PartTypeToolResult {
		t.Errorf("Type = %q, want %q", p.Type, PartTypeToolResult)
	}
	if p.ToolResult != tr {
		t.Error("ToolResult pointer mismatch")
	}
}

func TestTokenUsageTotalTokens(t *testing.T) {
	u := TokenUsage{InputTokens: 100, OutputTokens: 50, CachedInputTokens: 20}
	if got := u.TotalTokens(); got != 150 {
		t.Errorf("TotalTokens() = %d, want 150", got)
	}
}

func TestTokenUsageZero(t *testing.T) {
	var u TokenUsage
	if got := u.TotalTokens(); got != 0 {
		t.Errorf("zero TokenUsage.TotalTokens() = %d, want 0", got)
	}
}

func TestLLMRequestExtraParams(t *testing.T) {
	req := LLMRequest{
		Model:       "claude-3",
		ExtraParams: map[string]any{"top_k": 40, "stream": true},
	}
	if req.ExtraParams["top_k"] != 40 {
		t.Error("ExtraParams[top_k] mismatch")
	}
}

func TestToolDefinition(t *testing.T) {
	td := ToolDefinition{
		Name:        "calculator",
		Description: "Performs arithmetic",
		InputSchema: []byte(`{"type":"object","properties":{"expr":{"type":"string"}}}`),
	}
	if td.Name != "calculator" {
		t.Errorf("Name = %q, want %q", td.Name, "calculator")
	}
	if len(td.InputSchema) == 0 {
		t.Error("InputSchema should not be empty")
	}
}

func TestLLMToolCallFields(t *testing.T) {
	tc := LLMToolCall{
		CallID:        "call-abc",
		Name:          "search",
		ArgumentsJSON: []byte(`{"query":"test"}`),
	}
	if tc.CallID != "call-abc" {
		t.Errorf("CallID = %q, want %q", tc.CallID, "call-abc")
	}
}

func TestLLMToolResultFields(t *testing.T) {
	tr := LLMToolResult{CallID: "call-abc", Content: "found 3 results", IsError: false}
	if tr.IsError {
		t.Error("IsError should be false")
	}

	trErr := LLMToolResult{CallID: "call-def", Content: "timeout", IsError: true}
	if !trErr.IsError {
		t.Error("IsError should be true")
	}
}

func TestLLMStreamChunkError(t *testing.T) {
	chunk := LLMStreamChunk{
		Err: context.DeadlineExceeded,
	}
	if chunk.Err != context.DeadlineExceeded {
		t.Error("Err should be context.DeadlineExceeded")
	}
	if chunk.Final {
		t.Error("error chunks should not be Final")
	}
}

func TestLLMToolCallDelta(t *testing.T) {
	delta := LLMToolCallDelta{
		CallID:         "call-123",
		Name:           "search",
		ArgumentsDelta: `{"q":`,
	}
	if delta.CallID != "call-123" {
		t.Errorf("CallID = %q, want %q", delta.CallID, "call-123")
	}
}

func TestMessageConstruction(t *testing.T) {
	msg := Message{
		Role: RoleAssistant,
		Parts: []MessagePart{
			TextPart("Let me search for that."),
			ToolCallPart(&LLMToolCall{CallID: "c1", Name: "search", ArgumentsJSON: []byte(`{"q":"go"}`)}),
		},
	}
	if len(msg.Parts) != 2 {
		t.Fatalf("len(Parts) = %d, want 2", len(msg.Parts))
	}
	if msg.Parts[0].Type != PartTypeText {
		t.Errorf("Parts[0].Type = %q, want %q", msg.Parts[0].Type, PartTypeText)
	}
	if msg.Parts[1].Type != PartTypeToolCall {
		t.Errorf("Parts[1].Type = %q, want %q", msg.Parts[1].Type, PartTypeToolCall)
	}
}
