// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"errors"
	"fmt"
	"testing"
)

// TestAllConcreteTypesImplementTypedError verifies compile-time interface conformance.
func TestAllConcreteTypesImplementTypedError(t *testing.T) {
	var _ TypedError = (*TransientLLMError)(nil)
	var _ TypedError = (*PermanentLLMError)(nil)
	var _ TypedError = (*ToolError)(nil)
	var _ TypedError = (*PolicyDeniedError)(nil)
	var _ TypedError = (*BudgetExceededError)(nil)
	var _ TypedError = (*CancellationError)(nil)
	var _ TypedError = (*SystemError)(nil)
	var _ TypedError = (*ApprovalRequiredError)(nil)
}

func TestTransientLLMError(t *testing.T) {
	cause := fmt.Errorf("connection reset")
	err := NewTransientLLMError("anthropic", 503, cause)

	assertKind(t, err, ErrorKindTransientLLM)
	assertHTTPStatus(t, err, 503)
	assertUnwrap(t, err, cause)
	assertContains(t, err.Error(), "transient LLM error")
	assertContains(t, err.Error(), "anthropic")
	assertContains(t, err.Error(), "503")

	if err.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", err.Provider, "anthropic")
	}
}

func TestTransientLLMErrorNilCause(t *testing.T) {
	err := NewTransientLLMError("openai", 429, nil)
	assertUnwrap(t, err, nil)
	assertContains(t, err.Error(), "429")
}

func TestPermanentLLMError(t *testing.T) {
	cause := fmt.Errorf("invalid API key")
	err := NewPermanentLLMError("anthropic", 401, cause)

	assertKind(t, err, ErrorKindPermanentLLM)
	assertHTTPStatus(t, err, 502)
	assertUnwrap(t, err, cause)
	assertContains(t, err.Error(), "permanent LLM error")
}

func TestPermanentLLMErrorNilCause(t *testing.T) {
	err := NewPermanentLLMError("openai", 400, nil)
	assertUnwrap(t, err, nil)
}

func TestToolError(t *testing.T) {
	cause := fmt.Errorf("timeout")
	err := NewToolError("web_search", "call-123", ToolSubKindNetwork, cause)

	assertKind(t, err, ErrorKindTool)
	assertHTTPStatus(t, err, 502)
	assertUnwrap(t, err, cause)
	assertContains(t, err.Error(), "web_search")
	assertContains(t, err.Error(), "call-123")
	assertContains(t, err.Error(), "network")

	if err.ToolName != "web_search" {
		t.Errorf("ToolName = %q, want %q", err.ToolName, "web_search")
	}
	if err.SubKind != ToolSubKindNetwork {
		t.Errorf("SubKind = %q, want %q", err.SubKind, ToolSubKindNetwork)
	}
}

func TestToolErrorNilCause(t *testing.T) {
	err := NewToolError("db_query", "call-456", ToolSubKindServerError, nil)
	assertUnwrap(t, err, nil)
}

func TestToolSubKindValues(t *testing.T) {
	subKinds := []ToolSubKind{
		ToolSubKindNetwork,
		ToolSubKindServerError,
		ToolSubKindCircuitOpen,
		ToolSubKindSchemaViolation,
	}
	if len(subKinds) != 4 {
		t.Errorf("expected 4 ToolSubKind values, got %d", len(subKinds))
	}
	seen := make(map[ToolSubKind]bool)
	for _, sk := range subKinds {
		if seen[sk] {
			t.Errorf("duplicate ToolSubKind: %s", sk)
		}
		seen[sk] = true
	}
}

func TestPolicyDeniedError(t *testing.T) {
	err := NewPolicyDeniedError("pre_invocation", "rate limit exceeded")

	assertKind(t, err, ErrorKindPolicyDenied)
	assertHTTPStatus(t, err, 403)
	assertUnwrap(t, err, nil)
	assertContains(t, err.Error(), "policy denied")
	assertContains(t, err.Error(), "pre_invocation")
	assertContains(t, err.Error(), "rate limit exceeded")
}

func TestBudgetExceededError(t *testing.T) {
	err := NewBudgetExceededError("tokens", "10000", "10523")

	assertKind(t, err, ErrorKindBudgetExceeded)
	assertHTTPStatus(t, err, 429)
	assertUnwrap(t, err, nil)
	assertContains(t, err.Error(), "budget exceeded")
	assertContains(t, err.Error(), "tokens")
	assertContains(t, err.Error(), "10000")
	assertContains(t, err.Error(), "10523")
}

func TestCancellationErrorSoft(t *testing.T) {
	cause := fmt.Errorf("context canceled")
	err := NewCancellationError(CancellationKindSoft, cause)

	assertKind(t, err, ErrorKindCancellation)
	assertHTTPStatus(t, err, 499)
	assertUnwrap(t, err, cause)
	assertContains(t, err.Error(), "soft")

	if err.CancelKind() != CancellationKindSoft {
		t.Errorf("CancelKind() = %q, want %q", err.CancelKind(), CancellationKindSoft)
	}
}

func TestCancellationErrorHard(t *testing.T) {
	cause := fmt.Errorf("deadline exceeded")
	err := NewCancellationError(CancellationKindHard, cause)

	assertContains(t, err.Error(), "hard")
	if err.CancelKind() != CancellationKindHard {
		t.Errorf("CancelKind() = %q, want %q", err.CancelKind(), CancellationKindHard)
	}
}

