// SPDX-License-Identifier: Apache-2.0

// Package groq provides an [llm.Provider] for the Groq API.
//
// Groq exposes an OpenAI-compatible Chat Completions endpoint optimised for
// low-latency inference on dedicated LPU hardware. This package is a thin
// wrapper around the [openai.Provider] with Groq-specific defaults.
//
// Usage:
//
//	p := groq.New("gsk_...",
//	    groq.WithModel("llama-3.3-70b-versatile"),
//	)
//	orch, _ := orchestrator.New(p)
package groq

import (
	"github.com/praxis-os/praxis/llm/openai"
)

const (
	// defaultBaseURL is the Groq API base URL.
	defaultBaseURL = "https://api.groq.com/openai"

	// defaultModel is used when LLMRequest.Model is empty.
	defaultModel = "llama-3.3-70b-versatile"

	// name is the canonical provider name for budget lookups.
	name = "groq"
)

// Option configures a provider returned by [New].
type Option func(*config)

type config struct {
	model string
}

// New constructs a Groq provider with the given API key.
// The returned provider satisfies [llm.Provider].
func New(apiKey string, opts ...Option) *openai.Provider {
	cfg := config{model: defaultModel}
	for _, o := range opts {
		o(&cfg)
	}

	return openai.New(apiKey,
		openai.WithBaseURL(defaultBaseURL),
		openai.WithDefaultModel(cfg.model),
		openai.WithName(name),
	)
}

// WithModel overrides the default model identifier.
func WithModel(model string) Option {
	return func(c *config) {
		if model != "" {
			c.model = model
		}
	}
}
