# Phase 6: Release, Versioning and Community Governance

## Goal

Specify the release process, CI pipeline, versioning policy, contribution model,
and community governance so that praxis can ship v0.1.0 through v1.0.0 with
repeatable quality gates, a clear contributor experience, and no ambiguity about
what "stable" means at each tag.

## Scope

**In scope** (refines seed §8–10, §14; carries forward items from Phases 1–5):

- **Semver policy and deprecation windows.** Concrete deprecation timeline for
  v1.x interfaces: how many minor releases, what "deprecated" looks like in
  godoc and `CHANGELOG.md`, removal rules requiring v2 module path.
- **v0 to v1.0 stability commitment.** What callers can rely on during v0.x,
  what they cannot, and how breaking changes are communicated.
- **v2+ module path rules.** When a v2 module path is required, how the
  transition is managed, what happens to v1.x maintenance.
- **Release process.** Conventional commits, release-please configuration,
  `CHANGELOG.md` generation, tag signing, release artifact checklist.
- **CI pipeline.** Complete specification: golangci-lint, `go test -race -cover`,
  `go test -bench` (change-triggered), `govulncheck`, CodeQL, banned-identifier
  grep (seed §6.1 + Phase 5 D79 additions), coverage gate (85%), property-based
  test iteration counts (10k CI / 100k nightly), LLM conformance suite, SPDX
  header check, D10 tripwire enforcement.
- **D10 tripwire enforcement.** Module path resolution process: what must happen
  before v0.1.0, what happens if resolution fails, fallback names.
- **Bus-factor mitigation (seed §14.1).** Concrete measures beyond "rigorous
  documentation": maintainer onboarding checklist, knowledge distribution
  strategy, minimum reviewer count policy.
- **First-production-consumer gating (seed §8).** The v1.0.0 precondition that
  a production consumer ships against v0.5.x. How this is verified, what
  "shipped in production" means, what happens if the consumer is delayed.
- **Contribution model.** CLA vs DCO, PR review requirements, commit message
  enforcement, branch protection rules, issue and PR templates.
- **Code of conduct.** Contributor Covenant 2.1 adoption, enforcement contacts,
  escalation path.
- **RFC process.** How post-v1.0 feature proposals are submitted, evaluated,
  and accepted or rejected. Lightweight — this is a library, not a platform.
- **`SECURITY.md` content.** Vulnerability disclosure process, responsible
  disclosure timeline, known limitations (OI-1: private key in-memory lifetime;
  OI-2: enricher attribute log-injection vector from Phase 5).
- **Package layout amendment.** Record `internal/jwt` addition (Phase 5
  go-architect) in the canonical package layout.
- **MetricsRecorder extension story.** Confirm the interface extension approach
  for v1.x (Phase 4 REVIEW.md OQ2).
- **Claim constant enforcement.** Whether `praxis.invocation_id`,
  `praxis.tool_name`, `praxis.parent_token` must be package-level constants
  enforced by CI (Phase 5 REVIEW.md OQ2).
- **Release milestones.** Concrete exit criteria for v0.1.0, v0.3.0, v0.5.0,
  v1.0.0 — refining seed §8 with all decisions from Phases 1–5.

**Out of scope:**

- Implementation code, CI YAML, Makefile targets — those are implementation
  artifacts, not design decisions.
- Governance beyond the library (no foundation, no steering committee, no
  working groups — this is a single-maintainer-team OSS library).
- Marketing, website, social media strategy.
- Licensing decisions — Apache 2.0 is settled (seed §10).

## Key Questions

1. What is the concrete deprecation window for v1.x interfaces? Seed §9 says
   "at least two subsequent minor releases" — is that sufficient, or should it
   be calendar-based (e.g., 6 months)?
2. Should release-please run on `main` only, or also on a `release/v0.x`
   branch for backport patches?
3. What is the minimum reviewer count for PRs that touch `frozen-v1.0`
   interfaces vs. internal code?
