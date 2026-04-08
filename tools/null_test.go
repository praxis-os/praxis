// SPDX-License-Identifier: Apache-2.0

package tools_test

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/tools"
)

func TestNullInvoker_ImplementsInvoker(_ *testing.T) {
	var _ tools.Invoker = tools.NullInvoker{}
}

func TestNullInvoker_Invoke(t *testing.T) {
	tests := []struct {
		name        string
		call        tools.ToolCall
		wantCallID  string
		wantContent string
		wantStatus  tools.ToolStatus
		wantErr     bool
	}{
		{
			name:        "returns not-implemented result with matching call ID",
			call:        tools.ToolCall{CallID: "call-123", Name: "some_tool"},
			wantCallID:  "call-123",
			wantContent: "no tool invoker configured",
			wantStatus:  tools.ToolStatusNotImplemented,
			wantErr:     false,
		},
		{
			name:        "empty call ID is preserved",
			call:        tools.ToolCall{CallID: "", Name: "another_tool"},
			wantCallID:  "",
			wantContent: "no tool invoker configured",
			wantStatus:  tools.ToolStatusNotImplemented,
			wantErr:     false,
		},
	}

	inv := tools.NullInvoker{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := inv.Invoke(context.Background(), tools.InvocationContext{}, tt.call)

			if (err != nil) != tt.wantErr {
				t.Errorf("Invoke() error = %v, wantErr %v", err, tt.wantErr)
			}
			if result.CallID != tt.wantCallID {
				t.Errorf("CallID = %q, want %q", result.CallID, tt.wantCallID)
			}
			if result.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", result.Content, tt.wantContent)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", result.Status, tt.wantStatus)
			}
		})
	}
}

func TestInvokerFunc_Adapter(t *testing.T) {
	called := false
	f := tools.InvokerFunc(func(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
		called = true
		return tools.ToolResult{CallID: call.CallID, Status: tools.ToolStatusSuccess}, nil
	})

	var _ tools.Invoker = f // compile-time check

	result, err := f.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{CallID: "test"})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !called {
		t.Error("InvokerFunc was not called")
	}
	if result.CallID != "test" {
		t.Errorf("CallID: want test, got %q", result.CallID)
	}
}
