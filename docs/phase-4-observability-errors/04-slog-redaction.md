# Phase 4 — slog Integration and Redaction

**Decisions:** D58
**Cross-references:** `06-filter-event-mapping.md` (filter-event log levels),
`05-error-event-mapping.md` (error log levels), Phase 3 `08-telemetry-interfaces.md`
(emitter interface), Phase 5 (credential isolation)

---

## 1. Overview

The framework emits structured log records alongside lifecycle events. It does
not own the global slog logger. All log output goes through a caller-provided
`slog.Handler`, optionally wrapped by the framework's `RedactingHandler` to
strip sensitive material before records are forwarded.

**What the framework logs:** lifecycle events, state transitions, error
classifications, filter decisions, and operational warnings. Never: raw message
content, raw tool output, or credential values.

**What the framework never logs:** credential values (API keys, tokens,
signing keys), raw LLM request/response content, raw tool output, or any
field whose value is a `map[string]string` from `AttributeEnricher` without
inspection (enricher attributes are loggable individually but not dumped as a
blob).

---

## 2. Log-level mapping per EventType

The following table maps each `EventType` to the slog level at which the
framework emits a structured log record when the event is processed.

### 2.1 Non-terminal events

| `EventType` | slog level | Rationale |
|---|---|---|
| `invocation.started` | `DEBUG` | High-frequency in production; debug only |
| `invocation.initialized` | `DEBUG` | Config resolution; debug only |
| `prehook.started` | `DEBUG` | Hook entry; debug only |
| `prehook.completed` | `DEBUG` | Hook completion with verdict; debug only |
| `llmcall.started` | `DEBUG` | LLM call entry; debug only |
| `llmcall.completed` | `DEBUG` | LLM call completion; debug only |
| `tooldecision.started` | `DEBUG` | Routing decision; debug only |
| `toolcall.started` | `INFO` | Tool invocations are operationally significant |
| `toolcall.completed` | `DEBUG` | Completion detail; debug only |
| `posttoolfilter.started` | `DEBUG` | Filter entry; debug only |
| `posttoolfilter.completed` | `DEBUG` | Filter completion; debug only |
| `llmcontinuation.started` | `DEBUG` | Loop re-entry; debug only |
| `posthook.started` | `DEBUG` | Hook entry; debug only |
| `posthook.completed` | `DEBUG` | Hook completion; debug only |

### 2.2 Content-analysis events

| `EventType` | slog level | Rationale |
|---|---|---|
| `filter.pii_redacted` | `INFO` | PII detection is operationally significant; callers need to track redaction rates |
| `filter.prompt_injection_suspected` | `WARN` | Potential security signal; always surfaced at WARN |

### 2.3 Terminal events

| `EventType` | slog level | Rationale |
|---|---|---|
| `invocation.completed` | `INFO` | Successful completion is operationally significant |
| `invocation.failed` | `ERROR` | Failure requires attention |
| `invocation.cancelled` | `INFO` | Cancellation is often intentional; INFO not WARN |
| `budget.exceeded` | `WARN` | Budget breach is abnormal but not necessarily an error |
| `approval.required` | `INFO` | Deliberate pause; not an error |

### 2.4 VerdictLog event (D64)

| Condition | slog level | Rationale |
|---|---|---|
| `prehook.completed` or `posthook.completed` with `AuditNote` non-empty | `INFO` | Audit notes are operationally significant by definition |

---

## 3. Structured log fields

Each log record emitted by the framework includes a base field set plus
event-specific fields.

### 3.1 Base fields (on every record)

| Field key | Type | Source |
|---|---|---|
| `praxis.invocation_id` | string | Per-invocation ID |
| `praxis.event_type` | string | `EventType` string value |
| `praxis.state` | string | Current state machine state name |

### 3.2 Event-specific fields

**`toolcall.started`, `toolcall.completed`:**
- `praxis.tool_name` string
- `praxis.tool_call_id` string

**`filter.pii_redacted`:**
- `praxis.filter_field` string — `FilterDecision.Field` (the dot-path)
- `praxis.filter_reason` string — `FilterDecision.Reason` (truncated to 256 chars)
- `praxis.filter_phase` string — `"pre_llm"` or `"post_tool"`

**`filter.prompt_injection_suspected`:**
- `praxis.filter_field` string
- `praxis.filter_reason` string (truncated to 256 chars)
- `praxis.filter_action` string — `"log"` or `"block"`
- `praxis.filter_phase` string

**`invocation.failed`, `invocation.cancelled`, `budget.exceeded`:**
- `praxis.error_kind` string
- `praxis.error_message` string — `TypedError.Error()` (subject to redaction)

**`budget.exceeded`:**
- `praxis.exceeded_dimension` string — `BudgetSnapshot.ExceededDimension`

**`prehook.completed`, `posthook.completed` with `AuditNote`:**
- `praxis.verdict` string
- `praxis.audit_note` string — `Decision.Reason` (truncated to 512 chars)
- `praxis.hook_phase` string

**Reasoning for truncation limits:** `FilterDecision.Reason` and
`Decision.Reason` are human-written strings. Unbounded strings in structured
log fields can exhaust log indexer limits and produce large log records.
The 256/512 char truncation is applied at the log call site; the original
field value is not modified.

---

## 4. What is never logged

