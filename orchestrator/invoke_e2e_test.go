// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/invocation"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/tools"
)

// funcInvoker adapts a plain function to the tools.Invoker interface, enabling
// inline tool implementations in test cases without a dedicated struct.
type funcInvoker func(ctx context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error)

func (f funcInvoker) Invoke(ctx context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
	return f(ctx, call)
}

// Compile-time interface check.
var _ tools.Invoker = funcInvoker(nil)

// httpError implements errors.HTTPStatusError for testing the classifier's
// HTTP heuristic path without needing a real HTTP client.
type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string  { return e.msg }
func (e *httpError) HTTPStatus() int { return e.status }

// userMsg is a helper to build a single-user-message slice.
func userMsg(text string) []llm.Message {
	return []llm.Message{
		{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart(text)}},
	}
}

// toolCallResponse builds a mock.Response that returns one or more tool calls.
func toolCallResponse(inputTokens, outputTokens int64, calls ...*llm.LLMToolCall) mock.Response {
	parts := make([]llm.MessagePart, 0, len(calls))
	for _, c := range calls {
		parts = append(parts, llm.ToolCallPart(c))
	}
	return mock.Response{
		LLMResponse: llm.LLMResponse{
			Message: llm.Message{
				Role:  llm.RoleAssistant,
				Parts: parts,
			},
			StopReason: llm.StopReasonToolUse,
			Usage:      llm.TokenUsage{InputTokens: inputTokens, OutputTokens: outputTokens},
		},
	}
}

// textResponse builds a mock.Response that returns a plain text completion.
func textResponse(text string, inputTokens, outputTokens int64) mock.Response {
	return mock.Response{
		LLMResponse: llm.LLMResponse{
			Message: llm.Message{
				Role:  llm.RoleAssistant,
				Parts: []llm.MessagePart{llm.TextPart(text)},
			},
			StopReason: llm.StopReasonEndTurn,
			Usage:      llm.TokenUsage{InputTokens: inputTokens, OutputTokens: outputTokens},
		},
	}
}

// TestE2E_MultiTurnWithToolInvoker verifies that a real tools.Invoker is called
// and its result is forwarded to the LLM in the continuation turn.
func TestE2E_MultiTurnWithToolInvoker(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "call-1", Name: "weather", ArgumentsJSON: []byte(`{"city":"Berlin"}`)}

	invokerCalled := false
	inv := funcInvoker(func(_ context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
		invokerCalled = true
		if call.CallID != tc1.CallID {
			return llm.LLMToolResult{}, fmt.Errorf("unexpected call ID %q", call.CallID)
		}
		return llm.LLMToolResult{
			CallID:  call.CallID,
			Content: `{"temperature":"18°C","condition":"cloudy"}`,
			IsError: false,
		}, nil
	})

	p := mock.New(
		toolCallResponse(100, 20, tc1),
		textResponse("It is 18°C and cloudy in Berlin.", 150, 30),
	)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: userMsg("What is the weather in Berlin?"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if result.Iterations != 2 {
		t.Errorf("Iterations: want 2, got %d", result.Iterations)
	}
	if !invokerCalled {
		t.Error("tool invoker was never called")
	}

	// The second LLM call should have received the tool result message.
	calls := p.Calls()
	if len(calls) != 2 {
		t.Fatalf("provider call count: want 2, got %d", len(calls))
	}
	// Second call: user + assistant(tool_call) + user(tool_result) = 3 messages.
	if len(calls[1].Messages) != 3 {
		t.Errorf("second LLM call message count: want 3, got %d", len(calls[1].Messages))
	}
	// Verify the tool result part is present in the third message.
	toolResultMsg := calls[1].Messages[2]
	if toolResultMsg.Role != llm.RoleUser {
		t.Errorf("tool result message role: want user, got %v", toolResultMsg.Role)
	}
	found := false
	for _, part := range toolResultMsg.Parts {
		if part.Type == llm.PartTypeToolResult && part.ToolResult != nil &&
			part.ToolResult.CallID == "call-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("tool result part with CallID call-1 not found in third message")
	}
}

