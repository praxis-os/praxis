# Phase 3 — LLM Provider Interface

**Stability tier:** `frozen-v1.0`
**Decision:** D41
**Package:** `MODULE_PATH_TBD/llm`

---

## Overview

The `llm` package defines the provider-agnostic interface and all message types
that cross the provider boundary. Every provider-specific format (tool-use
blocks, function-call objects, thinking blocks, streaming chunk shapes) is
absorbed inside the adapter and exposed via the types defined here.

Shipped adapters: `llm/anthropic` and `llm/openai`. A shared conformance suite
in `llm/conformance/` runs against every adapter.

---

## `Provider` interface

```go
// Provider is the provider-agnostic interface over LLM adapters.
//
// Each method may perform network I/O. All methods take a context.Context
// as the first parameter; callers must respect context cancellation.
//
// Implementations must be safe for concurrent use. The orchestrator may
// call Complete or Stream from multiple goroutines on the same Provider
// instance.
//
// Stability: frozen-v1.0.
type Provider interface {
    // Complete sends a request to the LLM and blocks until the full response
    // is available. Use InvokeStream for streaming responses.
    //
    // Returns a classified TypedError on failure:
    //   - TransientLLMError for retryable failures (rate limit, 5xx, timeout).
    //   - PermanentLLMError for non-retryable failures (invalid request, 4xx).
    //
    // The orchestrator's retry policy (3x exponential backoff for transient
    // errors) operates at this boundary.
    Complete(ctx context.Context, req LLMRequest) (LLMResponse, error)

    // Stream sends a request to the LLM and returns a channel of stream
    // chunks. The channel is closed when the response is complete or when
    // an error occurs.
    //
    // The returned error is non-nil only for setup failures (auth, network
    // before first token). Errors occurring after the channel is open are
    // delivered as LLMStreamChunk with a non-nil Err field.
    //
    // The returned channel is closed by the provider adapter when the
    // response is complete. Callers must drain the channel to avoid leaks.
    //
    // Adapters that do not natively support streaming must implement Stream
    // by calling Complete and delivering a single chunk containing the
    // full response.
    Stream(ctx context.Context, req LLMRequest) (<-chan LLMStreamChunk, error)

    // Name returns the provider's canonical name (e.g., "anthropic", "openai").
    // Used in budget.PriceProvider lookups.
    Name() string

    // SupportsParallelToolCalls reports whether the provider can process
    // multiple tool calls returned in a single response concurrently.
    // The orchestrator uses this to gate parallel dispatch (D24).
    SupportsParallelToolCalls() bool

    // Capabilities returns a snapshot of the provider's capabilities at
    // construction time. The snapshot is immutable for the lifetime of the
    // provider instance.
    Capabilities() Capabilities
}
```

---

## Agnostic message types

```go
// Role identifies the author of a Message.
type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleSystem    Role = "system"
    RoleTool      Role = "tool"
)

// Message is a single turn in the conversation history.
// It contains one or more parts representing the turn content.
type Message struct {
    Role  Role
    Parts []MessagePart
}

// PartType distinguishes message content kinds.
type PartType string

const (
    PartTypeText      PartType = "text"
    PartTypeToolCall  PartType = "tool_call"
    PartTypeToolResult PartType = "tool_result"
    PartTypeImageURL  PartType = "image_url"
)

// MessagePart is a single content element within a Message.
// Exactly one of the content fields is non-zero, determined by Type.
type MessagePart struct {
    Type PartType

    // Text is the text content. Non-empty when Type == PartTypeText.
    Text string

    // ToolCall describes a tool invocation requested by the LLM.
    // Non-nil when Type == PartTypeToolCall.
    ToolCall *LLMToolCall

    // ToolResult carries the output of a tool invocation.
    // Non-nil when Type == PartTypeToolResult.
    ToolResult *LLMToolResult

    // ImageURL is a URL pointing to an image.
    // Non-empty when Type == PartTypeImageURL.
    ImageURL string
}
```

---

## Request and response types

```go
// LLMRequest is the provider-agnostic input to a single LLM call.
// All provider-specific parameters (sampling, penalties, extensions)
// are expressed through the ExtraParams escape hatch.
type LLMRequest struct {
    // Messages is the full conversation history for this call, including
    // any tool results from previous turns.
    Messages []Message

    // Model is the provider-specific model identifier.
    Model string

    // Tools is the list of tools the LLM may call in this turn.
    Tools []ToolDefinition

    // SystemPrompt is the system-level instruction.
    // Empty string means no system prompt.
    SystemPrompt string

    // MaxTokens limits the response length. Zero means provider default.
    MaxTokens int

    // Temperature controls sampling randomness. Zero uses provider default.
    Temperature float64

    // ExtraParams is an opaque key-value map for provider-specific
    // parameters not covered by the standard fields. The orchestrator
    // forwards ExtraParams to the adapter unchanged. The adapter is
    // responsible for interpreting or ignoring entries.
    ExtraParams map[string]any
}

// LLMResponse is the provider-agnostic output from a single LLM call.
type LLMResponse struct {
    // Message is the assistant's response, containing text parts and/or
    // tool-call parts.
    Message Message

    // StopReason identifies why the LLM stopped generating.
    StopReason StopReason

    // Usage reports token consumption for budget accounting.
    Usage TokenUsage
}

// StopReason identifies the completion reason for an LLM response.
type StopReason string

const (
    StopReasonEndTurn      StopReason = "end_turn"
    StopReasonToolUse      StopReason = "tool_use"
    StopReasonMaxTokens    StopReason = "max_tokens"
    StopReasonStopSequence StopReason = "stop_sequence"
)

// TokenUsage reports token counts for a single LLM call.
// Used by the orchestrator to record against budget.Guard.
type TokenUsage struct {
    InputTokens  int64
    OutputTokens int64
    // CachedInputTokens is the number of input tokens served from cache.
    // Zero if the provider does not support or report prompt caching.
    CachedInputTokens int64
}

// LLMStreamChunk is a single chunk in a streaming LLM response.
// The stream channel emits one chunk per token or per tool-call delta.
type LLMStreamChunk struct {
    // Delta is the incremental text content for this chunk.
    // Empty for non-text chunks.
    Delta string

    // ToolCallDelta carries a partial tool call for this chunk.
    // Non-nil when the LLM is streaming a tool-call argument payload.
    ToolCallDelta *LLMToolCallDelta

    // Final is true on the last chunk. The Final chunk carries the
    // complete LLMResponse summary (usage, stop reason).
    Final bool

    // Response is populated only on the Final chunk.
    Response *LLMResponse

    // Err is non-nil if an error occurred after streaming began.
    // When non-nil, no further chunks follow and the channel is closed.
    Err error
}
```

