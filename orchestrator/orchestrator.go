// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/identity"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/telemetry"
	"github.com/praxis-os/praxis/tools"
)

const (
	defaultMaxTurns = 10
)

// Orchestrator runs agent invocations through the praxis state machine.
//
// An Orchestrator is safe for concurrent use: multiple goroutines may call
// [Orchestrator.Invoke] simultaneously on the same instance. Each invocation
// owns its own state machine and context.
//
// Create an Orchestrator with [New].
type Orchestrator struct {
	provider     llm.Provider
	defaultModel string
	maxTurns     int
	logger       *slog.Logger

	toolInvoker        tools.Invoker
	policyHook         hooks.PolicyHook
	preLLMFilter       hooks.PreLLMFilter
	preToolFilter      hooks.PreToolFilter
	postToolFilter     hooks.PostToolFilter
	budgetGuard        budget.Guard
	priceProvider      budget.PriceProvider
	lifecycleEmitter   telemetry.LifecycleEventEmitter
	attributeEnricher  telemetry.AttributeEnricher
	credentialResolver credentials.Resolver
	identitySigner     identity.Signer
	classifier         errors.Classifier
}

// New creates an Orchestrator backed by the given provider.
//
// provider must not be nil; New returns a non-nil error if it is.
// Options are applied in order after defaults are set. If any option returns
// an error, New returns that error immediately.
//
// Default values:
//   - maxTurns: 10
//   - defaultModel: "" (provider's own default)
//   - toolInvoker: tools.NullInvoker{}
//   - policyHook: hooks.AllowAllPolicyHook{}
//   - preLLMFilter: hooks.PassThroughPreLLMFilter{}
//   - preToolFilter: hooks.PassThroughPreToolFilter{}
//   - postToolFilter: hooks.PassThroughPostToolFilter{}
//   - budgetGuard: budget.NullGuard{}
//   - priceProvider: budget.NullPriceProvider{}
//   - lifecycleEmitter: telemetry.NullEmitter{}
//   - attributeEnricher: telemetry.NullEnricher{}
//   - credentialResolver: credentials.NullResolver{}
//   - identitySigner: identity.NullSigner{}
//   - classifier: errors.DefaultClassifier{}
//   - logger: slog.Default()
func New(provider llm.Provider, opts ...Option) (*Orchestrator, error) {
	if provider == nil {
		return nil, fmt.Errorf("orchestrator: provider must not be nil")
	}

	o := &Orchestrator{
		provider:           provider,
		maxTurns:           defaultMaxTurns,
		logger:             slog.Default(),
		toolInvoker:        tools.NullInvoker{},
		policyHook:         hooks.AllowAllPolicyHook{},
		preLLMFilter:       hooks.PassThroughPreLLMFilter{},
		preToolFilter:      hooks.PassThroughPreToolFilter{},
		postToolFilter:     hooks.PassThroughPostToolFilter{},
		budgetGuard:        budget.NullGuard{},
		priceProvider:      budget.NullPriceProvider{},
		lifecycleEmitter:   telemetry.NullEmitter{},
		attributeEnricher:  telemetry.NullEnricher{},
		credentialResolver: credentials.NullResolver{},
		identitySigner:     identity.NullSigner{},
		classifier:         errors.NewDefaultClassifier(),
	}

	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
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
// terminal state and a CancellationError.
func (o *Orchestrator) Invoke(ctx context.Context, req praxis.InvocationRequest) (*praxis.InvocationResult, error) {
	model := req.Model
	if model == "" {
		model = o.defaultModel
	}
	if model == "" {
		return nil, fmt.Errorf("orchestrator: no model configured: set WithDefaultModel or InvocationRequest.Model")
	}

	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = o.maxTurns
	}

	return runInvocation(ctx, o, model, maxTurns, req)
}