// TestE2E_ParallelToolCalls verifies that when a single LLM response contains
// multiple tool calls, all are dispatched and all results are sent in the next turn.
func TestE2E_ParallelToolCalls(t *testing.T) {
	calls := []*llm.LLMToolCall{
		{CallID: "c1", Name: "tool_a", ArgumentsJSON: []byte(`{}`)},
		{CallID: "c2", Name: "tool_b", ArgumentsJSON: []byte(`{}`)},
		{CallID: "c3", Name: "tool_c", ArgumentsJSON: []byte(`{}`)},
	}

	invokedIDs := make(map[string]bool)
	inv := funcInvoker(func(_ context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
		invokedIDs[call.CallID] = true
		return llm.LLMToolResult{
			CallID:  call.CallID,
			Content: "ok",
			IsError: false,
		}, nil
	})

	p := mock.New(
		toolCallResponse(50, 15, calls...),
		textResponse("all done", 200, 10),
	)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: userMsg("do three things"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if result.Iterations != 2 {
		t.Errorf("Iterations: want 2, got %d", result.Iterations)
	}

	for _, c := range calls {
		if !invokedIDs[c.CallID] {
			t.Errorf("tool call %q was not invoked", c.CallID)
		}
	}

	// Second LLM call must include 3 tool result parts.
	providerCalls := p.Calls()
	if len(providerCalls) != 2 {
		t.Fatalf("provider call count: want 2, got %d", len(providerCalls))
	}
	toolResultMsg := providerCalls[1].Messages[len(providerCalls[1].Messages)-1]
	resultCount := 0
	for _, part := range toolResultMsg.Parts {
		if part.Type == llm.PartTypeToolResult {
			resultCount++
		}
	}
	if resultCount != 3 {
		t.Errorf("tool result part count in second LLM call: want 3, got %d", resultCount)
	}
}

// TestE2E_ToolInvokerFrameworkError verifies that when the tool invoker returns
// a non-nil framework error (not IsError), the invocation fails with a SystemError.
func TestE2E_ToolInvokerFrameworkError(t *testing.T) {
	inv := funcInvoker(func(_ context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
		return llm.LLMToolResult{}, fmt.Errorf("invoker crashed: connection refused")
	})

	tc1 := &llm.LLMToolCall{CallID: "c1", Name: "search", ArgumentsJSON: []byte(`{}`)}
	p := mock.New(toolCallResponse(100, 20, tc1))

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, invokeErr := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: userMsg("search for something"),
	})
	if invokeErr == nil {
		t.Fatal("expected error from framework-level invoker failure, got nil")
	}
	if result.FinalState != state.Failed {
		t.Errorf("FinalState: want Failed, got %v", result.FinalState)
	}

	var sysErr *praxiserrors.SystemError
	if !stderrors.As(invokeErr, &sysErr) {
		t.Errorf("expected *praxiserrors.SystemError, got %T: %v", invokeErr, invokeErr)
	}
}

