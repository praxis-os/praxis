// SPDX-License-Identifier: Apache-2.0

package mock_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
)

// Compile-time interface conformance check.
var _ llm.Provider = (*mock.Provider)(nil)

// makeReq is a helper that returns a minimal valid LLMRequest.
func makeReq() llm.LLMRequest {
	return llm.LLMRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hello")}},
		},
		Model: "test-model",
	}
}

func TestNew_ScriptedResponsesInOrder(t *testing.T) {
	r1 := llm.LLMResponse{
		Message:    llm.Message{Role: llm.RoleAssistant, Parts: []llm.MessagePart{llm.TextPart("first")}},
		StopReason: llm.StopReasonEndTurn,
	}
	r2 := llm.LLMResponse{
		Message:    llm.Message{Role: llm.RoleAssistant, Parts: []llm.MessagePart{llm.TextPart("second")}},
		StopReason: llm.StopReasonEndTurn,
	}

	p := mock.New(mock.Response{LLMResponse: r1}, mock.Response{LLMResponse: r2})
	ctx := context.Background()

	resp1, err := p.Complete(ctx, makeReq())
	if err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}
	if resp1.Message.Parts[0].Text != "first" {
		t.Errorf("first response text = %q, want %q", resp1.Message.Parts[0].Text, "first")
	}

	resp2, err := p.Complete(ctx, makeReq())
	if err != nil {
		t.Fatalf("second Complete() error = %v", err)
	}
	if resp2.Message.Parts[0].Text != "second" {
		t.Errorf("second response text = %q, want %q", resp2.Message.Parts[0].Text, "second")
	}
}

func TestNew_ScriptExhaustedReturnsError(t *testing.T) {
	p := mock.New(mock.Response{
		LLMResponse: llm.LLMResponse{StopReason: llm.StopReasonEndTurn},
	})
	ctx := context.Background()

	if _, err := p.Complete(ctx, makeReq()); err != nil {
		t.Fatalf("first call unexpected error = %v", err)
	}

	_, err := p.Complete(ctx, makeReq())
	if err == nil {
		t.Fatal("expected error when script exhausted, got nil")
	}
	if !errors.Is(err, mock.ErrScriptExhausted) {
		t.Errorf("error = %v, want wrapping ErrScriptExhausted", err)
	}
}

func TestNewSimple_ReturnsTextResponse(t *testing.T) {
	p := mock.NewSimple("hello world")
	resp, err := p.Complete(context.Background(), makeReq())
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if len(resp.Message.Parts) == 0 {
		t.Fatal("expected at least one message part")
	}
	if resp.Message.Parts[0].Text != "hello world" {
		t.Errorf("text = %q, want %q", resp.Message.Parts[0].Text, "hello world")
	}
	if resp.StopReason != llm.StopReasonEndTurn {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, llm.StopReasonEndTurn)
	}
	if resp.Message.Role != llm.RoleAssistant {
		t.Errorf("Role = %q, want %q", resp.Message.Role, llm.RoleAssistant)
	}
}

func TestNewWithToolCalls_ReturnsToolCallResponse(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "c1", Name: "search", ArgumentsJSON: []byte(`{"q":"test"}`)}
	tc2 := &llm.LLMToolCall{CallID: "c2", Name: "calc", ArgumentsJSON: []byte(`{"expr":"1+1"}`)}

	p := mock.NewWithToolCalls(tc1, tc2)
	resp, err := p.Complete(context.Background(), makeReq())
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if resp.StopReason != llm.StopReasonToolUse {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, llm.StopReasonToolUse)
	}
	if len(resp.Message.Parts) != 2 {
		t.Fatalf("len(Parts) = %d, want 2", len(resp.Message.Parts))
	}
	for i, part := range resp.Message.Parts {
		if part.Type != llm.PartTypeToolCall {
			t.Errorf("Parts[%d].Type = %q, want %q", i, part.Type, llm.PartTypeToolCall)
		}
		if part.ToolCall == nil {
			t.Errorf("Parts[%d].ToolCall is nil", i)
		}
	}
	if resp.Message.Parts[0].ToolCall.Name != "search" {
		t.Errorf("Parts[0] tool name = %q, want %q", resp.Message.Parts[0].ToolCall.Name, "search")
	}
	if resp.Message.Parts[1].ToolCall.Name != "calc" {
		t.Errorf("Parts[1] tool name = %q, want %q", resp.Message.Parts[1].ToolCall.Name, "calc")
	}
}

