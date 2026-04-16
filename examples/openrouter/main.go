// SPDX-License-Identifier: Apache-2.0

// Package main demonstrates a single praxis invocation using the OpenRouter
// provider. OpenRouter routes requests to many upstream models.
//
// Usage:
//
//	OPENROUTER_API_KEY=<key> go run examples/openrouter/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/openrouter"
	"github.com/praxis-os/praxis/orchestrator"
)

func main() {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: OPENROUTER_API_KEY environment variable is not set")
		os.Exit(1)
	}

	provider := openrouter.New(apiKey,
		openrouter.WithReferer("https://github.com/praxis-os/praxis"),
		openrouter.WithTitle("praxis-example"),
	)

	orch, err := orchestrator.New(provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create orchestrator: %v\n", err)
		os.Exit(1)
	}

	req := praxis.InvocationRequest{
		Model: "anthropic/claude-haiku-4-20250514",
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

	if result.Response != nil {
		for _, part := range result.Response.Parts {
			if part.Type == llm.PartTypeText {
				fmt.Println(part.Text)
			}
		}
	}
}
