# Phase 5 — Decisions Log

**Phase:** 5 — Security and Trust Boundaries
**Decision range:** D67–D80
**Status:** adopted working position

---

## Amendability

All decisions in this log follow the amendment protocol in
`docs/phase-1-api-scope/01-decisions-log.md`. They are working positions, not
immutable commitments. Later phases that discover contradictions or missed cases
may open new decision IDs with `Supersedes:` annotations.

---

## D67 — Credential zeroing technique

**Status:** decided
**Summary:** `Credential.Close()` must overwrite the backing array with zeros
using a loop that is protected from dead-store elimination by a
`runtime.KeepAlive` fence.

**Decision.** The zeroing pattern mandated for all `Credential` implementations
is:

```go
func zeroBytes(b []byte) {
    for i := range b {
        b[i] = 0
    }
    runtime.KeepAlive(b)
}
```

The `runtime.KeepAlive(b)` call appears immediately after the loop and ensures
the Go compiler cannot classify the writes as dead stores. Without the fence,
the compiler is permitted to elide the zeroing loop when it can prove the slice
is unreachable after the loop — the classic dead-store elimination scenario.
`runtime.KeepAlive` introduces a use of `b` at the call site, preventing the
compiler from eliding any preceding writes to `b`'s backing array.

`crypto/subtle.ConstantTimeCopy` is not used because it copies from a source
slice (intended for constant-time comparison), not for in-place zeroing. It
adds unnecessary indirection.

An explicit memory barrier (via `atomic.StoreUint32` or assembly) is not used
because the threat is compiler optimization, not CPU reordering. The Go memory
model guarantees that a goroutine observes its own writes in program order.
`runtime.KeepAlive` is the idiomatic, stdlib-supported mechanism for preventing
compiler elision.

**Security implication.** The zeroing occurs before `Close()` returns. After
`Close()`, the backing array contains zeros. A caller who retains the slice
reference after `Close()` will read zeros — a protocol violation (see
`Credential.Value()` contract) but not a credential leak.

**Constraint on implementors.** Every `Credential` implementation must call
the zeroing pattern in D67 (or the `credentials.ZeroBytes` helper from D68)
before returning from `Close()`. The framework cannot enforce this at
compile time; it is a code-review invariant for all `Credential`
implementations, including those produced by callers.

---

## D68 — `credentials.ZeroBytes` utility

**Status:** decided
**Summary:** The framework ships `credentials.ZeroBytes(b []byte)` as an
exported helper in the `credentials` package.

**Decision.** The `credentials` package exports:

```go
// ZeroBytes overwrites every element of b with zero and prevents the
// compiler from eliding the writes via a runtime.KeepAlive fence.
//
// Credential implementations must call ZeroBytes on all secret material
// before Close() returns.
//
// ZeroBytes is safe to call with a nil or zero-length slice.
//
// Package: MODULE_PATH_TBD/credentials
func ZeroBytes(b []byte) {
    for i := range b {
        b[i] = 0
    }
    runtime.KeepAlive(b)
}
```

**Arguments for shipping this helper:**

1. **Single correct implementation.** The dead-store elimination risk is
   non-obvious. Shipping a tested, documented utility reduces the probability
   that a third-party `Credential` implementation silently omits the
   `runtime.KeepAlive` fence.
2. **Discoverability.** A caller building a `Credential` implementation will
   find `credentials.ZeroBytes` when exploring the `credentials` package.
   Without it, they must independently discover the `runtime.KeepAlive`
   pattern.
3. **Testability.** The utility is trivially testable: write a known value,
   call `ZeroBytes`, assert that all bytes are zero.
4. **Minimal footprint.** The function is eight lines; it adds no dependency
   beyond `runtime`.

**Arguments against (and why rejected):** Some implementors may use
`bytes.NewBuffer` or a sync pool whose zeroing is handled elsewhere. They are
not required to use `ZeroBytes`; the helper is available, not mandatory. The
existence of `ZeroBytes` does not prevent implementors from using a different
(correct) zeroing approach.

