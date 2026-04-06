# Review: Phase 5 — Security and Trust Boundaries

## Overall Assessment

Phase 5 delivers a thorough security specification: 14 decisions (D67–D80),
26 security invariants with traceability, and concrete resolutions for both
forward-carried concerns (C4, CP6). The credential zeroing technique is well-
grounded in Go's memory model, the identity signing contract is complete
enough to freeze `identity.Signer`, and the trust boundary classification
between pre-LLM and post-tool filters is clearly reasoned. After two rounds
of review (reviewer subagent + review-phase), all critical and important
issues have been resolved in-place.

## Critical Issues

None remaining.

The following critical issue was found and fixed during review:

1. **`InvocationContext.Credential` phantom field (fixed).** Three Phase 5
   documents (`02-credential-lifecycle.md` §3.2, `04-trust-boundaries.md` §3.1,
   `05-security-invariants.md` C3) referenced a `tools.InvocationContext.Credential`
   field that does not exist in Phase 3's definition (D36). `InvocationContext`
   has `InvocationID`, `Budget`, `SpanContext`, `SignedIdentity`, and `Metadata` —
   no `Credential` field. The credential is fetched and used within the tool-call
   goroutine but is not propagated via `InvocationContext`. All three references
   have been corrected to accurately describe the credential isolation mechanism
   without referencing this non-existent field.

## Important Weaknesses

1. **Soft-cancel timeout is absolute 500ms, not remaining grace budget.**
   D69 applies a fresh 500ms `context.WithTimeout` at the `Fetch` call site.
   If the tool-dispatch goroutine consumed time before reaching `Fetch`, the
   total grace window exceeds Phase 2 D21's 500ms specification. Practical
   impact is small (worst case ~700ms). This is an acknowledged semantic gap,
   documented here for implementation-time attention. Not a blocker.

2. **Private key in-memory lifetime (OI-1).** The `ed25519.PrivateKey` in
   `Ed25519Signer` is not zeroed on GC. Documented in `04-trust-boundaries.md`
   §5 and `03-identity-signing.md` §5.5. Callers with strict requirements
   should use KMS/HSM-backed Signer implementations. Not a blocker for v1.0.

3. **Enricher attribute log-injection vector (OI-2).** Caller-provided
   `AttributeEnricher` values may contain sensitive data that the framework
   cannot redact by key pattern alone. Documented in `04-trust-boundaries.md`
   §5 as caller responsibility. Not a framework design flaw.

4. **Credential delivery to `tools.Invoker` is under-specified.** With the
   `InvocationContext.Credential` phantom field corrected, the mechanism by
   which the orchestrator passes the fetched `Credential` to the `Invoker`
   is not fully specified. Phase 3 defines `Resolver.Fetch` as called by the
   orchestrator, and `Credential.Close()` as deferred in the same goroutine,
   but the `Invoker.Invoke` signature does not accept a `Credential` parameter.
   The credential is likely passed via the `ctx context.Context` argument or
   the invoker is expected to call `Resolver.Fetch` itself. This ambiguity
   should be resolved at implementation time. It does not affect the security
   invariants (the goroutine-scope isolation holds regardless of the delivery
   mechanism).

## Open Questions

1. Should the credential delivery mechanism be an explicit `Credential`
   parameter on `Invoker.Invoke`, a context value, or should the invoker
   call `Resolver.Fetch` directly? This was ambiguous in Phase 3 and remains
   so. It affects the `frozen-v1.0` surface if it changes `Invoker.Invoke`.

2. Should Phase 6's CI pipeline enforce that `praxis.invocation_id`,
   `praxis.tool_name`, and `praxis.parent_token` are package-level
   constants rather than inline string literals?

## Decoupling Contract Check

**PASS.** Case-insensitive grep for `custos`, `reef`, `governance.event`,
`governance_event`, `org.id`, `agent.id`, `user.id`, `tenant.id` across all
Phase 5 artifacts returns matches only in REVIEW.md's compliance declaration
(negation-mentions). No actual identifier leakage anywhere.

## Recommendations

- At implementation time, resolve the credential delivery mechanism (OQ1
  above) and back-annotate Phase 3 `06-tools-and-invocation-context.md` if
  the answer changes the `Invoker.Invoke` signature.
- Add `internal/jwt` to the canonical package layout (Phase 3
  `go-architect-package-layout.md`) as a back-annotation or in Phase 6.
- Document OI-1 and OI-2 in `SECURITY.md` (Phase 6 scope) as known
  limitations of the initial release.
- At implementation time, verify that the 500ms grace-window timing for
  credential fetch (weakness 1) does not create surprising shutdown latency
  in the integration test suite.

## Verdict: READY

All 14 decisions (D67–D80) are adopted with rationale. C4 and CP6 are
resolved. `identity.Signer` is promoted to `frozen-v1.0`. The RedactingHandler
deny-list is extended. 26 security invariants are enumerated with traceability.
The decoupling contract is clean. The `InvocationContext.Credential` phantom
field has been corrected. Two open issues (OI-1, OI-2) are documented and
survivable. Four forward-carried concerns are flagged for Phase 6. Phase 5
may close.
