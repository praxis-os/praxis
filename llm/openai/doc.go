// SPDX-License-Identifier: Apache-2.0

// Package openai provides an [llm.Provider] implementation that calls the
// OpenAI Chat Completions API (https://platform.openai.com/docs/api-reference/chat).
//
// Use [New] to construct a provider instance. The provider uses only the Go
// standard library for HTTP transport; no third-party SDK is required.
//
// The provider supports Azure OpenAI deployments via [WithBaseURL]. For Azure,
// supply the full deployment endpoint as the base URL.
//
// Example:
//
//	p := openai.New(os.Getenv("OPENAI_API_KEY"),
//	    openai.WithDefaultModel("gpt-4o"),
//	)
//	resp, err := p.Complete(ctx, req)
package openai