// TestE2E_ToolReturnsErrorResult verifies that when a tool invoker returns
// LLMToolResult.IsError=true, the invocation continues (the LLM receives the
// error result and can act on it).
func TestE2E_ToolReturnsErrorResult(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "err-call", Name: "failing_tool", ArgumentsJSON: []byte(`{}`)}

	inv := funcInvoker(func(_ context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
		// Returning IsError=true but framework error = nil.
		return llm.LLMToolResult{
			CallID:  call.CallID,
			Content: "tool failed: resource not found",
			IsError: true,
		}, nil
	})

	p := mock.New(
		toolCallResponse(80, 10, tc1),
		textResponse("I could not complete that task because the tool failed.", 120, 25),
	)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: userMsg("use the failing tool"),
	})
	if err != nil {
		t.Fatalf("Invoke returned unexpected error: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if result.Iterations != 2 {
		t.Errorf("Iterations: want 2, got %d", result.Iterations)
	}

	// Verify the error result was delivered to the LLM.
	providerCalls := p.Calls()
	if len(providerCalls) != 2 {
		t.Fatalf("provider call count: want 2, got %d", len(providerCalls))
	}
	toolResultMsg := providerCalls[1].Messages[len(providerCalls[1].Messages)-1]
	found := false
	for _, part := range toolResultMsg.Parts {
		if part.Type == llm.PartTypeToolResult &&
			part.ToolResult != nil &&
			part.ToolResult.IsError &&
			part.ToolResult.CallID == "err-call" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected IsError=true tool result part in second LLM call message")
	}
}

// TestE2E_TransientProviderError verifies that a provider error that carries
// an HTTP 429 status is classified as TransientLLMError with Retryable=true.
func TestE2E_TransientProviderError(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
	}{
		{"http_429", 429},
		{"http_503", 503},
		{"http_500", 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := mock.New(mock.Response{
				Err: &httpError{status: tt.status, msg: fmt.Sprintf("provider error %d", tt.status)},
			})
			o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			result, invokeErr := o.Invoke(context.Background(), invocation.InvocationRequest{
				Messages: userMsg("hi"),
			})
			if invokeErr == nil {
				t.Fatal("expected error from transient provider failure, got nil")
			}
			if result.FinalState != state.Failed {
				t.Errorf("FinalState: want Failed, got %v", result.FinalState)
			}

			var transient *praxiserrors.TransientLLMError
			if !stderrors.As(invokeErr, &transient) {
				t.Errorf("expected *praxiserrors.TransientLLMError, got %T: %v", invokeErr, invokeErr)
				return
			}
			if !transient.Kind().IsRetryable() {
				t.Errorf("TransientLLMError.Kind().IsRetryable(): want true, got false")
			}
		})
	}
}

// TestE2E_PermanentProviderError verifies that a provider error with HTTP 400
// is classified as PermanentLLMError with Retryable=false.
func TestE2E_PermanentProviderError(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
	}{
		{"http_400", 400},
		{"http_401", 401},
		{"http_403", 403},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := mock.New(mock.Response{
				Err: &httpError{status: tt.status, msg: fmt.Sprintf("provider error %d", tt.status)},
			})
			o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			result, invokeErr := o.Invoke(context.Background(), invocation.InvocationRequest{
				Messages: userMsg("hi"),
			})
			if invokeErr == nil {
				t.Fatal("expected error from permanent provider failure, got nil")
			}
			if result.FinalState != state.Failed {
				t.Errorf("FinalState: want Failed, got %v", result.FinalState)
			}

			var permanent *praxiserrors.PermanentLLMError
			if !stderrors.As(invokeErr, &permanent) {
				t.Errorf("expected *praxiserrors.PermanentLLMError, got %T: %v", invokeErr, invokeErr)
				return
			}
			if permanent.Kind().IsRetryable() {
				t.Errorf("PermanentLLMError.Kind().IsRetryable(): want false, got true")
			}
		})
	}
}

// TestE2E_ContextCancelledDuringToolCall verifies that cancelling the context
// while a tool is in-flight causes the invocation to terminate.
func TestE2E_ContextCancelledDuringToolCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	tc1 := &llm.LLMToolCall{CallID: "slow-call", Name: "slow_tool", ArgumentsJSON: []byte(`{}`)}

	inv := funcInvoker(func(ctx context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
		// Cancel the context mid-tool, then return the context error.
		cancel()
		return llm.LLMToolResult{}, ctx.Err()
	})

	p := mock.New(
		toolCallResponse(100, 20, tc1),
		// The second response would complete the invocation, but should not
		// be reached after cancellation.
		textResponse("unreachable", 100, 10),
	)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, invokeErr := o.Invoke(ctx, invocation.InvocationRequest{
		Messages: userMsg("do something slow"),
	})
	if invokeErr == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	// The invocation must have reached a terminal state.
	if !result.FinalState.IsTerminal() {
		t.Errorf("FinalState: want terminal, got %v", result.FinalState)
	}
}

