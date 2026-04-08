// SPDX-License-Identifier: Apache-2.0

package openai

import "net/http"

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
