# Phase 5: Security and Trust Boundaries

## Goal

Specify the credential lifecycle, identity signing contract, untrusted-output
handling model, and filter trust boundaries so that praxis enforces security
invariants by construction rather than by documentation.

## Scope

**In scope** (refines seed §5 `credentials.Resolver`, `identity.Signer`; seed
§4.3 step 12–14; seed §6):

- **Credential lifecycle:** zero-on-close technique, `Credential.Value()` access
  rules, GC interaction, runtime isolation invariant ("credential values never
  escape the tool-call goroutine scope").
- **Soft-cancel credential resolution (C4):** full specification of the
  `context.WithoutCancel` requirement documented in Phase 2 D21 §2.4 and
  Phase 3 `09-credentials-and-identity.md` §Security properties.
- **Identity signing:** JWT claim set (registered + custom claims), token
  lifetime, key lifecycle (rotation, revocation), Ed25519 reference impl
  contract, promotion of `identity.Signer` from `stable-v0.x-candidate` to
  `frozen-v1.0`.
- **Identity chaining (CP6):** how a nested orchestrator's Signer reads
  `InvocationContext.SignedIdentity` to construct an identity chain claim.
- **Untrusted tool output handling:** the trust model for `ToolResult.Content`
  flowing back through `PostToolFilter` — what the framework guarantees vs. what
  the filter implementor must handle.
- **Filter hook trust model:** which hooks are trust boundaries (pre-LLM filters
  trust the input; post-tool filters distrust the input) and what that means for
  error handling and telemetry.
- **RedactingHandler extension review:** whether Phase 4's deny-list (D58) needs
  additional entries based on credential and identity field patterns discovered
  in this phase.
- **Security properties summary:** a single document enumerating all security
  invariants enforced by the framework, cross-referenced to the decisions that
  establish them.

**Out of scope:**

- Concrete Signer implementations beyond the reference `Ed25519Signer` contract
  (KMS integrations, HSM drivers are caller-level concerns).
- Network-level security (TLS configuration, mTLS between services).
- Authentication/authorization of callers invoking the orchestrator (that is the
  caller's HTTP/RPC layer).
- Vault integration specifics (HashiCorp Vault, AWS Secrets Manager, etc.) —
  these are `credentials.Resolver` implementations, not framework concerns.
- Prompt-injection detection algorithms — the framework provides the
  `PostToolFilter` seam; detection logic is the filter implementor's concern.

## Key Questions

1. What zeroing technique should `Credential.Close()` use? Direct byte-slice
   overwrite, or a `runtime.KeepAlive`-guarded pattern to prevent the compiler
   from optimizing away the zero write?
2. Should the framework provide a `credential.ZeroBytes([]byte)` utility, or is
   zeroing entirely the `Credential` implementor's responsibility?
3. What are the required JWT registered claims (`iss`, `sub`, `aud`, `exp`,
   `iat`, `jti`) and what custom claims does the framework mandate vs. recommend?
4. What is the token lifetime policy? Fixed 60s? Configurable? Per-call vs.
   per-invocation?
5. How does the Ed25519 reference implementation handle key rotation — does it
   accept a `KeyProvider` interface, or a static key pair?
6. For CP6 identity chaining, does the inner Signer embed the outer JWT as a
   `parent_token` claim, or does it use a separate `x5c`-style chain header?
7. Does Phase 4's `RedactingHandler` deny-list need new entries for
   `praxis.signed_identity` or `praxis.jwt_*` field patterns?
8. Should the framework validate JWT structure (well-formed, not expired) on
   the `Sign` return path, or is the Signer trusted to return valid tokens?

## Decisions Required

- **D67 — Credential zeroing technique.** Concrete `Close()` implementation
  contract: byte-slice overwrite with compiler-optimization guard.
- **D68 — `credential.ZeroBytes` utility.** Whether the framework ships a
  zeroing helper in the `credentials` package.
- **D69 — C4 resolution: soft-cancel credential context.** Full specification
  of the `context.WithoutCancel` + 500ms timeout pattern for
  `credentials.Resolver.Fetch` during soft cancel.
- **D70 — JWT registered claim set.** Which registered claims are mandatory
  (`iss`, `sub`, `exp`, `iat`, `jti`) and which are optional (`aud`, `nbf`).
- **D71 — JWT custom claims.** Framework-defined custom claims
  (`invocation_id`, `tool_name`) and the extension point for caller claims.
- **D72 — Token lifetime policy.** Fixed or configurable; scope (per-call).
- **D73 — Ed25519 reference implementation contract.** `NewEd25519Signer`
  constructor, key input, and what it commits to.
- **D74 — Key rotation model.** Whether the reference impl accepts a
  `KeyProvider` interface or a static key, and the key-ID (`kid`) header
  contract.
- **D75 — CP6 identity chaining claim structure.** How inner Signers reference
  the outer invocation's signed identity.
- **D76 — `identity.Signer` promotion to `frozen-v1.0`.** Formal tier change
  after claim set and key lifecycle are specified.
- **D77 — Untrusted tool output trust model.** Explicit statement of what the
  framework guarantees about `ToolResult.Content` and what `PostToolFilter`
  must handle.
- **D78 — Filter trust boundary classification.** Which filters are
  trust-boundary-crossing (post-tool) vs. trust-boundary-internal (pre-LLM).
- **D79 — RedactingHandler deny-list amendments.** Additional entries (if any)
  for identity/credential patterns discovered in this phase.
- **D80 — Security invariants summary.** Enumeration of all framework-enforced
  security properties with decision cross-references.

## Assumptions

- The Ed25519 JWT approach from the seed is confirmed. No RSA/ECDSA alternative
  is needed for v1.0 (the `Signer` interface is algorithm-agnostic; the reference
  impl is Ed25519-specific).
- `context.WithoutCancel` (Go 1.21+) is available (project minimum is Go 1.23+).
- The `golang.org/x/crypto/ed25519` package (or `crypto/ed25519` from stdlib) is
  sufficient; no external JWT library is vendored by the framework (the reference
  impl constructs JWTs manually or uses a minimal internal helper).
- **Weak assumption:** A static key pair is sufficient for the reference
  implementation. Production consumers with key rotation needs implement their
  own `Signer` backed by their KMS. This needs validation against the claim-set
  design.

## Risks

**Critical:**

- **Credential zeroing may be optimized away by the Go compiler.** If
  `Credential.Close()` writes zeros to a byte slice that is immediately
  unreachable, the compiler or GC may elide the write. The zeroing technique
  must be robust against this (e.g., `runtime.KeepAlive` or a volatile-style
  write pattern). Failure here is a real security vulnerability.

**Secondary:**

- **JWT library dependency.** If the reference `Ed25519Signer` requires JWT
  construction, the framework either vendors a JWT helper or depends on a
  third-party package. A third-party dependency in the `identity` package adds
  supply-chain risk. Mitigation: Ed25519 JWTs are simple enough to construct
  with `encoding/json` + `crypto/ed25519` + `encoding/base64`.
- **Identity chaining (CP6) complexity.** The chain-claim design must be simple
  enough that callers can implement it without understanding JWT internals
  beyond the praxis contract. Over-specification risks making the identity
  package opaque.
- **RedactingHandler deny-list scope creep.** Adding too many patterns risks
  false-positive redaction of legitimate log fields. The deny-list must remain
  narrow and framework-scoped.

## Deliverables

- `00-plan.md` — this document.
- `01-decisions-log.md` — D67–D80.
- `02-credential-lifecycle.md` — zeroing technique, access rules, GC
  interaction, runtime isolation invariant, C4 resolution.
- `03-identity-signing.md` — JWT claim set, token lifetime, Ed25519 reference
  impl contract, key lifecycle, CP6 identity chaining.
- `04-trust-boundaries.md` — untrusted tool output model, filter trust
  classification, credential isolation at log/span boundaries.
- `05-security-invariants.md` — enumerated security properties with decision
  cross-references.
- `REVIEW.md` — final review with decoupling grep and verdict.

## Recommended Subagents

1. **security-architect** — owns credential zeroing technique, trust boundary
   classification, identity chain design, and the security invariants summary.
   This is the primary domain expert for Phase 5.
2. **go-architect** — validates that the credential lifecycle and Ed25519
   reference impl align with Go stdlib patterns (`crypto/ed25519`,
   `runtime.KeepAlive`, `context.WithoutCancel`) and that the `identity`
   package's dependency footprint stays minimal.

## Exit Criteria

1. All decisions D67–D80 are adopted with rationale and alternatives considered.
2. C4 (soft-cancel credential resolution) is fully resolved with a concrete
   context-derivation specification.
3. CP6 (identity chaining) is fully resolved with a concrete claim structure.
4. `identity.Signer` is promoted to `frozen-v1.0` with the gating conditions
   (JWT claim set, key lifecycle) satisfied.
5. The `RedactingHandler` deny-list is reviewed and any amendments are recorded.
6. The security invariants summary covers all framework-enforced properties
   with traceable decision cross-references.
7. The reviewer subagent returns PASS.
8. `REVIEW.md` verdict is READY.
9. No banned-identifier leakage in any Phase 5 artifact.
