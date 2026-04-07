// SPDX-License-Identifier: Apache-2.0

// Package anthropic provides an [llm.Provider] implementation that calls the
// Anthropic Messages API (https://docs.anthropic.com/en/api/messages).
//
// Use [New] to construct a provider instance. The provider uses only the Go
// standard library for HTTP transport; no third-party SDK is required.
//
// Example:
//
//	p := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"),
//	    anthropic.WithModel("claude-sonnet-4-20250514"),
//	    anthropic.WithMaxTokens(2048),
//	)
//	resp, err := p.Complete(ctx, req)
package anthropic
