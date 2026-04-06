# Phase 5 — Security Invariants Summary

**Decision:** D80
**Cross-references:** D45, D46 (Phase 3), D58 (Phase 4), D67–D79 (Phase 5)

---

## Overview

This document enumerates all security invariants that the praxis framework
enforces. Each invariant is assigned a short identifier, a one-sentence
statement, an enforcement mechanism, and a decision cross-reference.

Invariants are grouped into four categories:
- **C-series:** Credential isolation
- **I-series:** Identity signing
- **T-series:** Trust boundaries
- **O-series:** Observability safety

An "enforcement mechanism" is one of:
- **Structural:** The Go type system, interface design, or data-flow constraints
  make violation impossible without modifying framework internals.
- **Runtime check:** The framework performs an explicit check and returns an
  error or panics on violation.
- **Code-review invariant:** Enforced by human review of framework code against
  a documented rule. Violations are possible through future code changes but
  are detectable in review.
- **Documentation invariant:** The rule is stated in the public contract.
  Callers who violate it do so at their own risk; the framework cannot enforce
  the rule at compile time or runtime.

---

## Credential isolation (C-series)

### C1 — Credential zeroing on close

**Statement:** `Credential.Close()` must overwrite the secret material with
zeros before returning, and the zeroing must use `runtime.KeepAlive` to prevent
dead-store elision.

**Enforcement mechanism:** Code-review invariant for framework-provided
`Credential` implementations. Documentation invariant for caller-provided
implementations. `credentials.ZeroBytes` (D68) provides the canonical correct
implementation for callers to use.

**Decision:** D67, D68

---

### C2 — Per-call credential fetch, no caching

**Statement:** The framework calls `credentials.Resolver.Fetch` once per tool
call immediately before tool dispatch; it never caches credential values
between calls or between invocations.

**Enforcement mechanism:** Structural. The `Credential` object is created in
the tool-call goroutine, used once, and `Close()`d in a deferred call within
the same goroutine. There is no credential store, cache, or session in the
framework.

**Decision:** D45 (Phase 3)

---

### C3 — Credential goroutine scope isolation

**Statement:** The `Credential` value returned by `Resolver.Fetch` is used
only within the tool-call goroutine that fetched it; it is not passed to
other goroutines via channels, shared state, or closures.

**Enforcement mechanism:** Structural. The `Credential` is fetched and used
within the tool-call goroutine scope. `tools.InvocationContext` (Phase 3 D36)
has no `Credential` field — the credential does not leave the goroutine via
the framework's public API. The framework does not store the `Credential` in
any field accessible from other goroutines.

**Decision:** D45 (Phase 3)

---

### C4 — Credential value absent from span attributes

**Statement:** No OTel span attribute set by the framework contains a
`Credential.Value()` byte slice or any string derived from it.

**Enforcement mechanism:** Code-review invariant. Structural backstop: the
`Credential` object is not in scope at span-attribute set call sites in the
framework's orchestration loop.

**Decision:** D45 (Phase 3), D67 (Phase 5)

---

### C5 — Credential value absent from log records

**Statement:** No log record emitted by the framework contains a credential
value; `Credential.Value()` is not passed to any `slog.Logger` call.

**Enforcement mechanism:** Code-review invariant (primary). Structural backstop:
`Credential` objects are not reachable at log call sites in the invocation
loop. Defence in depth: `RedactingHandler` (D58) strips `praxis.credential.*`
prefix keys.

**Decision:** D45 (Phase 3), D58 (Phase 4)

---

### C6 — Credential value absent from error messages

**Statement:** `error.Error()` strings produced by the framework do not embed
`Credential.Value()` bytes.

**Enforcement mechanism:** Code-review invariant. Framework error constructors
use static strings and non-secret runtime values (`ErrorKind`, state names,
tool names). `Credential.Value()` is never passed to an error format string.

**Decision:** D45 (Phase 3), D67 (Phase 5)

---

### C7 — Credential value absent from `InvocationEvent` fields

**Statement:** No `InvocationEvent` field is populated from a credential value.

**Enforcement mechanism:** Structural. The `InvocationEvent` struct (Phase 3
D32, amended D65) has no field whose type is `[]byte` or whose documented
source is `Credential.Value()`. The struct is populated from state machine
metadata, not from runtime credential material.

**Decision:** D45 (Phase 3)

---

### C8 — Soft-cancel credential resolution is bounded

**Statement:** During the 500ms soft-cancel grace window, `Resolver.Fetch`
receives a `context.WithoutCancel`-derived context with a 500ms timeout, so
that credential resolution is not hard-cancelled but is still time-bounded.

**Enforcement mechanism:** Structural. The `credentialFetchCtx` function
(D69) derives the fetch context. It is called unconditionally on the
credential-fetch path; the soft-cancel detection is in the orchestrator loop,
not in the `Resolver`.