4. CLA or DCO? CLA adds friction but protects IP assignment; DCO
   (Developer Certificate of Origin) is lighter and preferred by most Go
   projects.
5. How is the "first production consumer shipped" precondition for v1.0.0
   verified? Self-attestation, public reference, or something else?
6. Should the RFC process be GitHub Discussions, a dedicated `rfcs/` directory
   with numbered markdown files, or GitHub Issues with an `rfc` label?
7. What is the responsible disclosure timeline in `SECURITY.md`? 90 days
   (industry standard) or shorter?
8. Should the banned-identifier CI grep cover test files, or only production
   code? (Test fixtures might legitimately reference banned terms in
   assertion messages.)
9. What nightly CI cadence is appropriate for the 100k-iteration property-based
   tests and the LLM conformance suite (which requires live API credentials)?
10. Should v0.x minor releases go through a formal phase-style review (seed
    §14.2), or is PR review sufficient?

## Decisions Required

| ID | Topic | Context |
|---|---|---|
| D81 | Deprecation window policy | Seed §9 says "at least two subsequent minor releases." Confirm or amend with calendar floor. |
| D82 | Release branch strategy | Single `main` branch with release-please vs. `release/v0.x` branches for backports. |
| D83 | Conventional commit enforcement | How commits are validated (CI check, pre-commit hook, or both). Scope of commit types. |
| D84 | Release-please configuration | Which release-please strategy (`simple`, `go-yoshi`, custom), changelog sections, versioning source. |
| D85 | CI pipeline specification | Complete job list, triggers, gates. Includes banned-identifier grep scope, coverage threshold, benchmark policy. |
| D86 | Coverage gate policy | 85% line coverage (seed §10). Confirm scope (public packages only, or including `internal/`). |
| D87 | Property-based test CI policy | 10k iterations in PR CI, 100k nightly. Confirm runner requirements and failure policy. |
| D88 | LLM conformance suite CI policy | When the conformance suite runs, credential management, failure-as-blocker vs. warning. |
| D89 | D10 tripwire enforcement | Concrete steps and deadline for module path resolution. What blocks v0.1.0. |
| D90 | Bus-factor mitigation | Minimum maintainer count, onboarding checklist, knowledge distribution. |
| D91 | First-production-consumer gate | Verification mechanism for the v1.0.0 precondition. |
| D92 | CLA vs DCO | Contributor agreement model. |
| D93 | PR review policy | Minimum reviewers, approval requirements for frozen-surface vs. internal changes. |
| D94 | Branch protection rules | Required checks, merge strategy (squash, rebase, merge commit). |
| D95 | RFC process | Mechanism for post-v1.0 feature proposals. |
| D96 | `SECURITY.md` content and disclosure timeline | Vulnerability reporting channel, response timeline, known limitations. |
| D97 | SPDX header enforcement | How Apache 2.0 SPDX headers are enforced across source files. |
| D98 | Tag signing policy | Whether release tags are GPG-signed, and key management. |
| D99 | Package layout amendment | Record `internal/jwt` in canonical layout; confirm no other Phase 5 additions missed. |
| D100 | MetricsRecorder extension story | Confirm the `MetricsRecorderV2` embedding approach for v1.x metric additions. |
| D101 | Claim constant CI enforcement | Whether JWT claim keys must be package-level constants checked by CI. |
| D102 | v0.x breaking-change communication | How breaking changes in v0.x minors are communicated beyond `CHANGELOG.md`. |
| D103 | Release milestone exit criteria | Concrete checklists for v0.1.0, v0.3.0, v0.5.0, v1.0.0. |
| D104 | v0.x interface-change review | Seed §14.2 mitigation: decisions-log entry for interface changes during v0.x. |
| D105 | Benchmark baseline cache workflow | Post-merge workflow that caches benchmark results for PR comparison. |

## Assumptions

- **release-please is the release automation tool.** Seed §10 names it
  explicitly. Assumed to be the correct choice; the solution-researcher
  subagent should validate this against alternatives (GoReleaser, manual
  tagging, goreleaser + release-please combo).
