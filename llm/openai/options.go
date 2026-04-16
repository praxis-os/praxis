// SPDX-License-Identifier: Apache-2.0

package openai

import (
	"net/http"

	"github.com/praxis-os/praxis/llm"
)

// Option configures a [Provider] at construction time via [New].
type Option func(*Provider)

// WithBaseURL overrides the OpenAI API base URL.
// The default is "https://api.openai.com".
// Useful for Azure OpenAI deployments, testing with a mock server, or a
// corporate proxy.
func WithBaseURL(url string) Option {
	return func(p *Provider) {
		if url != "" {
			p.baseURL = url
		}
	}
}

// WithHTTPClient replaces the default [http.Client] used for API requests.
// The supplied client must be non-nil.
func WithHTTPClient(c *http.Client) Option {
	return func(p *Provider) {
		if c != nil {
			p.httpClient = c
		}
	}
}

// WithDefaultModel sets the default OpenAI model identifier used when
// [llm.LLMRequest.Model] is empty.
// The default is "gpt-4o".
func WithDefaultModel(model string) Option {
	return func(p *Provider) {
		if model != "" {
			p.defaultModel = model
		}
	}
}

// WithName overrides the canonical provider name returned by [Provider.Name]
// and used in error messages and budget.PriceProvider lookups.
// The default is "openai". Useful for OpenAI-compatible services like
// OpenRouter, Groq, or Ollama.
func WithName(name string) Option {
	return func(p *Provider) {
		if name != "" {
			p.name = name
		}
	}
}

// WithExtraHeaders adds custom HTTP headers to every API request.
// Headers are set after the default content-type and authorization headers.
// Useful for services like OpenRouter that require additional headers
// (e.g., HTTP-Referer, X-Title).
func WithExtraHeaders(headers map[string]string) Option {
	return func(p *Provider) {
		if len(headers) > 0 {
			p.extraHeaders = headers
		}
	}
}

// WithCapabilities overrides the default [llm.Capabilities] snapshot returned
// by [Provider.Capabilities] and [Provider.SupportsParallelToolCalls].
// Use this when wrapping the provider for a service with different capability
// profiles (e.g., Ollama models that don't support parallel tool calls).
func WithCapabilities(caps llm.Capabilities) Option {
	return func(p *Provider) {
		p.caps = &caps
	}
}
