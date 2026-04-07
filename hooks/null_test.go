// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
)

func TestInterfaces(t *testing.T) {
	// Compile-time checks documented as runtime assertions.
	var _ hooks.PolicyHook = hooks.AllowAllPolicyHook{}
	var _ hooks.PreLLMFilter = hooks.NoOpPreLLMFilter{}
	var _ hooks.PostToolFilter = hooks.NoOpPostToolFilter{}
}

func TestAllow(t *testing.T) {
	d := hooks.Allow()
	if !d.Allowed {
		t.Error("Allow() returned Allowed=false")
	}
	if d.Reason != "" {
		t.Errorf("Allow() returned non-empty Reason %q", d.Reason)
	}
}

func TestDeny(t *testing.T) {
	const reason = "policy violation"
	d := hooks.Deny(reason)
	if d.Allowed {
		t.Error("Deny() returned Allowed=true")
	}
	if d.Reason != reason {
		t.Errorf("Deny() Reason = %q, want %q", d.Reason, reason)
	}
}

func TestAllowAllPolicyHook_Evaluate(t *testing.T) {
	phases := []hooks.Phase{
		hooks.PhasePreInvocation,
		hooks.PhasePreLLM,
		hooks.PhasePreTool,
		hooks.PhasePostInvocation,
	}

	hook := hooks.AllowAllPolicyHook{}

	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			decision, err := hook.Evaluate(context.Background(), phase, map[string]string{"key": "value"})
			if err != nil {
				t.Errorf("Evaluate() unexpected error: %v", err)
			}
			if !decision.Allowed {
				t.Errorf("Evaluate() Allowed = false, want true")
			}
		})
	}
}

func TestAllowAllPolicyHook_NilMetadata(t *testing.T) {
	hook := hooks.AllowAllPolicyHook{}
	decision, err := hook.Evaluate(context.Background(), hooks.PhasePreLLM, nil)
	if err != nil {
		t.Errorf("Evaluate() unexpected error with nil metadata: %v", err)
	}
	if !decision.Allowed {
		t.Error("Evaluate() Allowed = false, want true")
	}
}

func TestNoOpPreLLMFilter_Filter(t *testing.T) {
	f := hooks.NoOpPreLLMFilter{}
	req := &llm.LLMRequest{Model: "test-model", SystemPrompt: "hello"}

	if err := f.Filter(context.Background(), req); err != nil {
		t.Errorf("Filter() unexpected error: %v", err)
	}
	// Verify the request was not mutated.
	if req.Model != "test-model" {
		t.Errorf("Filter() mutated Model: got %q", req.Model)
	}
	if req.SystemPrompt != "hello" {
		t.Errorf("Filter() mutated SystemPrompt: got %q", req.SystemPrompt)
	}
}

func TestNoOpPostToolFilter_Filter(t *testing.T) {
	f := hooks.NoOpPostToolFilter{}
	result := &llm.LLMToolResult{CallID: "call-1", Content: "output", IsError: false}

	if err := f.Filter(context.Background(), result); err != nil {
		t.Errorf("Filter() unexpected error: %v", err)
	}
	// Verify the result was not mutated.
	if result.CallID != "call-1" {
		t.Errorf("Filter() mutated CallID: got %q", result.CallID)
	}
	if result.Content != "output" {
		t.Errorf("Filter() mutated Content: got %q", result.Content)
	}
}
