// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/praxis-os/praxis"
	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/tools"
)

// --- Panicking implementations ---

type panickingPolicyHook struct{}

func (panickingPolicyHook) Evaluate(_ context.Context, _ hooks.Phase, _ hooks.PolicyInput) (hooks.Decision, error) {
	panic("policy hook exploded")
}

type panickingPreLLMFilter struct{}

func (panickingPreLLMFilter) Filter(_ context.Context, _ []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	panic("pre-LLM filter exploded")
}

type panickingPostToolFilter struct{}

func (panickingPostToolFilter) Filter(_ context.Context, _ tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	panic("post-tool filter exploded")
}

// --- Tests ---

func TestPanicRecovery_PolicyHook(t *testing.T) {
	p := mock.NewSimple("unreachable")
	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPolicyHook(panickingPolicyHook{}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})

	// Must not panic — should return an error instead.
	if err == nil {
		t.Fatal("expected error from panicking policy hook, got nil")
	}
	if result.FinalState != state.Failed {
		t.Errorf("FinalState: want Failed, got %v", result.FinalState)
	}

	var sysErr *praxiserrors.SystemError
	if !stderrors.As(err, &sysErr) {
		t.Errorf("expected SystemError wrapping the panic, got %T: %v", err, err)
	}
}

func TestPanicRecovery_PreLLMFilter(t *testing.T) {
	p := mock.NewSimple("unreachable")
	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPreLLMFilter(panickingPreLLMFilter{}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})

	if err == nil {
		t.Fatal("expected error from panicking pre-LLM filter, got nil")
	}
	if result.FinalState != state.Failed {
		t.Errorf("FinalState: want Failed, got %v", result.FinalState)
	}

	var sysErr *praxiserrors.SystemError
	if !stderrors.As(err, &sysErr) {
		t.Errorf("expected SystemError wrapping the panic, got %T: %v", err, err)
	}
}

func TestPanicRecovery_PostToolFilter(t *testing.T) {
	// Provider returns a tool call, then a text response on the second call.
	tc1 := &llm.LLMToolCall{CallID: "c1", Name: "tool", ArgumentsJSON: []byte(`{}`)}
	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		return tools.ToolResult{CallID: call.CallID, Content: "result", Status: tools.ToolStatusSuccess}, nil
	})

	p := mock.New(
		mock.Response{LLMResponse: llm.LLMResponse{
			Message:    llm.Message{Role: llm.RoleAssistant, Parts: []llm.MessagePart{llm.ToolCallPart(tc1)}},
			StopReason: llm.StopReasonToolUse,
		}},
		mock.Response{LLMResponse: llm.LLMResponse{
			Message:    llm.Message{Role: llm.RoleAssistant, Parts: []llm.MessagePart{llm.TextPart("done")}},
			StopReason: llm.StopReasonEndTurn,
		}},
	)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
		orchestrator.WithPostToolFilter(panickingPostToolFilter{}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("use the tool"),
	})

	if err == nil {
		t.Fatal("expected error from panicking post-tool filter, got nil")
	}
	if result.FinalState != state.Failed {
		t.Errorf("FinalState: want Failed, got %v", result.FinalState)
	}
}
