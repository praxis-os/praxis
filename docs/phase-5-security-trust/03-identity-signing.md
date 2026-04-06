# Phase 5 — Identity Signing

**Decisions:** D70, D71, D72, D73, D74, D75, D76
**Cross-references:** Phase 3 `09-credentials-and-identity.md` (Signer interface),
Phase 3 `06-tools-and-invocation-context.md` (InvocationContext.SignedIdentity),
Phase 4 D55 (nested span semantics — parallel concern)

---

## 1. Overview

The `identity.Signer` interface produces short-lived Ed25519 JWTs asserting the
identity of the invoking agent on a per-tool-call basis. Each `Sign` call
produces a fresh token bound to a specific invocation and tool name.

The framework does not require callers to use identity signing. `NullSigner`
returns empty strings without error and is the default. Identity tokens flow
into `tools.InvocationContext.SignedIdentity`; the `tools.Invoker` implementation
decides how (or whether) to use them.

---

## 2. JWT claim set

### 2.1 Registered claims (D70)

| Claim | Mandatory | Value |
|---|---|---|
| `iss` | Yes | Caller-configured string. Default: `"praxis"`. |
| `sub` | Yes | The `invocationID` parameter passed to `Sign`. |
| `exp` | Yes | `iat + tokenLifetime` (Unix timestamp, seconds). |
| `iat` | Yes | Unix timestamp (seconds) at time of `Sign` call. |
| `jti` | Yes | UUIDv7 generated per-call via `crypto/rand`. |
| `aud` | Optional | Caller-configured; omitted from payload if not set. |
| `nbf` | No | Omitted. Per-call tokens are valid immediately upon issuance. |

**`iss` must be caller-configured in production.** The default `"praxis"` is
suitable for development and testing. A relying party that validates `iss` will
reject tokens with `iss: "praxis"` unless it explicitly expects that value.
Callers must set `iss` to a meaningful deployment-specific value via
`WithIssuer`.

**`sub` maps to `invocationID`.** The subject identifies the agent invocation,
not a human user. This is intentional: the token asserts agent identity, not
end-user identity. Callers who need to assert user identity add it as a custom
claim via `WithExtraClaims`.

**`jti` as UUIDv7.** UUIDv7 (draft-ietf-uuidrev-rfc4122bis) uses a 48-bit
millisecond timestamp prefix followed by 74 random bits. The time-ordered
property benefits token-store implementations that expire JTIs by creation
time. The framework constructs UUIDv7 values using `crypto/rand` for the
random portion and `time.Now()` for the timestamp prefix. No external UUID
library is required.

**`nbf` omission.** `nbf` prevents token use before a specified time, which
is relevant when tokens are pre-issued. Per-call tokens are signed immediately
before use; `nbf == iat` adds no security value and increases payload size.
Relying parties that require `nbf` must use a `Signer` implementation that
includes it.

### 2.2 Custom claims (D71)

Two custom claims are mandatory in all tokens produced by the reference
implementation. They use the `praxis.` namespace to prevent collision with
registered claims and caller-defined claims.

| Claim key | Type | Value |
|---|---|---|
| `praxis.invocation_id` | string | The `invocationID` parameter from `Sign` |
| `praxis.tool_name` | string | The `toolName` parameter from `Sign` |

**Why `praxis.invocation_id` alongside `sub`.** `sub` is the standard claim
for subject identity and is used by generic JWT validators. `praxis.invocation_id`
provides a framework-semantically-named claim for praxis-aware verifiers.
Verifiers that understand praxis conventions can use `praxis.invocation_id`
without inspecting `sub`.

**Caller extension via `WithExtraClaims`:**

```go
// WithExtraClaims adds static caller-defined claims to every token.
//
// Claims are merged into the payload before signing. If a key in claims
// collides with a mandatory registered or custom claim key (e.g., "iss",
// "sub", "exp", "iat", "jti", "praxis.invocation_id",
// "praxis.tool_name"), the mandatory claim wins and the caller's value
// for that key is silently dropped.
//
// The claims map is deep-copied at NewEd25519Signer construction time.
// Subsequent mutations to the caller's map have no effect on the Signer.
//
// Callers who need per-call dynamic claims (e.g., embedding a user ID
// from the context) must implement their own Signer.
func WithExtraClaims(claims map[string]any) SignerOption
```

The extension point accepts a static map because dynamic per-call claim
generation (claims that vary per `Sign` call based on context values) requires
callers to implement `Signer` directly. The reference implementation is for
fixed-claim scenarios; keeping it simple reduces implementation and
verification surface.