func TestCancellationErrorNilCause(t *testing.T) {
	err := NewCancellationError(CancellationKindSoft, nil)
	assertUnwrap(t, err, nil)
	assertContains(t, err.Error(), "soft")
}

func TestSystemError(t *testing.T) {
	cause := fmt.Errorf("nil pointer")
	err := NewSystemError("illegal state transition", cause)

	assertKind(t, err, ErrorKindSystem)
	assertHTTPStatus(t, err, 500)
	assertUnwrap(t, err, cause)
	assertContains(t, err.Error(), "system error")
	assertContains(t, err.Error(), "illegal state transition")
}

func TestSystemErrorNilCause(t *testing.T) {
	err := NewSystemError("config error", nil)
	assertUnwrap(t, err, nil)
	assertContains(t, err.Error(), "config error")
}

func TestApprovalRequiredError(t *testing.T) {
	err := NewApprovalRequiredError("pre_invocation", "sensitive operation")

	assertKind(t, err, ErrorKindApprovalRequired)
	assertHTTPStatus(t, err, 202)
	assertUnwrap(t, err, nil)
	assertContains(t, err.Error(), "approval required")
	assertContains(t, err.Error(), "pre_invocation")
	assertContains(t, err.Error(), "sensitive operation")
}

func TestErrorsAsChaining(t *testing.T) {
	// Verify errors.As works through wrapping.
	inner := NewTransientLLMError("anthropic", 503, nil)
	wrapped := fmt.Errorf("invocation failed: %w", inner)

	var te *TransientLLMError
	if !errors.As(wrapped, &te) {
		t.Fatal("errors.As should find TransientLLMError through wrapping")
	}
	if te.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", te.Provider, "anthropic")
	}
}

func TestTypedErrorAsInterface(t *testing.T) {
	// Verify errors.As works with TypedError interface.
	inner := NewSystemError("test", nil)
	wrapped := fmt.Errorf("wrapped: %w", inner)

	var te TypedError
	if !errors.As(wrapped, &te) {
		t.Fatal("errors.As should find TypedError interface through wrapping")
	}
	if te.Kind() != ErrorKindSystem {
		t.Errorf("Kind() = %s, want %s", te.Kind(), ErrorKindSystem)
	}
}

func TestErrorKindToHTTPStatusMapping(t *testing.T) {
	// Verify each error type returns the correct HTTP status per spec.
	tests := []struct {
		name string
		err  TypedError
		http int
	}{
		{"TransientLLM", NewTransientLLMError("p", 503, nil), 503},
		{"PermanentLLM", NewPermanentLLMError("p", 400, nil), 502},
		{"Tool", NewToolError("t", "c", ToolSubKindNetwork, nil), 502},
		{"PolicyDenied", NewPolicyDeniedError("pre", "reason"), 403},
		{"BudgetExceeded", NewBudgetExceededError("tokens", "100", "200"), 429},
		{"CancellationSoft", NewCancellationError(CancellationKindSoft, nil), 499},
		{"CancellationHard", NewCancellationError(CancellationKindHard, nil), 499},
		{"System", NewSystemError("msg", nil), 500},
		{"ApprovalRequired", NewApprovalRequiredError("pre", "reason"), 202},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.HTTPStatusCode(); got != tt.http {
				t.Errorf("%s.HTTPStatusCode() = %d, want %d", tt.name, got, tt.http)
			}
		})
	}
}

func TestErrorKindMapping(t *testing.T) {
	// Verify each concrete type returns the correct ErrorKind.
	tests := []struct {
		name string
		err  TypedError
		kind ErrorKind
	}{
		{"TransientLLM", NewTransientLLMError("p", 503, nil), ErrorKindTransientLLM},
		{"PermanentLLM", NewPermanentLLMError("p", 400, nil), ErrorKindPermanentLLM},
		{"Tool", NewToolError("t", "c", ToolSubKindNetwork, nil), ErrorKindTool},
		{"PolicyDenied", NewPolicyDeniedError("pre", "r"), ErrorKindPolicyDenied},
		{"BudgetExceeded", NewBudgetExceededError("tok", "1", "2"), ErrorKindBudgetExceeded},
		{"Cancellation", NewCancellationError(CancellationKindSoft, nil), ErrorKindCancellation},
		{"System", NewSystemError("msg", nil), ErrorKindSystem},
		{"ApprovalRequired", NewApprovalRequiredError("pre", "r"), ErrorKindApprovalRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Kind(); got != tt.kind {
				t.Errorf("%s.Kind() = %s, want %s", tt.name, got, tt.kind)
			}
		})
	}
}

// helpers

func assertKind(t *testing.T, err TypedError, want ErrorKind) {
	t.Helper()
	if got := err.Kind(); got != want {
		t.Errorf("Kind() = %s, want %s", got, want)
	}
}

func assertHTTPStatus(t *testing.T, err TypedError, want int) {
	t.Helper()
	if got := err.HTTPStatusCode(); got != want {
		t.Errorf("HTTPStatusCode() = %d, want %d", got, want)
	}
}

func assertUnwrap(t *testing.T, err TypedError, want error) {
	t.Helper()
	if got := err.Unwrap(); got != want {
		t.Errorf("Unwrap() = %v, want %v", got, want)
	}
}

func assertContains(t *testing.T, got, substr string) {
	t.Helper()
	for i := 0; i <= len(got)-len(substr); i++ {
		if got[i:i+len(substr)] == substr {
			return
		}
	}
	t.Errorf("string %q does not contain %q", got, substr)
}