**Decision:** D21 (Phase 2), D69 (Phase 5)

---

## Identity signing (I-series)

### I1 — Tokens are per-call and short-lived

**Statement:** Each `Sign` call produces a distinct token with a fresh `jti`,
`iat`, and `exp`; tokens are not cached or reused across calls.

**Enforcement mechanism:** Structural (for the reference `Ed25519Signer`).
`NewEd25519Signer.Sign` generates a new UUIDv7 `jti` and reads `time.Now()`
on each invocation. There is no token cache in the reference implementation.

**Decision:** D72, D73

---

### I2 — Token lifetime is bounded

**Statement:** `NewEd25519Signer` rejects token lifetimes below 5 seconds or
above 300 seconds at construction time.

**Enforcement mechanism:** Runtime check. `NewEd25519Signer` returns a non-nil
error if `WithTokenLifetime` receives an out-of-range value.

**Decision:** D72

---

### I3 — Mandatory claims are not overridable by callers

**Statement:** Caller-supplied extra claims via `WithExtraClaims` cannot
override the mandatory registered claims (`iss`, `sub`, `exp`, `iat`, `jti`)
or the mandatory custom claims (`praxis.invocation_id`, `praxis.tool_name`).

**Enforcement mechanism:** Structural. The reference implementation merges
extra claims first and then overwrites with mandatory claims. Mandatory claim
keys are never read from the extra claims map.

**Decision:** D71

---

### I4 — Signing uses only stdlib cryptography

**Statement:** `NewEd25519Signer` uses only `crypto/ed25519`, `encoding/json`,
`encoding/base64`, and `crypto/rand` for token construction. No external JWT
library is imported.

**Enforcement mechanism:** Code-review invariant. The `identity` package's
import list must not include any third-party JWT, JOSE, or cryptography package.

**Decision:** D73

---

### I5 — `NullSigner` is the safe default

**Statement:** When no `Signer` is configured, `NullSigner` returns an empty
string without error; no unsigned or malformed token is produced.

**Enforcement mechanism:** Structural. `NullSigner` is the zero-wiring default
(Phase 3 D12). It cannot produce a token by construction.

**Decision:** D46 (Phase 3)

---

### I6 — Signed identity is absent from log records

**Statement:** The signed identity token (`InvocationContext.SignedIdentity`)
is not logged by the framework at any level.

**Enforcement mechanism:** Code-review invariant (primary). Defence in depth:
`RedactingHandler` deny-list includes `praxis.signed_identity` (D79).

**Decision:** D79

---

## Trust boundaries (T-series)

### T1 — All tool output passes through `PostToolFilter` before history injection

**Statement:** No `ToolResult` produced by `tools.Invoker.Invoke` is appended
to the conversation history without passing through the configured
`PostToolFilter` chain.

**Enforcement mechanism:** Structural. The invocation loop's state machine
transitions from `ToolCall` to `PostToolFilter` before `LLMContinuation`. The
conversation-history append call is in `LLMContinuation`, after filtering.
There is no code path that skips `PostToolFilter`.

**Decision:** D77

---

### T2 — `FilterActionBlock` prevents history injection

**Statement:** A `FilterActionBlock` decision from `PostToolFilter` immediately
routes the invocation to `Failed` before the tool result is appended to the
conversation history.

**Enforcement mechanism:** Structural. The orchestrator checks filter decisions
before the history append. A `Block` decision triggers a state transition to
`Failed`; the history append is never reached.

**Decision:** D77

---

### T3 — Only the filtered tool result enters conversation history

**Statement:** After `PostToolFilter.Filter` returns, only the `filtered
tools.ToolResult` return value is used. The original unfiltered result is not
retained in orchestrator state.

**Enforcement mechanism:** Structural. The unfiltered `ToolResult` is a local
variable in the filter loop. After the filter call, only the `filtered` return
value is passed to the history-append path. There is no reference to the
original value beyond that point.

**Decision:** D77

---

### T4 — `PostToolFilter` errors route to `Failed` at `ERROR` severity

**Statement:** A non-nil error from `PostToolFilter.Filter` routes the
invocation to `Failed` and is logged at `ERROR` level (not `WARN`).

**Enforcement mechanism:** Code-review invariant. The orchestrator's filter
error handling path must use `ERROR` level for `PostToolFilter` errors and
`WARN` level for `PreLLMFilter` errors.

**Decision:** D78

---

### T5 — Hook and filter panics are recovered, not propagated

**Statement:** Panics in `PolicyHook.Evaluate`, `PreLLMFilter.Filter`, and
`PostToolFilter.Filter` are recovered by the orchestrator and surfaced as
`SystemError`; they do not crash the calling goroutine.

**Enforcement mechanism:** Structural. Deferred `recover()` calls wrap each
hook/filter invocation site.

**Decision:** D78

---

