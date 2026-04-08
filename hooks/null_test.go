// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

func TestInterfaces(_ *testing.T) {
	var _ hooks.PolicyHook = hooks.AllowAllPolicyHook{}
	var _ hooks.PreLLMFilter = hooks.PassThroughPreLLMFilter{}
	var _ hooks.PostToolFilter = hooks.PassThroughPostToolFilter{}
}

func TestAllow(t *testing.T) {
	d := hooks.Allow()
	if d.Verdict != hooks.VerdictAllow {
		t.Errorf("Allow() returned Verdict=%q, want %q", d.Verdict, hooks.VerdictAllow)
	}
	if d.Reason != "" {
		t.Errorf("Allow() returned non-empty Reason %q", d.Reason)
	}
}

func TestDeny(t *testing.T) {
	const reason = "policy violation"
	d := hooks.Deny(reason)
	if d.Verdict != hooks.VerdictDeny {
		t.Errorf("Deny() returned Verdict=%q, want %q", d.Verdict, hooks.VerdictDeny)
	}
	if d.Reason != reason {
		t.Errorf("Deny() Reason = %q, want %q", d.Reason, reason)
	}
}

func TestRequireApproval(t *testing.T) {
	const reason = "needs review"
	meta := map[string]any{"reviewer": "admin"}
	d := hooks.RequireApproval(reason, meta)
	if d.Verdict != hooks.VerdictRequireApproval {
		t.Errorf("RequireApproval() Verdict = %q, want %q", d.Verdict, hooks.VerdictRequireApproval)
	}
	if d.Reason != reason {
		t.Errorf("RequireApproval() Reason = %q, want %q", d.Reason, reason)
	}
	if d.Metadata["reviewer"] != "admin" {
		t.Errorf("RequireApproval() Metadata[reviewer] = %v, want admin", d.Metadata["reviewer"])
	}
}

func TestLog(t *testing.T) {
	const reason = "audit trail"
	d := hooks.Log(reason)
	if d.Verdict != hooks.VerdictLog {
		t.Errorf("Log() Verdict = %q, want %q", d.Verdict, hooks.VerdictLog)
	}
	if d.Reason != reason {
		t.Errorf("Log() Reason = %q, want %q", d.Reason, reason)
	}
}

func TestAllowAllPolicyHook_Evaluate(t *testing.T) {
	phases := []hooks.Phase{
		hooks.PhasePreInvocation,
		hooks.PhasePreLLMInput,
		hooks.PhasePostToolOutput,
		hooks.PhasePostInvocation,
	}

	hook := hooks.AllowAllPolicyHook{}

	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			decision, err := hook.Evaluate(context.Background(), phase, hooks.PolicyInput{
				Metadata: map[string]string{"key": "value"},
			})
			if err != nil {
				t.Errorf("Evaluate() unexpected error: %v", err)
			}
			if decision.Verdict != hooks.VerdictAllow {
				t.Errorf("Evaluate() Verdict = %q, want %q", decision.Verdict, hooks.VerdictAllow)
			}
		})
	}
}

func TestPassThroughPreLLMFilter_Filter(t *testing.T) {
	f := hooks.PassThroughPreLLMFilter{}
	messages := []llm.Message{
		{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hello")}},
	}

	filtered, decisions, err := f.Filter(context.Background(), messages)
	if err != nil {
		t.Errorf("Filter() unexpected error: %v", err)
	}
	if len(filtered) != len(messages) {
		t.Errorf("Filter() returned %d messages, want %d", len(filtered), len(messages))
	}
	if len(decisions) != 0 {
		t.Errorf("Filter() returned %d decisions, want 0", len(decisions))
	}
}

func TestPassThroughPostToolFilter_Filter(t *testing.T) {
	f := hooks.PassThroughPostToolFilter{}
	result := tools.ToolResult{
		CallID:  "call-1",
		Content: "output",
		Status:  tools.ToolStatusSuccess,
	}

	filtered, decisions, err := f.Filter(context.Background(), result)
	if err != nil {
		t.Errorf("Filter() unexpected error: %v", err)
	}
	if filtered.CallID != result.CallID {
		t.Errorf("Filter() mutated CallID: got %q", filtered.CallID)
	}
	if filtered.Content != result.Content {
		t.Errorf("Filter() mutated Content: got %q", filtered.Content)
	}
	if len(decisions) != 0 {
		t.Errorf("Filter() returned %d decisions, want 0", len(decisions))
	}
}
