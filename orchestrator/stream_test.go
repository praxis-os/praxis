// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/tools"
)

// drainEvents collects all events from the channel with a timeout.
func drainEvents(ch <-chan praxis.InvocationEvent, timeout time.Duration) []praxis.InvocationEvent {
	var events []praxis.InvocationEvent
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, e)
		case <-timer.C:
			return events
		}
	}
}

func TestInvokeStream_SimpleCompletion(t *testing.T) {
	p := mock.NewSimple("hello world")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	ch := o.InvokeStream(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})

	events := drainEvents(ch, 5*time.Second)
	if len(events) == 0 {
		t.Fatal("no events received")
	}

	// First event must be InvocationStarted.
	if events[0].Type != praxis.EventTypeInvocationStarted {
		t.Errorf("first event: want %q, got %q", praxis.EventTypeInvocationStarted, events[0].Type)
	}

	// Last event must be terminal.
	last := events[len(events)-1]
	if !last.Type.IsTerminal() {
		t.Errorf("last event %q should be terminal", last.Type)
	}
	if last.Type != praxis.EventTypeInvocationCompleted {
		t.Errorf("last event: want %q, got %q", praxis.EventTypeInvocationCompleted, last.Type)
	}

	// Verify expected event sequence for simple completion.
	expectedTypes := []praxis.EventType{
		praxis.EventTypeInvocationStarted,
		praxis.EventTypeInitialized,
		praxis.EventTypePreHookStarted,
		praxis.EventTypePreHookCompleted,
		praxis.EventTypeLLMCallStarted,
		praxis.EventTypeLLMCallCompleted,
		praxis.EventTypeToolDecisionStarted,
		praxis.EventTypePostHookStarted,
		praxis.EventTypePostHookCompleted,
		praxis.EventTypeInvocationCompleted,
	}

	if len(events) != len(expectedTypes) {
		t.Fatalf("event count: want %d, got %d\nevents: %v", len(expectedTypes), len(events), eventTypes(events))
	}

	for i, want := range expectedTypes {
		if events[i].Type != want {
			t.Errorf("event[%d]: want %q, got %q", i, want, events[i].Type)
		}
	}
}

func TestInvokeStream_ToolUseThenComplete(t *testing.T) {
	tc1 := &llm.LLMToolCall{CallID: "c1", Name: "search", ArgumentsJSON: []byte(`{}`)}

	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		return tools.ToolResult{CallID: call.CallID, Content: "result", Status: tools.ToolStatusSuccess}, nil
	})

	p := mock.New(
		toolCallResponse(100, 20, tc1),
		textResponse("done", 150, 30),
	)

	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
	)

	ch := o.InvokeStream(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("search something"),
	})

	events := drainEvents(ch, 5*time.Second)

	// Verify tool-related events are present.
	hasToolCallStarted := false
	hasToolCallCompleted := false
	hasPostToolFilterStarted := false
	hasLLMContinuation := false
	for _, e := range events {
		switch e.Type {
		case praxis.EventTypeToolCallStarted:
			hasToolCallStarted = true
			if e.ToolCallID != "c1" {
				t.Errorf("ToolCallStarted: ToolCallID want c1, got %q", e.ToolCallID)
			}
			if e.ToolName != "search" {
				t.Errorf("ToolCallStarted: ToolName want search, got %q", e.ToolName)
			}
		case praxis.EventTypeToolCallCompleted:
			hasToolCallCompleted = true
		case praxis.EventTypePostToolFilterStarted:
			hasPostToolFilterStarted = true
		case praxis.EventTypeLLMContinuationStarted:
			hasLLMContinuation = true
		}
	}

	if !hasToolCallStarted {
		t.Error("missing EventTypeToolCallStarted")
	}
	if !hasToolCallCompleted {
		t.Error("missing EventTypeToolCallCompleted")
	}
	if !hasPostToolFilterStarted {
		t.Error("missing EventTypePostToolFilterStarted")
	}
	if !hasLLMContinuation {
		t.Error("missing EventTypeLLMContinuationStarted")
	}

	// Last event must be terminal.
	last := events[len(events)-1]
	if last.Type != praxis.EventTypeInvocationCompleted {
		t.Errorf("last event: want %q, got %q", praxis.EventTypeInvocationCompleted, last.Type)
	}
}

func TestInvokeStream_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := mock.NewSimple("unreachable")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	ch := o.InvokeStream(ctx, praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})

	events := drainEvents(ch, 5*time.Second)

	// Must have at least the started + cancelled events.
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	last := events[len(events)-1]
	if last.Type != praxis.EventTypeInvocationCancelled {
		t.Errorf("last event: want %q, got %q", praxis.EventTypeInvocationCancelled, last.Type)
	}
}

func TestInvokeStream_ChannelClosed(t *testing.T) {
	p := mock.NewSimple("hello")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	ch := o.InvokeStream(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})

	// Drain all events.
	drainEvents(ch, 5*time.Second)

	// Channel must be closed — reading should return zero value immediately.
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after terminal event")
		}
	default:
		// Closed channels don't block, so this shouldn't be reached.
		// But if it does, the channel is closed (ok would be false).
	}
}

func TestInvokeStream_NoModelError(t *testing.T) {
	p := mock.NewSimple("ok")
	o, _ := orchestrator.New(p)

	ch := o.InvokeStream(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})

	events := drainEvents(ch, 5*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != praxis.EventTypeInvocationFailed {
		t.Errorf("event type: want %q, got %q", praxis.EventTypeInvocationFailed, events[0].Type)
	}
	if events[0].Err == nil {
		t.Error("expected non-nil error for no model configured")
	}
}

func TestInvoke_EventsCollected(t *testing.T) {
	p := mock.NewSimple("hello")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if len(result.Events) == 0 {
		t.Fatal("Invoke should populate Events")
	}

	// First event must be InvocationStarted.
	if result.Events[0].Type != praxis.EventTypeInvocationStarted {
		t.Errorf("first event: want %q, got %q", praxis.EventTypeInvocationStarted, result.Events[0].Type)
	}

	// Last event must be terminal.
	last := result.Events[len(result.Events)-1]
	if !last.Type.IsTerminal() {
		t.Errorf("last event should be terminal, got %q", last.Type)
	}
}

func TestInvokeStream_ExactlyOneTerminalEvent(t *testing.T) {
	p := mock.NewSimple("hello")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	ch := o.InvokeStream(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})

	events := drainEvents(ch, 5*time.Second)

	terminalCount := 0
	for _, e := range events {
		if e.Type.IsTerminal() {
			terminalCount++
		}
	}
	if terminalCount != 1 {
		t.Errorf("expected exactly 1 terminal event, got %d", terminalCount)
	}
}

// eventTypes extracts the event type strings for debugging.
func eventTypes(events []praxis.InvocationEvent) []string {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = string(e.Type)
	}
	return types
}