### 2.3 Payload JSON structure

A representative token payload for the reference implementation:

```json
{
  "iss": "payments-agent.acme.internal",
  "sub": "inv_01HZM3ABCDEFGHIJK",
  "exp": 1712345678,
  "iat": 1712345618,
  "jti": "01908a5d-e123-7456-b789-a12b34c56def",
  "praxis.invocation_id": "inv_01HZM3ABCDEFGHIJK",
  "praxis.tool_name": "payments.ChargeCard"
}
```

Optional fields when set:

```json
{
  "aud": ["payments-service.acme.internal"],
  "praxis.parent_token": "<outer-jwt-compact-string>"
}
```

---

## 3. Token lifetime policy (D72)

Lifetime is configurable at Signer construction time. Default: 60 seconds.

```go
// WithTokenLifetime sets the token validity window for all tokens
// produced by this Signer.
//
// The token exp claim is set to iat + d (rounded to seconds).
//
// Default: 60 seconds.
// Minimum: 5 seconds. Values below 5s are rejected at NewEd25519Signer
// construction time with an error.
// Maximum: 300 seconds. Values above 300s are rejected at NewEd25519Signer
// construction time with an error.
//
// There is no per-invocation or per-tool override; lifetime applies
// uniformly to all Sign calls from this Signer instance.
func WithTokenLifetime(d time.Duration) SignerOption
```

**60-second default rationale.** Per-call tokens are signed immediately before
tool dispatch. A 60-second window covers round-trip time to the tool service
plus processing time for all but the most latency-sensitive or slow-running
tools. Tools that consistently take longer than 60 seconds to begin verification
(unusual) can use a longer lifetime configured at Signer construction.

**300-second maximum.** Tokens valid for more than 5 minutes approach
session-token territory. The maximum encourages short-lifetime token design
while accommodating legitimate slow-dispatch scenarios.

**Per-call scope.** Each `Sign` call sets `iat` to the current time and
computes `exp = iat + lifetime`. Tokens are not cached or reused across calls.
A `Sign` call for the same invocation and tool name produces a distinct token
with a fresh `jti` and fresh `iat`/`exp` values.

---

## 4. Ed25519 reference implementation (D73)

### 4.1 Constructor

```go
// NewEd25519Signer constructs a Signer that produces Ed25519-signed JWTs.
//
// key must be a valid ed25519.PrivateKey (64 bytes). If key is nil or
// has incorrect length, NewEd25519Signer returns a non-nil error.
//
// Options are evaluated left to right. Invalid option values (e.g.,
// a token lifetime outside the allowed range) cause a non-nil error.
//
// The returned Signer is safe for concurrent use from multiple goroutines.
//
// Package: MODULE_PATH_TBD/identity
func NewEd25519Signer(key ed25519.PrivateKey, opts ...SignerOption) (Signer, error)
```

### 4.2 Available options

```go
// WithIssuer sets the "iss" (issuer) claim.
// Default: "praxis".
// Production callers must set a deployment-specific issuer.
func WithIssuer(iss string) SignerOption

// WithAudience sets the "aud" (audience) claim.
// If not set, the aud claim is omitted from the token payload.
// aud is a slice to support multi-audience tokens (RFC 7519 §4.1.3).
func WithAudience(aud []string) SignerOption

// WithKeyID sets the "kid" (key ID) JOSE header parameter.
// If not set, kid is omitted from the header.
// See D74 for key lifecycle and rotation contract.
func WithKeyID(kid string) SignerOption

// WithTokenLifetime sets the token validity window. Default: 60s. See D72.
func WithTokenLifetime(d time.Duration) SignerOption

// WithExtraClaims adds static caller-defined claims to every token. See D71.
func WithExtraClaims(claims map[string]any) SignerOption
```

### 4.3 Token construction (no external dependencies)

The reference implementation constructs tokens using stdlib only:

```
token = base64url(header_json) + "." + base64url(payload_json) + "." + base64url(signature)
```

where:
- `header_json` = `{"alg":"EdDSA","typ":"JWT"}` (plus `"kid"` if configured)
- `payload_json` = the JSON-marshalled claim set
- `signature` = `ed25519.Sign(key, signingInput)` where `signingInput` = `base64url(header_json) + "." + base64url(payload_json)`
- `base64url` = `base64.RawURLEncoding.EncodeToString`

Imports used: `crypto/ed25519`, `encoding/json`, `encoding/base64`, `crypto/rand`
(for UUIDv7), `time` (for `iat`/`exp`). No external JWT library.

### 4.4 `Sign` return on empty `toolName`

