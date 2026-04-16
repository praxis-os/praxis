// SPDX-License-Identifier: Apache-2.0

//go:build conformance

package conformance_test

import (
	"os"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
	"github.com/praxis-os/praxis/llm/conformance"
	"github.com/praxis-os/praxis/llm/gemini"
	"github.com/praxis-os/praxis/llm/groq"
	"github.com/praxis-os/praxis/llm/ollama"
	"github.com/praxis-os/praxis/llm/openai"
	"github.com/praxis-os/praxis/llm/openrouter"
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

// TestLive_Groq runs the conformance suite against the live Groq API.
//
// Requires GROQ_API_KEY. Budget-capped to short prompts.
func TestLive_Groq(t *testing.T) {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		t.Skip("GROQ_API_KEY not set")
	}

	p := groq.New(key)

	suiteReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "llama-3.3-70b-versatile",
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello in one word.")},
			}},
		}
	}

	toolReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "llama-3.3-70b-versatile",
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

// TestLive_OpenRouter runs the conformance suite against the live OpenRouter API.
//
// Requires OPENROUTER_API_KEY. Budget-capped to short prompts.
func TestLive_OpenRouter(t *testing.T) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		t.Skip("OPENROUTER_API_KEY not set")
	}

	p := openrouter.New(key, openrouter.WithModel("openai/gpt-4o-mini"))

	suiteReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "openai/gpt-4o-mini",
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello in one word.")},
			}},
		}
	}

	toolReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "openai/gpt-4o-mini",
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

// TestLive_Ollama runs the conformance suite against a local Ollama instance.
//
// Requires a running Ollama server. Set OLLAMA_HOST to override the default
// address. Skips tool tests since many local models don't support tool calling.
func TestLive_Ollama(t *testing.T) {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}

	p := ollama.New(ollama.WithBaseURL(host), ollama.WithModel("llama3.2"))

	suiteReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello in one word.")},
			}},
		}
	}

	// Skip tool tests for Ollama — most local models don't support them.
	conformance.RunSuite(t, p, suiteReq, nil)
}

// TestLive_Gemini runs the conformance suite against the live Gemini API.
//
// Requires GEMINI_API_KEY. Budget-capped to short prompts.
func TestLive_Gemini(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY not set")
	}

	p := gemini.New(key)

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
