// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/tools"
)

func TestBudget_TokenBreachAfterLLMCall(t *testing.T) {
	// LLM returns 200 input tokens, limit is 100.
	p := mock.New(mock.Response{
		LLMResponse: llm.LLMResponse{
			Message: llm.Message{
				Role:  llm.RoleAssistant,
				Parts: []llm.MessagePart{llm.TextPart("hello")},
			},
			StopReason: llm.StopReasonEndTurn,
			Usage:      llm.TokenUsage{InputTokens: 200, OutputTokens: 10},
		},
	})

	guard := budget.NewBudgetGuard(budget.Config{MaxInputTokens: 100})
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithBudgetGuard(guard),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err == nil {
		t.Fatal("expected budget exceeded error")
	}
	if result.FinalState != state.BudgetExceeded {
		t.Errorf("FinalState: want BudgetExceeded, got %v", result.FinalState)
	}
}

func TestBudget_TokenBreachEmitsBudgetExceededEvent(t *testing.T) {
	p := mock.New(mock.Response{
		LLMResponse: llm.LLMResponse{
			Message: llm.Message{
				Role:  llm.RoleAssistant,
				Parts: []llm.MessagePart{llm.TextPart("hello")},
			},
			StopReason: llm.StopReasonEndTurn,
			Usage:      llm.TokenUsage{InputTokens: 200, OutputTokens: 10},
		},
	})

	guard := budget.NewBudgetGuard(budget.Config{MaxInputTokens: 100})
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithBudgetGuard(guard),
	)

	ch := o.InvokeStream(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	events := drainEvents(ch)

	last := events[len(events)-1]
	if last.Type != event.EventTypeBudgetExceeded {
		t.Errorf("last event: want BudgetExceeded, got %q", last.Type)
	}
}

func TestBudget_ToolCallBreachDuringToolCycle(t *testing.T) {
	// Allow 1 tool call max, LLM requests 2 tools.
	calls := []*llm.LLMToolCall{
		{CallID: "c1", Name: "tool_a", ArgumentsJSON: []byte(`{}`)},
		{CallID: "c2", Name: "tool_b", ArgumentsJSON: []byte(`{}`)},
	}

	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		return tools.ToolResult{CallID: call.CallID, Content: "ok", Status: tools.ToolStatusSuccess}, nil
	})

	p := mock.New(
		toolCallResponse(50, 10, calls...),
		textResponse("done", 50, 10),
	)

	guard := budget.NewBudgetGuard(budget.Config{MaxToolCalls: 1})
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithBudgetGuard(guard),
		orchestrator.WithToolInvoker(inv),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("use tools"),
	})
	if err == nil {
		t.Fatal("expected budget exceeded error")
	}
	if result.FinalState != state.BudgetExceeded {
		t.Errorf("FinalState: want BudgetExceeded, got %v", result.FinalState)
	}
}

func TestBudget_NoBreach(t *testing.T) {
	p := mock.NewSimple("hello")
	guard := budget.NewBudgetGuard(budget.Config{MaxInputTokens: 10000})
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithBudgetGuard(guard),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
}