- **GitHub Actions is the CI platform.** Not stated in the seed but implied
  by the `.github/workflows/` directory in seed §7.
- **Single maintainer team at launch.** Bus-factor mitigation is about
  preparing for growth, not about having multiple maintainers on day one.
  *Weak assumption* — if a second maintainer is available before v0.1.0,
  the bus-factor decisions change.
- **No paid CI infrastructure.** GitHub Actions free tier (or OSS credits)
  is sufficient for the CI pipeline. The LLM conformance suite may need
  secrets management for API keys.
- **Contributor Covenant 2.1 is the code of conduct.** Stated in seed §10.
  No alternatives considered.

## Risks

**Critical (block v1.0 or break decoupling contract):**

- **R1 — D10 unresolved at v0.1.0.** The module path is conditional (D10).
  If the GitHub org and brand review are not completed before v0.1.0, the
  first public tag cannot ship. This is the only remaining external blocker
  for v0.1.0.
- **R2 — First-production-consumer delay.** v1.0.0 is gated on a production
  consumer shipping against v0.5.x (seed §8). If the consumer is delayed,
  v1.0.0 is delayed. This is by design but must be documented as an explicit
  dependency.

**Secondary:**

- **R3 — CI credential management for LLM conformance suite.** Running
  Anthropic/OpenAI API calls in CI requires secrets. Leaking secrets or
  running against production APIs with unbounded budgets is a cost and
  security risk.
- **R4 — Release-please configuration complexity.** Go module releases have
  nuances (major version suffixes, `go.sum` handling) that release-please
  must handle correctly. Misconfiguration can produce invalid tags.
- **R5 — Over-governance for a small project.** The RFC process, CLA/DCO,
  and review policies must be proportional to the project size. Over-engineering
  governance creates friction that discourages early contributors.

## Deliverables

- `00-plan.md` — this file.
- `01-decisions-log.md` — D81–D103 decisions with rationale.
- `02-release-process.md` — conventional commits, release-please, changelog,
  tag signing, release checklist per milestone.
- `03-ci-pipeline.md` — complete CI specification: jobs, triggers, gates,
  banned-identifier grep, coverage, benchmarks, conformance suite, nightly.
- `04-versioning-policy.md` — semver rules, deprecation windows, v0 contract,
  v1.0 freeze, v2+ module path, breaking-change communication.
- `05-contribution-and-governance.md` — CLA/DCO, PR review policy, branch
  protection, code of conduct, RFC process, `SECURITY.md` content,
  bus-factor mitigation, maintainer onboarding.
- `06-release-milestones.md` — exit criteria checklists for v0.1.0, v0.3.0,
  v0.5.0, v1.0.0, incorporating all decisions from Phases 1–5.
- `REVIEW.md` — final review verdict.

## Recommended Subagents

1. **solution-researcher** — validate release-please as the correct tool for
   a Go library release workflow; survey CLA vs DCO adoption in comparable
   Go OSS projects; check if GoReleaser should complement release-please.

2. **dx-designer** — review the contributor experience: PR templates, issue
   templates, the `CONTRIBUTING.md` outline, the RFC process ergonomics, and
   the v0.1.0 "hello world" developer journey from `go get` to first
   successful invocation.

## Exit Criteria

1. All decisions D81–D103 adopted with rationale.
2. CI pipeline fully specified (no "TBD" jobs).
3. Release milestones for v0.1.0, v0.3.0, v0.5.0, v1.0.0 have concrete
   exit criteria incorporating all Phase 1–5 decisions.
4. D10 tripwire enforcement mechanism specified (even if D10 itself is not
   yet resolved).
5. `SECURITY.md` content specified including OI-1 and OI-2.
6. `internal/jwt` recorded in canonical package layout.
7. MetricsRecorder extension story confirmed.
8. Reviewer subagent PASS.
9. `REVIEW.md` verdict: READY.
10. No banned-identifier leakage in phase artifacts (decoupling grep clean).
