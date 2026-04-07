// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/praxis-os/praxis/llm"
)

// ErrScriptExhausted is returned by Complete and Stream when all scripted
// responses have been consumed.
var ErrScriptExhausted = errors.New("mock: script exhausted — no more responses")

// Provider is a configurable mock implementation of [llm.Provider] for testing.
//
// Responses are returned in FIFO order from the script supplied to [New].
// When the script is exhausted, subsequent calls return [ErrScriptExhausted].
// All completed calls are recorded in the call history accessible via [Provider.Calls].
//
// Provider is safe for concurrent use.
type Provider struct {
	mu          sync.Mutex
	script      []Response
	idx         int
	calls       []llm.LLMRequest
	parallelTC  bool
}

// New creates a mock provider with the given scripted responses.
// Responses are returned in FIFO order. When all responses have been consumed,
// subsequent calls return [ErrScriptExhausted].
func New(responses ...Response) *Provider {
	return &Provider{
		script:     responses,
		parallelTC: true,
	}
}

// Complete satisfies [llm.Provider]. It returns the next scripted response.
// If the response has a Delay, it waits for that duration, aborting early if
// ctx is cancelled. Returns [ErrScriptExhausted] when the script is empty.
func (p *Provider) Complete(ctx context.Context, req llm.LLMRequest) (llm.LLMResponse, error) {
	if err := ctx.Err(); err != nil {
		return llm.LLMResponse{}, err
	}

	p.mu.Lock()
	if p.idx >= len(p.script) {
		p.mu.Unlock()
		return llm.LLMResponse{}, fmt.Errorf("%w: call #%d received no matching entry", ErrScriptExhausted, p.idx+1)
	}
	entry := p.script[p.idx]
	p.idx++
	p.calls = append(p.calls, req)
	p.mu.Unlock()

	if entry.Delay > 0 {
		select {
		case <-time.After(entry.Delay):
		case <-ctx.Done():
			return llm.LLMResponse{}, ctx.Err()
		}
	}

	if entry.Err != nil {
		return llm.LLMResponse{}, entry.Err
	}

	return entry.LLMResponse, nil
}

// Stream satisfies [llm.Provider]. It converts the next scripted response into a
// single-chunk stream. The returned channel is buffered and closed after the
// final chunk is sent.
//
// If the response has a Delay the delay is applied before writing the chunk.
// Context cancellation is respected during the delay.
func (p *Provider) Stream(ctx context.Context, req llm.LLMRequest) (<-chan llm.LLMStreamChunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ch := make(chan llm.LLMStreamChunk, 1)

	go func() {
		defer close(ch)

		resp, err := p.Complete(ctx, req)
		if err != nil {
			ch <- llm.LLMStreamChunk{Err: err}
			return
		}

		// Extract text delta from the first text part, if any.
		var delta string
		for _, part := range resp.Message.Parts {
			if part.Type == llm.PartTypeText {
				delta = part.Text
				break
			}
		}

		ch <- llm.LLMStreamChunk{
			Delta:    delta,
			Final:    true,
			Response: &resp,
		}
	}()

	return ch, nil
}

// Name returns "mock".
func (p *Provider) Name() string { return "mock" }

// SupportsParallelToolCalls reports whether parallel tool calls are enabled.
// Defaults to true; override with [Provider.SetParallelToolCalls].
func (p *Provider) SupportsParallelToolCalls() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.parallelTC
}

// SetParallelToolCalls sets whether the mock reports parallel tool-call support.
// May be called at any time and is safe for concurrent use.
func (p *Provider) SetParallelToolCalls(v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.parallelTC = v
}

// Capabilities returns sensible defaults representing a capable provider:
// streaming, parallel tool calls, system prompt, all stop reasons, 200k context.
func (p *Provider) Capabilities() llm.Capabilities {
	return llm.Capabilities{
		SupportsStreaming:         true,
		SupportsParallelToolCalls: true,
		SupportsSystemPrompt:      true,
		SupportedStopReasons: []llm.StopReason{
			llm.StopReasonEndTurn,
			llm.StopReasonToolUse,
			llm.StopReasonMaxTokens,
			llm.StopReasonStopSequence,
		},
		MaxContextTokens: 200_000,
	}
}

// Calls returns a snapshot of all [llm.LLMRequest] values received so far,
// in the order they were received.
func (p *Provider) Calls() []llm.LLMRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]llm.LLMRequest, len(p.calls))
	copy(out, p.calls)
	return out
}

// CallCount returns the total number of Complete or Stream calls made.
func (p *Provider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}