---

## D69 — C4 resolution: soft-cancel credential context

**Status:** decided
**Summary:** During the 500ms soft-cancel grace window, `credentials.Resolver.Fetch`
receives a context derived from `context.WithoutCancel(operationCtx)` with a
500ms timeout added. This ensures credential resolution is not hard-cancelled
during graceful shutdown while still bounding its duration.

**Decision.** The orchestrator's tool-dispatch path constructs the credential
fetch context as follows when a soft-cancel is in progress:

```go
// credentialFetchCtx builds the context for credentials.Resolver.Fetch.
//
// During normal operation, operationCtx is the Layer 3 operation context
// (D21) — it carries the caller's cancellation signal and deadline.
//
// During the 500ms soft-cancel grace window (D21 §2.4), the caller's
// context is already cancelled. The orchestrator detects this condition
// (ctx.Err() != nil at the point of tool dispatch) and constructs a
// detached context so that credential resolution can complete within
// the grace window rather than failing immediately.
func credentialFetchCtx(operationCtx context.Context, softCancel bool) (context.Context, context.CancelFunc) {
    if !softCancel {
        return operationCtx, func() {}
    }
    // operationCtx is cancelled; derive a fresh context that is not
    // cancelled, then layer the 500ms grace budget on top.
    detached := context.WithoutCancel(operationCtx)
    return context.WithTimeout(detached, 500*time.Millisecond)
}
```

**Why `context.WithoutCancel`:** `context.WithoutCancel` (Go 1.21+, available
in Go 1.23+ project baseline) returns a copy of the parent context with the
cancellation signal stripped but all values preserved. This means:
- The fetch call is not immediately cancelled.
- The 500ms timeout is the only deadline in force.
- Values propagated through context (trace span, baggage) are still
  accessible to the `Resolver` implementation.

**Why a 500ms timeout instead of no timeout:** without a timeout, a slow vault
backend could hold up graceful shutdown indefinitely. The 500ms matches the
soft-cancel grace window defined in D21, so the entire tool-call goroutine
(credential fetch + tool dispatch) completes or times out within the same
window.

**What happens if Fetch exceeds 500ms:** the `context.WithTimeout` deadline
fires, causing `Fetch` to return `context.DeadlineExceeded`. The orchestrator
classifies this as a `CancellationError` (D63 rule 2) and routes the
invocation to `Cancelled`. The credential value is never obtained; the tool
is not dispatched.

**Integration with the Layer 3 operation context (D21):** the Layer 3 context
is the normal carrier for the tool-call goroutine. During soft cancel, the
goroutine's outer context is already done. The `credentialFetchCtx` function
is called only at the credential-fetch point; the rest of the goroutine
continues to observe the Layer 3 context (which is done, so subsequent work
is short-circuited). The detached context is used exclusively for the `Fetch`
call and is cancelled/released after `Fetch` returns.

---

## D70 — JWT registered claim set

**Status:** decided
**Summary:** Five registered claims are mandatory; two are optional.
Claim values and lifetime rules are specified below.

**Decision.**

| Claim | Mandatory | Value |
|---|---|---|
| `iss` | Yes | Caller-configured string; defaults to `"praxis"` if not set via `SignerOption` |
| `sub` | Yes | The `invocationID` parameter passed to `Sign` |
| `exp` | Yes | `iat + tokenLifetime` (see D72) |
| `iat` | Yes | Unix timestamp (seconds) at time of `Sign` call |
| `jti` | Yes | A UUIDv7 generated per `Sign` call |
| `aud` | Optional | Caller-configured; absent from the token if not set |
| `nbf` | No | Omitted. The token is valid immediately upon issuance; `nbf == iat` adds no security value for short-lived per-call tokens |

