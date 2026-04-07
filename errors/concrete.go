// SPDX-License-Identifier: Apache-2.0

package errors

import "fmt"

// TransientLLMError represents a retryable LLM provider error (e.g., HTTP 429, 503).
// Retry policy: 3 retries with exponential backoff + jitter, base 500ms.
type TransientLLMError struct {
	// Provider is the Name() of the llm.Provider that produced the error.
	Provider string
	// StatusCode is the HTTP status code from the provider, if available.
	StatusCode int

	cause error
}

// NewTransientLLMError creates a TransientLLMError.
func NewTransientLLMError(provider string, statusCode int, cause error) *TransientLLMError {
	return &TransientLLMError{Provider: provider, StatusCode: statusCode, cause: cause}
}

func (e *TransientLLMError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("transient LLM error from %s (HTTP %d): %v", e.Provider, e.StatusCode, e.cause)
	}
	return fmt.Sprintf("transient LLM error from %s (HTTP %d)", e.Provider, e.StatusCode)
}

func (e *TransientLLMError) Kind() ErrorKind     { return ErrorKindTransientLLM }
func (e *TransientLLMError) HTTPStatusCode() int { return 503 }
func (e *TransientLLMError) Unwrap() error       { return e.cause }

// PermanentLLMError represents a non-retryable LLM provider error (e.g., HTTP 400, 401).
type PermanentLLMError struct {
	// Provider is the Name() of the llm.Provider that produced the error.
	Provider string
	// StatusCode is the HTTP status code from the provider, if available.
	StatusCode int

	cause error
}

// NewPermanentLLMError creates a PermanentLLMError.
func NewPermanentLLMError(provider string, statusCode int, cause error) *PermanentLLMError {
	return &PermanentLLMError{Provider: provider, StatusCode: statusCode, cause: cause}
}

func (e *PermanentLLMError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("permanent LLM error from %s (HTTP %d): %v", e.Provider, e.StatusCode, e.cause)
	}
	return fmt.Sprintf("permanent LLM error from %s (HTTP %d)", e.Provider, e.StatusCode)
}

func (e *PermanentLLMError) Kind() ErrorKind     { return ErrorKindPermanentLLM }
func (e *PermanentLLMError) HTTPStatusCode() int { return 502 }
func (e *PermanentLLMError) Unwrap() error       { return e.cause }

// ToolSubKind further classifies tool errors.
type ToolSubKind string

const (
	ToolSubKindNetwork         ToolSubKind = "network"
	ToolSubKindServerError     ToolSubKind = "server_error"
	ToolSubKindCircuitOpen     ToolSubKind = "circuit_open"
	ToolSubKindSchemaViolation ToolSubKind = "schema_violation"
)

// ToolError represents a tool invocation failure.
// Not retried by the framework; the tool invoker owns retry logic.
type ToolError struct {
	// ToolName is the name of the failing tool.
	ToolName string
	// CallID is the ToolCall.CallID from the LLM response.
	CallID string
	// SubKind provides further classification of the tool error.
	SubKind ToolSubKind

	cause error
}

// NewToolError creates a ToolError.
func NewToolError(toolName, callID string, subKind ToolSubKind, cause error) *ToolError {
	return &ToolError{ToolName: toolName, CallID: callID, SubKind: subKind, cause: cause}
}

func (e *ToolError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("tool %q (call %s, %s): %v", e.ToolName, e.CallID, e.SubKind, e.cause)
	}
	return fmt.Sprintf("tool %q (call %s, %s)", e.ToolName, e.CallID, e.SubKind)
}

func (e *ToolError) Kind() ErrorKind     { return ErrorKindTool }
func (e *ToolError) HTTPStatusCode() int { return 502 }
func (e *ToolError) Unwrap() error       { return e.cause }

// PolicyDeniedError represents a policy hook deny verdict.
type PolicyDeniedError struct {
	// Phase is the hook phase at which the denial occurred (e.g., "pre_invocation").
	Phase string
	// Reason is the Decision.Reason from the denying hook.
	Reason string
}

// NewPolicyDeniedError creates a PolicyDeniedError.
func NewPolicyDeniedError(phase, reason string) *PolicyDeniedError {
	return &PolicyDeniedError{Phase: phase, Reason: reason}
}

