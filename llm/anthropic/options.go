// SPDX-License-Identifier: Apache-2.0

package anthropic

import "net/http"

// Option configures a [Provider] at construction time via [New].
type Option func(*Provider)

// WithBaseURL overrides the Anthropic API base URL.
// The default is "https://api.anthropic.com".
// Useful for testing with a mock server or a corporate proxy.
func WithBaseURL(url string) Option {
	return func(p *Provider) {
		p.baseURL = url
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

// WithModel sets the default Anthropic model identifier used when
// [llm.LLMRequest.Model] is empty.
// The default is "claude-sonnet-4-20250514".
func WithModel(model string) Option {
	return func(p *Provider) {
		if model != "" {
			p.defaultModel = model
		}
	}
}

// WithMaxTokens sets the default max_tokens value used when
// [llm.LLMRequest.MaxTokens] is zero.
// The default is 4096.
func WithMaxTokens(n int) Option {
	return func(p *Provider) {
		if n > 0 {
			p.defaultMaxTokens = n
		}
	}
}
