// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis/invocation"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/state"
)

const (
	defaultMaxIterations = 10
)

// Orchestrator runs agent invocations through the praxis state machine.
//
// An Orchestrator is safe for concurrent use: multiple goroutines may call
// [Orchestrator.Invoke] simultaneously on the same instance. Each invocation
// owns its own state machine and context.
//
// Create an Orchestrator with [New].
type Orchestrator struct {
	provider      llm.Provider
	defaultModel  string
	maxIterations int

	// Future extension points (placeholders for upcoming tasks):
	// toolInvoker        tools.Invoker
	// policyHooks        []hooks.PolicyHook
	// preLLMFilters      []hooks.PreLLMFilter
	// postToolFilters    []hooks.PostToolFilter
	// budgetGuard        budget.Guard
	// telemetryEmitter   telemetry.LifecycleEventEmitter
	// metricsRecorder    telemetry.MetricsRecorder
	// credentialResolver credentials.Resolver
	// identitySigner     identity.Signer
}

// New creates an Orchestrator backed by the given provider.
//
// provider must not be nil; New returns a non-nil error if it is.
// Options are applied in order after defaults are set.
//
// Default values:
//   - maxIterations: 10
//   - defaultModel: "" (provider's own default)
func New(provider llm.Provider, opts ...Option) (*Orchestrator, error) {
	if provider == nil {
		return nil, fmt.Errorf("orchestrator: provider must not be nil")
	}

	o := &Orchestrator{
		provider:      provider,
		maxIterations: defaultMaxIterations,
	}

	for _, opt := range opts {
		opt(o)
	}

	return o, nil
}

// Invoke runs a single agent invocation and returns its result.
//
// The invocation progresses through the praxis state machine, calling the
// provider's LLM API one or more times and dispatching any tool calls
// returned by the model.
//
// Invoke respects ctx cancellation at each blocking point. A cancelled
// context causes the invocation to terminate with a [state.Cancelled]
// terminal state and a [errors.CancellationError].
//
// NOTE: This is a stub implementation. The real invocation loop will be
// implemented in T3.3. Until then, Invoke always returns a Failed result.
func (o *Orchestrator) Invoke(_ context.Context, _ invocation.InvocationRequest) (invocation.InvocationResult, error) {
	err := fmt.Errorf("orchestrator: Invoke not yet implemented")
	return invocation.InvocationResult{
		FinalState: state.Failed,
		Error:      err,
	}, err
}