If `toolName` is empty, `Sign` returns an empty string and `nil` error. The
`NullSigner` contract is that an empty string means "no identity assertion for
this call." The reference implementation follows this contract rather than
producing a token with an empty `praxis.tool_name` claim.

### 4.5 Framework does not validate returned tokens

The framework does not parse or validate the JWT string returned by `Sign`
before placing it in `InvocationContext.SignedIdentity`. The `Signer` is a
trusted component (see Phase 5 trust model); the framework trusts it to return
a well-formed token or an error. Validation on the return path would require
the framework to carry the public key, introducing a verification dependency
in the signing path.

---

## 5. Key lifecycle (D74)

### 5.1 Static key model

The reference implementation holds a single `ed25519.PrivateKey` for its
lifetime. All `Sign` calls use this key. The key is set at construction time
and does not change.

This model is appropriate for:
- Development and testing (fixed test keys).
- Deployments where the process restarts trigger key rotation (new key loaded
  at startup).
- Deployments where the private key is injected as an environment secret and
  does not need in-process rotation.

### 5.2 `kid` header support

The `kid` (key ID) header parameter in the JOSE header identifies which public
key should be used for verification. When `WithKeyID` is set, the reference
implementation includes `kid` in every token's JOSE header.

This enables callers to:
- Maintain a key set (JWK Set) with multiple public keys indexed by `kid`.
- Rotate keys by starting a new `Signer` instance with the new key and a new
  `kid`, without affecting tokens already in flight under the old `kid`.

The reference implementation does not implement key set management; it produces
tokens with a single, fixed `kid`.

### 5.3 Rotation pattern for callers

Callers who need in-process key rotation implement their own `Signer`:

```go
// Example: a rotating Signer that wraps a caller-controlled KeyProvider.
// The framework does not define KeyProvider; this is an illustration.
type rotatingSigner struct {
    provider KeyProvider // caller-defined interface
}

func (s *rotatingSigner) Sign(ctx context.Context, invocationID, toolName string) (string, error) {
    key, kid := s.provider.CurrentKey()
    signer, err := identity.NewEd25519Signer(key, identity.WithKeyID(kid))
    if err != nil {
        return "", err
    }
    return signer.Sign(ctx, invocationID, toolName)
}
```

The framework does not prescribe a `KeyProvider` interface because rotation
strategies vary widely. Callers who use KMS, HSM, or secret-manager rotation
implement their own key-acquisition logic.

**Caching note.** The example above constructs a new `Signer` on every `Sign`
call, which is safe but allocates. Callers with high `Sign` call rates should
cache the `Signer` and replace it when the `KeyProvider` signals a key change.
Caching is the caller's optimization to make; the framework does not constrain it.

### 5.4 Key revocation

Token revocation (maintaining a JTI blocklist) is out of scope for the
framework. Relying parties who need revocation maintain their own token store
keyed by `jti`. The UUIDv7 `jti` value is time-ordered, which simplifies
JTI-store cleanup by creation-time window.

The token lifetime (D72) is the primary expiry mechanism. Short lifetimes
(60s default) limit the exposure window for compromised tokens without
requiring a revocation store.

### 5.5 Private key lifetime in memory (open issue)

The `ed25519.PrivateKey` passed to `NewEd25519Signer` is stored in the
`Signer` struct for the struct's lifetime. The framework does not zero the
private key when the `Signer` is garbage collected.

This is an open issue for v1.x. Go does not provide a finalizer-based zeroing
guarantee (finalizers are not guaranteed to run, and the order is
non-deterministic). The long-lived private key creates a larger in-memory
lifetime exposure window than the short-lived `Credential` material (which is
zeroed on `Close()`).

**Mitigation for v1.0:** callers with strict key-material lifetime requirements
implement their own `Signer` backed by a KMS or HSM where the private key never
enters the process memory. The reference implementation's static key model is
explicitly not appropriate for these deployments.

---

## 6. CP6 identity chaining (D75)

### 6.1 Use case

When a `tools.Invoker` implementation calls a nested `AgentOrchestrator`, the
inner orchestrator may need to prove that it was invoked by an authorised outer
orchestrator. The identity chain allows the inner token to reference the outer
token, creating a verifiable chain of delegation.

### 6.2 How the inner Signer reads the outer token

The outer orchestrator places the signed token in
`tools.InvocationContext.SignedIdentity` before calling `tools.Invoker.Invoke`.
The inner orchestrator reads this value via its own `InvocationContext` (or
via context-propagated values). The inner `Signer` embeds the outer token:

```go
// Inner Signer implementation pattern (callers implement this):
type chainingSigner struct {
    base *identity.Ed25519Signer // or any Signer
}

func (s *chainingSigner) Sign(
    ctx context.Context,
    invocationID string,
    toolName string,
) (string, error) {
    outerToken := tools.InvocationContextFromContext(ctx).SignedIdentity
    opts := []identity.SignerOption{}
    if outerToken != "" {
        opts = append(opts, identity.WithExtraClaims(map[string]any{
            "praxis.parent_token": outerToken,
        }))
    }
    // Construct a per-call Signer with the extra claim, or add the claim
    // to the base Signer's pre-configured extra claims map.
    // ...
}
```

The framework does not implement `chainingSigner`. It is a caller-provided
`Signer` implementation. The framework's contribution is:
1. The `praxis.parent_token` claim name (documented in D71, D75).
2. The `InvocationContext.SignedIdentity` field that carries the outer token.

### 6.3 `praxis.parent_token` claim

The `praxis.parent_token` custom claim value is the complete outer token string
(compact JWS serialization: `header.payload.signature`). This embeds the full
verifiable outer token in the inner token's payload.

A verifier that wants to validate the chain:
1. Decodes the inner token and extracts `praxis.parent_token`.
2. Decodes and verifies the outer token against the outer public key.
3. Checks that the outer token's `sub` matches the expected outer invocation.

The chain is a singly-linked list terminated at a token with no
`praxis.parent_token` claim. Verifiers walk the chain iteratively.

### 6.4 Why not x5c-style chaining

RFC 7515's `x5c` header parameter carries a chain of X.509 certificates in
the JOSE header, not in the JWT payload. Emulating `x5c` for JWT chains would
require embedding JWTs in the JOSE header as a custom header parameter. This:
- Is non-standard (no RFC for JWT-in-JWT-header chaining).
- Requires verifiers to understand a praxis-specific header parameter that is
  not processed by standard JWT libraries.
- Embeds verification-time material (the chain) in the header rather than the
  payload, making it harder to inspect with standard tooling.

The `praxis.parent_token` payload claim is decoded by any JSON parser and
inspectable with any JWT debugging tool. It follows the JWT convention of
using payload claims for content and keeps the JOSE header minimal.

### 6.5 Chain depth

The framework does not enforce a chain depth limit. Enforcement would require
the `Signer` to parse and validate the outer token (counting chain depth),
which introduces a verification dependency in the signing path.

**Documentation recommendation.** Chain depth beyond 3 levels (outer +
2 nested) is discouraged. Deep chains produce larger tokens, add verification
latency, and indicate an architecture that may benefit from a different trust
model. Callers who need hard depth limits implement the check in their `Signer`
before constructing the token.

### 6.6 Empty outer token

If `InvocationContext.SignedIdentity` is empty (the outer orchestrator used
`NullSigner`, or no identity signing is configured), the inner Signer omits
`praxis.parent_token`. The inner token is valid as a standalone assertion;
verifiers that require a chain claim should reject it via their own policy,
not via token parsing.

---

## 7. `identity.Signer` promotion to `frozen-v1.0` (D76)

The `identity.Signer` interface is promoted from `stable-v0.x-candidate` to
`frozen-v1.0` effective Phase 5 sign-off.

**Gating conditions satisfied:**

| Condition | Satisfied by |
|---|---|
| JWT registered claim set specified | D70 |
| JWT custom claims specified | D71 |
| Token lifetime policy specified | D72 |
| Ed25519 reference impl contract specified | D73 |
| Key lifecycle documented | D74 |
| CP6 identity chaining resolved | D75 |

**What is frozen:**
- The `Signer` interface method signature:
  `Sign(ctx context.Context, invocationID string, toolName string) (string, error)`
- The `NullSigner` semantics (returns empty string, nil error).
- The `NewEd25519Signer` constructor signature and the `SignerOption` type.
- The `praxis.invocation_id` and `praxis.tool_name` custom claim keys.
- The `praxis.parent_token` chain claim key.
- The JOSE algorithm (`EdDSA`) and header structure.

**What is NOT frozen (extensible in v1.x):**
- The set of `SignerOption` constructors (new options may be added in minor
  releases without breaking existing callers).
- The deny-list of mandatory claim keys that `WithExtraClaims` cannot override
  (additions are backward-compatible; removals are not permitted).

**What requires a v2 module path to change:**
- The `Signer` interface method signature.
- The mandatory custom claim keys (`praxis.invocation_id`, `praxis.tool_name`).
- The JOSE algorithm (`EdDSA`).
- The `NewEd25519Signer` constructor signature.