---

## Tool types

```go
// ToolDefinition describes a tool the LLM may call.
// The orchestrator passes tool definitions from InvocationRequest.Tools
// directly to LLMRequest.Tools.
type ToolDefinition struct {
    // Name is the tool identifier the LLM uses in tool-call requests.
    // Must match the Name returned by tools.Invoker.
    Name string

    // Description is a human-readable description of the tool's purpose.
    // LLMs use this for tool selection. Required.
    Description string

    // InputSchema is the JSON Schema for the tool's input parameters,
    // serialized as a raw JSON message (json.RawMessage-compatible []byte).
    // The framework forwards this to the provider without interpretation.
    InputSchema []byte
}

// LLMToolCall is a single tool invocation requested by the LLM.
// Corresponds to a MessagePart with Type == PartTypeToolCall.
type LLMToolCall struct {
    // CallID is the provider-assigned unique ID for this call.
    // Used to correlate LLMToolResult with LLMToolCall in multi-tool turns.
    CallID string

    // Name is the tool name from ToolDefinition.Name.
    Name string

    // ArgumentsJSON is the tool's input arguments as a raw JSON object.
    // The framework forwards this to tools.ToolCall.ArgumentsJSON.
    ArgumentsJSON []byte
}

// LLMToolCallDelta is an incremental update to a tool call in a streaming
// response. Arguments are delivered in chunks and must be concatenated.
type LLMToolCallDelta struct {
    CallID        string
    Name          string
    ArgumentsDelta string
}

// LLMToolResult carries the output of a tool invocation back to the LLM.
// Corresponds to a MessagePart with Type == PartTypeToolResult.
type LLMToolResult struct {
    // CallID matches LLMToolCall.CallID.
    CallID string

    // Content is the tool's output as a string. May be structured JSON
    // or plain text; the LLM interprets it based on context.
    Content string

    // IsError indicates that the tool returned an error state.
    // The LLM will be informed the tool call failed.
    IsError bool
}
```

---

## Provider capabilities

```go
// Capabilities is a snapshot of a provider's supported features.
// Returned by Provider.Capabilities(); immutable for the provider's lifetime.
type Capabilities struct {
    // SupportsStreaming reports whether the provider's LLM API supports
    // token-by-token streaming.
    SupportsStreaming bool

    // SupportsParallelToolCalls reports whether the provider returns
    // multiple tool calls in a single response. Mirrors
    // Provider.SupportsParallelToolCalls().
    SupportsParallelToolCalls bool

    // SupportsSystemPrompt reports whether the provider accepts a
    // system-level prompt separate from the message list.
    SupportsSystemPrompt bool

    // SupportedStopReasons is the set of StopReason values this provider
    // may return. Providers that do not support a stop reason map it to
    // the nearest equivalent.
    SupportedStopReasons []StopReason

    // MaxContextTokens is the provider-reported context window size.
    // Zero means unreported or unknown.
    MaxContextTokens int64
}
```

---

## Default / null implementation

```go
// EchoProvider is the in-memory mock shipped for consumer tests and
// the conformance suite. It echoes a caller-configured response for
// every Complete or Stream call without making network requests.
//
// EchoProvider is safe for concurrent use.
// Package: MODULE_PATH_TBD/llm/mock
type EchoProvider struct { /* unexported */ }

// NewEchoProvider constructs an EchoProvider that returns resp for every
// call. If resp is zero-valued, EchoProvider returns an empty assistant
// message with StopReasonEndTurn.
func NewEchoProvider(resp LLMResponse) *EchoProvider
```

---

## Concurrency contract

All `Provider` implementations must be safe for concurrent use. The
orchestrator may call `Complete` or `Stream` from the loop goroutine of any
concurrent invocation. The shipped adapters (`anthropic.Provider`,
`openai.Provider`) satisfy this requirement via stateless HTTP clients.

---

## Conformance contract

Every implementation of `Provider` must pass the shared test suite in
`MODULE_PATH_TBD/llm/conformance`. The suite verifies:

1. `Complete` returns a well-formed `LLMResponse` for each test case.
2. `Stream` returns a channel that emits chunks in order, closes cleanly,
   and delivers usage totals on the Final chunk.
3. `Name()` is non-empty and stable across calls.
4. `Capabilities()` is consistent with observed adapter behavior.
5. Context cancellation is respected: a cancelled context causes `Complete`
   or `Stream` to return a `TransientLLMError` wrapping `context.Canceled`.
