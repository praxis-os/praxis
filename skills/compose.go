// SPDX-License-Identifier: Apache-2.0

package skills

import (
	"context"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/orchestrator"
)

// WithSkill returns an [orchestrator.Option] that injects the skill's
// instruction text into the orchestrator's system prompt.
//
// Instruction fragments are appended in the order WithSkill is called,
// separated by "--- Skills ---" markers.
//
// Panics at orchestrator construction time if two skills share the same
// [Skill.Name] (D127).
func WithSkill(s *Skill) orchestrator.Option {
	return orchestrator.WithSystemPromptFragment(s.Name(), s.Instructions())
}

// ComposedInstructions returns the system prompt that would result from
// composing the given base prompt with the provided skills' instruction
// fragments. This is a debug/test helper.
//
// Note: the Phase 8 design (D132) specified ComposedInstructions(opts ...Option).
// This signature is a deliberate improvement — it is more type-safe and does
// not require callers to pass non-skill orchestrator options.
func ComposedInstructions(base string, skills ...*Skill) string {
	if len(skills) == 0 {
		return base
	}
	// Build a temporary orchestrator to extract the composed prompt.
	// Use a nil-safe stub; we never call Invoke.
	opts := make([]orchestrator.Option, 0, len(skills))
	for _, sk := range skills {
		opts = append(opts, WithSkill(sk))
	}
	// Invariant: orchestrator.New cannot fail here because nopProvider is
	// non-nil and WithSystemPromptFragment always returns nil error.
	// A panic is appropriate if this invariant is violated.
	o, err := orchestrator.New(nopProvider{}, opts...)
	if err != nil {
		panic("skills: ComposedInstructions: " + err.Error())
	}
	return o.ComposedSystemPrompt(base)
}

// nopProvider is a minimal llm.Provider stub used only by ComposedInstructions.
// It is never called; it exists solely to satisfy orchestrator.New's non-nil
// provider requirement.
type nopProvider struct{}

func (nopProvider) Complete(context.Context, llm.LLMRequest) (llm.LLMResponse, error) {
	panic("nopProvider: Complete must not be called")
}
func (nopProvider) Stream(context.Context, llm.LLMRequest) (<-chan llm.LLMStreamChunk, error) {
	panic("nopProvider: Stream must not be called")
}
func (nopProvider) Name() string                      { return "nop" }
func (nopProvider) SupportsParallelToolCalls() bool   { return false }
func (nopProvider) Capabilities() llm.Capabilities    { return llm.Capabilities{} }
