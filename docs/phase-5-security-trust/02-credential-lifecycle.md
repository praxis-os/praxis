# Phase 5 — Credential Lifecycle

**Decisions:** D67, D68, D69
**Cross-references:** Phase 3 `09-credentials-and-identity.md` (interface contract),
Phase 2 D21 (soft-cancel grace window), Phase 4 `04-slog-redaction.md` (log
isolation)

---

## 1. Zeroing technique (D67)

### 1.1 Threat

The Go compiler may eliminate writes to a byte slice as dead stores when it can
prove the slice is unreachable after the write. If a `Credential.Close()`
implementation zeros the backing array but the compiler elides the writes, the
secret material remains in memory after `Close()` returns. This is a real
vulnerability, not a theoretical one: the Go specification permits optimising
compilers to remove dead writes, and the gc compiler does perform this
optimisation in practice.

### 1.2 Required zeroing pattern

Every `Credential` implementation must zero secret material using the following
pattern before `Close()` returns:

```go
import "runtime"

// zeroBytes overwrites every byte in b with zero and prevents the compiler
// from treating the writes as dead stores via a runtime.KeepAlive fence.
// This function is available as credentials.ZeroBytes; implementations
// may call it directly.
func zeroBytes(b []byte) {
    for i := range b {
        b[i] = 0
    }
    runtime.KeepAlive(b)
}
```

`runtime.KeepAlive(b)` introduces a use of `b` at the end of the function.
The Go compiler cannot elide any write to `b` (or its backing array) that
happens-before the `KeepAlive` call in program order. This is the idiomatic
Go pattern for preventing dead-store elision without resorting to assembly or
`unsafe`.

### 1.3 Why not other approaches

**`crypto/subtle.ConstantTimeCopy`:** Designed for constant-time comparison,
not in-place zeroing. It copies bytes from a source slice into a destination
slice, requiring a zero-value source buffer. The added allocation and copy add
complexity without addressing the dead-store problem differently from
`runtime.KeepAlive`.

**`atomic` writes or assembly barriers:** These prevent CPU reordering, not
compiler elision. The threat is the compiler removing the write entirely before
code generation; an atomic store would preserve the write but introduces
unnecessary synchronisation overhead on the hot path.

**`unsafe.Pointer` with a volatile write pattern:** Non-idiomatic, introduces
`unsafe` dependency into a security-critical package, and is not portable
across Go versions. `runtime.KeepAlive` is the explicitly supported mechanism
for this use case.

### 1.4 `credentials.ZeroBytes` utility (D68)

The `credentials` package exports `ZeroBytes` as the canonical implementation
of the zeroing pattern:

```go
// ZeroBytes overwrites every byte of b with zero.
//
// The runtime.KeepAlive fence prevents the Go compiler from treating
// the zero-writes as dead stores. Credential implementations must call
// ZeroBytes on all secret material in their Close() method.
//
// ZeroBytes is safe to call with a nil or zero-length slice; it returns
// immediately without allocation.
//
// Package: MODULE_PATH_TBD/credentials
func ZeroBytes(b []byte) {
    for i := range b {
        b[i] = 0
    }
    runtime.KeepAlive(b)
}
```

**Obligation.** Every `Credential.Close()` implementation — whether in the
framework or in caller code — must call `credentials.ZeroBytes` (or implement
an equivalent pattern with `runtime.KeepAlive`) on every secret-carrying byte
slice before returning.

**Idempotency.** `Credential.Close()` must be idempotent: calling it multiple
times must not panic. The zeroing pattern satisfies this: zeroing an already-
zero slice is a no-op. Implementations that use a `sync.Once` guard to prevent
double-free on resources (e.g., vault lease cancellation) must still zero the
byte slice on first close and return nil on subsequent closes.

---

## 2. Soft-cancel credential resolution (D69)

### 2.1 Background

Phase 2 D21 §2.4 defines a 500ms soft-cancel grace window. When the caller
cancels the context during a tool dispatch, the orchestrator does not
hard-cancel all in-flight goroutines immediately. Instead:

1. It stops launching new tool calls.
2. It allows in-flight tool calls to complete within 500ms.
3. After 500ms, remaining goroutines are abandoned (their contexts are cancelled).

The soft-cancel window exists to prevent partial tool execution from leaving
external systems in inconsistent state. Credential fetching must participate in
this model: hard-cancelling a vault fetch mid-flight could leave the vault
session in an undefined state (or simply fail the tool that has already been
dispatched).

### 2.2 Context derivation specification

The orchestrator's tool-dispatch path maintains a `softCancel` boolean that is
set to `true` when it detects that the operation context (`operationCtx`) has
been cancelled at the point of credential fetching:

```go
// credentialFetchCtx constructs the context for credentials.Resolver.Fetch.
//
// softCancel is true when the caller's context is cancelled and the
// orchestrator is in its 500ms grace window. In this case, the operation
// context is already done; a fresh detached context with the grace timeout
// is used instead.
//
// The returned CancelFunc must be called after Fetch returns to release
// the timeout timer. The caller defers it.
func credentialFetchCtx(
    operationCtx context.Context,
    softCancel bool,
) (context.Context, context.CancelFunc) {
    if !softCancel {
        // Normal path: forward the operation context unchanged.
        return operationCtx, func() {}
    }
    // Soft-cancel path: operationCtx.Err() != nil.
    // Strip cancellation but preserve context values (trace spans, baggage).
    detached := context.WithoutCancel(operationCtx)
    // Layer the 500ms grace budget as a hard deadline.
    return context.WithTimeout(detached, 500*time.Millisecond)
}
```

The call site in the tool-dispatch goroutine:

```go
fetchCtx, cancel := credentialFetchCtx(operationCtx, operationCtx.Err() != nil)
defer cancel()
cred, err := resolver.Fetch(fetchCtx, ref)
if err != nil {
    // Handle per-classifier rules (D63).
    return
}
defer func() {
    if closeErr := cred.Close(); closeErr != nil {
        // Log at WARN; close errors are non-fatal but observable.
    }
}()
// dispatch tool...
```

### 2.3 `context.WithoutCancel` semantics

`context.WithoutCancel` returns a new context that:
- Is **not** cancelled when the parent is cancelled.
- Carries all parent **values** (trace spans, baggage, request-scoped data).
- Has **no deadline** of its own (the `context.WithTimeout` layer adds one).

This means the `Resolver` implementation can:
- Access the trace span for instrumentation.
- Access any context-propagated authentication material it needs.
- Not observe the caller's cancellation signal.

### 2.4 Timeout semantics

If `Resolver.Fetch` does not return within 500ms, `context.DeadlineExceeded`
is returned from the context. The `Resolver` may propagate this error or
wrap it. Either way, the orchestrator's error classifier (D63) recognises
`context.DeadlineExceeded` and routes the invocation to `Cancelled`.

The credential is not obtained. The tool is not dispatched. The 500ms grace
window has been consumed by the credential fetch attempt; any remaining grace
time is not extended.

### 2.5 Normal-path behaviour

When the operation context is not cancelled (`softCancel == false`), the
credential fetch context is the operation context unchanged. No extra
allocation occurs. The soft-cancel path is the exception, not the rule.

---

## 3. Runtime isolation invariant

### 3.1 Invariant statement

Credential values (the bytes returned by `Credential.Value()`) must not escape
the following scopes:

| Boundary | Invariant |
|---|---|
| Tool-call goroutine | The `Credential` value is used only within the goroutine that received it from `Resolver.Fetch`. It is not passed to other goroutines. |
| Log records | No log record emitted by the framework contains a credential value. |
| Span attributes | No span attribute set by the framework contains a credential value. |
| Error messages | `TypedError.Error()` and `error.Error()` produced by the framework do not embed credential values. |
| `InvocationEvent` fields | No field of `InvocationEvent` carries a credential value. |

### 3.2 Structural enforcement