// TestE2E_MaxTokensStopReason verifies that StopReasonMaxTokens is treated as
// a successful completion (same terminal semantics as EndTurn).
func TestE2E_MaxTokensStopReason(t *testing.T) {
	p := mock.New(mock.Response{
		LLMResponse: llm.LLMResponse{
			Message: llm.Message{
				Role:  llm.RoleAssistant,
				Parts: []llm.MessagePart{llm.TextPart("truncated output")},
			},
			StopReason: llm.StopReasonMaxTokens,
			Usage:      llm.TokenUsage{InputTokens: 50, OutputTokens: 4096},
		},
	})

	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: userMsg("write a very long story"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if result.Iterations != 1 {
		t.Errorf("Iterations: want 1, got %d", result.Iterations)
	}
	if result.Response.StopReason != llm.StopReasonMaxTokens {
		t.Errorf("StopReason: want MaxTokens, got %v", result.Response.StopReason)
	}
}

// TestE2E_TokenUsageAccumulation verifies that token counts are correctly summed
// across all LLM round-trips in a multi-turn invocation.
func TestE2E_TokenUsageAccumulation(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "t1", Name: "tool_x", ArgumentsJSON: []byte(`{}`)}
	tc2 := &llm.LLMToolCall{CallID: "t2", Name: "tool_y", ArgumentsJSON: []byte(`{}`)}

	inv := funcInvoker(func(_ context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
		return llm.LLMToolResult{CallID: call.CallID, Content: "result"}, nil
	})

	// Three LLM calls with known token counts.
	p := mock.New(
		toolCallResponse(100, 20, tc1),        // turn 1: in=100, out=20
		toolCallResponse(200, 30, tc2),        // turn 2: in=200, out=30
		textResponse("final answer", 300, 40), // turn 3: in=300, out=40
	)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: userMsg("do complex work"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if result.Iterations != 3 {
		t.Errorf("Iterations: want 3, got %d", result.Iterations)
	}

	wantInput := int64(100 + 200 + 300)
	wantOutput := int64(20 + 30 + 40)
	wantTotal := wantInput + wantOutput

	if result.TokenUsage.InputTokens != wantInput {
		t.Errorf("InputTokens: want %d, got %d", wantInput, result.TokenUsage.InputTokens)
	}
	if result.TokenUsage.OutputTokens != wantOutput {
		t.Errorf("OutputTokens: want %d, got %d", wantOutput, result.TokenUsage.OutputTokens)
	}
	if result.TokenUsage.TotalTokens != wantTotal {
		t.Errorf("TotalTokens: want %d, got %d", wantTotal, result.TokenUsage.TotalTokens)
	}
}

// TestE2E_ModelPassedToProvider verifies that request-level Model overrides the
// orchestrator default and is forwarded verbatim to every LLM call.
func TestE2E_ModelPassedToProvider(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "m1", Name: "tool", ArgumentsJSON: []byte(`{}`)}
	inv := funcInvoker(func(_ context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
		return llm.LLMToolResult{CallID: call.CallID, Content: "ok"}, nil
	})

	p := mock.New(
		toolCallResponse(50, 10, tc1),
		textResponse("done", 60, 5),
	)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("default-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = o.Invoke(context.Background(), invocation.InvocationRequest{
		Model:    "override-model",
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	for i, call := range p.Calls() {
		if call.Model != "override-model" {
			t.Errorf("call[%d].Model: want override-model, got %q", i, call.Model)
		}
	}
}

// TestE2E_ConversationHistoryGrows verifies that the full conversation history is
// carried forward on each LLM call: after one tool-use turn the second LLM call
// must contain the original user message, the assistant tool call, and the
// user tool result.
func TestE2E_ConversationHistoryGrows(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "hist-1", Name: "lookup", ArgumentsJSON: []byte(`{"q":"test"}`)}

	inv := funcInvoker(func(_ context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
		return llm.LLMToolResult{
			CallID:  call.CallID,
			Content: "lookup result",
		}, nil
	})

	p := mock.New(
		toolCallResponse(70, 15, tc1),
		textResponse("here is what I found", 180, 20),
	)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: userMsg("find something"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	calls := p.Calls()
	if len(calls) != 2 {
		t.Fatalf("provider call count: want 2, got %d", len(calls))
	}

	// First call: just the user message.
	if len(calls[0].Messages) != 1 {
		t.Errorf("first call message count: want 1, got %d", len(calls[0].Messages))
	}
	if calls[0].Messages[0].Role != llm.RoleUser {
		t.Errorf("first call message role: want user, got %v", calls[0].Messages[0].Role)
	}

	// Second call: user + assistant(tool_call) + user(tool_result).
	if len(calls[1].Messages) != 3 {
		t.Fatalf("second call message count: want 3, got %d", len(calls[1].Messages))
	}

	roles := [3]llm.Role{llm.RoleUser, llm.RoleAssistant, llm.RoleUser}
	for i, want := range roles {
		if calls[1].Messages[i].Role != want {
			t.Errorf("second call messages[%d].Role: want %v, got %v", i, want, calls[1].Messages[i].Role)
		}
	}

	// The second message (assistant) must contain the tool call part.
	assistantMsg := calls[1].Messages[1]
	hasToolCall := false
	for _, part := range assistantMsg.Parts {
		if part.Type == llm.PartTypeToolCall {
			hasToolCall = true
			break
		}
	}
	if !hasToolCall {
		t.Error("assistant message in second call: expected PartTypeToolCall, none found")
	}

	// The third message (user) must contain the tool result part.
	toolResultMsg := calls[1].Messages[2]
	hasToolResult := false
	for _, part := range toolResultMsg.Parts {
		if part.Type == llm.PartTypeToolResult {
			hasToolResult = true
			break
		}
	}
	if !hasToolResult {
		t.Error("user message in second call: expected PartTypeToolResult, none found")
	}
}

// TestE2E_ZeroWiringInvocation verifies that an orchestrator created with only a
// provider (no options beyond model) works correctly for a simple invocation,
// using NullInvoker as the default tool dispatcher.
func TestE2E_ZeroWiringInvocation(t *testing.T) {
	p := mock.NewSimple("zero wiring works")
	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("bare-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: userMsg("hello"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if result.Iterations != 1 {
		t.Errorf("Iterations: want 1, got %d", result.Iterations)
	}
}

// TestE2E_ZeroWiringWithToolUse verifies that when no tool invoker is configured
// (NullInvoker default) and the LLM returns a tool call, the NullInvoker returns
// an error result and the invocation continues to the next turn successfully.
func TestE2E_ZeroWiringWithToolUse(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "null-c1", Name: "any_tool", ArgumentsJSON: []byte(`{}`)}

	p := mock.New(
		toolCallResponse(50, 10, tc1),
		textResponse("ok, no invoker available", 80, 15),
	)

	// No WithToolInvoker — defaults to NullInvoker.
	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("bare-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: userMsg("do something"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if result.Iterations != 2 {
		t.Errorf("Iterations: want 2, got %d", result.Iterations)
	}

	// The NullInvoker should have delivered an IsError=true tool result to LLM.
	calls := p.Calls()
	if len(calls) != 2 {
		t.Fatalf("provider call count: want 2, got %d", len(calls))
	}
	toolResultMsg := calls[1].Messages[len(calls[1].Messages)-1]
	for _, part := range toolResultMsg.Parts {
		if part.Type == llm.PartTypeToolResult && part.ToolResult != nil {
			if !part.ToolResult.IsError {
				t.Error("NullInvoker tool result: want IsError=true, got false")
			}
			return
		}
	}
	t.Error("no tool result part found in second LLM call")
}

