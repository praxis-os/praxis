// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"fmt"

	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/identity"
	"github.com/praxis-os/praxis/telemetry"
	"github.com/praxis-os/praxis/tools"
)

// Option is a functional option that configures an [Orchestrator].
// Options are applied in the order they are passed to [New]. An option may
// return a non-nil error to abort construction immediately.
type Option func(*Orchestrator) error

// WithDefaultModel sets the model identifier used when an
// [invocation.InvocationRequest] does not specify a model.
//
// The model string is passed verbatim to the [llm.Provider]; praxis does
// not validate or normalise it. An empty string is a no-op: the existing
// default (empty string, meaning the provider's own default) is preserved.
func WithDefaultModel(model string) Option {
	return func(o *Orchestrator) error {
		o.defaultModel = model
		return nil
	}
}

// WithMaxIterations sets the default maximum number of LLM round-trips
// allowed per invocation.
//
// Values below 1 are clamped to 1; values above 100 are clamped to 100.
// An individual [invocation.InvocationRequest] may override this value
// by setting its own MaxIterations field.
func WithMaxIterations(n int) Option {
	return func(o *Orchestrator) error {
		switch {
		case n < 1:
			o.maxIterations = 1
		case n > 100:
			o.maxIterations = 100
		default:
			o.maxIterations = n
		}
		return nil
	}
}

// WithToolInvoker sets the [tools.Invoker] used to dispatch tool calls
// requested by the LLM. Passing nil returns an error.
//
// The last call to WithToolInvoker wins when multiple options are provided.
func WithToolInvoker(inv tools.Invoker) Option {
	return func(o *Orchestrator) error {
		if inv == nil {
			return fmt.Errorf("orchestrator: WithToolInvoker must not be nil")
		}
		o.toolInvoker = inv
		return nil
	}
}

// WithPolicyHook sets the [hooks.PolicyHook] evaluated at invocation
// lifecycle phases. Passing nil returns an error.
//
// The last call to WithPolicyHook wins when multiple options are provided.
func WithPolicyHook(hook hooks.PolicyHook) Option {
	return func(o *Orchestrator) error {
		if hook == nil {
			return fmt.Errorf("orchestrator: WithPolicyHook must not be nil")
		}
		o.policyHook = hook
		return nil
	}
}

// WithPreLLMFilter sets the [hooks.PreLLMFilter] applied to each outgoing
// LLM request before it is dispatched. Passing nil returns an error.
//
// The last call to WithPreLLMFilter wins when multiple options are provided.
func WithPreLLMFilter(f hooks.PreLLMFilter) Option {
	return func(o *Orchestrator) error {
		if f == nil {
			return fmt.Errorf("orchestrator: WithPreLLMFilter must not be nil")
		}
		o.preLLMFilter = f
		return nil
	}
}

// WithPostToolFilter sets the [hooks.PostToolFilter] applied to each tool
// result before it is included in the next LLM turn. Passing nil returns an error.
//
// The last call to WithPostToolFilter wins when multiple options are provided.
func WithPostToolFilter(f hooks.PostToolFilter) Option {
	return func(o *Orchestrator) error {
		if f == nil {
			return fmt.Errorf("orchestrator: WithPostToolFilter must not be nil")
		}
		o.postToolFilter = f
		return nil
	}
}

// WithBudgetGuard sets the [budget.Guard] that enforces resource limits at
// each turn boundary. Passing nil returns an error.
//
// The last call to WithBudgetGuard wins when multiple options are provided.
func WithBudgetGuard(g budget.Guard) Option {
	return func(o *Orchestrator) error {
		if g == nil {
			return fmt.Errorf("orchestrator: WithBudgetGuard must not be nil")
		}
		o.budgetGuard = g
		return nil
	}
}

// WithPriceProvider sets the [budget.PriceProvider] used to compute cost
// estimates for [budget.Usage.CostMicros]. Passing nil returns an error.
//
// The last call to WithPriceProvider wins when multiple options are provided.
func WithPriceProvider(p budget.PriceProvider) Option {
	return func(o *Orchestrator) error {
		if p == nil {
			return fmt.Errorf("orchestrator: WithPriceProvider must not be nil")
		}
		o.priceProvider = p
		return nil
	}
}

// WithLifecycleEmitter sets the [telemetry.LifecycleEventEmitter] that
// receives invocation state-transition events. Passing nil returns an error.
//
// The last call to WithLifecycleEmitter wins when multiple options are provided.
func WithLifecycleEmitter(e telemetry.LifecycleEventEmitter) Option {
	return func(o *Orchestrator) error {
		if e == nil {
			return fmt.Errorf("orchestrator: WithLifecycleEmitter must not be nil")
		}
		o.lifecycleEmitter = e
		return nil
	}
}

// WithAttributeEnricher sets the [telemetry.AttributeEnricher] that adds
// caller-specific attributes to telemetry metadata. Passing nil returns an error.
//
// The last call to WithAttributeEnricher wins when multiple options are provided.
func WithAttributeEnricher(e telemetry.AttributeEnricher) Option {
	return func(o *Orchestrator) error {
		if e == nil {
			return fmt.Errorf("orchestrator: WithAttributeEnricher must not be nil")
		}
		o.attributeEnricher = e
		return nil
	}
}

// WithCredentialResolver sets the [credentials.Resolver] used to fetch named
// credentials at invocation time. Passing nil returns an error.
//
// The last call to WithCredentialResolver wins when multiple options are provided.
func WithCredentialResolver(r credentials.Resolver) Option {
	return func(o *Orchestrator) error {
		if r == nil {
			return fmt.Errorf("orchestrator: WithCredentialResolver must not be nil")
		}
		o.credentialResolver = r
		return nil
	}
}

// WithIdentitySigner sets the [identity.Signer] that produces signed identity
// tokens at the start of each invocation. Passing nil returns an error.
//
// The last call to WithIdentitySigner wins when multiple options are provided.
func WithIdentitySigner(s identity.Signer) Option {
	return func(o *Orchestrator) error {
		if s == nil {
			return fmt.Errorf("orchestrator: WithIdentitySigner must not be nil")
		}
		o.identitySigner = s
		return nil
	}
}

// WithErrorClassifier sets the [errors.Classifier] used to convert arbitrary
// errors into [errors.TypedError] values. Passing nil returns an error.
//
// The last call to WithErrorClassifier wins when multiple options are provided.
func WithErrorClassifier(c errors.Classifier) Option {
	return func(o *Orchestrator) error {
		if c == nil {
			return fmt.Errorf("orchestrator: WithErrorClassifier must not be nil")
		}
		o.classifier = c
		return nil
	}
}
