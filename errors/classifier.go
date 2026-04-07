// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"context"
	stderrors "errors"
)

// DefaultClassifier implements [Classifier] with the CP5 precedence rules (D63).
//
// Classification precedence:
//  1. Identity rule — if err is already a TypedError (via errors.As), return it.
//  2. Context cancellation — context.Canceled or DeadlineExceeded → CancellationError.
//  3. HTTP status heuristic — if the error carries an HTTP status code via
//     the [HTTPStatusError] interface, classify based on the status code.
//  4. Default fallback — wrap in SystemError.
type DefaultClassifier struct{}

// Classify converts err into a TypedError using CP5 precedence.
// Returns nil if err is nil.
func (c *DefaultClassifier) Classify(err error) TypedError {
	if err == nil {
		return nil
	}

	// 1. Identity rule: already a TypedError → return unchanged.
	var te TypedError
	if stderrors.As(err, &te) {
		return te
	}

	// 2. Context cancellation.
	if stderrors.Is(err, context.Canceled) {
		return NewCancellationError(CancellationKindSoft, err)
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		return NewCancellationError(CancellationKindHard, err)
	}

	// 3. HTTP status heuristic.
	var httpErr HTTPStatusError
	if stderrors.As(err, &httpErr) {
		code := httpErr.HTTPStatus()
		switch {
		case code == 429 || (code >= 500 && code < 600):
			return NewTransientLLMError("unknown", code, err)
		case code >= 400 && code < 500:
			return NewPermanentLLMError("unknown", code, err)
		}
	}

	// 4. Default fallback.
	return NewSystemError(err.Error(), err)
}

// HTTPStatusError is an optional interface that errors can implement to
// provide an HTTP status code hint for the classifier's heuristic rule.
type HTTPStatusError interface {
	HTTPStatus() int
}

// NewDefaultClassifier returns a new [DefaultClassifier].
func NewDefaultClassifier() *DefaultClassifier {
	return &DefaultClassifier{}
}

// RetryPolicy describes the retry behavior for a given [ErrorKind].
type RetryPolicy struct {
	// Retryable indicates whether the error should be retried.
	Retryable bool
	// MaxRetries is the maximum number of retry attempts. Zero if not retryable.
	MaxRetries int
	// BaseDelay is the base delay for exponential backoff in milliseconds.
	// Zero if not retryable.
	BaseDelayMs int
}

// retryPolicies maps each ErrorKind to its retry policy.
var retryPolicies = map[ErrorKind]RetryPolicy{
	ErrorKindTransientLLM:     {Retryable: true, MaxRetries: 3, BaseDelayMs: 500},
	ErrorKindPermanentLLM:     {Retryable: false},
	ErrorKindTool:             {Retryable: false},
	ErrorKindPolicyDenied:     {Retryable: false},
	ErrorKindBudgetExceeded:   {Retryable: false},
	ErrorKindCancellation:     {Retryable: false},
	ErrorKindSystem:           {Retryable: false},
	ErrorKindApprovalRequired: {Retryable: false},
}

// RetryPolicyFor returns the retry policy for the given ErrorKind.
func RetryPolicyFor(kind ErrorKind) RetryPolicy {
	if p, ok := retryPolicies[kind]; ok {
		return p
	}
	return RetryPolicy{Retryable: false}
}
