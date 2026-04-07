// SPDX-License-Identifier: Apache-2.0

package tools_test

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

func TestNullInvoker_ImplementsInvoker(_ *testing.T) {
	// Compile-time check duplicated here as a documentation test.
	var _ tools.Invoker = tools.NullInvoker{}
}

func TestNullInvoker_Invoke(t *testing.T) {
	tests := []struct {
		name        string
		call        llm.LLMToolCall
		wantCallID  string
		wantContent string
		wantIsError bool
		wantErr     bool
	}{
		{
			name:        "returns error result with matching call ID",
			call:        llm.LLMToolCall{CallID: "call-123", Name: "some_tool"},
			wantCallID:  "call-123",
			wantContent: "no tool invoker configured",
			wantIsError: true,
			wantErr:     false,
		},
		{
			name:        "empty call ID is preserved",
			call:        llm.LLMToolCall{CallID: "", Name: "another_tool"},
			wantCallID:  "",
			wantContent: "no tool invoker configured",
			wantIsError: true,
			wantErr:     false,
		},
	}

	inv := tools.NullInvoker{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := inv.Invoke(context.Background(), tt.call)

			if (err != nil) != tt.wantErr {
				t.Errorf("Invoke() error = %v, wantErr %v", err, tt.wantErr)
			}
			if result.CallID != tt.wantCallID {
				t.Errorf("CallID = %q, want %q", result.CallID, tt.wantCallID)
			}
			if result.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", result.Content, tt.wantContent)
			}
			if result.IsError != tt.wantIsError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantIsError)
			}
		})
	}
}
