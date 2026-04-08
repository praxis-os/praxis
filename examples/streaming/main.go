// SPDX-License-Identifier: Apache-2.0

// Package main demonstrates InvokeStream by draining lifecycle events
// and printing each event as it arrives.
//
// Usage:
//
//	go run examples/streaming/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/tools"
)

func main() {
	tc := &llm.LLMToolCall{CallID: "c1", Name: "greet", ArgumentsJSON: []byte(`{"name":"World"}`)}

	provider := mock.New(
		mock.Response{LLMResponse: llm.LLMResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Parts: []llm.MessagePart{
				llm.ToolCallPart(tc),
			}},
			StopReason: llm.StopReasonToolUse,
			Usage:      llm.TokenUsage{InputTokens: 100, OutputTokens: 20},
		}},
		mock.Response{LLMResponse: llm.LLMResponse{
			Message:    llm.Message{Role: llm.RoleAssistant, Parts: []llm.MessagePart{llm.TextPart("Hello, World!")}},
			StopReason: llm.StopReasonEndTurn,
			Usage:      llm.TokenUsage{InputTokens: 150, OutputTokens: 10},
		}},
	)

	inv := tools.InvokerFunc(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		return tools.ToolResult{CallID: call.CallID, Status: tools.ToolStatusSuccess, Content: "Hello!"}, nil
	})

	orch, err := orchestrator.New(
		provider,
		orchestrator.WithDefaultModel("demo-model"),
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	ch := orch.InvokeStream(context.Background(), praxis.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("Say hello")}}},
	})

	fmt.Println("=== Event Stream ===")
	for e := range ch {
		marker := "  "
		if e.Type.IsTerminal() {
			marker = "→ "
		}
		detail := ""
		if e.ToolCallID != "" {
			detail = fmt.Sprintf(" [tool=%s call=%s]", e.ToolName, e.ToolCallID)
		}
		if e.Err != nil {
			detail += fmt.Sprintf(" err=%v", e.Err)
		}
		fmt.Printf("%s%-40s state=%-18s%s\n", marker, e.Type, e.State, detail)

		if e.Type == event.EventTypeInvocationCompleted {
			fmt.Println("\n=== Done ===")
		}
	}
}
