# Review: Phase 6 — Release, Versioning and Community Governance

## Overall Assessment

Phase 6 delivers the final planning artifact: 25 decisions (D81–D105)
covering the release process, CI pipeline, versioning policy, contribution
model, community governance, and concrete release milestone checklists for
v0.1.0 through v1.0.0. All forward-carried items from Phases 1–5 are
addressed. After one round of reviewer feedback, three issues were resolved
in-place (seed §14.2 omission, D101 enforcement misrepresentation, bench
cache gap), bringing the phase to approval readiness.

## Critical Issues

None remaining.

The following critical and important issues were found and fixed during
review:

1. **Seed §14.2 unaddressed (fixed).** The seed mandates that v0.x minor
   releases with interface changes go through a decisions-log review. This
   was raised as Key Question 10 in `00-plan.md` but had no corresponding
   decision. Fixed by adding D104, which requires a decisions-log entry in
   `docs/decisions/v0.x-interface-changes.md` for any PR that changes an
   exported interface method signature during v0.x.

2. **D101 enforcement claim incorrect (fixed).** D101 originally claimed
   that `goconst` at `min-occurrences: 3` would catch inline string literal
   drift for JWT claim keys. This is factually incorrect — `goconst` cannot
   detect a single literal replacing a constant reference. Fixed by
   rewriting D101 to state that enforcement is architectural (single
   consumer site in `identity/`), supplemented by `goconst` for duplication
   detection.

3. **Bench cache workflow undefined (fixed).** The bench job referenced a
   cached baseline "stored by the post-merge workflow" that did not exist.
   Fixed by adding D105 and a `post-merge-bench` workflow specification in
   `03-ci-pipeline.md` §5.

## Important Weaknesses

1. **D88 vs seed §10 reconciliation is implicit.** D88 says the conformance
   suite "does not block PR merges." Seed §10 says "divergence between
   adapters is a release blocker." These are reconciled at the milestone
   gate (v0.5.0 requires conformance green), but the reconciliation is
   implicit. Future readers may be confused about why a non-blocking PR
   job satisfies a "release blocker" requirement. Not a design flaw — the
   milestone gate is the correct enforcement point — but worth a
   clarifying note at implementation time.

2. **24-hour self-merge waiting period is honor-system.** D93 specifies a
   24-hour cooling-off period for the sole maintainer's self-merge of
   exported-symbol changes. GitHub has no mechanism to enforce this. D93
   now explicitly states this is a process commitment, not a
   platform-enforced gate.

3. **D91 consumer disclosure section.** D91 now specifies that the
   production consumer attestation goes in a dedicated "Production
   Consumer" section of the v1.0.0 release notes, satisfying the
   CLAUDE.md constraint on consumer naming in dedicated sections.

## Open Questions

1. **Credential delivery mechanism.** Phase 5 REVIEW.md OQ1 asks whether
   the credential is passed to `Invoker.Invoke` as a parameter, context
   value, or whether the invoker calls `Resolver.Fetch` directly. This
   remains open from Phase 5 and is deferred to implementation time. It
   may affect the `frozen-v1.0` surface if it changes `Invoker.Invoke`.

2. **`praxis_errors_total` naming.** Phase 4 REVIEW.md M3 noted that the
   metric name "errors total" counts `approval_required` outcomes which
   are not errors. This naming concern is deferred to post-v1.0.

## Decoupling Contract Check

**PASS.** Case-insensitive grep for `custos`, `reef`, `governance.event`,
`governance_event`, `org.id`, `agent.id`, `user.id`, `tenant.id` across
all Phase 6 artifacts returns one match: `03-ci-pipeline.md` §2.5 contains
the banned-identifier grep's own `BANNED` shell variable definition, which
lists the banned terms as the subject of the enforcement check. This is a
negation-mention inside a compliance implementation — the same pattern
accepted in Phases 1–5.

No actual identifiers or hardcoded attribute keys leak anywhere.

## Exit Criteria Verification

| # | Criterion | Status |
|---|---|---|
| 1 | All decisions D81–D105 adopted with rationale | PASS — 25 decisions |
| 2 | CI pipeline fully specified (no "TBD" jobs) | PASS — bench cache resolved (D105) |
| 3 | Release milestones with concrete exit criteria | PASS |
| 4 | D10 tripwire enforcement specified | PASS — D89 |
| 5 | SECURITY.md content including OI-1 and OI-2 | PASS — D96 |
| 6 | internal/jwt in canonical package layout | PASS — D99 |
| 7 | MetricsRecorder extension story confirmed | PASS — D100 |
| 8 | Reviewer subagent PASS | PASS — issues resolved |
| 9 | REVIEW.md verdict | this document |
| 10 | No banned-identifier leakage | PASS |

## Forward-Carried Items Resolution

| Source | Item | Resolution |
|---|---|---|
| Phase 4 REVIEW.md OQ2 | MetricsRecorder extension story | D100 |
| Phase 5 REVIEW.md OQ2 | Claim constant CI enforcement | D101 |
| Phase 5 REVIEW.md rec. | Add internal/jwt to package layout | D99 |
| Phase 5 REVIEW.md rec. | Document OI-1 and OI-2 in SECURITY.md | D96 |
| Roadmap-status.md | internal/jwt package addition | D99 |
| Roadmap-status.md | Confirm MetricsRecorder extension story | D100 |
| Roadmap-status.md | D10 tripwire enforcement | D89 |
| Roadmap-status.md | Bus-factor mitigation | D90 |
| Roadmap-status.md | First-production-consumer gating | D91 |
| Seed §14.2 | v0.x interface-change review | D104 |

All forward-carried items are resolved.

## Verdict: READY

All 25 decisions (D81–D105) are adopted with rationale. The CI pipeline
is fully specified with no undefined workflows. Release milestones for
v0.1.0, v0.3.0, v0.5.0, and v1.0.0 have concrete exit criteria
incorporating all Phase 1–5 decisions. The seed §14.2 mitigation is
implemented. The decoupling contract is clean. Two open questions from
prior phases (credential delivery, metric naming) are carried to
implementation time with documented rationale for deferral.

Phase 6 may close. **All six planning phases are now approved.
Implementation may begin after D10 resolution.**