### T6 — `PreLLMFilter` input is trust-boundary-internal

**Statement:** The message list passed to `PreLLMFilter.Filter` consists only
of caller-constructed messages and previously `PostToolFilter`-filtered tool
results; it has never been in a lower-trust domain without passing through the
appropriate filter.

**Enforcement mechanism:** Structural. Tool results enter the conversation
history only after `PostToolFilter` (T1, T3). The `PreLLMFilter` call site
reads from the conversation history, which contains only filtered content.

**Decision:** D78

---

### T7 — Framework never inspects tool output for security patterns

**Statement:** The framework does not perform any content analysis on
`ToolResult.Content`; detection logic is exclusively the `PostToolFilter`
implementor's responsibility.

**Enforcement mechanism:** Code-review invariant. Framework code must not
contain pattern-matching, keyword scanning, or classifier calls on
`ToolResult.Content`.

**Decision:** D77

---

## Observability safety (O-series)

### O1 — `RedactingHandler` deny-list covers all credential and identity key patterns

**Statement:** The `RedactingHandler` deny-list covers `praxis.credential.*`,
`praxis.signed_identity`, and key-suffix patterns `_secret`, `_token`, `_key`,
`_password`, `_jwt`.

**Enforcement mechanism:** Runtime check (at the log-record level). Any log
record that reaches `RedactingHandler.Handle` and contains a matching key has
its value replaced with `[REDACTED]` before forwarding.

**Decision:** D58 (Phase 4), D79 (Phase 5)

---

### O2 — Raw LLM request and response content is never logged

**Statement:** The `messages []llm.Message` slice and LLM provider response
content are never logged at any level by the framework.

**Enforcement mechanism:** Code-review invariant. The framework's log call
sites (Phase 4 §6) do not accept LLM message or response objects.

**Decision:** D58 (Phase 4)

---

### O3 — Raw tool output is never logged

**Statement:** `ToolResult.Content` is never logged by the framework at any
level, before or after filtering.

**Enforcement mechanism:** Code-review invariant. The framework logs only tool
call metadata (`praxis.tool_name`, `praxis.tool_call_id`, filter decision
summaries). Content fields are excluded from all log call sites.

**Decision:** D58 (Phase 4), D77 (Phase 5)

---

### O4 — Enricher attributes are not dumped as a log blob

**Statement:** The `InvocationEvent.EnricherAttributes` map is not logged as
a `slog.Attr` group by the framework; individual enricher values may contain
PII and are not the framework's responsibility to redact.

**Enforcement mechanism:** Code-review invariant. Framework log call sites do
not pass `EnricherAttributes` as a log field. Callers who implement
`LifecycleEventEmitter` take responsibility for their own attribute logging.

**Decision:** D58 (Phase 4), D60 (Phase 4)

---

### O5 — Credential key patterns are not used as framework span attribute keys

**Statement:** The framework does not set OTel span attributes whose keys
match the `RedactingHandler` deny-list patterns (e.g., no
`praxis.credential.*` attribute is ever set on a span).

**Enforcement mechanism:** Code-review invariant. The framework's span
attribute sets are enumerated in Phase 4 `02-span-tree.md`; none use
deny-listed key patterns.

**Decision:** D58 (Phase 4), D67 (Phase 5)

---

## Invariant traceability matrix

| ID | Category | Primary decision | Phase |
|---|---|---|---|
| C1 | Credential isolation | D67, D68 | 5 |
| C2 | Credential isolation | D45 | 3 |
| C3 | Credential isolation | D45 | 3 |
| C4 | Credential isolation | D45, D67 | 3, 5 |
| C5 | Credential isolation | D45, D58 | 3, 4 |
| C6 | Credential isolation | D45, D67 | 3, 5 |
| C7 | Credential isolation | D45 | 3 |
| C8 | Credential isolation | D21, D69 | 2, 5 |
| I1 | Identity signing | D72, D73 | 5 |
| I2 | Identity signing | D72 | 5 |
| I3 | Identity signing | D71 | 5 |
| I4 | Identity signing | D73 | 5 |
| I5 | Identity signing | D46 | 3 |
| I6 | Identity signing | D79 | 5 |
| T1 | Trust boundaries | D77 | 5 |
| T2 | Trust boundaries | D77 | 5 |
| T3 | Trust boundaries | D77 | 5 |
| T4 | Trust boundaries | D78 | 5 |
| T5 | Trust boundaries | D78 | 5 |
| T6 | Trust boundaries | D78 | 5 |
| T7 | Trust boundaries | D77 | 5 |
| O1 | Observability safety | D58, D79 | 4, 5 |
| O2 | Observability safety | D58 | 4 |
| O3 | Observability safety | D58, D77 | 4, 5 |
| O4 | Observability safety | D58, D60 | 4 |
| O5 | Observability safety | D58, D67 | 4, 5 |
