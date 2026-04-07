// SPDX-License-Identifier: Apache-2.0

package orchestrator

// Option is a functional option that configures an [Orchestrator].
// Options are applied in the order they are passed to [New].
type Option func(*Orchestrator)

// WithDefaultModel sets the model identifier used when an
// [invocation.InvocationRequest] does not specify a model.
//
// The model string is passed verbatim to the [llm.Provider]; praxis does
// not validate or normalise it. An empty string is a no-op: the existing
// default (empty string, meaning the provider's own default) is preserved.
func WithDefaultModel(model string) Option {
	return func(o *Orchestrator) {
		o.defaultModel = model
	}
}

// WithMaxIterations sets the default maximum number of LLM round-trips
// allowed per invocation.
//
// Values below 1 are clamped to 1; values above 100 are clamped to 100.
// An individual [invocation.InvocationRequest] may override this value
// by setting its own MaxIterations field.
func WithMaxIterations(n int) Option {
	return func(o *Orchestrator) {
		switch {
		case n < 1:
			o.maxIterations = 1
		case n > 100:
			o.maxIterations = 100
		default:
			o.maxIterations = n
		}
	}
}
