// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

// fakeInvoker is a zero-cost double that satisfies both [tools.Invoker]
// and [io.Closer] — i.e., the full [Invoker] contract. It exists only
// to prove the composition at compile time; it is not exported and is
// not used anywhere outside this test file.
type fakeInvoker struct {
	closed bool
}

func (f *fakeInvoker) Invoke(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	return tools.ToolResult{
		Status:  tools.ToolStatusNotImplemented,
		Content: "",
		Err:     errors.New("fakeInvoker: not implemented"),
		CallID:  call.CallID,
	}, nil
}

func (f *fakeInvoker) Close() error {
	f.closed = true
	return nil
}

// Definitions satisfies the S32 Invoker.Definitions() contract. The
// fake advertises no tools — the compile-time assertion below only
// cares that the method exists with the correct signature; the
// behavioural test for Definitions lives in the router/new test
// suite that exercises the real adapter.
func (f *fakeInvoker) Definitions() []llm.ToolDefinition {
	return nil
}

// Compile-time assertion that a concrete type satisfying both
// [tools.Invoker] and [io.Closer] can be assigned to the public
// [Invoker] interface. If the interface composition ever drops
// either embedded constraint, this line fails to compile.
var _ Invoker = (*fakeInvoker)(nil)

// TestInvokerInterfaceShape asserts the two guarantees the public
// [Invoker] interface provides:
//
//  1. Any value implementing it satisfies [tools.Invoker] (so it
//     plugs directly into the praxis orchestrator's tool-dispatch
//     surface without an adapter).
//  2. Any value implementing it satisfies [io.Closer] (so the
//     caller can release MCP sessions via a single idiomatic call).
//
// The test does not exercise the actual Invoke or Close semantics
// of the returned [Invoker] — that is the responsibility of commit 5
// and onward, once the concrete adapter exists.
func TestInvokerInterfaceShape(t *testing.T) {
	t.Parallel()

	var inv Invoker = &fakeInvoker{}

	// 1. tools.Invoker projection.
	var toolInv tools.Invoker = inv
	result, err := toolInv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "test-call-1",
		Name:   "fake__probe",
	})
	if err != nil {
		t.Fatalf("tools.Invoker.Invoke: unexpected framework error: %v", err)
	}
	if result.CallID != "test-call-1" {
		t.Errorf("tools.Invoker.Invoke: echoed CallID = %q, want %q", result.CallID, "test-call-1")
	}
	if result.Status != tools.ToolStatusNotImplemented {
		t.Errorf("tools.Invoker.Invoke: status = %q, want %q", result.Status, tools.ToolStatusNotImplemented)
	}

	// 2. io.Closer projection.
	if err := inv.Close(); err != nil {
		t.Errorf("Invoker.Close: unexpected error: %v", err)
	}
	// Idempotency: the adapter guarantees Close is safe to call more
	// than once. A fake that flipped state the first time should still
	// return nil.
	if err := inv.Close(); err != nil {
		t.Errorf("Invoker.Close (second call): unexpected error: %v", err)
	}
}
