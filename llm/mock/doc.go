// SPDX-License-Identifier: Apache-2.0

// Package mock provides a configurable mock implementation of [llm.Provider]
// for use in unit and integration tests.
//
// The mock supports scripted responses delivered in FIFO order, optional per-response
// delays to simulate latency, and full call history tracking for assertions.
//
// Basic usage:
//
//	p := mock.NewSimple("Hello, world!")
//	resp, err := p.Complete(ctx, req)
//
// Scripted usage:
//
//	p := mock.New(
//	    mock.Response{LLMResponse: llm.LLMResponse{...}},
//	    mock.Response{Err: errors.New("rate limited")},
//	)
package mock
