// SPDX-License-Identifier: Apache-2.0

//go:build conformance

package conformance_test

import (
	"os"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
	"github.com/praxis-os/praxis/llm/conformance"
	"github.com/praxis-os/praxis/llm/openai"
)

// TestLive_Anthropic runs the conformance suite against the live Anthropic API.
//
// Requires ANTHROPIC_API_KEY. Budget-capped to short prompts (D88).
func TestLive_Anthropic(t *testing.T) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	p := anthropic.New(key, anthropic.WithMaxTokens(100))

	suiteReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello in one word.")},
			}},
		}
	}

	toolReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("What is the weather in Berlin?")},
			}},
			Tools: []llm.ToolDefinition{{
				Name:        "get_weather",
				Description: "Get weather for a city",
				InputSchema: []byte(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
			}},
		}
	}

	conformance.RunSuite(t, p, suiteReq, toolReq)
}

// TestLive_OpenAI runs the conformance suite against the live OpenAI API.
//
// Requires OPENAI_API_KEY. Budget-capped to short prompts (D88).
func TestLive_OpenAI(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	p := openai.New(key)

	suiteReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "gpt-4o-mini",
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello in one word.")},
			}},
		}
	}

	toolReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "gpt-4o-mini",
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("What is the weather in Berlin?")},
			}},
			Tools: []llm.ToolDefinition{{
				Name:        "get_weather",
				Description: "Get weather for a city",
				InputSchema: []byte(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
			}},
		}
	}

	conformance.RunSuite(t, p, suiteReq, toolReq)
}
