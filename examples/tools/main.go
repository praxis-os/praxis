// SPDX-License-Identifier: Apache-2.0

// Package main demonstrates a custom tools.Invoker that dispatches tool calls.
//
// Usage:
//
//	ANTHROPIC_API_KEY=<key> go run examples/tools/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/tools"
)

// weatherInvoker handles "get_weather" tool calls.
type weatherInvoker struct{}

func (weatherInvoker) Invoke(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	switch call.Name {
	case "get_weather":
		var args struct {
			City string `json:"city"`
		}
		_ = json.Unmarshal(call.ArgumentsJSON, &args)
		return tools.ToolResult{
			CallID:  call.CallID,
			Status:  tools.ToolStatusSuccess,
			Content: fmt.Sprintf(`{"city":%q,"temp":"18°C","condition":"cloudy"}`, args.City),
		}, nil
	default:
		return tools.ToolResult{
			CallID:  call.CallID,
			Status:  tools.ToolStatusNotImplemented,
			Content: fmt.Sprintf("unknown tool: %s", call.Name),
		}, nil
	}
}

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "ANTHROPIC_API_KEY not set")
		os.Exit(1)
	}

	orch, err := orchestrator.New(
		anthropic.New(apiKey),
		orchestrator.WithToolInvoker(weatherInvoker{}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	result, err := orch.Invoke(context.Background(), praxis.InvocationRequest{
		Model: "claude-haiku-4-20250514",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("What's the weather in Berlin?")}},
		},
		Tools: []llm.ToolDefinition{{
			Name:        "get_weather",
			Description: "Get current weather for a city",
			InputSchema: []byte(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
		}},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if result.Response != nil {
		for _, part := range result.Response.Parts {
			if part.Type == llm.PartTypeText {
				fmt.Println(part.Text)
			}
		}
	}
}
