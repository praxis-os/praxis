// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
)

// phaseVerdictHook returns a specific Decision for the given phase and Allow
// for all other phases.
type phaseVerdictHook struct {
	phase   hooks.Phase
	verdict hooks.Decision
}

func (h phaseVerdictHook) Evaluate(_ context.Context, phase hooks.Phase, _ hooks.PolicyInput) (hooks.Decision, error) {
	if phase == h.phase {
		return h.verdict, nil
	}
	return hooks.Allow(), nil
}

func TestAuditNote_PreHook_VerdictLog_PopulatesAuditNote(t *testing.T) {
	p := mock.NewSimple("hello")
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPolicyHook(phaseVerdictHook{
			phase:   hooks.PhasePreInvocation,
			verdict: hooks.Log("suspicious request pattern"),
		}),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}

	var completed *event.InvocationEvent
	for i := range result.Events {
		if result.Events[i].Type == event.EventTypePreHookCompleted {
			completed = &result.Events[i]
			break
		}
	}
	if completed == nil {
		t.Fatal("no prehook.completed event found")
	}
	if completed.AuditNote != "suspicious request pattern" {
		t.Errorf("AuditNote: want %q, got %q", "suspicious request pattern", completed.AuditNote)
	}
}

func TestAuditNote_PostHook_VerdictLog_PopulatesAuditNote(t *testing.T) {
	p := mock.NewSimple("hello")
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPolicyHook(phaseVerdictHook{
			phase:   hooks.PhasePostInvocation,
			verdict: hooks.Log("response flagged for review"),
		}),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}

	var completed *event.InvocationEvent
	for i := range result.Events {
		if result.Events[i].Type == event.EventTypePostHookCompleted {
			completed = &result.Events[i]
			break
		}
	}
	if completed == nil {
		t.Fatal("no posthook.completed event found")
	}
	if completed.AuditNote != "response flagged for review" {
		t.Errorf("AuditNote: want %q, got %q", "response flagged for review", completed.AuditNote)
	}
}

func TestAuditNote_VerdictAllow_AuditNoteEmpty(t *testing.T) {
	p := mock.NewSimple("hello")
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPolicyHook(phaseVerdictHook{
			phase:   hooks.PhasePreInvocation,
			verdict: hooks.Allow(),
		}),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	for _, e := range result.Events {
		if e.Type == event.EventTypePreHookCompleted || e.Type == event.EventTypePostHookCompleted {
			if e.AuditNote != "" {
				t.Errorf("event %q: AuditNote should be empty for VerdictAllow, got %q", e.Type, e.AuditNote)
			}
		}
	}
}

func TestAuditNote_VerdictLog_EmptyReason_AuditNoteEmpty(t *testing.T) {
	p := mock.NewSimple("hello")
	// hooks.Log("") — Reason is empty string.
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPolicyHook(phaseVerdictHook{
			phase:   hooks.PhasePreInvocation,
			verdict: hooks.Log(""),
		}),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	for _, e := range result.Events {
		if e.Type == event.EventTypePreHookCompleted {
			if e.AuditNote != "" {
				t.Errorf("AuditNote: want empty for empty Reason, got %q", e.AuditNote)
			}
		}
	}
}
