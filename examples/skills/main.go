// SPDX-License-Identifier: Apache-2.0

// Package main demonstrates loading a SKILL.md bundle and wiring it into
// a praxis orchestrator.
//
// Usage:
//
//	ANTHROPIC_API_KEY=<key> go run examples/skills/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/skills"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: ANTHROPIC_API_KEY environment variable is not set")
		os.Exit(1)
	}

	// 1. Load the skill bundle.
	sk, warnings, err := skills.Load("./skills/testdata/valid-bundle")
	if err != nil {
		log.Fatalf("skill load: %v", err)
	}
	for _, w := range warnings {
		log.Printf("skill warning [%s]: %s", w.Kind, w.Message)
	}

	fmt.Printf("Loaded skill: %s\n", sk.Name())
	fmt.Printf("Description:  %s\n", sk.Description())
	fmt.Printf("License:      %s\n", sk.License())
	if md := sk.Metadata(); md != nil {
		fmt.Printf("Metadata:     %v\n", md)
	}
	if tools := sk.AllowedTools(); tools != nil {
		fmt.Printf("AllowedTools: %v\n", tools)
	}

	// 2. Preview composed instructions (debug helper).
	composed := skills.ComposedInstructions("You are helpful.", sk)
	fmt.Printf("\nComposed system prompt:\n%s\n\n", composed)

	// 3. Wire orchestrator with skill.
	provider := anthropic.New(apiKey)
	orch, err := orchestrator.New(
		provider,
		orchestrator.WithDefaultModel("claude-sonnet-4-6"),
		skills.WithSkill(sk),
	)
	if err != nil {
		log.Fatalf("orchestrator: %v", err)
	}

	// 4. Invoke.
	result, err := orch.Invoke(context.Background(), praxis.InvocationRequest{
		SystemPrompt: "You are helpful.",
		Messages:     []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("Review this Go code: func add(a, b int) int { return a - b }")}}},
	})
	if err != nil {
		log.Fatalf("invoke: %v", err)
	}

	if result.Response != nil {
		for _, p := range result.Response.Parts {
			if p.Type == llm.PartTypeText {
				fmt.Println(p.Text)
			}
		}
	}
}