**`iss` default rationale.** The issuer identifies the signing party to
relying parties verifying the token. For the reference implementation, a
caller-configurable string is required because different deployments will
have different issuer semantics (service name, environment, etc.). The
`"praxis"` default is a safe fallback for development; production callers
must configure a meaningful issuer via `WithIssuer(iss string) SignerOption`
(see `03-identity-signing.md` §4.2 for the full option set).

**`sub` maps to `invocationID`.** The subject is the agent invocation making
the tool call, not a human user. `invocationID` is unique per invocation and
under the framework's control.

**`jti` as UUIDv7.** UUIDv7 is time-ordered and sortable, which benefits
token-store implementations that need to order or expire JTIs. It requires a
UUIDv7 generator in the `identity` package (no external dependency; a minimal
implementation using `crypto/rand` and time-prefix construction is sufficient
for v1.0).

**`nbf` omission rationale.** `nbf` adds value when tokens are issued before
their valid window (e.g., pre-fetched tokens). Per-call tokens are signed
immediately before use; `nbf == iat` is redundant. Omitting `nbf` reduces
token payload size and simplifies verification logic.

---

## D71 — JWT custom claims

**Status:** decided
**Summary:** Two framework-defined custom claims are mandatory. An extension
point for caller-provided claims is available via `SignerOption`.

**Decision.**

**Mandatory custom claims (always present in reference impl tokens):**

| Claim key | Type | Value |
|---|---|---|
| `praxis.invocation_id` | string | The `invocationID` parameter from `Sign` |
| `praxis.tool_name` | string | The `toolName` parameter from `Sign` |

**Rationale for prefixing.** Using the `praxis.` namespace for custom claims
prevents collision with future registered claim names and with caller-defined
claims. RFC 7519 §4.3 ("Private Claims") recommends collision-resistant names
for non-registered claims; the `praxis.` prefix satisfies this.

**`praxis.invocation_id` vs. `sub`:** `sub` is set to `invocationID` (D70)
for compatibility with standard JWT validators that use `sub`. The custom
`praxis.invocation_id` claim is additionally present to provide a
framework-semantically-named field for praxis-aware verifiers.

**Caller extension point:**

```go
// WithExtraClaims adds caller-defined claims to every token produced by the
// Signer. Claims are merged into the payload before signing.
//
// If a caller claim key collides with a mandatory registered or custom
// claim key (e.g., "iss", "sub", "praxis.invocation_id"), the mandatory
// claim wins and the caller claim for that key is silently dropped.
//
// The claims map is shallow-copied at construction time; mutations to the
// map after passing it to WithExtraClaims have no effect.
func WithExtraClaims(claims map[string]any) SignerOption
```

The extension point is intentionally simple — a static map, not a function.
Per-call claim generation (e.g., including a user ID from the call context)
requires callers to implement their own `Signer`. The reference implementation
is for fixed-claim scenarios.

---

## D72 — Token lifetime policy

**Status:** decided
**Summary:** Token lifetime is configurable at `NewEd25519Signer` construction
time. The default is 60 seconds. Minimum is 5 seconds; maximum is 300 seconds.
Lifetime is per-call (each `Sign` call produces a token valid for
`tokenLifetime` seconds from issuance).

**Decision.** Token lifetime is a constructor parameter:

```go
// WithTokenLifetime sets the duration for which each signed token is valid.
//
// Default: 60 seconds.
// Minimum: 5 seconds. Values below 5 seconds are rejected at construction
// time with an error.
// Maximum: 300 seconds. Values above 300 seconds are rejected at
// construction time with an error.
//
// The lifetime is measured from the time of the Sign call (iat) to
// expiration (exp = iat + lifetime).
func WithTokenLifetime(d time.Duration) SignerOption
```

**Why 60s default.** A per-call token that is valid for 60 seconds gives the
downstream tool service enough time to receive, validate, and process the
request even under moderate network latency. Tools that complete in under 60
seconds (the expected case) will never see token expiry. Tools that take
longer than 60 seconds (unusual; such tools should not require identity
tokens for each call) can use a longer lifetime configured at Signer
construction time.

