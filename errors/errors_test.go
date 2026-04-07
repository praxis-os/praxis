// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"testing"
)

func TestErrorKindValues(t *testing.T) {
	// Verify the 8 ErrorKind constants have correct string values.
	tests := []struct {
		kind ErrorKind
		want string
	}{
		{ErrorKindTransientLLM, "transient_llm"},
		{ErrorKindPermanentLLM, "permanent_llm"},
		{ErrorKindTool, "tool"},
		{ErrorKindPolicyDenied, "policy_denied"},
		{ErrorKindBudgetExceeded, "budget_exceeded"},
		{ErrorKindCancellation, "cancellation"},
		{ErrorKindSystem, "system"},
		{ErrorKindApprovalRequired, "approval_required"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("ErrorKind(%q).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestErrorKindCount(t *testing.T) {
	// There must be exactly 8 error kinds.
	kinds := []ErrorKind{
		ErrorKindTransientLLM,
		ErrorKindPermanentLLM,
		ErrorKindTool,
		ErrorKindPolicyDenied,
		ErrorKindBudgetExceeded,
		ErrorKindCancellation,
		ErrorKindSystem,
		ErrorKindApprovalRequired,
	}
	if got := len(kinds); got != 8 {
		t.Errorf("len(ErrorKinds) = %d, want 8", got)
	}

	// All must be unique.
	seen := make(map[ErrorKind]bool)
	for _, k := range kinds {
		if seen[k] {
			t.Errorf("duplicate ErrorKind: %s", k)
		}
		seen[k] = true
	}
}

func TestErrorKindIsRetryable(t *testing.T) {
	retryable := []ErrorKind{ErrorKindTransientLLM}
	nonRetryable := []ErrorKind{
		ErrorKindPermanentLLM,
		ErrorKindTool,
		ErrorKindPolicyDenied,
		ErrorKindBudgetExceeded,
		ErrorKindCancellation,
		ErrorKindSystem,
		ErrorKindApprovalRequired,
	}

	for _, k := range retryable {
		if !k.IsRetryable() {
			t.Errorf("%s.IsRetryable() = false, want true", k)
		}
	}
	for _, k := range nonRetryable {
		if k.IsRetryable() {
			t.Errorf("%s.IsRetryable() = true, want false", k)
		}
	}
}

// stubTypedError is a minimal TypedError implementation for interface tests.
type stubTypedError struct {
	kind       ErrorKind
	statusCode int
	msg        string
	cause      error
}

func (e *stubTypedError) Error() string       { return e.msg }
func (e *stubTypedError) Kind() ErrorKind     { return e.kind }
func (e *stubTypedError) HTTPStatusCode() int { return e.statusCode }
func (e *stubTypedError) Unwrap() error       { return e.cause }

func TestTypedErrorInterface(t *testing.T) {
	// Verify stubTypedError satisfies TypedError at compile time.
	var _ TypedError = (*stubTypedError)(nil)

	err := &stubTypedError{
		kind:       ErrorKindSystem,
		statusCode: 500,
		msg:        "test error",
		cause:      nil,
	}

	if got := err.Kind(); got != ErrorKindSystem {
		t.Errorf("Kind() = %s, want %s", got, ErrorKindSystem)
	}
	if got := err.HTTPStatusCode(); got != 500 {
		t.Errorf("HTTPStatusCode() = %d, want 500", got)
	}
	if got := err.Error(); got != "test error" {
		t.Errorf("Error() = %q, want %q", got, "test error")
	}
	if got := err.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestTypedErrorWithCause(t *testing.T) {
	cause := &stubTypedError{
		kind:       ErrorKindTransientLLM,
		statusCode: 503,
		msg:        "upstream timeout",
	}
	wrapped := &stubTypedError{
		kind:       ErrorKindSystem,
		statusCode: 500,
		msg:        "wrapper",
		cause:      cause,
	}

	if got := wrapped.Unwrap(); got != cause {
		t.Errorf("Unwrap() = %v, want %v", got, cause)
	}
}

func TestClassifierInterfaceCompiles(t *testing.T) {
	// Verify Classifier is a valid interface (compile-time check).
	var _ Classifier = (*stubClassifier)(nil)
}

type stubClassifier struct{}

func (c *stubClassifier) Classify(err error) TypedError {
	return &stubTypedError{
		kind:       ErrorKindSystem,
		statusCode: 500,
		msg:        err.Error(),
		cause:      err,
	}
}