func TestCalls_TracksRequestHistory(t *testing.T) {
	p := mock.New(
		mock.Response{LLMResponse: llm.LLMResponse{StopReason: llm.StopReasonEndTurn}},
		mock.Response{LLMResponse: llm.LLMResponse{StopReason: llm.StopReasonEndTurn}},
	)
	ctx := context.Background()

	if p.CallCount() != 0 {
		t.Fatalf("initial CallCount = %d, want 0", p.CallCount())
	}

	req1 := makeReq()
	req1.Model = "model-a"
	req2 := makeReq()
	req2.Model = "model-b"

	if _, err := p.Complete(ctx, req1); err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}
	if _, err := p.Complete(ctx, req2); err != nil {
		t.Fatalf("second Complete() error = %v", err)
	}

	if p.CallCount() != 2 {
		t.Fatalf("CallCount = %d, want 2", p.CallCount())
	}

	calls := p.Calls()
	if len(calls) != 2 {
		t.Fatalf("len(Calls()) = %d, want 2", len(calls))
	}
	if calls[0].Model != "model-a" {
		t.Errorf("calls[0].Model = %q, want %q", calls[0].Model, "model-a")
	}
	if calls[1].Model != "model-b" {
		t.Errorf("calls[1].Model = %q, want %q", calls[1].Model, "model-b")
	}
}

