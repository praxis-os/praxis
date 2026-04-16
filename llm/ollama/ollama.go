// SPDX-License-Identifier: Apache-2.0

// Package ollama provides an [llm.Provider] for local Ollama instances.
//
// Ollama exposes an OpenAI-compatible Chat Completions endpoint on
// localhost. This package is a thin wrapper around the [openai.Provider]
// with Ollama-specific defaults: no API key, conservative capabilities
// (most local models don't support parallel tool calls), and
// localhost base URL.
//
// Usage:
//
//	p := ollama.New(
//	    ollama.WithModel("llama3.2"),
//	)
//	orch, _ := orchestrator.New(p)
package ollama

import (
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/openai"
)

const (
	// defaultBaseURL is the Ollama API base URL.
	defaultBaseURL = "http://localhost:11434"

	// defaultModel is used when LLMRequest.Model is empty.
	defaultModel = "llama3.2"

	// name is the canonical provider name for budget lookups.
	name = "ollama"
)

// Option configures a provider returned by [New].
type Option func(*config)

type config struct {
	baseURL string
	model   string
}

// New constructs an Ollama provider. No API key is required because Ollama
// runs locally. The returned provider satisfies [llm.Provider].
func New(opts ...Option) *openai.Provider {
	cfg := config{
		baseURL: defaultBaseURL,
		model:   defaultModel,
	}
	for _, o := range opts {
		o(&cfg)
	}

	return openai.New("", // no API key — auth header skipped
		openai.WithBaseURL(cfg.baseURL),
		openai.WithDefaultModel(cfg.model),
		openai.WithName(name),
		openai.WithCapabilities(llm.Capabilities{
			SupportsStreaming:         false,
			SupportsParallelToolCalls: false, // most local models don't
			SupportsSystemPrompt:      true,
			SupportedStopReasons: []llm.StopReason{
				llm.StopReasonEndTurn,
				llm.StopReasonMaxTokens,
			},
			MaxContextTokens: 8192, // conservative default
		}),
	)
}

// WithBaseURL overrides the Ollama API base URL.
// The default is "http://localhost:11434".
func WithBaseURL(url string) Option {
	return func(c *config) {
		if url != "" {
			c.baseURL = url
		}
	}
}

// WithModel overrides the default model identifier.
func WithModel(model string) Option {
	return func(c *config) {
		if model != "" {
			c.model = model
		}
	}
}
