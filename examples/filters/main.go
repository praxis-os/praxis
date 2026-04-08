// SPDX-License-Identifier: Apache-2.0

// Package main demonstrates a PreLLMFilter that redacts PII patterns
// before they reach the LLM provider.
//
// Usage:
//
//	go run examples/filters/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
)

var ssnPattern = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)

// piiRedactFilter replaces SSN-like patterns with [REDACTED].
type piiRedactFilter struct{}

func (piiRedactFilter) Filter(_ context.Context, messages []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	var decisions []hooks.FilterDecision
	filtered := make([]llm.Message, len(messages))

	for i, msg := range messages {
		parts := make([]llm.MessagePart, len(msg.Parts))
		for j, part := range msg.Parts {
			if part.Type == llm.PartTypeText && ssnPattern.MatchString(part.Text) {
				parts[j] = llm.TextPart(ssnPattern.ReplaceAllString(part.Text, "[REDACTED]"))
				decisions = append(decisions, hooks.FilterDecision{
					Action: hooks.FilterActionRedact,
					Field:  fmt.Sprintf("messages[%d].parts[%d].text", i, j),
					Reason: "SSN pattern detected",
				})
			} else {
				parts[j] = part
			}
		}
		filtered[i] = llm.Message{Role: msg.Role, Parts: parts}
	}

	return filtered, decisions, nil
}

func main() {
	// Mock provider that echoes back what it receives.
	provider := mock.New(mock.Response{
		LLMResponse: llm.LLMResponse{
			Message:    llm.Message{Role: llm.RoleAssistant, Parts: []llm.MessagePart{llm.TextPart("I processed your data safely.")}},
			StopReason: llm.StopReasonEndTurn,
		},
	})

	orch, err := orchestrator.New(
		provider,
		orchestrator.WithDefaultModel("demo-model"),
		orchestrator.WithPreLLMFilter(piiRedactFilter{}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	result, err := orch.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.MessagePart{llm.TextPart("My SSN is 123-45-6789, please process my application.")},
		}},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Show the LLM received redacted content.
	calls := provider.Calls()
	fmt.Println("=== What the LLM received ===")
	for _, part := range calls[0].Messages[0].Parts {
		if part.Type == llm.PartTypeText {
			fmt.Println(part.Text)
		}
	}

	fmt.Println("\n=== Response ===")
	if result.Response != nil {
		for _, part := range result.Response.Parts {
			if part.Type == llm.PartTypeText {
				fmt.Println(part.Text)
			}
		}
	}
}
