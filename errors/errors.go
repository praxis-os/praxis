// SPDX-License-Identifier: Apache-2.0

// Package errors defines the typed error taxonomy for praxis.
//
// Every error produced by praxis implements the [TypedError] interface, which
// carries a stable [ErrorKind] driving retry and terminal-state decisions.
// The package also provides the [Classifier] interface for converting
// arbitrary errors into typed errors.
//
// Stability: frozen-v1.0. The TypedError interface, ErrorKind values, and
// Classifier contract are load-bearing and may not change without a D51
// amendment.
package errors

// TypedError is the common interface for all praxis errors.
//
// Every error produced by the praxis runtime implements this interface.
// The Kind method returns a stable classification used by the orchestrator
// to decide retry behavior and terminal state mapping.
//
// TypedError extends the standard error interface and supports unwrapping
// via Unwrap for use with [errors.Is] and [errors.As].
type TypedError interface {
	error

	// Kind returns the stable error classification.
	// The returned value drives retry decisions and terminal state mapping.
	Kind() ErrorKind

	// HTTPStatusCode returns an HTTP status code hint for this error.
	// This is advisory — callers are not required to use it.
	HTTPStatusCode() int

	// Unwrap returns the underlying error, if any.
	// Returns nil if there is no underlying cause.
	Unwrap() error
}

// ErrorKind classifies errors into stable categories that drive retry
// decisions and terminal state mapping.
//
// ErrorKind is a string type for debuggability and serialization safety.
// The set of valid ErrorKind values is fixed at v1.0.
type ErrorKind string

const (
	// ErrorKindTransientLLM indicates a retryable LLM provider error
	// (e.g., HTTP 429, 503). Retry policy: 3 retries with exponential
	// backoff + jitter, base 500ms.
	ErrorKindTransientLLM ErrorKind = "transient_llm"

	// ErrorKindPermanentLLM indicates a non-retryable LLM provider error
	// (e.g., HTTP 400, 401). No retry.
	ErrorKindPermanentLLM ErrorKind = "permanent_llm"

	// ErrorKindTool indicates a tool invocation failure.
	// Not retried by the framework; the tool invoker owns retry logic.
	ErrorKindTool ErrorKind = "tool"

	// ErrorKindPolicyDenied indicates a policy hook returned a deny verdict.
	// Not retried. Maps to Failed terminal state.
	ErrorKindPolicyDenied ErrorKind = "policy_denied"

	// ErrorKindBudgetExceeded indicates a budget dimension was breached
	// (tokens, cost, duration, or tool calls). Terminal, no retry.
	ErrorKindBudgetExceeded ErrorKind = "budget_exceeded"

	// ErrorKindCancellation indicates context cancellation (soft or hard).
	// Terminal, no retry.
	ErrorKindCancellation ErrorKind = "cancellation"

	// ErrorKindSystem indicates a framework-internal error (e.g., illegal
	// state transition, configuration error). Not retried.
	ErrorKindSystem ErrorKind = "system"

	// ErrorKindApprovalRequired indicates that a policy hook requires human
	// approval before the invocation can continue. Terminal but not a failure;
	// maps to ApprovalRequired state. HTTP 202 (Accepted).
	ErrorKindApprovalRequired ErrorKind = "approval_required"
)

// String returns the string representation of the ErrorKind.
func (k ErrorKind) String() string {
	return string(k)
}

// IsRetryable reports whether errors of this kind should be retried
// by the framework. Only ErrorKindTransientLLM is retryable.
func (k ErrorKind) IsRetryable() bool {
	return k == ErrorKindTransientLLM
}

// Classifier converts arbitrary errors into [TypedError] values.
//
// The classification precedence (CP5, D63) is:
//  1. Identity rule — if err is already a TypedError (via errors.As), return it.
//  2. Context cancellation — context.Canceled or DeadlineExceeded → CancellationError.
//  3. HTTP status heuristic — 5xx/429 → TransientLLMError; 4xx → PermanentLLMError.
//  4. Default fallback — wrap in SystemError.
type Classifier interface {
	// Classify converts err into a TypedError.
	// The returned value is never nil if err is non-nil.
	Classify(err error) TypedError
}
