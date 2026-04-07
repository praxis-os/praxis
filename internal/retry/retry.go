// SPDX-License-Identifier: Apache-2.0

package retry

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/praxis-os/praxis/errors"
)

const (
	defaultMaxAttempts = 3
	defaultBaseDelay   = 100 * time.Millisecond
	defaultMaxDelay    = 30 * time.Second
)

// Config holds retry configuration that overrides the values derived from the
// classifier's RetryPolicy. Zero values mean "use the classifier's policy" or
// fall back to package defaults when the policy is also zero.
type Config struct {
	// MaxAttempts is the total number of attempts (first call + retries).
	// Zero means use the classifier's policy MaxRetries+1, or the package
	// default of 3 if the policy is also zero.
	MaxAttempts int

	// BaseDelay is the initial delay for exponential backoff.
	// Zero means use the classifier's policy BaseDelayMs, or 100ms if zero.
	BaseDelay time.Duration

	// MaxDelay caps the computed delay before jitter is applied.
	// Zero means use the package default of 30s.
	MaxDelay time.Duration
}

// Do executes fn, retrying on retryable errors as determined by classifier.
//
// fn is called with the provided ctx so callers can propagate deadlines and
// values into each attempt. If ctx is cancelled while sleeping between
// attempts, Do returns immediately with the context error.
//
// Do returns the last error if all attempts are exhausted, or the first
// non-retryable error, or the context error on cancellation.
func Do(ctx context.Context, classifier errors.Classifier, fn func(ctx context.Context) error) error {
	return DoWithConfig(ctx, classifier, Config{}, fn)
}

// DoWithResult is like [Do] but returns a typed value on success.
//
// On failure, the zero value of T is returned along with the error.
func DoWithResult[T any](ctx context.Context, classifier errors.Classifier, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	var result T

	err := DoWithConfig(ctx, classifier, Config{}, func(ctx context.Context) error {
		var fnErr error
		result, fnErr = fn(ctx)
		return fnErr
	})
	if err != nil {
		return zero, err
	}
	return result, nil
}

// DoWithConfig is like [Do] but accepts explicit configuration overrides.
//
// Fields in cfg that are non-zero override the values from the classifier's
// RetryPolicy. Zero fields fall back to the classifier's policy and then to
// package defaults.
func DoWithConfig(ctx context.Context, classifier errors.Classifier, cfg Config, fn func(ctx context.Context) error) error {
	maxAttempts, baseDelay, maxDelay := resolveConfig(cfg, classifier)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Check context before each attempt.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Classify the error to determine retry eligibility.
		typed := classifier.Classify(lastErr)
		if !typed.Kind().IsRetryable() {
			return lastErr
		}

		// Last attempt — do not sleep, just return.
		if attempt == maxAttempts {
			break
		}

		delay := computeDelay(attempt, baseDelay, maxDelay)
		if !sleep(ctx, delay) {
			return ctx.Err()
		}
	}

	return lastErr
}

// resolveConfig merges explicit Config overrides with classifier policy
// defaults and package-level defaults.
//
// Priority for each field: cfg non-zero > classifier policy > package default.
func resolveConfig(cfg Config, classifier errors.Classifier) (maxAttempts int, baseDelay, maxDelay time.Duration) {
	// Obtain the policy for the most common retryable kind to seed defaults.
	policy := errors.RetryPolicyFor(errors.ErrorKindTransientLLM)

	// MaxAttempts
	switch {
	case cfg.MaxAttempts > 0:
		maxAttempts = cfg.MaxAttempts
	case policy.MaxRetries > 0:
		maxAttempts = policy.MaxRetries + 1 // MaxRetries is retries, we need total attempts
	default:
		maxAttempts = defaultMaxAttempts
	}

	// BaseDelay
	switch {
	case cfg.BaseDelay > 0:
		baseDelay = cfg.BaseDelay
	case policy.BaseDelayMs > 0:
		baseDelay = time.Duration(policy.BaseDelayMs) * time.Millisecond
	default:
		baseDelay = defaultBaseDelay
	}

	// MaxDelay
	switch {
	case cfg.MaxDelay > 0:
		maxDelay = cfg.MaxDelay
	default:
		maxDelay = defaultMaxDelay
	}

	return maxAttempts, baseDelay, maxDelay
}

// computeDelay returns the backoff delay for the given attempt number
// (1-based) with full jitter applied.
//
// Formula: min(baseDelay * 2^(attempt-1), maxDelay) + jitter
// where jitter is uniform in [0, cappedDelay * 0.5).
func computeDelay(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	// Compute exponential component, guarding against overflow.
	shift := attempt - 1
	if shift > 62 {
		shift = 62
	}
	exp := baseDelay * (1 << uint(shift))

	// Cap at maxDelay.
	if exp > maxDelay || exp < 0 { // exp < 0 catches overflow
		exp = maxDelay
	}

	// Add jitter in [0, exp*0.5).
	jitterBound := exp / 2
	if jitterBound > 0 {
		jitter := time.Duration(rand.Int64N(int64(jitterBound)))
		exp += jitter
	}

	// Final cap after jitter (jitter can push us slightly over maxDelay).
	if exp > maxDelay {
		exp = maxDelay
	}

	return exp
}

// sleep pauses for the given duration, returning true on normal completion
// or false if ctx is cancelled before the duration elapses.
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
