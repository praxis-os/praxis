// SPDX-License-Identifier: Apache-2.0

// Package openrouter provides an [llm.Provider] for the OpenRouter API.
//
// OpenRouter exposes an OpenAI-compatible Chat Completions endpoint that
// routes requests to many upstream models (Anthropic, OpenAI, Meta, etc.).
// This package is a thin wrapper around the [openai.Provider] with
// OpenRouter-specific defaults (base URL, model, provider name) and
// optional headers (HTTP-Referer, X-Title) recommended by OpenRouter for
// app identification.
//
// Usage:
//
//	p := openrouter.New("sk-or-...",
//	    openrouter.WithModel("anthropic/claude-sonnet-4-20250514"),
//	    openrouter.WithReferer("https://myapp.example.com"),
//	)
//	orch, _ := orchestrator.New(p)
package openrouter

import (
	"github.com/praxis-os/praxis/llm/openai"
)

const (
	// defaultBaseURL is the OpenRouter API base URL.
	defaultBaseURL = "https://openrouter.ai/api"

	// defaultModel is used when LLMRequest.Model is empty.
	defaultModel = "anthropic/claude-sonnet-4-20250514"

	// name is the canonical provider name for budget lookups.
	name = "openrouter"
)

// Option configures a provider returned by [New].
type Option func(*config)

type config struct {
	model   string
	referer string
	title   string
}

// New constructs an OpenRouter provider with the given API key.
// The returned provider satisfies [llm.Provider].
func New(apiKey string, opts ...Option) *openai.Provider {
	cfg := config{model: defaultModel}
	for _, o := range opts {
		o(&cfg)
	}

	var openaiOpts []openai.Option
	openaiOpts = append(openaiOpts,
		openai.WithBaseURL(defaultBaseURL),
		openai.WithDefaultModel(cfg.model),
		openai.WithName(name),
	)

	headers := make(map[string]string)
	if cfg.referer != "" {
		headers["HTTP-Referer"] = cfg.referer
	}
	if cfg.title != "" {
		headers["X-Title"] = cfg.title
	}
	if len(headers) > 0 {
		openaiOpts = append(openaiOpts, openai.WithExtraHeaders(headers))
	}

	return openai.New(apiKey, openaiOpts...)
}

// WithModel overrides the default model identifier.
func WithModel(model string) Option {
	return func(c *config) {
		if model != "" {
			c.model = model
		}
	}
}

// WithReferer sets the HTTP-Referer header sent with every request.
// OpenRouter uses this for app identification and analytics.
func WithReferer(url string) Option {
	return func(c *config) {
		c.referer = url
	}
}

// WithTitle sets the X-Title header sent with every request.
// OpenRouter uses this for app identification in their dashboard.
func WithTitle(title string) Option {
	return func(c *config) {
		c.title = title
	}
}
