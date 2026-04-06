# Phase 3 — Credentials and Identity

**Stability tiers:** `credentials.Resolver` — `frozen-v1.0`;
`identity.Signer` — `stable-v0.x-candidate` (promotion deferred to Phase 5)
**Decisions:** D45, D46
**Packages:** `MODULE_PATH_TBD/credentials`, `MODULE_PATH_TBD/identity`

---

## `credentials` package

### `Resolver` interface (D45)

```go
// Resolver fetches credentials for tool invocations.
//
// The orchestrator calls Fetch once per tool call, immediately before
// dispatching to tools.Invoker. Credentials are never cached across
// calls inside the framework. They are never logged, spanned, or
// serialized into error contexts.
//
// The returned Credential's Close() method is called in a deferred call
// immediately after the tool invocation returns, within the same
// goroutine as the Invoke call. Callers must ensure that the Credential
// is not retained beyond that scope.
//
// During the 500 ms soft-cancel grace window (D21), the orchestrator
// passes a context.WithoutCancel-derived context to Fetch so that
// credential resolution is not hard-cancelled while a graceful shutdown
// is in progress (C4 — full documentation deferred to Phase 5).
//
// Resolver implementations must be safe for concurrent use. Under
// parallel tool dispatch (D24), multiple Fetch calls may occur
// simultaneously.
//
// Stability: frozen-v1.0.
type Resolver interface {
    // Fetch retrieves the credential identified by ref.
    //
    // Returns a non-nil Credential on success. The caller must call
    // Credential.Close() exactly once after use.
    //
    // Returns a non-nil error if the credential cannot be fetched.
    // The error is classified by errors.Classifier before routing;
    // a transient fetch failure (e.g., vault unavailable) may produce
    // a retryable ToolError at the orchestrator's discretion.
    Fetch(ctx context.Context, ref CredentialRef) (Credential, error)
}

// CredentialRef is the logical reference to a credential.
// The Resolver interprets the Name and Scope fields; the framework
// never inspects them.
type CredentialRef struct {
    // Name is the logical credential identifier.
    // Examples: "stripe-api-key", "database-password".
    Name string

    // Scope is an optional access scope hint.
    // Examples: "read", "write", "admin".
    // May be empty; interpretation is Resolver-specific.
    Scope string
}

// Credential is the runtime handle for a fetched secret.
//
// The framework calls Close() in a deferred call after tool invocation.
// The Credential implementation must zero the secret material on Close()
// to minimize the in-memory lifetime of the secret. Close() is
// idempotent: multiple calls must not panic or return new errors.
//
// Credential is an interface so that implementations can control
// zeroing behavior and hold additional resources (e.g., vault lease
// renewal goroutines) alongside the secret value.
//
// Stability: frozen-v1.0.
type Credential interface {
    // Value returns the secret material.
    //
    // Callers must not retain the returned slice after calling Close().
    // The implementation may zero or replace the underlying array on Close().
    Value() []byte

    // Close zeroes secret material in memory and releases associated
    // resources (e.g., vault leases, buffer pool returns).
    //
    // Must be called exactly once per Fetch. Idempotent: subsequent
    // calls must return nil without panicking.
    Close() error
}
```

### Default (null) implementation

```go
// NullResolver is the default credentials.Resolver.
// It returns an error for every Fetch call, indicating that no
// credential store is configured.
//
// Use NullResolver for zero-wiring construction (D12) when the
// tools.Invoker does not require credential fetching.
//
// NullResolver is safe for concurrent use.
//
// Package: MODULE_PATH_TBD/credentials
var NullResolver Resolver = nullResolver{}
```

---

## `identity` package

### `Signer` interface (D46)

```go
// Signer produces short-lived Ed25519 JWTs that assert the identity of
// the invoking agent on a per-tool-call basis.
//
// Sign is called by the orchestrator immediately before dispatching each
// tool call, after credentials are fetched. The resulting token is placed
// in tools.InvocationContext.SignedIdentity and forwarded to the invoker.
//
// The JWT claim set is Phase 5's jurisdiction. The orchestrator passes
// invocationID and toolName as the minimum inputs required for signing;
// the Signer implementation maps these to appropriate JWT claims (subject,
// custom claims, etc.) using its own key management system.
//
// Signer implementations must be safe for concurrent use. Under parallel
// tool dispatch, multiple Sign calls may occur simultaneously with
// different (invocationID, toolName) pairs.
//
// Stability: stable-v0.x-candidate.
// Expected promotion to frozen-v1.0 at Phase 5 sign-off.
// Gating condition: JWT claim set and key lifecycle specification.
type Signer interface {
    // Sign returns a short-lived JWT asserting the agent's identity for
    // the given invocation and tool call.
    //
    // Returns an empty string if signing is not applicable for this call
    // (e.g., the tool does not require identity assertion). An empty
    // string is not an error.
    //
    // Returns a non-nil error if the signing operation fails (key
    // unavailable, KMS error). The orchestrator classifies the error
    // and routes the invocation to Failed.
    Sign(ctx context.Context, invocationID string, toolName string) (string, error)
}
```

### Default (null) implementation

```go
// NullSigner is the default identity.Signer.
// It returns an empty string for every Sign call without error.
// Use NullSigner for zero-wiring construction (D12) when per-call
// identity assertion is not required.
//
// NullSigner is safe for concurrent use.
//
// Package: MODULE_PATH_TBD/identity
var NullSigner Signer = nullSigner{}
```

---

## Concurrency contracts

Both `Resolver` and `Signer` implementations must be safe for concurrent use.
The orchestrator may call `Fetch` and `Sign` from multiple goroutines
simultaneously under parallel tool dispatch.

The `Credential` interface's `Close()` contract is per-instance, not per-type:
each `Credential` returned from a `Fetch` call is `Close()`d exactly once,
on the same goroutine that received it. Thread-safety at the `Credential`
instance level is not required.

---

## Security properties (Phase 5 handoff)

The following security properties are stated here as design intent. Phase 5
(Security and Trust Boundaries) owns the full specification:

1. **Zero-on-close:** `Credential.Close()` must zero the underlying byte slice
   before returning. The exact zeroing technique (overwrite with zeros,
   `runtime.KeepAlive`, GC interaction) is Phase 5's scope.
2. **No credential logging:** The framework must never pass a `Credential.Value()`
   to any log, span attribute, or event field. This is a code-review invariant,
   not a runtime check.
3. **Soft-cancel credential resolution (C4):** During the 500 ms grace window,
   `Fetch` receives a `context.WithoutCancel`-derived context so that an
   in-flight credential resolution is not hard-cancelled. Full documentation
   deferred to Phase 5.
4. **Identity chain (CP6):** The outer invocation's `SignedIdentity` is readable
   from `tools.InvocationContext.SignedIdentity`. An inner orchestrator's
   `Signer` may reference this value to construct an identity chain claim.
   The claim set structure is Phase 5's scope.