The following material must never appear in any log record emitted by the
framework, regardless of log level:

1. **Credential values:** API keys, bearer tokens, JWT material, signing keys.
   Phase 5 enforces this structurally — credential values are not reachable at
   log call sites in framework code. This is belt-and-suspenders documentation.

2. **Raw LLM request content:** the `messages []llm.Message` slice, including
   system prompt, user turns, and assistant turns. These may contain
   user-provided sensitive material (PII, proprietary data).

3. **Raw LLM response content:** the completion text from the LLM provider.
   Same reason as above.

4. **Raw tool output:** the `tools.ToolResult.Content` field. Tool outputs are
   untrusted and may contain prompt injection payloads or sensitive data.

5. **Raw `AttributeEnricher` output as a blob:** the `EnricherAttributes`
   map is not logged as a `slog.Attr` group. Individual enricher attribute
   values may contain PII (user IDs, names). Callers who want enricher
   attributes in logs implement their own `LifecycleEventEmitter`.

6. **`PolicyInput` content beyond field names:** `PolicyInput.SystemPrompt`,
   `PolicyInput.Messages`, `PolicyInput.ToolResult`, `PolicyInput.LLMResponse`
   are never logged. Only the phase name and verdict are logged.

7. **`ApprovalSnapshot.Messages`:** the conversation history in an approval
   snapshot is not logged. The snapshot is referenced by invocation ID only.

---

## 5. `RedactingHandler` contract

The `RedactingHandler` is an opt-in `slog.Handler` wrapper that the framework
provides in the `telemetry/slog` sub-package (D58). It wraps a caller-provided
`slog.Handler`
and intercepts `Handle` calls to strip or redact known-sensitive field keys
before forwarding.

### 5.1 Redaction rules applied by `RedactingHandler`

The handler operates on a deny-list of field keys. Records containing any of
these keys have the key's value replaced with `[REDACTED]` before the record
is forwarded to the inner handler.

**Always-redacted keys (framework-defined deny-list):**

| Key pattern | Rationale |
|---|---|
| `praxis.credential.*` | Any attribute key under this prefix is a credential value |
| `praxis.raw_content` | Reserved for any future raw-content field |
| Any key with suffix `_secret` | Convention for secret values |
| Any key with suffix `_token` | Convention for token values |
| Any key with suffix `_key` | Convention for key material |
| Any key with suffix `_password` | Convention for passwords |

**Note on the deny-list approach:** the framework cannot enumerate every
possible sensitive key that a caller's `AttributeEnricher` might inject into
a log record (since enricher attributes are not logged directly, this is mainly
for defense in depth on caller-constructed log records). The deny-list is
framework-defined and not extensible via the public API; callers who need
custom redaction implement their own `slog.Handler`.

### 5.2 `RedactingHandler` interface

```go
// NewRedactingHandler wraps inner with the framework's redaction rules.
// Records that contain deny-listed field keys have those values replaced
// with "[REDACTED]" before being forwarded to inner.
//
// The redaction is applied recursively to all slog.Attr values in the
// record's attribute list, including nested Groups.
//
// inner must not be nil. If inner is nil, NewRedactingHandler panics.
//
// NewRedactingHandler does not modify the slog.Default logger. Callers
// configure their logging pipeline independently.
//
// Package: MODULE_PATH_TBD/telemetry/slog
func NewRedactingHandler(inner slog.Handler) slog.Handler
```

### 5.3 Framework logger construction

The framework's internal logger is constructed at orchestrator build time from
a caller-provided `slog.Handler`:

```go
// WithSlogHandler sets the slog.Handler for framework-internal log records.
// If not called, the framework uses slog.Default().Handler() wrapped in
// NewRedactingHandler.
//
// The caller is responsible for wrapping handler with NewRedactingHandler
// if they want the framework's redaction rules applied. The framework does
// not automatically wrap a caller-provided handler.
func WithSlogHandler(h slog.Handler) OrchestratorOption
```

**Rationale for not auto-wrapping:** if the framework automatically wraps the
caller's handler with `RedactingHandler`, the caller loses visibility into what
is being redacted. The framework's default behavior (using
`slog.Default().Handler()` wrapped in `RedactingHandler`) is safe. Callers who
pass their own handler take responsibility for redaction.

---

## 6. Log call sites in framework code

Framework code emits log records at exactly these call sites:

| Call site | Level | Trigger |
|---|---|---|
| `LifecycleEventEmitter.Emit` returns error | `WARN` | Emitter failure (non-fatal per D22) |
| State transition to terminal state | see §2.3 | Terminal event emission |
| `FilterDecision` with non-`Pass` action | see §2.2 | Content-analysis event emission |
| `VerdictLog` decision | `INFO` | Audit note emission (D64) |
| Illegal state transition attempt | `ERROR` | State machine invariant violation |
| Enricher attribute key collision | `DEBUG` | Framework key protection |

The framework does not emit a log record for every state transition. It emits
records only for operationally significant transitions (as defined by the
log-level mapping in §2) and for warning/error conditions.

---

## 7. slog dependency note

The framework's slog integration requires Go 1.21+ (`log/slog` was added in
Go 1.21). The project's minimum Go version is 1.23+ (CLAUDE.md), so `log/slog`
is available without a separate dependency. No third-party slog packages are
imported by the framework.
