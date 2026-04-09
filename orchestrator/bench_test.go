// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
)

// BenchmarkOrchestratorOverhead measures the orchestrator overhead per
// invocation with LLM latency excluded. The mock provider returns instantly,
// so the measured time reflects only the framework's state-machine driver,
// event emission, hook/filter chain traversal, and identity signing.
//
// Target: under 15ms per invocation (T23.1 / PRAX-134).
func BenchmarkOrchestratorOverhead(b *testing.B) {
	// Build a provider that returns instantly on every call.
	responses := make([]mock.Response, b.N)
	for i := range responses {
		responses[i] = mock.Response{
			LLMResponse: llm.LLMResponse{
				Message: llm.Message{
					Role:  llm.RoleAssistant,
					Parts: []llm.MessagePart{llm.TextPart("ok")},
				},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.TokenUsage{InputTokens: 5, OutputTokens: 1},
			},
		}
	}

	o, err := orchestrator.New(
		mock.New(responses...),
		orchestrator.WithDefaultModel("bench-model"),
	)
	if err != nil {
		b.Fatal(err)
	}

	req := praxis.InvocationRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = o.Invoke(ctx, req)
	}
}

// TestOrchestratorOverheadUnder15ms is a non-benchmark assertion that the
// orchestrator overhead stays under the 15ms threshold. It runs a batch of
// invocations and checks that the mean overhead per invocation is acceptable.
func TestOrchestratorOverheadUnder15ms(t *testing.T) {
	const iterations = 500

	responses := make([]mock.Response, iterations)
	for i := range responses {
		responses[i] = mock.Response{
			LLMResponse: llm.LLMResponse{
				Message: llm.Message{
					Role:  llm.RoleAssistant,
					Parts: []llm.MessagePart{llm.TextPart("ok")},
				},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.TokenUsage{InputTokens: 5, OutputTokens: 1},
			},
		}
	}

	o, err := orchestrator.New(
		mock.New(responses...),
		orchestrator.WithDefaultModel("bench-model"),
	)
	if err != nil {
		t.Fatal(err)
	}

	req := praxis.InvocationRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
		},
	}

	ctx := context.Background()
	start := time.Now()
	for i := 0; i < iterations; i++ {
		result, invokeErr := o.Invoke(ctx, req)
		if invokeErr != nil {
			t.Fatalf("Invoke #%d error: %v", i, invokeErr)
		}
		if result == nil {
			t.Fatalf("Invoke #%d returned nil result", i)
		}
	}
	elapsed := time.Since(start)
	mean := elapsed / iterations

	t.Logf("orchestrator overhead: total=%v, mean=%v (n=%d)", elapsed, mean, iterations)
	if mean > 15*time.Millisecond {
		t.Errorf("mean overhead %v exceeds 15ms threshold", mean)
	}
}
