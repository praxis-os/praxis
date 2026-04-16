// SPDX-License-Identifier: Apache-2.0

// Package main demonstrates a single praxis invocation using a local Ollama
// instance. No API key required — Ollama runs locally.
//
// Prerequisites:
//   - Install Ollama: https://ollama.com
//   - Pull a model: ollama pull llama3.2
//
// Usage:
//
//	go run examples/ollama/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/ollama"
	"github.com/praxis-os/praxis/orchestrator"
)

func main() {
	provider := ollama.New(
		ollama.WithModel("llama3.2"),
	)

	orch, err := orchestrator.New(provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create orchestrator: %v\n", err)
		os.Exit(1)
	}

	req := praxis.InvocationRequest{
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