The framework enforces the goroutine-scope isolation **structurally** rather
than by documentation alone. The `Credential` value is never stored in
shared mutable state reachable by other goroutines:

- The `Credential` is fetched inside the tool-call goroutine, used, and
  `Close()`d within the same goroutine. It is never placed in the
  `InvocationRequest`, `tools.InvocationContext`, the orchestrator's shared
  state, or any channel.
- The orchestrator fetches the credential via `Resolver.Fetch`, then passes
  the credential to the `tools.Invoker` implementation via a mechanism
  outside `InvocationContext` (e.g., as a context value or via the invoker's
  own credential-aware dispatch). `tools.InvocationContext` (Phase 3 D36) has
  no `Credential` field — this is a deliberate structural isolation decision.
  The invoker receives the credential within its `Invoke` call scope and must
  not retain it beyond that scope.

The log, span, event, and error boundaries are enforced by **code-review
invariant**: framework code must never have `Credential.Value()` as an
expression in a log call, span attribute set, event field assignment, or
error format string. This is the same enforcement model as the no-raw-content
rule in Phase 4 D58.

The `RedactingHandler` (D58, D79) provides defence in depth for the log
boundary: even if future framework code accidentally logged a field with a
`_token`-suffixed key, the handler would redact the value before it reached
the backing handler. This is not a substitute for the code-review invariant;
it is a second layer.

### 3.3 Error message discipline

`TypedError.Error()` implementations in the framework must not format
credential-carrying values into the error string. The error string carries
only the error kind and a human-readable description that does not include
runtime values from the credential fetch or tool dispatch. This is enforced
by code review.

If `Resolver.Fetch` returns an error, that error may contain vault-backend
context (e.g., "connection refused to vault:8200"). The framework wraps this
error in a `ToolError` or `SystemError` but does not embed `CredentialRef.Name`
in the error string unless the name is a safe, non-sensitive logical identifier
(e.g., "stripe-api-key"). `CredentialRef.Scope` is also not embedded.

---

## 4. GC interaction and acceptable risk

### 4.1 What zeroing achieves

After `credentials.ZeroBytes(b)` completes, the backing array of `b` contains
all-zero bytes. Any code that reads from this array after `Close()` returns
(a protocol violation) will read zeros, not the original secret.

### 4.2 What zeroing does not achieve

**Slice header retention.** The `Credential.Value()` return value is a slice
— a three-word struct (pointer, length, capacity). After `Close()`, the slice
header on the stack (or in a caller's local variable) still points to the now-
zeroed backing array. The header itself is not zeroed by `ZeroBytes`. A caller
who inspects the slice header after `Close()` will see a pointer to a zeroed
array — they cannot reconstruct the original secret from the header.

**GC timing.** The backing array is not collected immediately after zeroing.
The GC may defer collection until the next GC cycle. During this window, the
zeroed bytes occupy memory. An adversary with the ability to read arbitrary
process memory (a separate vulnerability) would observe zeros, not the original
secret.

**Memory page swapping.** If the secret was in a memory page that was swapped
to disk before `ZeroBytes` ran, the on-disk copy is not zeroed. This is an
OS-level concern outside the framework's scope. Callers with strict key-material
confidentiality requirements (e.g., FIPS-compliance) should use OS primitives
like `mlock(2)` to prevent swapping; the framework does not provide this.

### 4.3 Acceptable risk statement

The zeroing invariant provides meaningful protection against:
- Heap dumps captured after `Close()` returns (zeroed array).
- Log scraping of heap objects (no credential value in log records by structural
  enforcement).
- Post-execution memory forensics that do not examine the zeroed array in the
  brief window between first write and GC collection.

The zeroing invariant does not protect against:
- Memory reads during the credential's live window (by design — the credential
  must be readable for its intended use).
- OS-level memory inspection with sufficient privilege.
- Swap-based recovery of secrets that were paged out before zeroing.

This risk profile is consistent with the Go security model for sensitive
in-memory material and with the design of comparable Go security libraries.
It is acceptable for v1.0.
