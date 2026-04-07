// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"context"

	"github.com/praxis-os/praxis/llm"
)

// Phase represents the lifecycle phase at which a [PolicyHook] is evaluated.
type Phase string

const (
	// PhasePreInvocation is evaluated before the first LLM call of an invocation.
	PhasePreInvocation Phase = "pre_invocation"

	// PhasePreLLM is evaluated immediately before each LLM call.
	PhasePreLLM Phase = "pre_llm"

	// PhasePreTool is evaluated before each tool call is dispatched.
	PhasePreTool Phase = "pre_tool"

	// PhasePostInvocation is evaluated after the invocation completes
	// (regardless of outcome).
	PhasePostInvocation Phase = "post_invocation"
)

// Decision is the verdict returned by a [PolicyHook].
type Decision struct {
	// Allowed reports whether the operation may proceed.
	Allowed bool

	// Reason is a human-readable explanation. Non-empty when Allowed is false.
	Reason string
}

// Allow returns a Decision that permits the operation to proceed.
func Allow() Decision { return Decision{Allowed: true} }

// Deny returns a Decision that halts the operation with the given reason.
func Deny(reason string) Decision { return Decision{Allowed: false, Reason: reason} }

// PolicyHook evaluates invocation state at defined lifecycle phases and
// returns a [Decision] indicating whether the operation may proceed.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type PolicyHook interface {
	// Evaluate is called at the given phase with a copy of the current
	// invocation metadata. Returning a Deny decision halts the invocation
	// with a policy error. Returning a non-nil error is treated as a
	// transient failure and subject to the orchestrator's retry policy.
	Evaluate(ctx context.Context, phase Phase, metadata map[string]string) (Decision, error)
}

// PreLLMFilter can inspect or mutate an [llm.LLMRequest] before it is sent
// to the provider.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type PreLLMFilter interface {
	// Filter receives a pointer to the outgoing request. Modifications are
	// applied in-place. A non-nil error halts the invocation.
	Filter(ctx context.Context, req *llm.LLMRequest) error
}

// PostToolFilter can inspect or mutate an [llm.LLMToolResult] before it is
// included in the next LLM turn.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type PostToolFilter interface {
	// Filter receives a pointer to the tool result. Modifications are
	// applied in-place. A non-nil error halts the invocation.
	Filter(ctx context.Context, result *llm.LLMToolResult) error
}

// Compile-time interface checks.
var _ PolicyHook = AllowAllPolicyHook{}
var _ PreLLMFilter = NoOpPreLLMFilter{}
var _ PostToolFilter = NoOpPostToolFilter{}

// AllowAllPolicyHook is a [PolicyHook] that unconditionally permits every
// operation. Used as the default when no policy is configured.
type AllowAllPolicyHook struct{}

// Evaluate always returns [Allow] with a nil error.
func (AllowAllPolicyHook) Evaluate(_ context.Context, _ Phase, _ map[string]string) (Decision, error) {
	return Allow(), nil
}

// NoOpPreLLMFilter is a [PreLLMFilter] that passes every request through
// without modification. Used as the default when no pre-LLM filter is configured.
type NoOpPreLLMFilter struct{}

// Filter leaves the request unchanged and returns nil.
func (NoOpPreLLMFilter) Filter(_ context.Context, _ *llm.LLMRequest) error { return nil }

// NoOpPostToolFilter is a [PostToolFilter] that passes every tool result
// through without modification. Used as the default when no post-tool filter
// is configured.
type NoOpPostToolFilter struct{}

// Filter leaves the tool result unchanged and returns nil.
func (NoOpPostToolFilter) Filter(_ context.Context, _ *llm.LLMToolResult) error { return nil }
