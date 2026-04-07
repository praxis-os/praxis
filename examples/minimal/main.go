// SPDX-License-Identifier: Apache-2.0

// Package main is a minimal runnable example that demonstrates a single
// praxis invocation using the Anthropic provider.
//
// Usage:
//
//	ANTHROPIC_API_KEY=<key> go run examples/minimal/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/praxis-os/praxis/invocation"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
	"github.com/praxis-os/praxis/orchestrator"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: ANTHROPIC_API_KEY environment variable is not set")
		os.Exit(1)
	}

	provider := anthropic.New(apiKey)

	orch, err := orchestrator.New(provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create orchestrator: %v\n", err)
		os.Exit(1)
	}

	req := invocation.InvocationRequest{
		Model: "claude-haiku-4-20250514",
		Messages: []llm.Message{
			{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("What is the capital of France?")},
			},
		},
	}

	result, err := orch.Invoke(context.Background(), req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invocation failed: %v\n", err)
		os.Exit(1)
	}

	for _, part := range result.Response.Message.Parts {
		if part.Type == llm.PartTypeText {
			fmt.Println(part.Text)
		}
	}
}