**Why not per-invocation tokens.** Per-invocation tokens would be valid for
the entire invocation lifetime (minutes to hours). A compromised token would
authorize all tool calls for that invocation's duration. Per-call tokens bound
the exposure window to the token lifetime regardless of invocation length.

**Why 300s maximum.** Tokens valid for more than 5 minutes approach session
token territory. The constraint encourages short lifetimes while giving callers
with slow tool dispatch some headroom.

---

## D73 — Ed25519 reference implementation contract

**Status:** decided
**Summary:** `NewEd25519Signer` accepts an `ed25519.PrivateKey` and options.
It produces compact JWS tokens (JSON Web Signature) using the `EdDSA` algorithm
header. No external JWT library is used.

**Decision.**

```go
// NewEd25519Signer returns a Signer that produces Ed25519-signed JWTs.
//
// key must be a valid ed25519.PrivateKey (64 bytes). NewEd25519Signer
// returns a non-nil error if key is nil, incorrectly sized, or if any
// option is invalid.
//
// The returned Signer is safe for concurrent use. Multiple goroutines
// may call Sign simultaneously.
//
// Token construction uses only stdlib packages: crypto/ed25519,
// encoding/json, encoding/base64, crypto/rand (for jti). No external
// JWT library is imported.
//
// Algorithm: EdDSA (Ed25519). JOSE header: {"alg":"EdDSA","typ":"JWT"}.
// If a kid is configured via WithKeyID, it is added to the header.
//
// Package: MODULE_PATH_TBD/identity
func NewEd25519Signer(key ed25519.PrivateKey, opts ...SignerOption) (Signer, error)
```

**What the constructor commits to:**
- The JOSE header contains `"alg":"EdDSA"` and `"typ":"JWT"` (RFC 7519).
- If `WithKeyID` is set, `"kid"` is added to the JOSE header.
- The payload contains all mandatory registered claims (D70) and mandatory
  custom claims (D71).
- The signature is produced by `ed25519.Sign(key, signingInput)` where
  `signingInput` is `base64url(header) + "." + base64url(payload)` (RFC 7515).
- Token format is compact serialization: `header.payload.signature`.