func (e *PolicyDeniedError) Error() string {
	return fmt.Sprintf("policy denied at %s: %s", e.Phase, e.Reason)
}

func (e *PolicyDeniedError) Kind() ErrorKind     { return ErrorKindPolicyDenied }
func (e *PolicyDeniedError) HTTPStatusCode() int { return 403 }
func (e *PolicyDeniedError) Unwrap() error       { return nil }

// BudgetExceededError represents a budget dimension breach.
// The ExceededDimension field indicates which dimension was breached.
//
// Note: token-dimension may overshoot by up to one LLM call (C3 caveat).
type BudgetExceededError struct {
	// ExceededDimension names the breached budget dimension
	// (e.g., "tokens", "cost", "duration", "tool_calls").
	ExceededDimension string
	// Limit is the configured limit for the exceeded dimension.
	Limit string
	// Actual is the actual value that exceeded the limit.
	Actual string
}

// NewBudgetExceededError creates a BudgetExceededError.
func NewBudgetExceededError(dimension, limit, actual string) *BudgetExceededError {
	return &BudgetExceededError{ExceededDimension: dimension, Limit: limit, Actual: actual}
}

func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("budget exceeded: %s (limit: %s, actual: %s)", e.ExceededDimension, e.Limit, e.Actual)
}

func (e *BudgetExceededError) Kind() ErrorKind     { return ErrorKindBudgetExceeded }
func (e *BudgetExceededError) HTTPStatusCode() int { return 429 }
func (e *BudgetExceededError) Unwrap() error       { return nil }

// CancellationKind distinguishes soft from hard cancellation.
type CancellationKind string

const (
	// CancellationKindSoft indicates a cooperative cancellation with a 500ms grace period.
	CancellationKindSoft CancellationKind = "soft"
	// CancellationKindHard indicates an immediate cancellation (deadline or budget breach).
	CancellationKindHard CancellationKind = "hard"
)

// CancellationError represents context cancellation.
type CancellationError struct {
	cancelKind CancellationKind
	cause      error
}

// NewCancellationError creates a CancellationError.
func NewCancellationError(kind CancellationKind, cause error) *CancellationError {
	return &CancellationError{cancelKind: kind, cause: cause}
}

// CancelKind returns whether this was a soft or hard cancellation.
func (e *CancellationError) CancelKind() CancellationKind { return e.cancelKind }

func (e *CancellationError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("cancellation (%s): %v", e.cancelKind, e.cause)
	}
	return fmt.Sprintf("cancellation (%s)", e.cancelKind)
}

func (e *CancellationError) Kind() ErrorKind     { return ErrorKindCancellation }
func (e *CancellationError) HTTPStatusCode() int { return 499 }
func (e *CancellationError) Unwrap() error       { return e.cause }

// SystemError represents a framework-internal error (e.g., illegal state
// transition, configuration error).
type SystemError struct {
	// Message describes the internal failure.
	Message string

	cause error
}

// NewSystemError creates a SystemError.
func NewSystemError(message string, cause error) *SystemError {
	return &SystemError{Message: message, cause: cause}
}

func (e *SystemError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("system error: %s: %v", e.Message, e.cause)
	}
	return fmt.Sprintf("system error: %s", e.Message)
}

func (e *SystemError) Kind() ErrorKind     { return ErrorKindSystem }
func (e *SystemError) HTTPStatusCode() int { return 500 }
func (e *SystemError) Unwrap() error       { return e.cause }

// ApprovalRequiredError indicates that a policy hook requires human approval
// before the invocation can continue. This is terminal but not a failure.
type ApprovalRequiredError struct {
	// Phase is the hook phase at which approval was requested.
	Phase string
	// Reason explains why approval is needed.
	Reason string
}

// NewApprovalRequiredError creates an ApprovalRequiredError.
func NewApprovalRequiredError(phase, reason string) *ApprovalRequiredError {
	return &ApprovalRequiredError{Phase: phase, Reason: reason}
}

func (e *ApprovalRequiredError) Error() string {
	return fmt.Sprintf("approval required at %s: %s", e.Phase, e.Reason)
}

func (e *ApprovalRequiredError) Kind() ErrorKind     { return ErrorKindApprovalRequired }
func (e *ApprovalRequiredError) HTTPStatusCode() int { return 202 }
func (e *ApprovalRequiredError) Unwrap() error       { return nil }
