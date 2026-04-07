// SPDX-License-Identifier: Apache-2.0

// Package retry provides exponential backoff with jitter for retrying operations
// that may fail with transient errors.
//
// The package integrates with the praxis typed error taxonomy via the
// [github.com/praxis-os/praxis/errors.Classifier] interface. Only errors
// classified as retryable (i.e., ErrorKindTransientLLM) are retried; all other
// error kinds cause immediate termination of the retry loop.
//
// # Backoff Algorithm
//
// The delay between attempts grows exponentially:
//
//	delay = min(baseDelay * 2^(attempt-1), maxDelay)
//
// A random jitter in the range [0, delay*0.5) is added to each computed delay
// to prevent thundering-herd behaviour when many callers retry simultaneously.
//
// # Context Cancellation
//
// All functions accept a [context.Context]. If the context is cancelled while
// sleeping between retries, the functions return immediately with the context's
// error rather than waiting for the full delay to elapse.
//
// # Default Values
//
// When the classifier's RetryPolicy does not provide configuration, the package
// falls back to:
//   - MaxAttempts: 3
//   - BaseDelay:   100ms
//   - MaxDelay:    30s
package retry