// TestE2E_MultipleSequentialInvocations verifies that the same orchestrator
// instance can handle multiple sequential invocations without state leaking
// between calls.
func TestE2E_MultipleSequentialInvocations(t *testing.T) {
	// Each invocation gets its own independent mock provider.
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprintf("invocation_%d", i), func(t *testing.T) {
			msg := fmt.Sprintf("response %d", i)
			p := mock.NewSimple(msg)
			o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
				Messages: userMsg(fmt.Sprintf("query %d", i)),
			})
			if err != nil {
				t.Fatalf("Invoke %d: %v", i, err)
			}
			if result.FinalState != state.Completed {
				t.Errorf("invocation %d: FinalState: want Completed, got %v", i, result.FinalState)
			}
		})
	}
}

// TestE2E_RequestMaxIterationsOverridesOrchestratorDefault verifies that the
// per-request MaxIterations field takes precedence over the orchestrator default.
func TestE2E_RequestMaxIterationsOverridesOrchestratorDefault(t *testing.T) {
	// Build enough responses to reach the request limit.
	responses := make([]mock.Response, 10)
	for i := range responses {
		responses[i] = toolCallResponse(10, 5,
			&llm.LLMToolCall{CallID: fmt.Sprintf("r%d", i), Name: "loop", ArgumentsJSON: []byte(`{}`)},
		)
	}

	inv := funcInvoker(func(_ context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
		return llm.LLMToolResult{CallID: call.CallID, Content: "ok"}, nil
	})

	p := mock.New(responses...)

	// Orchestrator default is 10 (the library default), but the request overrides to 2.
	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages:      userMsg("loop"),
		MaxIterations: 2,
	})
	if err == nil {
		t.Fatal("expected error from max iterations exceeded, got nil")
	}
	if result.FinalState != state.Failed {
		t.Errorf("FinalState: want Failed, got %v", result.FinalState)
	}
	if result.Iterations != 2 {
		t.Errorf("Iterations: want 2, got %d", result.Iterations)
	}
}

