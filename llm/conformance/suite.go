// SPDX-License-Identifier: Apache-2.0

// Package conformance provides a shared test suite that every [llm.Provider]
// implementation must pass. The suite validates the provider contract defined
// in the llm package: correct response mapping, token usage reporting,
// stop-reason semantics, tool-call handling, streaming, and error
// classification.
//
// Use [RunSuite] from provider-specific test files to exercise the full suite
// against an httptest-backed or live provider instance.
//
// Stability: internal testing contract, not part of the public API.
package conformance

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/praxis-os/praxis/llm"
)

// RunSuite runs the full conformance suite against the given provider.
// The provider should be configured to respond to the requests made by each
// sub-test (e.g., backed by an httptest server or live API).
//
// suiteReq is a factory that returns a fresh simple-text request for each test.
// toolReq is a factory that returns a request that triggers a tool call.
// If toolReq is nil, tool-related tests are skipped.
func RunSuite(t *testing.T, p llm.Provider, suiteReq func() llm.LLMRequest, toolReq func() llm.LLMRequest) {
	t.Helper()

	t.Run("Name", func(t *testing.T) {
		name := p.Name()
		if name == "" {
			t.Error("Name() returned empty string")
		}
	})

	t.Run("Capabilities", func(t *testing.T) {
		caps := p.Capabilities()
		if len(caps.SupportedStopReasons) == 0 {
			t.Error("SupportedStopReasons is empty")
		}
	})

	t.Run("Complete_SimpleText", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := p.Complete(ctx, suiteReq())
		if err != nil {
			t.Fatalf("Complete: %v", err)
		}

		// Response must have assistant role.
		if resp.Message.Role != llm.RoleAssistant {
			t.Errorf("Message.Role = %q, want %q", resp.Message.Role, llm.RoleAssistant)
		}

		// Must have at least one text part.
		hasText := false
		for _, part := range resp.Message.Parts {
			if part.Type == llm.PartTypeText && strings.TrimSpace(part.Text) != "" {
				hasText = true
				break
			}
		}
		if !hasText {
			t.Error("response has no non-empty text part")
		}

		// Stop reason must be EndTurn for a simple text response.
		if resp.StopReason != llm.StopReasonEndTurn {
			t.Errorf("StopReason = %q, want %q", resp.StopReason, llm.StopReasonEndTurn)
		}

		// Token usage must be reported.
		if resp.Usage.InputTokens <= 0 {
			t.Errorf("InputTokens = %d, want > 0", resp.Usage.InputTokens)
		}
		if resp.Usage.OutputTokens <= 0 {
			t.Errorf("OutputTokens = %d, want > 0", resp.Usage.OutputTokens)
		}
	})

	t.Run("Complete_ToolUse", func(t *testing.T) {
		if toolReq == nil {
			t.Skip("toolReq factory not provided")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := p.Complete(ctx, toolReq())
		if err != nil {
			t.Fatalf("Complete: %v", err)
		}

		if resp.StopReason != llm.StopReasonToolUse {
			t.Errorf("StopReason = %q, want %q", resp.StopReason, llm.StopReasonToolUse)
		}

		hasToolCall := false
		for _, part := range resp.Message.Parts {
			if part.Type == llm.PartTypeToolCall && part.ToolCall != nil {
				hasToolCall = true
				if part.ToolCall.CallID == "" {
					t.Error("ToolCall.CallID is empty")
				}
				if part.ToolCall.Name == "" {
					t.Error("ToolCall.Name is empty")
				}
			}
		}
		if !hasToolCall {
			t.Error("expected at least one tool call part")
		}
	})

	t.Run("Stream_SingleChunk", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		ch, err := p.Stream(ctx, suiteReq())
		if err != nil {
			t.Fatalf("Stream: %v", err)
		}

		var chunks []llm.LLMStreamChunk
		for c := range ch {
			if c.Err != nil {
				t.Fatalf("stream chunk error: %v", c.Err)
			}
			chunks = append(chunks, c)
		}

		if len(chunks) == 0 {
			t.Fatal("no chunks received")
		}

		// Last chunk must be final.
		last := chunks[len(chunks)-1]
		if !last.Final {
			t.Error("last chunk should have Final=true")
		}
		if last.Response == nil {
			t.Fatal("final chunk should have non-nil Response")
		}
		if last.Response.Usage.InputTokens <= 0 {
			t.Errorf("final chunk InputTokens = %d, want > 0", last.Response.Usage.InputTokens)
		}
	})

	t.Run("Complete_CancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // immediately cancel

		_, err := p.Complete(ctx, suiteReq())
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})
}
