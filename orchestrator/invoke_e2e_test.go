// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	"github.com/praxis-os/praxis"
	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/tools"
)

// funcInvoker adapts a plain function to the tools.Invoker interface.
type funcInvoker func(ctx context.Context, ictx tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error)

func (f funcInvoker) Invoke(ctx context.Context, ictx tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	return f(ctx, ictx, call)
}

// Compile-time interface check.
var _ tools.Invoker = funcInvoker(nil)

// httpError implements errors.HTTPStatusError for testing the classifier's
// HTTP heuristic path.
type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string   { return e.msg }
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
	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		invokerCalled = true
		if call.CallID != tc1.CallID {
			return tools.ToolResult{}, fmt.Errorf("unexpected call ID %q", call.CallID)
		}
		return tools.ToolResult{
			CallID:  call.CallID,
			Content: `{"temperature":"18°C","condition":"cloudy"}`,
			Status:  tools.ToolStatusSuccess,
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

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("What is the weather in Berlin?"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
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

// TestE2E_ParallelToolCalls verifies multiple tool calls in one response.
func TestE2E_ParallelToolCalls(t *testing.T) {
	calls := []*llm.LLMToolCall{
		{CallID: "c1", Name: "tool_a", ArgumentsJSON: []byte(`{}`)},
		{CallID: "c2", Name: "tool_b", ArgumentsJSON: []byte(`{}`)},
		{CallID: "c3", Name: "tool_c", ArgumentsJSON: []byte(`{}`)},
	}

	invokedIDs := make(map[string]bool)
	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		invokedIDs[call.CallID] = true
		return tools.ToolResult{
			CallID:  call.CallID,
			Content: "ok",
			Status:  tools.ToolStatusSuccess,
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

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("do three things"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
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

// TestE2E_ToolInvokerFrameworkError verifies framework-level invoker failure.
func TestE2E_ToolInvokerFrameworkError(t *testing.T) {
	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, _ tools.ToolCall) (tools.ToolResult, error) {
		return tools.ToolResult{}, fmt.Errorf("invoker crashed: connection refused")
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

	result, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
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

// TestE2E_ToolReturnsErrorResult verifies error tool results continue the loop.
func TestE2E_ToolReturnsErrorResult(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "err-call", Name: "failing_tool", ArgumentsJSON: []byte(`{}`)}

	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		return tools.ToolResult{
			CallID:  call.CallID,
			Content: "tool failed: resource not found",
			Status:  tools.ToolStatusError,
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

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("use the failing tool"),
	})
	if err != nil {
		t.Fatalf("Invoke returned unexpected error: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
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

// TestE2E_TransientProviderError verifies HTTP 429/503/500 classification.
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

			result, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
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

// TestE2E_PermanentProviderError verifies HTTP 400/401/403 classification.
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

			result, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
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

// TestE2E_ContextCancelledDuringToolCall verifies cancel during tool execution.
func TestE2E_ContextCancelledDuringToolCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	tc1 := &llm.LLMToolCall{CallID: "slow-call", Name: "slow_tool", ArgumentsJSON: []byte(`{}`)}

	inv := funcInvoker(func(ctx context.Context, _ tools.InvocationContext, _ tools.ToolCall) (tools.ToolResult, error) {
		cancel()
		return tools.ToolResult{}, ctx.Err()
	})

	p := mock.New(
		toolCallResponse(100, 20, tc1),
		textResponse("unreachable", 100, 10),
	)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, invokeErr := o.Invoke(ctx, praxis.InvocationRequest{
		Messages: userMsg("do something slow"),
	})
	if invokeErr == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !result.FinalState.IsTerminal() {
		t.Errorf("FinalState: want terminal, got %v", result.FinalState)
	}
}

// TestE2E_MaxTokensStopReason verifies MaxTokens is treated as successful.
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

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("write a very long story"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
}

// TestE2E_ModelPassedToProvider verifies model forwarding to provider.
func TestE2E_ModelPassedToProvider(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "m1", Name: "tool", ArgumentsJSON: []byte(`{}`)}
	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		return tools.ToolResult{CallID: call.CallID, Content: "ok", Status: tools.ToolStatusSuccess}, nil
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

	_, err = o.Invoke(context.Background(), praxis.InvocationRequest{
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

// TestE2E_ConversationHistoryGrows verifies conversation history is carried forward.
func TestE2E_ConversationHistoryGrows(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "hist-1", Name: "lookup", ArgumentsJSON: []byte(`{"q":"test"}`)}

	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		return tools.ToolResult{
			CallID:  call.CallID,
			Content: "lookup result",
			Status:  tools.ToolStatusSuccess,
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

	_, err = o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("find something"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	calls := p.Calls()
	if len(calls) != 2 {
		t.Fatalf("provider call count: want 2, got %d", len(calls))
	}

	if len(calls[0].Messages) != 1 {
		t.Errorf("first call message count: want 1, got %d", len(calls[0].Messages))
	}

	if len(calls[1].Messages) != 3 {
		t.Fatalf("second call message count: want 3, got %d", len(calls[1].Messages))
	}

	roles := [3]llm.Role{llm.RoleUser, llm.RoleAssistant, llm.RoleUser}
	for i, want := range roles {
		if calls[1].Messages[i].Role != want {
			t.Errorf("second call messages[%d].Role: want %v, got %v", i, want, calls[1].Messages[i].Role)
		}
	}
}

// TestE2E_ZeroWiringInvocation verifies bare orchestrator works.
func TestE2E_ZeroWiringInvocation(t *testing.T) {
	p := mock.NewSimple("zero wiring works")
	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("bare-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hello"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
}

// TestE2E_ZeroWiringWithToolUse verifies NullInvoker default with tool calls.
func TestE2E_ZeroWiringWithToolUse(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "null-c1", Name: "any_tool", ArgumentsJSON: []byte(`{}`)}

	p := mock.New(
		toolCallResponse(50, 10, tc1),
		textResponse("ok, no invoker available", 80, 15),
	)

	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("bare-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("do something"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
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

// TestE2E_MultipleSequentialInvocations verifies no state leaks between calls.
func TestE2E_MultipleSequentialInvocations(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprintf("invocation_%d", i), func(t *testing.T) {
			msg := fmt.Sprintf("response %d", i)
			p := mock.NewSimple(msg)
			o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
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

// TestE2E_RequestMaxTurnsOverridesOrchestratorDefault verifies per-request
// MaxTurns takes precedence.
func TestE2E_RequestMaxTurnsOverridesOrchestratorDefault(t *testing.T) {
	responses := make([]mock.Response, 10)
	for i := range responses {
		responses[i] = toolCallResponse(10, 5,
			&llm.LLMToolCall{CallID: fmt.Sprintf("r%d", i), Name: "loop", ArgumentsJSON: []byte(`{}`)},
		)
	}

	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		return tools.ToolResult{CallID: call.CallID, Content: "ok", Status: tools.ToolStatusSuccess}, nil
	})

	p := mock.New(responses...)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("loop"),
		MaxTurns: 2,
	})
	if err == nil {
		t.Fatal("expected error from max turns exceeded, got nil")
	}
	if result.FinalState != state.Failed {
		t.Errorf("FinalState: want Failed, got %v", result.FinalState)
	}
}

// TestE2E_ContextCancelledBeforeFirstLLMCall verifies pre-cancelled context.
func TestE2E_ContextCancelledBeforeFirstLLMCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := mock.NewSimple("should not be reached")
	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, invokeErr := o.Invoke(ctx, praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if invokeErr == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if result.FinalState != state.Cancelled {
		t.Errorf("FinalState: want Cancelled, got %v", result.FinalState)
	}
	if p.CallCount() != 0 {
		t.Errorf("provider call count: want 0, got %d", p.CallCount())
	}
}

// TestE2E_ContextDeadlineExceededClassifiedAsHardCancellation verifies hard cancel.
func TestE2E_ContextDeadlineExceededClassifiedAsHardCancellation(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	p := mock.NewSimple("unreachable")
	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, invokeErr := o.Invoke(ctx, praxis.InvocationRequest{
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