**What the constructor does NOT commit to:**
- Key management, storage, or rotation (caller's responsibility).
- Verification logic (callers verify using the corresponding public key and
  any RFC 7517 JWK set they maintain).
- Key derivation or wrapping.

**Stdlib-only constraint.** The `identity` package must not import any
third-party JWT library. The compact serialization for Ed25519 JWTs is
straightforward to construct from `encoding/json` + `encoding/base64` +
`crypto/ed25519`. Vendoring a JWT library introduces a supply-chain dependency
in a security-critical package without commensurate benefit.

---

## D74 — Key lifecycle model

**Status:** decided
**Summary:** The reference implementation holds a single static key pair.
A `kid` (key ID) header is supported to assist verifier key selection.
Key rotation requires callers to implement their own `Signer`. Key revocation
is out of scope.

**Decision.**

**Static key model.** `NewEd25519Signer` accepts one `ed25519.PrivateKey`.
The key is used for all `Sign` calls until the `Signer` is discarded. This
is the simplest correct model for the reference implementation.

**`kid` support:**

```go
// WithKeyID sets the "kid" (key ID) header parameter on all tokens
// produced by this Signer.
//
// kid is an arbitrary opaque string identifying the signing key.
// Relying parties use kid to select the correct public key from a
// key set for verification.
//
// If not set, the kid header parameter is omitted from the token header.
func WithKeyID(kid string) SignerOption
```

**Rotation model.** Callers who need key rotation implement a `Signer` that
wraps a `KeyProvider` interface of their own design. The reference implementation
does not accept a `KeyProvider`. Example pattern for callers:

```go
type RotatingKeyProvider interface {
    CurrentKey() (ed25519.PrivateKey, kid string)
}
// The caller's Signer calls CurrentKey() on each Sign invocation,
// then constructs the JWT using the current key and kid.
```

The framework does not prescribe the `KeyProvider` interface because key
rotation strategies vary widely (time-based rotation, event-driven rotation,
KMS-backed rotation) and a library-level interface would force a specific
model on all callers.

**Key revocation.** Token revocation (e.g., a blocklist of JTI values) is
out of scope for the framework. The token lifetime (D72) is the primary
expiry mechanism. Callers who need revocation implement it at the verification
layer, not in the `Signer`.

**Key material lifetime in memory.** The `ed25519.PrivateKey` passed to
`NewEd25519Signer` is retained in the `Signer` struct for its lifetime.
The caller is responsible for not discarding the source key material before
the `Signer` is GC'd. The framework does not zero the private key on
`Signer` discard — this is an open issue (see Open Issues in
`04-trust-boundaries.md`).

---

## D75 — CP6 identity chaining claim structure

**Status:** decided
**Summary:** An inner Signer references the outer invocation's signed identity
via a `praxis.parent_token` custom claim containing the outer token string.
Chain depth is not enforced by the framework; documentation recommends a
maximum of 3 levels.

**Decision.**

**How the inner Signer reads the outer token.** The outer invocation's signed
identity is available as `tools.InvocationContext.SignedIdentity` (a string).
The inner orchestrator's Signer reads this field and embeds it in the token:

```go
// Inner Signer usage pattern (callers implement this):
func (s *innerSigner) Sign(ctx context.Context, invocationID string, toolName string) (string, error) {
    outerToken := invocationCtxFromContext(ctx).SignedIdentity
    extraClaims := map[string]any{}
    if outerToken != "" {
        extraClaims["praxis.parent_token"] = outerToken
    }
    // Construct and sign the JWT with the extra claim included.
}
```

**`praxis.parent_token` claim.** The claim value is the complete outer token
string (compact JWS serialization). Verifiers who need to validate the chain
decode both tokens independently using their respective public keys. The inner
token's `praxis.parent_token` value is the outer token; the outer token's
`sub` field is the outer invocation ID.

**Why not x5c-style chaining.** RFC 7515 `x5c` is a header parameter
designed for X.509 certificate chains, not JWT chains. Embedding one JWT
inside another JWT's header is non-standard, produces awkward nested
deserialization, and requires verifiers to understand praxis-specific
conventions rather than standard JWT libraries. The `praxis.parent_token`
payload claim approach uses standard JWT claim deserialization; the chain
is a simple linked list of tokens readable with any JWT library.

**Chain depth recommendation.** The framework documents a maximum chain depth
of 3 (outer + 2 nested levels). This is a documentation recommendation, not
an enforced limit. Enforcement would require the `Signer` to parse and
validate the outer token, introducing a verification dependency in the signing
path. Callers who need hard depth limits implement them in their `Signer`.

**Empty outer token.** If `InvocationContext.SignedIdentity` is empty (the
outer orchestrator used `NullSigner`), the inner Signer omits
`praxis.parent_token` from the token payload. This is not an error.

---

## D76 — `identity.Signer` promotion to `frozen-v1.0`

**Status:** decided
**Summary:** `identity.Signer` is promoted from `stable-v0.x-candidate` to
`frozen-v1.0`. All gating conditions are satisfied by Phase 5.

**Decision.** The gating conditions for promotion are:

| Condition | Satisfied by |
|---|---|
| JWT registered claim set specified | D70 |
| JWT custom claims specified | D71 |
| Token lifetime policy specified | D72 |
| Ed25519 reference impl contract specified | D73 |
| Key lifecycle documented | D74 |
| CP6 identity chaining resolved | D75 |

All conditions are satisfied. `identity.Signer` is frozen at the interface
level: the method signature `Sign(ctx context.Context, invocationID string, toolName string) (string, error)` will not change in v1.x.

The stability of the reference implementation (`NewEd25519Signer`) is also
`frozen-v1.0` for the constructor signature and the token format. Changes to
the JOSE algorithm or claim structure require a v2 module path.

---

## D77 — Untrusted tool output trust model

**Status:** decided
**Summary:** `ToolResult.Content` is untrusted by contract. The framework
passes it through `PostToolFilter` without sanitization and honors filter
decisions. Detection and remediation logic belong to the filter implementor.

**Decision.** See `04-trust-boundaries.md` §1 for the full specification.

Key commitments:
- The framework never inspects `ToolResult.Content` for security-relevant
  patterns. Content analysis is exclusively a `PostToolFilter` concern.
- A `FilterActionBlock` from `PostToolFilter` causes an immediate transition
  to `Failed` with a `PolicyDeniedError` before the tool result is appended
  to the conversation history.
- The `ToolResult` passed to `PostToolFilter` is the unmodified output from
  `tools.Invoker`. The filter receives the raw, potentially hostile content.
- After filtering, only the `filtered` return value from `PostToolFilter.Filter`
  is appended to the conversation history. The original unfiltered content is
  not retained in the orchestrator's state.

---

## D78 — Filter trust boundary classification

**Status:** decided
**Summary:** `PostToolFilter` is a trust-boundary-crossing filter (input is
external, untrusted tool output). `PreLLMFilter` is trust-boundary-internal
(input is caller-constructed or previously filtered messages). This asymmetry
drives error-severity and telemetry differences.

**Decision.** See `04-trust-boundaries.md` §2 for the full classification.

Key commitments:
- Errors from `PostToolFilter` are treated as more security-critical than
  errors from `PreLLMFilter`. A `PostToolFilter` that errors (not blocks —
  errors) has potentially allowed untrusted content to evade filtering. The
  orchestrator logs this at `ERROR` level and routes to `Failed`.
- `PostToolFilter` block decisions emit `filter.prompt_injection_suspected`
  at `WARN` level (Phase 4 D58). This is unchanged.
- `PreLLMFilter` errors are also routed to `Failed` but logged at `WARN`
  (the input was already trusted-origin; failure is operational, not a
  security breach).

---

## D79 — RedactingHandler deny-list amendments

**Status:** decided
**Summary:** Two new entries are added to the Phase 4 deny-list (D58).
`praxis.signed_identity` is added because signed JWT tokens are bearer
credentials. The `_jwt` suffix pattern is added for defense in depth.

**Decision.** The following entries are added to the `RedactingHandler`
deny-list established in Phase 4 D58:

| New key pattern | Rationale |
|---|---|
| `praxis.signed_identity` | The `SignedIdentity` field carries a bearer JWT. If a future code path logs this field, it must be redacted. |
| Any key with suffix `_jwt` | Convention for JWT values in caller-constructed log records. Defense in depth. |

The deny-list (complete, post-amendment):

| Key pattern | Source |
|---|---|
| `praxis.credential.*` | D58 |
| `praxis.raw_content` | D58 |
| Any key with suffix `_secret` | D58 |
| Any key with suffix `_token` | D58 |
| Any key with suffix `_key` | D58 |
| Any key with suffix `_password` | D58 |
| `praxis.signed_identity` | D79 (this decision) |
| Any key with suffix `_jwt` | D79 (this decision) |

**`praxis.jwt_*` patterns considered and rejected.** The plan proposed a
`praxis.jwt_*` prefix pattern. The framework does not currently define any
`praxis.jwt_*` log fields; adding a wildcard prefix for fields that do not
exist is speculative. `praxis.signed_identity` is the only specific JWT-carrying
field the framework defines. The `_jwt` suffix covers the caller convention
without enumerating non-existent fields.

---

## D80 — Security invariants summary

**Status:** decided
**Summary:** 16 security invariants across four categories (credential
isolation, identity signing, trust boundaries, observability safety) are
enumerated in `05-security-invariants.md`.

**Decision.** See `05-security-invariants.md` for the complete enumeration.
All invariants are traceable to a decision in D67–D79 or to a prior-phase
decision (D45, D46, D58).
