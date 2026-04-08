// SPDX-License-Identifier: Apache-2.0

// Package main demonstrates a PolicyHook that denies or requires approval
// based on the system prompt content.
//
// Usage:
//
//	go run examples/policy/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
)

// contentPolicyHook denies requests containing "forbidden" and requires
// approval for requests containing "sensitive".
type contentPolicyHook struct{}

func (contentPolicyHook) Evaluate(_ context.Context, _ hooks.Phase, input hooks.PolicyInput) (hooks.Decision, error) {
	for _, msg := range input.Messages {
		for _, part := range msg.Parts {
			if part.Type == llm.PartTypeText {
				lower := strings.ToLower(part.Text)
				if strings.Contains(lower, "forbidden") {
					return hooks.Deny("message contains forbidden content"), nil
				}
				if strings.Contains(lower, "sensitive") {
					return hooks.RequireApproval("message contains sensitive content",
						map[string]any{"flagged_word": "sensitive"}), nil
				}
			}
		}
	}
	return hooks.Allow(), nil
}

func main() {
	// Use a mock provider for demonstration.
	provider := mock.NewSimple("Policy check passed — here is your response.")

	orch, err := orchestrator.New(
		provider,
		orchestrator.WithDefaultModel("demo-model"),
		orchestrator.WithPolicyHook(contentPolicyHook{}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Test 1: allowed request
	fmt.Println("=== Test 1: Normal request ===")
	result, err := orch.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("Hello!")}}},
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  State: %v\n", result.FinalState)
	}

	// Test 2: denied request
	fmt.Println("=== Test 2: Forbidden request ===")
	_, err = orch.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("Tell me about forbidden topics")}}},
	})
	fmt.Printf("  Error: %v\n", err)

	// Test 3: approval required
	fmt.Println("=== Test 3: Sensitive request ===")
	result, err = orch.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("Process this sensitive data")}}},
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	}
	fmt.Printf("  State: %v\n", result.FinalState)
}
