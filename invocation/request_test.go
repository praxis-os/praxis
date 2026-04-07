// SPDX-License-Identifier: Apache-2.0

package invocation_test

import (
	"testing"

	"github.com/praxis-os/praxis/invocation"
	"github.com/praxis-os/praxis/llm"
)

func TestInvocationRequest_ZeroValue(t *testing.T) {
	// The zero value must be constructable without panicking.
	var req invocation.InvocationRequest

	if req.Messages != nil {
		t.Errorf("zero-value Messages: want nil, got %v", req.Messages)
	}
	if req.Model != "" {
		t.Errorf("zero-value Model: want empty string, got %q", req.Model)
	}
	if req.Tools != nil {
		t.Errorf("zero-value Tools: want nil, got %v", req.Tools)
	}
	if req.MaxIterations != 0 {
		t.Errorf("zero-value MaxIterations: want 0, got %d", req.MaxIterations)
	}
	if req.Metadata != nil {
		t.Errorf("zero-value Metadata: want nil, got %v", req.Metadata)
	}
}

func TestInvocationRequest_FieldAssignment(t *testing.T) {
	msg := llm.Message{
		Role:  llm.RoleUser,
		Parts: []llm.MessagePart{llm.TextPart("hello")},
	}
	tool := llm.ToolDefinition{
		Name:        "search",
		Description: "Search the web",
		InputSchema: []byte(`{"type":"object"}`),
	}

	req := invocation.InvocationRequest{
		Messages:      []llm.Message{msg},
		Model:         "claude-3-5-sonnet-20241022",
		Tools:         []llm.ToolDefinition{tool},
		MaxIterations: 10,
		Metadata:      map[string]string{"request_id": "abc123"},
	}

	if len(req.Messages) != 1 {
		t.Errorf("Messages length: want 1, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != llm.RoleUser {
		t.Errorf("Messages[0].Role: want %q, got %q", llm.RoleUser, req.Messages[0].Role)
	}
	if req.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Model: want %q, got %q", "claude-3-5-sonnet-20241022", req.Model)
	}
	if len(req.Tools) != 1 {
		t.Errorf("Tools length: want 1, got %d", len(req.Tools))
	}
	if req.Tools[0].Name != "search" {
		t.Errorf("Tools[0].Name: want %q, got %q", "search", req.Tools[0].Name)
	}
	if req.MaxIterations != 10 {
		t.Errorf("MaxIterations: want 10, got %d", req.MaxIterations)
	}
	if req.Metadata["request_id"] != "abc123" {
		t.Errorf("Metadata[request_id]: want %q, got %q", "abc123", req.Metadata["request_id"])
	}
}

func TestInvocationRequest_NilMetadataReadSafe(t *testing.T) {
	req := invocation.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser}},
		Model:    "some-model",
	}

	// Reading from a nil map must not panic.
	val := req.Metadata["key"]
	if val != "" {
		t.Errorf("nil Metadata read: want empty string, got %q", val)
	}
}

func TestInvocationRequest_EmptyToolsSlice(t *testing.T) {
	req := invocation.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser}},
		Model:    "some-model",
		Tools:    []llm.ToolDefinition{},
	}

	if len(req.Tools) != 0 {
		t.Errorf("empty Tools: want length 0, got %d", len(req.Tools))
	}
}