// TestE2E_ContextCancelledBeforeFirstLLMCall verifies that a pre-cancelled context
// prevents any LLM call and produces Cancelled state.
func TestE2E_ContextCancelledBeforeFirstLLMCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before Invoke.

	p := mock.NewSimple("should not be reached")
	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, invokeErr := o.Invoke(ctx, invocation.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if invokeErr == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if result.FinalState != state.Cancelled {
		t.Errorf("FinalState: want Cancelled, got %v", result.FinalState)
	}
	if p.CallCount() != 0 {
		t.Errorf("provider call count: want 0, got %d — provider should not have been called", p.CallCount())
	}
}

// TestE2E_ContextDeadlineExceededClassifiedAsHardCancellation verifies that a
// deadline-exceeded context produces a CancellationError with CancellationKindHard.
func TestE2E_ContextDeadlineExceededClassifiedAsHardCancellation(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	p := mock.NewSimple("unreachable")
	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, invokeErr := o.Invoke(ctx, invocation.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if invokeErr == nil {
		t.Fatal("expected error from expired deadline, got nil")
	}
	if result.FinalState != state.Cancelled {
		t.Errorf("FinalState: want Cancelled, got %v", result.FinalState)
	}

	var cancelErr *praxiserrors.CancellationError
	if !stderrors.As(invokeErr, &cancelErr) {
		t.Errorf("expected *praxiserrors.CancellationError, got %T: %v", invokeErr, invokeErr)
		return
	}
	if cancelErr.CancelKind() != praxiserrors.CancellationKindHard {
		t.Errorf("CancelKind: want Hard, got %v", cancelErr.CancelKind())
	}
}