func TestComplete_ContextCancellation(t *testing.T) {
	p := mock.New(mock.Response{LLMResponse: llm.LLMResponse{StopReason: llm.StopReasonEndTurn}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := p.Complete(ctx, makeReq())
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestComplete_DelayRespected(t *testing.T) {
	delay := 50 * time.Millisecond
	p := mock.New(mock.Response{
		LLMResponse: llm.LLMResponse{StopReason: llm.StopReasonEndTurn},
		Delay:       delay,
	})

	start := time.Now()
	_, err := p.Complete(context.Background(), makeReq())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if elapsed < delay {
		t.Errorf("elapsed %v < delay %v, delay was not respected", elapsed, delay)
	}
}

func TestComplete_DelayAbortedOnContextCancel(t *testing.T) {
	p := mock.New(mock.Response{
		LLMResponse: llm.LLMResponse{StopReason: llm.StopReasonEndTurn},
		Delay:       10 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := p.Complete(ctx, makeReq())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}
	// Should have cancelled well before the 10s delay.
	if elapsed > 2*time.Second {
		t.Errorf("delay was not aborted: elapsed %v", elapsed)
	}
}

func TestStream_DeliversSingleFinalChunk(t *testing.T) {
	p := mock.NewSimple("streaming text")
	ch, err := p.Stream(context.Background(), makeReq())
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var chunks []llm.LLMStreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 1 {
		t.Fatalf("received %d chunks, want 1", len(chunks))
	}
	chunk := chunks[0]

	if !chunk.Final {
		t.Error("chunk.Final should be true")
	}
	if chunk.Response == nil {
		t.Fatal("Final chunk.Response is nil")
	}
	if chunk.Delta != "streaming text" {
		t.Errorf("chunk.Delta = %q, want %q", chunk.Delta, "streaming text")
	}
	if chunk.Response.StopReason != llm.StopReasonEndTurn {
		t.Errorf("StopReason = %q, want %q", chunk.Response.StopReason, llm.StopReasonEndTurn)
	}
}

func TestStream_ErrorChunkOnScriptExhausted(t *testing.T) {
	p := mock.New() // empty script
	ch, err := p.Stream(context.Background(), makeReq())
	if err != nil {
		t.Fatalf("Stream() setup error = %v", err)
	}

	var gotErr error
	for chunk := range ch {
		if chunk.Err != nil {
			gotErr = chunk.Err
		}
	}

	if gotErr == nil {
		t.Fatal("expected error chunk, got none")
	}
	if !errors.Is(gotErr, mock.ErrScriptExhausted) {
		t.Errorf("chunk error = %v, want wrapping ErrScriptExhausted", gotErr)
	}
}

func TestStream_ContextCancellation(t *testing.T) {
	p := mock.New(mock.Response{LLMResponse: llm.LLMResponse{StopReason: llm.StopReasonEndTurn}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch, err := p.Stream(ctx, makeReq())
	if err != nil {
		// Some providers return the error immediately on setup; both are valid.
		return
	}

	var gotErr error
	for chunk := range ch {
		if chunk.Err != nil {
			gotErr = chunk.Err
		}
	}

	if gotErr == nil {
		t.Fatal("expected error from cancelled context, got none")
	}
	if !errors.Is(gotErr, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", gotErr)
	}
}

func TestName(t *testing.T) {
	p := mock.New()
	if got := p.Name(); got != "mock" {
		t.Errorf("Name() = %q, want %q", got, "mock")
	}
}

func TestSupportsParallelToolCalls_DefaultTrue(t *testing.T) {
	p := mock.New()
	if !p.SupportsParallelToolCalls() {
		t.Error("SupportsParallelToolCalls() should default to true")
	}
}

func TestSupportsParallelToolCalls_Configurable(t *testing.T) {
	p := mock.New()
	p.SetParallelToolCalls(false)
	if p.SupportsParallelToolCalls() {
		t.Error("SupportsParallelToolCalls() should be false after SetParallelToolCalls(false)")
	}
	p.SetParallelToolCalls(true)
	if !p.SupportsParallelToolCalls() {
		t.Error("SupportsParallelToolCalls() should be true after SetParallelToolCalls(true)")
	}
}

func TestCapabilities_SensibleDefaults(t *testing.T) {
	p := mock.New()
	caps := p.Capabilities()

	if !caps.SupportsStreaming {
		t.Error("SupportsStreaming should be true")
	}
	if !caps.SupportsParallelToolCalls {
		t.Error("SupportsParallelToolCalls should be true")
	}
	if !caps.SupportsSystemPrompt {
		t.Error("SupportsSystemPrompt should be true")
	}
	if caps.MaxContextTokens != 200_000 {
		t.Errorf("MaxContextTokens = %d, want 200000", caps.MaxContextTokens)
	}
	if len(caps.SupportedStopReasons) == 0 {
		t.Error("SupportedStopReasons should not be empty")
	}
}

func TestScriptedError_IsReturned(t *testing.T) {
	sentinel := errors.New("provider unavailable")
	p := mock.New(mock.Response{Err: sentinel})

	_, err := p.Complete(context.Background(), makeReq())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want wrapping sentinel", err)
	}
}

func TestCalls_SnapshotIsIsolated(t *testing.T) {
	p := mock.New(
		mock.Response{LLMResponse: llm.LLMResponse{StopReason: llm.StopReasonEndTurn}},
		mock.Response{LLMResponse: llm.LLMResponse{StopReason: llm.StopReasonEndTurn}},
	)
	ctx := context.Background()

	if _, err := p.Complete(ctx, makeReq()); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	snapshot1 := p.Calls()
	if len(snapshot1) != 1 {
		t.Fatalf("len(snapshot1) = %d, want 1", len(snapshot1))
	}

	if _, err := p.Complete(ctx, makeReq()); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// snapshot1 must not be mutated.
	if len(snapshot1) != 1 {
		t.Errorf("snapshot1 was mutated: len = %d, want 1", len(snapshot1))
	}
	snapshot2 := p.Calls()
	if len(snapshot2) != 2 {
		t.Fatalf("len(snapshot2) = %d, want 2", len(snapshot2))
	}
}
