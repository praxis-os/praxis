// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/tools"
)

// delayedInvoker sleeps for a fixed duration before returning a tool result.
type delayedInvoker struct {
	delay time.Duration
}

func (d *delayedInvoker) Invoke(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	time.Sleep(d.delay)
	return tools.ToolResult{
		CallID:  call.CallID,
		Content: "done:" + call.Name,
		Status:  tools.ToolStatusSuccess,
	}, nil
}

// sequenceInvoker records the invocation order using an atomic counter.
type sequenceInvoker struct {
	counter atomic.Int32
	order   []int32 // written by Invoke, read after all invocations
}

func newSequenceInvoker(n int) *sequenceInvoker {
	return &sequenceInvoker{order: make([]int32, n)}
}

func (s *sequenceInvoker) Invoke(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	seq := s.counter.Add(1)
	// Use CallID as index hint (e.g., "pt-call-0", "pt-call-1", "pt-call-2").
	for i := range s.order {
		if call.CallID == ptCallID(i) {
			s.order[i] = seq
			break
		}
	}
	return tools.ToolResult{
		CallID:  call.CallID,
		Content: "ok",
		Status:  tools.ToolStatusSuccess,
	}, nil
}

func ptCallID(i int) string {
	return "pt-call-" + string(rune('0'+i)) //nolint:gosec // test-only, i is always 0-9
}

// ptToolCalls builds N LLMToolCall pointers for parallel tool dispatch tests.
func ptToolCalls(n int) []*llm.LLMToolCall {
	calls := make([]*llm.LLMToolCall, n)
	for i := range calls {
		calls[i] = &llm.LLMToolCall{
			CallID:        ptCallID(i),
			Name:          "tool_" + string(rune('a'+i)),
			ArgumentsJSON: []byte("{}"),
		}
	}
	return calls
}

// TestParallelToolDispatch_WallClock verifies that parallel dispatch of two
// tool calls with equal latency takes roughly one tool's latency, not the sum.
func TestParallelToolDispatch_WallClock(t *testing.T) {
	const toolDelay = 50 * time.Millisecond

	calls := ptToolCalls(2)
	p := mock.New(
		toolCallResponse(10, 5, calls...),
		textResponse("done", 5, 1),
	)
	// Mock provider defaults to SupportsParallelToolCalls=true.

	invoker := &delayedInvoker{delay: toolDelay}

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithToolInvoker(invoker),
	)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	result, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("go"),
	})
	elapsed := time.Since(start)

	if invokeErr != nil {
		t.Fatalf("Invoke error: %v", invokeErr)
	}
	if result == nil {
		t.Fatal("nil result")
	}

	// With parallel dispatch, 2 tools × 50ms should take ~50ms + overhead,
	// not 100ms. Allow generous headroom for CI variability.
	if elapsed > 90*time.Millisecond {
		t.Errorf("wall clock %v exceeds 90ms; parallel dispatch may not be working (expected ~50ms + overhead)", elapsed)
	}
	t.Logf("parallel dispatch wall clock: %v", elapsed)
}

// TestParallelToolDispatch_EventOrder verifies that all ToolCallStarted events
// are emitted before any ToolCallCompleted event, and that completion events
// appear in original call-ID order.
func TestParallelToolDispatch_EventOrder(t *testing.T) {
	calls := ptToolCalls(3)
	p := mock.New(
		toolCallResponse(10, 5, calls...),
		textResponse("done", 5, 1),
	)

	invoker := &delayedInvoker{delay: 10 * time.Millisecond}

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithToolInvoker(invoker),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("go"),
	})
	if invokeErr != nil {
		t.Fatalf("Invoke error: %v", invokeErr)
	}

	// Collect tool-related events.
	var started []string
	var completed []string
	lastStartedIdx := -1
	firstCompletedIdx := len(result.Events) // sentinel

	for i, e := range result.Events {
		switch e.Type {
		case event.EventTypeToolCallStarted:
			started = append(started, e.ToolCallID)
			lastStartedIdx = i
		case event.EventTypeToolCallCompleted:
			completed = append(completed, e.ToolCallID)
			if i < firstCompletedIdx {
				firstCompletedIdx = i
			}
		}
	}

	if len(started) != 3 {
		t.Fatalf("expected 3 ToolCallStarted events, got %d", len(started))
	}
	if len(completed) != 3 {
		t.Fatalf("expected 3 ToolCallCompleted events, got %d", len(completed))
	}

	// All Started must precede all Completed.
	if lastStartedIdx >= firstCompletedIdx {
		t.Errorf("ToolCallStarted[%d] at index %d is not before first ToolCallCompleted at index %d",
			lastStartedIdx, lastStartedIdx, firstCompletedIdx)
	}

	// Completed events must be in original call-ID order.
	for i, id := range completed {
		want := ptCallID(i)
		if id != want {
			t.Errorf("completed[%d] = %q, want %q", i, id, want)
		}
	}
}

// TestParallelToolDispatch_SequentialFallback verifies that when the provider
// does not support parallel tool calls, tools execute sequentially.
func TestParallelToolDispatch_SequentialFallback(t *testing.T) {
	calls := ptToolCalls(3)
	p := mock.New(
		toolCallResponse(10, 5, calls...),
		textResponse("done", 5, 1),
	)
	p.SetParallelToolCalls(false)

	invoker := newSequenceInvoker(3)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithToolInvoker(invoker),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("go"),
	})
	if invokeErr != nil {
		t.Fatalf("Invoke error: %v", invokeErr)
	}

	// Verify sequential order: each tool was invoked in order 1, 2, 3.
	for i, seq := range invoker.order {
		want := int32(i + 1)
		if seq != want {
			t.Errorf("tool %d invoked at sequence %d, want %d (sequential order)", i, seq, want)
		}
	}
}
