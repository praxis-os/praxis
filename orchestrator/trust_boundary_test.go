// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/tools"
)

// levelCapture is a slog.Handler that records the levels of all log records.
type levelCapture struct {
	mu     sync.Mutex
	levels []slog.Level
}

func (c *levelCapture) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (c *levelCapture) Handle(_ context.Context, r slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.levels = append(c.levels, r.Level)
	return nil
}
func (c *levelCapture) WithAttrs(_ []slog.Attr) slog.Handler { return c }
func (c *levelCapture) WithGroup(_ string) slog.Handler       { return c }

func (c *levelCapture) hasLevel(level slog.Level) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range c.levels {
		if l == level {
			return true
		}
	}
	return false
}

// erroringPreLLMFilter returns an error (not panic) for trust boundary testing.
type erroringPreLLMFilter struct{}

func (erroringPreLLMFilter) Filter(_ context.Context, _ []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	return nil, nil, context.DeadlineExceeded
}

// erroringPostToolFilter returns an error (not panic) for trust boundary testing.
type erroringPostToolFilter struct{}

func (erroringPostToolFilter) Filter(_ context.Context, _ tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	return tools.ToolResult{}, nil, context.DeadlineExceeded
}

func TestTrustBoundary_PreLLMFilterError_LogsWarn(t *testing.T) {
	capture := &levelCapture{}
	logger := slog.New(capture)

	p := mock.NewSimple("unreachable")
	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPreLLMFilter(erroringPreLLMFilter{}),
		orchestrator.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, _ = o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})

	if !capture.hasLevel(slog.LevelWarn) {
		t.Error("expected WARN log for pre-LLM filter error (trust-boundary-internal)")
	}
}

func TestTrustBoundary_PreLLMFilterPanic_LogsWarn(t *testing.T) {
	capture := &levelCapture{}
	logger := slog.New(capture)

	p := mock.NewSimple("unreachable")
	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPreLLMFilter(panickingPreLLMFilter{}),
		orchestrator.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, _ = o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})

	if !capture.hasLevel(slog.LevelWarn) {
		t.Error("expected WARN log for pre-LLM filter panic (trust-boundary-internal)")
	}
}

func TestTrustBoundary_PostToolFilterError_LogsError(t *testing.T) {
	capture := &levelCapture{}
	logger := slog.New(capture)

	tc1 := &llm.LLMToolCall{CallID: "c1", Name: "tool", ArgumentsJSON: []byte(`{}`)}
	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		return tools.ToolResult{CallID: call.CallID, Content: "result", Status: tools.ToolStatusSuccess}, nil
	})

	p := mock.New(
		mock.Response{LLMResponse: llm.LLMResponse{
			Message:    llm.Message{Role: llm.RoleAssistant, Parts: []llm.MessagePart{llm.ToolCallPart(tc1)}},
			StopReason: llm.StopReasonToolUse,
		}},
	)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
		orchestrator.WithPostToolFilter(erroringPostToolFilter{}),
		orchestrator.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, _ = o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("use tool"),
	})

	if !capture.hasLevel(slog.LevelError) {
		t.Error("expected ERROR log for post-tool filter error (trust-boundary-crossing)")
	}
}

func TestTrustBoundary_PostToolFilterPanic_LogsError(t *testing.T) {
	capture := &levelCapture{}
	logger := slog.New(capture)

	tc1 := &llm.LLMToolCall{CallID: "c1", Name: "tool", ArgumentsJSON: []byte(`{}`)}
	inv := funcInvoker(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		return tools.ToolResult{CallID: call.CallID, Content: "result", Status: tools.ToolStatusSuccess}, nil
	})

	p := mock.New(
		mock.Response{LLMResponse: llm.LLMResponse{
			Message:    llm.Message{Role: llm.RoleAssistant, Parts: []llm.MessagePart{llm.ToolCallPart(tc1)}},
			StopReason: llm.StopReasonToolUse,
		}},
	)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithToolInvoker(inv),
		orchestrator.WithPostToolFilter(panickingPostToolFilter{}),
		orchestrator.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, _ = o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("use tool"),
	})

	if !capture.hasLevel(slog.LevelError) {
		t.Error("expected ERROR log for post-tool filter panic (trust-boundary-crossing)")
	}
}
