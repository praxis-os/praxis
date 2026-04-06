# Phase 5 — Trust Boundaries

**Decisions:** D77, D78, D79
**Cross-references:** Phase 3 `04-hooks-and-filters.md` (filter interfaces),
Phase 4 `04-slog-redaction.md` (RedactingHandler), Phase 4 D58 (deny-list),
Phase 3 `09-credentials-and-identity.md` (credential contract)

---

## 1. Untrusted tool output model (D77)

### 1.1 Trust classification

`ToolResult.Content` is **untrusted by contract**. The framework treats all
content returned by `tools.Invoker.Invoke` as potentially hostile from the
moment it is received. This is not a probabilistic risk assessment — it is a
design invariant.

Concretely, tool output:
- **May contain prompt injection payloads.** A tool backed by an external
  service (web search, email fetch, document retrieval) may return content
  crafted to subvert the LLM's behavior on the next turn.
- **May contain PII or sensitive data.** A database query tool may return
  personal information that should not propagate to the LLM conversation
  history.
- **May be malformed.** A buggy or compromised tool may return content that
  does not conform to the expected schema.
- **May reference credentials or signing material.** A tool that reads
  environment variables or configuration files may inadvertently include
  sensitive material in its output.

The framework does not claim to detect any of these conditions. It provides
the `PostToolFilter` seam where a caller-supplied filter makes these
determinations.

### 1.2 Framework responsibilities

The framework's responsibilities regarding untrusted tool output are exactly:

1. **Pass `ToolResult` to `PostToolFilter` before appending to conversation
   history.** No `ToolResult` produced by a tool invocation is injected into
   the conversation history without first passing through the filter chain.
   This is a structural guarantee, not a documentation recommendation.

2. **Honor `FilterActionBlock` decisions.** A `Block` decision immediately
   halts the invocation and routes it to `Failed` with a `PolicyDeniedError`.
   The tool result is not appended to the conversation history.

3. **Use only the `filtered` return value.** After `PostToolFilter.Filter`
   returns, the orchestrator uses only the `filtered tools.ToolResult` value
   returned by the filter. The original unfiltered `ToolResult` is not retained
   in the orchestrator's per-invocation state after the filter call returns.

4. **Emit `filter.prompt_injection_suspected` at WARN.** If the filter returns
   a `FilterDecision` with a `FilterActionLog` or `FilterActionBlock` and a
   reason containing an injection signal term (Phase 4 D59/D66), the framework
   emits a lifecycle event at WARN level.

5. **Never log `ToolResult.Content`.** The unfiltered tool content is not
   logged by the framework at any level. The filtered content is also not
   logged. Only metadata (tool name, tool call ID, filter decision summary)
   is logged.

### 1.3 Filter implementor responsibilities

The `PostToolFilter` implementor is responsible for:

1. **Prompt injection detection.** The filter receives the raw, potentially
   hostile tool output. Detection logic (pattern matching, classifier calls,
   sandboxed parsing) is the filter's domain.

2. **PII detection and redaction.** The filter is the appropriate place to
   identify and redact personally identifiable information before it enters
   the conversation history.

3. **Content sanitisation.** Normalising encoding, stripping unexpected
   markup, truncating oversized output.

4. **Returning the appropriate `FilterAction`.** The filter communicates its
   conclusions through `FilterDecision` values. `FilterActionBlock` is the
   mechanism for rejecting content that must not reach the LLM.

5. **Correctness under adversarial input.** Because the input to
   `PostToolFilter.Filter` is untrusted, the filter itself must be robust
   to adversarial content — e.g., overly long strings, nested encodings,
   Unicode tricks. Panics in `PostToolFilter` are recovered by the
   orchestrator (see panic recovery model below) and surfaced as errors,
   but a panicking filter transitions the invocation to `Failed`. Callers
   should ensure their filter implementations are panic-safe.

### 1.4 Panic recovery in filters and hooks

Hook and filter implementations are caller-supplied. The orchestrator recovers
from panics in the following call sites:

- `PolicyHook.Evaluate`
- `PreLLMFilter.Filter`
- `PostToolFilter.Filter`

Recovery uses a deferred `recover()` call. If a panic is recovered, the
orchestrator wraps it in a `SystemError` with the recovered value included in
the error message (via `fmt.Sprintf("%v", r)`) and routes the invocation to
`Failed`. The recovered value is not a credential or identity token; it is
typically a nil-pointer dereference or assertion failure from the hook/filter.

The recovered value is included in the error message because it is diagnostic
information that helps callers debug their hook/filter implementations. The
error is classified as `ErrorKindSystem` and is not retried.

---

## 2. Filter trust boundary classification (D78)

### 2.1 The two filter positions

The framework runs two filter types at different points in the invocation loop:

| Filter | Position | Input source | Trust classification |
|---|---|---|---|
| `PreLLMFilter` | Before each LLM call | Caller-constructed messages + previously filtered tool results | Trust-boundary-internal |
| `PostToolFilter` | After each tool invocation | Raw `tools.Invoker` output | Trust-boundary-crossing |

**Trust-boundary-internal** means the input to the filter was produced or
validated within the trust boundary: the caller constructed the initial
messages, and any tool results have already passed through `PostToolFilter`.
The filter's job is additional policy enforcement (PII removal, content
shaping), not adversarial content handling.

**Trust-boundary-crossing** means the input to the filter was produced by an
external system (the tool) and may be hostile. This is a qualitatively
different threat posture.

### 2.2 Error handling asymmetry

This asymmetry drives different error-handling and telemetry rules:

**`PostToolFilter` error (not a block — a filter-internal failure):**
- Severity: `ERROR` in the framework's logger.
- Rationale: a `PostToolFilter` that errors has potentially allowed untrusted
  content to proceed (or has halted processing at the most sensitive boundary).
  This is a security-significant event, not merely an operational failure.
- Route: `Failed` with a `SystemError`.
- Obligation: the caller must investigate why their `PostToolFilter`
  implementation failed.

**`PreLLMFilter` error:**
- Severity: `WARN` in the framework's logger.
- Rationale: the input was already trusted-origin. A filter failure here is
  an operational problem (misconfigured filter, timeout), not a security breach.
- Route: `Failed` with a `SystemError`.

Both filter types route to `Failed` on error. The distinction is in telemetry
and in the caller's obligation to treat a `PostToolFilter` error as a potential
security incident.

### 2.3 Telemetry at the `PostToolFilter` boundary

`PostToolFilter` block decisions emit `filter.prompt_injection_suspected` at
`WARN` level (Phase 4 D58, D59). This is unchanged by Phase 5.

`PostToolFilter` errors (not blocks) emit at `ERROR` level via the framework's
structured logger with field `praxis.filter_phase: "post_tool"` and
`praxis.error_kind: "system"`.

`PreLLMFilter` errors emit at `WARN` level with `praxis.filter_phase: "pre_llm"`.

### 2.4 Parallel tool dispatch and filter order

Under parallel tool dispatch (Phase 2 D24), multiple tool calls execute
concurrently. After all tool calls in a batch complete, the results are
filtered **sequentially** through `PostToolFilter`, one result at a time, in
the same goroutine that drives the invocation loop. This means:

- `PostToolFilter` does not need to be re-entrant per batch.
- The filter sees tool results in a deterministic order (the same order as
  the LLM's `tool_use` blocks).
- A `Block` decision on the first result in a batch prevents the remaining
  results from being filtered or injected.

---

## 3. Credential isolation at observability boundaries

### 3.1 Span attributes

**Invariant:** no span attribute set by the framework contains a credential
value.

Credential values are fetched in the tool-call goroutine. The `Credential`
interface is never stored in `tools.InvocationContext` (which has no
`Credential` field — Phase 3 D36), the orchestrator's shared invocation
state, or any channel. It is not referenced at span-attribute set call sites.

The `praxis.toolcall` span (Phase 4 D53) carries attributes: `praxis.tool_name`
and `praxis.tool_call_id`. Neither references the credential value. No
framework span carries a `praxis.credential.*` attribute.

**Defence in depth:** `RedactingHandler` redacts `praxis.credential.*` prefix
keys. If a future code change accidentally logs a field under this prefix, the
redaction handler strips the value before it reaches the backing log store.
There is no equivalent OTel-span-level redaction (OTel attribute filtering is
caller-level); the structural invariant is the primary control.

### 3.2 Log records

**Invariant:** no log record emitted by the framework contains a credential
value.

Structural enforcement: credential values are not reachable at framework log
call sites because:
1. The `Credential` is fetched and used within the tool-call goroutine.
2. Framework log call sites (Phase 4 §6) do not receive `Credential` objects.
3. The `Credential` interface is not passed to the `LifecycleEventEmitter`.

**Defence in depth:** `RedactingHandler` deny-list (D58 + D79 amendments)
strips any field matching `praxis.credential.*`, `_token`, `_key`, `_password`,
`_secret`, `praxis.signed_identity`, `_jwt`.

### 3.3 `InvocationEvent` fields

**Invariant:** no `InvocationEvent` field carries a credential value or signed
identity token.

The `InvocationEvent` struct (Phase 3 D32, amended by D65) carries:
- `InvocationID`, `Type`, `State`, `Timestamp`, `BudgetSnapshot`, etc.
- `EnricherAttributes`, `FilterPhase`, `FilterField`, `FilterReason`,
  `FilterAction`, `AuditNote`.

None of these fields are populated from `Credential.Value()` or
`InvocationContext.SignedIdentity`. The structural absence of credential-
carrying fields in `InvocationEvent` is the enforcement mechanism.

`tools.InvocationContext.SignedIdentity` is passed to the `tools.Invoker` but
is never propagated to `InvocationEvent`. Callers who want to observe which
identity tokens were used must implement a custom `LifecycleEventEmitter` that
reads from context values they control.

### 3.4 Error messages

**Invariant:** `error.Error()` strings produced by the framework do not embed
credential values or signed identity tokens.

Framework-produced errors (Phase 3 `07-errors-and-classifier.md`) embed:
- The `ErrorKind` string.
- A human-readable description constructed from static strings and non-secret
  runtime values (e.g., state names, tool names, model identifiers).

`CredentialRef.Name` (a logical credential identifier, e.g., `"stripe-api-key"`)
is a non-secret value; it may appear in error messages where it aids
diagnosis. `CredentialRef.Scope` follows the same rule. `Credential.Value()`
— the actual secret bytes — never appears in an error message.

If a `Resolver.Fetch` call fails, the error from the resolver is wrapped in a
`ToolError` or `SystemError`. The wrapped error may contain the resolver's
error message (e.g., vault connection details), but not the credential value.

---

## 4. RedactingHandler deny-list amendments (D79)

### 4.1 New entries

Two entries are added to the deny-list established in Phase 4 D58:

**`praxis.signed_identity`**

The `tools.InvocationContext.SignedIdentity` field carries a bearer JWT. If a
future code path (in framework code or in a caller-constructed log record)
logs this field under the key `praxis.signed_identity`, the JWT must be
redacted. Bearer JWTs are credentials: possession of a valid JWT authorises
tool calls on behalf of the agent.

**Any key with suffix `_jwt`**

By convention, fields suffixed with `_jwt` carry JWT values. This suffix
pattern covers caller-constructed log records that follow the convention,
providing defence in depth against inadvertent JWT leakage.

### 4.2 Complete deny-list (post-amendment)

| Key pattern | Phase introduced |
|---|---|
| `praxis.credential.*` | Phase 4 D58 |
| `praxis.raw_content` | Phase 4 D58 |
| Any key with suffix `_secret` | Phase 4 D58 |
| Any key with suffix `_token` | Phase 4 D58 |
| Any key with suffix `_key` | Phase 4 D58 |
| Any key with suffix `_password` | Phase 4 D58 |
| `praxis.signed_identity` | Phase 5 D79 |
| Any key with suffix `_jwt` | Phase 5 D79 |

### 4.3 What the deny-list is not

The deny-list is a defence-in-depth mechanism, not the primary security
control. The primary controls are:
- Structural: credential values are not reachable at log call sites.
- Code review: framework code never passes credential or identity values
  to any log call.

The deny-list catches mistakes. It does not replace the structural and
code-review controls.

The deny-list is also not extensible via the public API (Phase 4 D58 §5.1
rationale). Callers who need custom deny-list entries implement their own
`slog.Handler`.

---

## 5. Open issues

### OI-1: Private key zeroing on `Signer` discard

The `ed25519.PrivateKey` held by `NewEd25519Signer` is not zeroed when the
`Signer` is garbage collected. Go finalizers are not guaranteed to run.

**Impact.** The private key occupies memory for the lifetime of the `Signer`
struct plus an indeterminate GC window. An adversary who can read process
memory could recover the key.

**Current mitigation.** This is documented in D74 §5.5. Callers with strict
key-material requirements use KMS/HSM-backed `Signer` implementations.

**Resolution path.** A v1.x minor release could add a `Close() error` method
to `Signer` (as an optional interface, not on the `Signer` interface itself —
that would be a breaking change) that callers call when done with the Signer.
Alternatively, callers can implement a wrapper that zeros the key on an
explicit `Shutdown` call. This is deferred post-v1.0.

### OI-2: `AttributeEnricher` output as a log-injection vector

`InvocationEvent.EnricherAttributes` (Phase 4 D60) is a `map[string]string`
controlled by the caller. The framework does not log this map as a blob (Phase 4
D58 §4), but callers who implement `LifecycleEventEmitter` and log enricher
attributes are responsible for their own redaction.

**Impact.** If a caller's `AttributeEnricher` injects a value that is also a
credential (e.g., a user's session token as a `user.session` attribute),
the framework has no mechanism to detect or redact it. The `RedactingHandler`
only redacts by key pattern, not by value content.

**Current mitigation.** The framework's contract states that the caller takes
responsibility for enricher attribute values (Phase 3 trust model). This is
documented.

**Resolution path.** No framework-level resolution is planned for v1.0. The
threat is caller-misconfiguration, not a framework design flaw.
