// SPDX-License-Identifier: Apache-2.0

//go:build smoke

package anthropic_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
)

// TestSmoke_Complete_RealAPI performs a live call to the Anthropic Messages API.
//
// The test is excluded from normal "go test ./..." runs by the "smoke" build tag.
// Run it explicitly with:
//
//	go test -tags smoke ./llm/anthropic/ -run TestSmoke_Complete_RealAPI
//
// The ANTHROPIC_API_KEY environment variable must be set, otherwise the test is
// skipped.
func TestSmoke_Complete_RealAPI(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	p := anthropic.New(apiKey, anthropic.WithMaxTokens(64))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello in exactly one word.")},
			},
		},
	}

	resp, err := p.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete() error = %v; want nil", err)
	}

	// Provider name must be stable.
	if got := p.Name(); got != "anthropic" {
		t.Errorf("Name() = %q; want %q", got, "anthropic")
	}

	// The response must have ended normally.
	if resp.StopReason != llm.StopReasonEndTurn {
		t.Errorf("StopReason = %q; want %q", resp.StopReason, llm.StopReasonEndTurn)
	}

	// The response message must carry at least one text part.
	if len(resp.Message.Parts) == 0 {
		t.Fatal("response has no message parts")
	}
	if resp.Message.Parts[0].Type != llm.PartTypeText {
		t.Errorf("Parts[0].Type = %q; want %q", resp.Message.Parts[0].Type, llm.PartTypeText)
	}
	text := resp.Message.Parts[0].Text
	if strings.TrimSpace(text) == "" {
		t.Error("response text is empty")
	}
	t.Logf("response text: %q", text)

	// Token usage must be reported.
	if resp.Usage.InputTokens <= 0 {
		t.Errorf("InputTokens = %d; want > 0", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens <= 0 {
		t.Errorf("OutputTokens = %d; want > 0", resp.Usage.OutputTokens)
	}
	t.Logf("token usage: input=%d output=%d", resp.Usage.InputTokens, resp.Usage.OutputTokens)
}

// TestSmoke_Stream_RealAPI performs a live streaming call to the Anthropic Messages API.
//
// Because the Anthropic adapter currently delegates Stream to Complete, this test
// validates that the single-chunk delivery path works end-to-end against the real
// API. The test is excluded from normal runs by the "smoke" build tag.
func TestSmoke_Stream_RealAPI(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	p := anthropic.New(apiKey, anthropic.WithMaxTokens(64))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := llm.LLMRequest{
		Messages: []llm.Message{
			{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello in exactly one word.")},
			},
		},
	}

	ch, err := p.Stream(ctx, req)
	if err != nil {
		t.Fatalf("Stream() setup error = %v; want nil", err)
	}

	var finalChunk *llm.LLMStreamChunk
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("stream chunk error = %v", chunk.Err)
		}
		if chunk.Final {
			c := chunk
			finalChunk = &c
		}
	}

	if finalChunk == nil {
		t.Fatal("no final chunk received")
	}
	if finalChunk.Response == nil {
		t.Fatal("final chunk has nil Response")
	}

	resp := finalChunk.Response
	if resp.StopReason != llm.StopReasonEndTurn {
		t.Errorf("StopReason = %q; want %q", resp.StopReason, llm.StopReasonEndTurn)
	}
	if len(resp.Message.Parts) == 0 {
		t.Fatal("final response has no message parts")
	}
	text := resp.Message.Parts[0].Text
	if strings.TrimSpace(text) == "" {
		t.Error("stream response text is empty")
	}
	t.Logf("stream response text: %q", text)
}
