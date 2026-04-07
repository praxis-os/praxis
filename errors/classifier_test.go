// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"
)

func TestDefaultClassifierNil(t *testing.T) {
	c := NewDefaultClassifier()
	if got := c.Classify(nil); got != nil {
		t.Errorf("Classify(nil) = %v, want nil", got)
	}
}

func TestClassifierIdentityRule(t *testing.T) {
	c := NewDefaultClassifier()

	// A TypedError should be returned unchanged.
	original := NewTransientLLMError("anthropic", 503, nil)
	got := c.Classify(original)
	if got != original {
		t.Error("identity rule: Classify should return the same TypedError")
	}
}

func TestClassifierIdentityRuleWrapped(t *testing.T) {
	c := NewDefaultClassifier()

	// A TypedError wrapped with fmt.Errorf should still be found via errors.As.
	original := NewPermanentLLMError("openai", 400, nil)
	wrapped := fmt.Errorf("request failed: %w", original)
	got := c.Classify(wrapped)

	var pe *PermanentLLMError
	if !stderrors.As(got, &pe) {
		t.Fatalf("expected PermanentLLMError, got %T", got)
	}
	if pe.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", pe.Provider, "openai")
	}
}

func TestClassifierContextCanceled(t *testing.T) {
	c := NewDefaultClassifier()
	got := c.Classify(context.Canceled)

	var ce *CancellationError
	if !stderrors.As(got, &ce) {
		t.Fatalf("expected CancellationError, got %T", got)
	}
	if ce.CancelKind() != CancellationKindSoft {
		t.Errorf("CancelKind() = %q, want %q", ce.CancelKind(), CancellationKindSoft)
	}
}

func TestClassifierContextDeadlineExceeded(t *testing.T) {
	c := NewDefaultClassifier()
	got := c.Classify(context.DeadlineExceeded)

	var ce *CancellationError
	if !stderrors.As(got, &ce) {
		t.Fatalf("expected CancellationError, got %T", got)
	}
	if ce.CancelKind() != CancellationKindHard {
		t.Errorf("CancelKind() = %q, want %q", ce.CancelKind(), CancellationKindHard)
	}
}

func TestClassifierWrappedContextCanceled(t *testing.T) {
	c := NewDefaultClassifier()
	wrapped := fmt.Errorf("operation: %w", context.Canceled)
	got := c.Classify(wrapped)

	var ce *CancellationError
	if !stderrors.As(got, &ce) {
		t.Fatalf("expected CancellationError for wrapped context.Canceled, got %T", got)
	}
}

// httpError implements HTTPStatusError for testing.
type httpError struct {
	code int
	msg  string
}

func (e *httpError) Error() string   { return e.msg }
func (e *httpError) HTTPStatus() int { return e.code }

func TestClassifierHTTPStatus429(t *testing.T) {
	c := NewDefaultClassifier()
	got := c.Classify(&httpError{code: 429, msg: "rate limited"})

	var te *TransientLLMError
	if !stderrors.As(got, &te) {
		t.Fatalf("expected TransientLLMError for 429, got %T", got)
	}
	if te.Kind() != ErrorKindTransientLLM {
		t.Errorf("Kind() = %s, want %s", te.Kind(), ErrorKindTransientLLM)
	}
}

func TestClassifierHTTPStatus503(t *testing.T) {
	c := NewDefaultClassifier()
	got := c.Classify(&httpError{code: 503, msg: "service unavailable"})

	var te *TransientLLMError
	if !stderrors.As(got, &te) {
		t.Fatalf("expected TransientLLMError for 503, got %T", got)
	}
}

func TestClassifierHTTPStatus500(t *testing.T) {
	c := NewDefaultClassifier()
	got := c.Classify(&httpError{code: 500, msg: "internal error"})

	var te *TransientLLMError
	if !stderrors.As(got, &te) {
		t.Fatalf("expected TransientLLMError for 500, got %T", got)
	}
}

func TestClassifierHTTPStatus400(t *testing.T) {
	c := NewDefaultClassifier()
	got := c.Classify(&httpError{code: 400, msg: "bad request"})

	var pe *PermanentLLMError
	if !stderrors.As(got, &pe) {
		t.Fatalf("expected PermanentLLMError for 400, got %T", got)
	}
}

func TestClassifierHTTPStatus401(t *testing.T) {
	c := NewDefaultClassifier()
	got := c.Classify(&httpError{code: 401, msg: "unauthorized"})

	var pe *PermanentLLMError
	if !stderrors.As(got, &pe) {
		t.Fatalf("expected PermanentLLMError for 401, got %T", got)
	}
}

func TestClassifierHTTPStatus404(t *testing.T) {
	c := NewDefaultClassifier()
	got := c.Classify(&httpError{code: 404, msg: "not found"})

	var pe *PermanentLLMError
	if !stderrors.As(got, &pe) {
		t.Fatalf("expected PermanentLLMError for 404, got %T", got)
	}
}

func TestClassifierDefaultFallback(t *testing.T) {
	c := NewDefaultClassifier()
	got := c.Classify(fmt.Errorf("some random error"))

	var se *SystemError
	if !stderrors.As(got, &se) {
		t.Fatalf("expected SystemError for unknown error, got %T", got)
	}
	if se.Kind() != ErrorKindSystem {
		t.Errorf("Kind() = %s, want %s", se.Kind(), ErrorKindSystem)
	}
}

func TestClassifierPrecedenceIdentityBeforeContext(t *testing.T) {
	c := NewDefaultClassifier()

	// A TypedError wrapping context.Canceled should be returned as-is
	// (identity rule takes precedence over context check).
	te := NewSystemError("wrapped cancel", context.Canceled)
	got := c.Classify(te)
	if got != te {
		t.Error("identity rule should take precedence over context cancellation")
	}
}

func TestClassifierImplementsInterface(t *testing.T) {
	var _ Classifier = (*DefaultClassifier)(nil)
}

func TestRetryPolicyForTransientLLM(t *testing.T) {
	p := RetryPolicyFor(ErrorKindTransientLLM)
	if !p.Retryable {
		t.Error("transient_llm should be retryable")
	}
	if p.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", p.MaxRetries)
	}
	if p.BaseDelayMs != 500 {
		t.Errorf("BaseDelayMs = %d, want 500", p.BaseDelayMs)
	}
}

func TestRetryPolicyForNonRetryable(t *testing.T) {
	nonRetryable := []ErrorKind{
		ErrorKindPermanentLLM,
		ErrorKindTool,
		ErrorKindPolicyDenied,
		ErrorKindBudgetExceeded,
		ErrorKindCancellation,
		ErrorKindSystem,
		ErrorKindApprovalRequired,
	}
	for _, kind := range nonRetryable {
		p := RetryPolicyFor(kind)
		if p.Retryable {
			t.Errorf("%s should not be retryable", kind)
		}
		if p.MaxRetries != 0 {
			t.Errorf("%s MaxRetries = %d, want 0", kind, p.MaxRetries)
		}
	}
}

func TestRetryPolicyForUnknownKind(t *testing.T) {
	p := RetryPolicyFor(ErrorKind("unknown"))
	if p.Retryable {
		t.Error("unknown kind should not be retryable")
	}
}
