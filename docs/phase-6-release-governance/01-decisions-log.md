# Phase 6 — Decisions Log

**Phase:** Release, Versioning and Community Governance
**Range owned:** D81–D105 (contiguous)
**Status:** decided, pending reviewer confirmation
**Starting baseline:** `docs/PRAXIS-SEED-CONTEXT.md` §8–10, §14; Phases 1–5
decisions (D01–D80)

---

## D81 — Deprecation window policy

**Status:** decided
**Summary:** Two minor releases AND six calendar months, whichever is longer.
Removal requires a v2 module path.

**Decision.** A v1.x interface, type, or exported function may be marked
deprecated in any minor release. The deprecated symbol must remain functional
for at least **two subsequent minor releases** and at least **six calendar
months** from the release that introduced the deprecation notice, whichever is
longer. Removal of the symbol is itself a breaking change requiring a v2 module
path (`github.com/praxis-go/praxis/v2`).

Deprecation is signalled by:
1. A `// Deprecated:` godoc comment on the symbol (per Go convention).
2. A `Deprecated` entry in `CHANGELOG.md` for the release that marks it.
3. A `staticcheck` `SA1019` lint that warns callers at build time.

**Rationale.** Seed §9 says "at least two subsequent minor releases." For a
library with infrequent releases, two minors could be separated by a week
(if both are patch-driven) or a year. The six-month calendar floor gives
consumers a predictable minimum migration window regardless of release
cadence. The dual threshold matches the approach used by `grpc-go` and
`google-cloud-go`.

**Alternatives considered.** (a) Two minors only (seed §9 verbatim) — rejected;
insufficient for slow-release cadences. (b) One year — rejected; excessive for
a small library, delays v2 module path work.

---

## D82 — Release branch strategy

**Status:** decided
**Summary:** Single `main` branch. No release branches in v0.x or v1.0.
Release branches introduced only if v1.x maintenance is needed after v2.0.0.

**Decision.** All development happens on `main`. release-please monitors `main`
and opens release PRs against it. There are no `release/v0.x` or `release/v1.x`
branches during the v0.x→v1.0 lifecycle.

If a v2.0.0 is ever shipped and v1.x maintenance is required, a
`release/v1.x` branch is created from the last v1.x tag at that time. This
is a future concern — the branch strategy is not pre-built.

**Rationale.** Backport branches add merge-conflict overhead and
cherry-pick discipline that a single-maintainer project cannot sustain. The
Go module system's compatibility guarantees (`go get module@v1.2.3`) make
backport branches less necessary than in ecosystems without immutable
versioned imports.

**Alternatives considered.** (a) `release/v0.x` branch from day one — rejected;
no consumer needs v0.x maintenance patches while v0.x+1 is available. (b)
Git flow — rejected; designed for binaries with deployment pipelines, not
libraries with semver tags.

---

## D83 — Conventional commit enforcement

**Status:** decided
**Summary:** `commitsar` as a required PR check. Validates all commits in the
PR branch use conventional commit format.

**Decision.** Conventional commits are enforced on PRs targeting `main` via
`commitsar` (Go-native, MIT license). The tool validates that every commit in
the PR follows the format `<type>(<scope>): <description>`.

Allowed types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `perf`,
`ci`, `build`.

Scope is optional; when used, it should be the package name (e.g., `budget`,
`orchestrator`, `llm/anthropic`).

Breaking changes are signalled by a `BREAKING CHANGE:` footer, not by `!`
suffix (release-please recognises the footer form reliably across versions).

**Rationale.** release-please generates changelogs from conventional commits.
Without enforcement, non-conventional commits silently degrade changelog
quality. `commitsar` is Go-native (no npm runtime), MIT-licensed, and runs
as a single binary in CI.

**Alternatives considered.** (a) `commitlint` — rejected; requires Node.js
runtime. (b) No enforcement, rely on contributor discipline — rejected;
changelog quality is a public artifact that cannot be retroactively fixed.

---

## D84 — release-please configuration

**Status:** decided
**Summary:** `release-type: go`, single component on `main`, with a
`version.go` extra-file for runtime version reporting.

**Decision.** release-please runs via `googleapis/release-please-action@v4` on
pushes to `main`. Configuration:

```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "packages": {
    ".": {
      "release-type": "go",
      "bump-minor-pre-major": true,
      "bump-patch-for-minor-pre-major": false,
      "changelog-sections": [
        {"type": "feat", "section": "Added"},
        {"type": "fix", "section": "Fixed"},
        {"type": "docs", "section": "Documentation"},
        {"type": "perf", "section": "Performance"},
        {"type": "refactor", "section": "Changed"},
        {"type": "test", "section": "Testing"},
        {"type": "chore", "section": "Maintenance", "hidden": true},
        {"type": "ci", "section": "CI", "hidden": true},
        {"type": "build", "section": "Build", "hidden": true}
      ],
      "extra-files": ["internal/version/version.go"]
    }
  }
}
```

The `internal/version/version.go` file contains a `const Version = "X.Y.Z"`
constant updated by release-please on each release PR. This allows
`praxis.Version()` or similar runtime inspection without manual maintenance.

**Rationale.** The `go` release type is the correct strategy for Go modules:
the canonical version is the git tag, not a file. `bump-minor-pre-major`
ensures that pre-v1 breaking changes stay on the minor line rather than
jumping to `v1.0.0`, while `bump-patch-for-minor-pre-major: false` preserves
the standard `feat -> minor`, `fix/perf -> patch` behavior. praxis layers one
additional workflow rule on top: when the manifest version is still `< 1.0.0`,
any unreleased feature or breaking change is released as the next odd minor
milestone (`v0.1.x -> v0.3.0`, `v0.3.x -> v0.5.0`). The changelog sections are
remapped to the keep-a-changelog convention (Added/Fixed/Changed) from
release-please's default section names.

**Alternatives considered.** (a) `simple` release type — rejected; does not
understand Go versioning conventions. (b) GoReleaser — rejected; designed for
binary releases, adds no value for a library. (c) Manual tagging — rejected;
error-prone and does not generate changelogs.

---

## D85 — CI pipeline specification

**Status:** decided
**Summary:** Six required PR checks, two informational PR checks, two
scheduled jobs, one release workflow.

**Decision.** Full specification in
[`03-ci-pipeline.md`](03-ci-pipeline.md). Summary of jobs:

**On every PR (6 required, 2 informational):**
1. `lint` — golangci-lint with curated `.golangci.yml` **(required)**
2. `test` — `go test -race -cover ./...` with 85% coverage gate **(required)**
3. `commitsar` — conventional commit validation **(required)**
4. `banned-grep` — banned-identifier grep (seed §6.1 + D79 additions) **(required)**
5. `spdx-check` — SPDX header presence on all `.go` files **(required)**
6. `dco` — probot/dco contributor agreement **(required)**
7. `bench` — `go test -bench ./...` with benchstat comparison **(informational)**
8. `govulncheck` — vulnerability scan (informational in v0.x, required in v1.x)

**Scheduled (nightly):**
8. `property-tests` — 100k-iteration state machine property tests
9. `conformance` — LLM provider conformance suite (requires API credentials)

**On release-please merge:**
10. `release` — tag creation, GitHub release with changelog

**Rationale.** The pipeline mirrors the seed §10 specification with concrete
job definitions. Benchmark CI is informational (not blocking) because
GitHub Actions runners have known variance. `govulncheck` is informational
during v0.x to avoid false-positive blocks from transitive dev dependencies;
it becomes blocking at v1.0 when the dependency tree is stable.

---

## D86 — Coverage gate policy

**Status:** decided
**Summary:** 85% line coverage on all packages (public + `internal/`).
Measured by `go test -cover ./...`.

**Decision.** The coverage gate applies to the entire module (`./...`),
including `internal/` packages. The threshold is 85% line coverage, enforced
by CI. Coverage is measured by `go test -coverprofile=coverage.out ./...`
and verified with `go tool cover -func=coverage.out`.

Exclusions:
- `examples/` directory is excluded from coverage measurement.
- Generated code (if any) is excluded via `//go:generate` patterns.

**Rationale.** The seed §10 specifies 85% on "the public package tree."
Extending to `internal/` is appropriate because `internal/loop` and
`internal/retry` contain the most correctness-critical code. Excluding
them would allow the hot path to be under-tested while the coverage number
looks healthy.

**Alternatives considered.** (a) Public packages only — rejected; `internal/`
is where bugs hide. (b) 90% — rejected as unrealistic for a v0.1.0 target;
85% is achievable and standard for Go OSS libraries.

---

## D87 — Property-based test CI policy

**Status:** decided
**Summary:** 10k iterations on PR CI, 100k nightly. Nightly failures open
an issue automatically.

**Decision.** Property-based state machine tests (using `gopter`) run with
10,000 iterations on every PR as part of the `test` job. A scheduled nightly
job runs 100,000 iterations. If the nightly job fails, a GitHub issue is
opened automatically via `actions/github-script` or `peter-evans/create-issue-from-file`.

The nightly job runs on `ubuntu-latest` to minimize cost. The runner
must have sufficient memory for 100k iterations; if the test generator
produces large state sequences, the `GOMAXPROCS` setting is tuned to
match available cores.

**Rationale.** Seed §10 specifies this split. 10k iterations keep PR CI
under 2 minutes for the property test step. 100k iterations catch rare
transition sequences that 10k misses. Automatic issue creation ensures
nightly failures are not silently ignored.

**Alternatives considered.** (a) 100k on every PR — rejected; adds 10+
minutes to PR feedback loop. (b) No nightly, only PR — rejected; 10k is
insufficient for confidence on a 14-state machine with 5 terminal states.

---

## D88 — LLM conformance suite CI policy

**Status:** decided
**Summary:** Nightly only. Requires encrypted API secrets. Failure opens
an issue; does not block PRs.

**Decision.** The LLM conformance suite (`llm/conformance/`) runs as a
scheduled nightly job, not on PRs. It requires Anthropic and OpenAI API
keys stored as GitHub Actions encrypted secrets. The job:

1. Runs the conformance suite against `anthropic.Provider` and
   `openai.Provider` (and Azure OpenAI when credentials are available).
2. Uses budget-capped invocations (max 100 tokens per test, max $0.50
   total per run) to control costs.
3. On failure, opens a GitHub issue with the test output.
4. Does not block PR merges.

**Rationale.** Running live LLM calls on every PR is expensive ($0.50+ per
run × N PRs/day) and introduces flakiness from provider rate limits,
transient errors, and model behaviour changes. Nightly cadence catches
adapter regressions within 24 hours while keeping PR feedback fast and free.

**Alternatives considered.** (a) Run on PRs that touch `llm/` — rejected;
still flaky, and budget-capped tests may miss regressions outside the cap.
(b) Skip conformance entirely in CI — rejected; adapters are the primary
integration surface and must be tested against live providers.

---

## D89 — D10 tripwire enforcement

**Status:** decided
**Summary:** Module path must be resolved before the first `go.mod` is
committed. Fallback names: `praxis-kernel`, `invokekit`. Resolution
checklist documented.

**Decision.** D10's two preconditions (GitHub org acquisition, brand/trademark
review) must be resolved before the first `go.mod` file is committed to the
repository. This is the hard gate — no code lands until the module path is
confirmed.

Resolution checklist:
1. **GitHub org.** Confirm ownership of `github.com/praxis-go`, acquire it
   from GitHub support if dormant, or adopt `praxis-kernel` as the org slug.
2. **Brand review.** Complete a trademark search for "praxis" in the
   software/AI/governance space. Document the outcome in a D10 amendment.
3. **Fallback.** If either check fails, the project renames to the first
   available fallback: `praxis-kernel` → `invokekit`. The rename is a
   one-time cost: update `go.mod`, all import paths, all documentation,
   and all examples.

**Timeline.** Resolution must happen before v0.1.0 implementation begins.
The implementation phase cannot start with `MODULE_PATH_TBD`.

**Rationale.** Phase 3 used `MODULE_PATH_TBD` throughout, which was
appropriate for a design phase. Implementation code with a placeholder module
path cannot be compiled or tested. The hard gate at `go.mod` creation ensures
the path is real before any code depends on it.

---

## D90 — Bus-factor mitigation

**Status:** decided
**Summary:** Documented knowledge, executable specifications, and a
maintainer onboarding checklist. Target: a second maintainer can ship a
release within one week of reading the onboarding guide.

**Decision.** Bus-factor mitigation is achieved through three mechanisms:

1. **Design documentation.** The six-phase planning tree (`docs/phase-*`)
   is the design specification. Every non-trivial decision has a rationale
   and alternatives-considered record. A new maintainer reads these, not
   the code, to understand why.

2. **Executable specifications.** Property-based state machine tests, the
   LLM conformance suite, the banned-identifier grep, and the coverage
   gate collectively serve as executable documentation. A new maintainer
   who passes all CI checks has high confidence they have not broken the
   contract.

3. **Maintainer onboarding checklist** (in `CONTRIBUTING.md` §Maintainers):
   - Read `docs/PRAXIS-SEED-CONTEXT.md` and all six phase directories.
   - Run `make check` and `make bench` locally.
   - Create a test release via release-please dry-run.
   - Review the GitHub Actions workflows.
   - Confirm access to: GitHub org admin, API key secrets for CI,
     release-please token.
   - Review the `SECURITY.md` disclosure process and confirm contact info.

**Rationale.** Seed §14.1 flags bus factor as a risk. The mitigation is
not "get more maintainers" (which is outside the project's control) but
"make the project legible enough that a new maintainer can be productive
quickly." The onboarding checklist is the concrete artifact that measures
legibility.

**Alternatives considered.** (a) Mandate two maintainers before v1.0 —
rejected; imposes an external dependency the project cannot control. (b) No
specific mitigation — rejected; the seed flags it as a known risk that
deserves a concrete response.

---

## D91 — First-production-consumer gate

**Status:** decided
**Summary:** v1.0.0 is tagged only after a production consumer ships against
a v0.5.x tag. Verification is by maintainer attestation in the release
checklist.

**Decision.** The v1.0.0 release checklist (see
[`06-release-milestones.md`](06-release-milestones.md)) includes a mandatory
line item: "A production consumer has shipped against a v0.5.x tag." This is
verified by maintainer attestation — the maintainer records which consumer,
which version, and the date of production deployment in the v1.0.0 release
notes.

"Shipped in production" means: the consumer's code importing a v0.5.x tag is
running in a production environment serving real traffic, not a staging or
canary deployment.

The consumer attestation is recorded in a dedicated "Production Consumer"
section of the v1.0.0 GitHub release notes. This is the designated
transparency section per CLAUDE.md's constraint on consumer naming. If the
consumer is the primary consumer disclosed in seed §11, the section may name
it; otherwise, the section uses a neutral description (e.g., "an enterprise
agent platform in production since YYYY-MM-DD").

If the first production consumer is delayed, v1.0.0 is delayed. v0.5.x
continues to receive patch releases. No v1.0.0-rc tags are issued without
the consumer attestation.

**Rationale.** Seed §8 establishes this as a hard precondition. The
verification mechanism is simple (maintainer attestation) because the primary
consumer is known (see seed §11) and the maintainer has direct visibility
into its deployment status. External consumers who wish to attest can open a
GitHub Discussion.

**Alternatives considered.** (a) Public reference required (blog post, case
study) — rejected; imposes a marketing dependency on a technical gate. (b)
Self-attestation via GitHub issue — rejected as indistinguishable from
maintainer attestation with extra process.

---

## D92 — CLA vs DCO

**Status:** decided
**Summary:** DCO (Developer Certificate of Origin) via `probot/dco`.
No CLA.

**Decision.** Contributors attest to their right to submit code via the
Developer Certificate of Origin (DCO), enforced by `Signed-off-by:` lines
in commit messages. The `probot/dco` GitHub App is installed at the org
level and configured as a required check on PRs targeting `main`.

Contributors sign off with `git commit -s`. The DCO text (v1.1) is
included in a `DCO` file at the repository root.

**Rationale.** Apache 2.0 provides sufficient downstream IP protection
without a CLA. The 2025–2026 trend among Apache 2.0 Go OSS projects not
backed by a commercial entity is strongly toward DCO (Prometheus, OpenInfra).
CLA adds friction for first-time contributors (who must sign a separate
legal document) with no legal benefit beyond what Apache 2.0 + DCO already
provides. For a single-maintainer-team library seeking contributors, low
friction is the priority.

**Alternatives considered.** (a) CLA via CLA Assistant — rejected; adds
friction, requires external service dependency, no legal benefit for
Apache 2.0 project without commercial backer. (b) No contributor agreement —
rejected; leaves the project without a formal attestation that contributors
have the right to submit their code.

---

## D93 — PR review policy

**Status:** decided
**Summary:** One approval for all changes during v0.x. Two approvals for
`frozen-v1.0` interface changes after v1.0 ships.

**Decision.** During v0.x (single-maintainer phase), one maintainer approval
is sufficient for all PRs. Self-merge by the sole maintainer is permitted
with a mandatory 24-hour waiting period for PRs that change exported symbols.

After v1.0 ships and a second maintainer is onboarded:
- PRs touching `frozen-v1.0` interfaces require two maintainer approvals.
- All other PRs require one maintainer approval.
- Self-merge is no longer permitted for interface-changing PRs.

The PR template includes a checkbox: "[ ] Interface change (requires
maintainer discussion before merge)" to flag frozen-surface PRs.

**Rationale.** A single-maintainer project cannot require two approvals.
The 24-hour waiting period for exported-symbol changes gives the sole
maintainer a cooling-off period to catch mistakes. This is a process
commitment, not a platform-enforced gate — GitHub branch protection has no
time-based delay mechanism. It is enforced by maintainer discipline and
is recorded here as a binding convention. The two-approval rule for frozen
interfaces post-v1.0 is the standard protection for stability commitments.

---

## D94 — Branch protection rules

**Status:** decided
**Summary:** Squash merge only. Required checks: lint, test, commitsar,
banned-grep, spdx-check, dco.

**Decision.** Branch protection on `main`:
- **Merge strategy:** squash merge only. This ensures one conventional
  commit per PR in the `main` branch history, which release-please
  consumes cleanly.
- **Required status checks:** `lint`, `test`, `commitsar`, `banned-grep`,
  `spdx-check`, `dco` (probot/dco).
- **Dismiss stale reviews:** enabled.
- **Require branches to be up to date:** enabled.
- **No force pushes to `main`.**
- **No deletions of `main`.**

**Rationale.** Squash merge produces the cleanest conventional-commit
history for release-please. Required checks gate the decoupling contract
(banned-grep), code quality (lint, test), contributor agreement (dco),
license compliance (spdx-check), and commit hygiene (commitsar).

**Alternatives considered.** (a) Rebase merge — rejected; preserves
individual commits from the PR branch, which may include WIP or fixup
commits that degrade the changelog. (b) Merge commits — rejected; creates
non-linear history that release-please handles less cleanly.

---

## D95 — RFC process

**Status:** decided
**Summary:** GitHub Discussions with an "RFC" category. RFCs are valid only
post-v1.0.

**Decision.** Post-v1.0 feature proposals that would change a `frozen-v1.0`
interface or add a new public interface are submitted as GitHub Discussions
in an "RFC" category. The RFC template requires three fields:

1. **Motivation:** what problem the proposal solves, with a concrete use case.
2. **Proposed interface:** Go code showing the new or changed interface.
3. **Alternatives considered:** at least one alternative approach.

Acceptance workflow:
1. Maintainer reviews and invites community discussion.
2. After discussion, maintainer closes with a pinned comment:
   - **ACCEPTED:** a tracking issue is opened and linked.
   - **REJECTED:** rationale is documented in the closing comment, with a
     reference to the relevant non-goal or design decision.

Before v1.0, feature proposals are regular GitHub issues. The RFC process
exists specifically for changes to frozen interfaces, which require more
deliberation than v0.x feature work.

**Rationale.** GitHub Discussions is the lightest-weight option that provides
threaded discussion, community visibility, and separation from the issue
tracker (which is for bugs and concrete work). A `docs/rfcs/` directory is
appropriate for projects with formal governance (TC39-style); praxis is not
that project. GitHub Issues with an `rfc` label conflates deliberation with
actionable work.

**Alternatives considered.** (a) `docs/rfcs/` directory — rejected; higher
friction (requires a PR), overkill for a project expecting a handful of RFCs
per year. (b) GitHub Issues with `rfc` label — rejected; mixes deliberation
with bug tracking.

---

## D96 — `SECURITY.md` content and disclosure timeline

**Status:** decided
**Summary:** 90-day responsible disclosure. Report via GitHub private
vulnerability reporting. Known limitations OI-1 and OI-2 documented.

**Decision.** `SECURITY.md` contains:

1. **Reporting channel:** GitHub's private vulnerability reporting feature
   (Settings > Security > Advisories). No email address — GitHub's built-in
   feature provides encrypted communication without maintaining a PGP key.

2. **Response timeline:**
   - Acknowledgement within 48 hours.
   - Triage and severity assessment within 7 days.
   - Fix or mitigation within 90 days of report (the industry standard
     timeline, aligned with Google Project Zero and CERT/CC).
   - Public disclosure after the fix is released, or after 90 days if no
     fix is available.

3. **Known limitations:**
   - **OI-1 (Private key in-memory lifetime).** The `ed25519.PrivateKey`
     held by `Ed25519Signer` is not zeroed on garbage collection. Callers
     with strict key hygiene requirements should use a KMS/HSM-backed
     `identity.Signer` implementation. See Phase 5 `03-identity-signing.md`
     §5.5.
   - **OI-2 (Enricher attribute log-injection vector).** Caller-provided
     `AttributeEnricher` values are included in spans and lifecycle events.
     The framework's `RedactingHandler` redacts by key pattern but cannot
     redact by value. Callers must ensure enricher values do not contain
     sensitive data that would be exposed if spans are exported to an
     untrusted backend. See Phase 5 `04-trust-boundaries.md` §5.

4. **Scope:** `SECURITY.md` covers the praxis library itself. Security issues
   in caller-provided implementations (custom `PolicyHook`, `Resolver`,
   `Signer`, etc.) are the caller's responsibility.

**Rationale.** 90 days is the industry standard (Google Project Zero, CERT/CC,
most CNCF projects). GitHub's private vulnerability reporting is the
lowest-maintenance option: no PGP key to rotate, no mailing list to manage,
built into the platform the project already uses.

---

## D97 — SPDX header enforcement

**Status:** decided
**Summary:** Every `.go` file carries the Apache 2.0 SPDX header. Enforced
by CI via a shell script.

**Decision.** Every `.go` source file must begin with the SPDX license
identifier:

```go
// SPDX-License-Identifier: Apache-2.0
```

This is checked by a CI job (`spdx-check`) that greps all `.go` files for
the header and fails if any are missing. The check script is a simple
`find + grep` in the Makefile:

```makefile
spdx-check:
	@missing=$$(find . -name '*.go' -not -path './vendor/*' \
	  | xargs grep -L 'SPDX-License-Identifier: Apache-2.0'); \
	if [ -n "$$missing" ]; then \
	  echo "Missing SPDX header:"; echo "$$missing"; exit 1; \
	fi
```

**Rationale.** Seed §10 requires the SPDX header on every source file. A
CI check ensures this is enforced mechanically, not by reviewer discipline.
The SPDX short-form identifier is the recommended approach per the Linux
Foundation and REUSE specification.

---

## D98 — Tag signing policy

**Status:** decided
**Summary:** Release tags are lightweight (not GPG-signed). GitHub release
attestations provide provenance.

**Decision.** release-please creates lightweight tags (not annotated or
GPG-signed). Provenance is established via GitHub's release attestation
feature (Sigstore-based), which is automatically attached to GitHub releases
created by the release workflow.

GPG-signed tags are not used because:
- They require a long-lived GPG key that must be rotated and distributed.
- Go's `go get` and `go install` do not verify tag signatures.
- GitHub's attestation feature provides stronger provenance guarantees
  (Sigstore transparency log) without key management overhead.

**Rationale.** The Go ecosystem does not consume GPG tag signatures. The
`go` tool verifies module integrity via `go.sum` checksums and the Go
checksum database (`sum.golang.org`), not via tag signatures. Adding GPG
signing would impose key management overhead with no security benefit for
Go consumers.

**Alternatives considered.** (a) GPG-signed tags — rejected; overhead with
no consumer benefit. (b) No provenance at all — rejected; GitHub attestations
are free and provide supply-chain transparency.

---

## D99 — Package layout amendment

**Status:** decided
**Summary:** `internal/jwt` added to canonical layout. No other Phase 5
additions missed.

**Decision.** The canonical package layout (Phase 3
`go-architect-package-layout.md`) is amended to include:

```
internal/
    loop/      invocation loop driver
    retry/     backoff + jitter
    ctxutil/   DetachedWithSpan
    jwt/       Ed25519 JWT construction helper (Phase 5)
```

This is a back-annotation of the Phase 5 go-architect validation (§2.5),
which placed `internal/jwt` at Level 0 in the package DAG. No other Phase 5
additions are missing from the layout.

The `identity/` package's intra-praxis import of `internal/jwt` is recorded
as an existing edge in the dependency graph.

**Rationale.** The Phase 5 go-architect validation document records this
addition but the canonical layout in Phase 3 was not updated. D99 closes
this gap as a formal amendment.

---

## D100 — MetricsRecorder extension story

**Status:** decided
**Summary:** New metrics in v1.x require a `MetricsRecorderV2` interface
embedding `MetricsRecorder`. This is a known limitation, not a blocker.

**Decision.** The `MetricsRecorder` interface (Phase 4 D57, D65) is
`frozen-v1.0`. If a new Prometheus metric is added in v1.x, it requires a
new interface:

```go
type MetricsRecorderV2 interface {
    MetricsRecorder
    RecordNewMetric(...)
}
```

The orchestrator detects `MetricsRecorderV2` via type assertion at
construction time. Callers providing a `MetricsRecorder` without the V2
extension silently skip the new metric (no error, no panic).

This is the standard Go interface extension pattern (used by `net/http`'s
`Flusher`, `Hijacker`, etc.) and is consistent with seed §9's rule that
"adding a method to an existing interface is a breaking change."

**Rationale.** Phase 4 REVIEW.md OQ2 asked this question. The answer is the
Go-idiomatic embedding pattern. The alternative (adding optional methods to
`MetricsRecorder`) would break the v1.0 freeze. The type-assertion pattern
is well-understood and imposes no cost on callers who do not need the new
metric.

---

## D101 — Claim constant CI enforcement

**Status:** decided
**Summary:** JWT claim keys (`praxis.invocation_id`, `praxis.tool_name`,
`praxis.parent_token`) are package-level constants in `internal/jwt`.
Enforcement is architectural (single consumer site), supplemented by
`goconst` lint.

**Decision.** The three praxis-specific JWT claim keys are defined as
package-level constants in `internal/jwt`:

```go
const (
    ClaimInvocationID = "praxis.invocation_id"
    ClaimToolName     = "praxis.tool_name"
    ClaimParentToken  = "praxis.parent_token"
)
```

Enforcement that these constants are used (rather than inline string
literals) relies on **architectural isolation**, not a lint rule:

- The constants are in `internal/jwt`, so only the `identity/` package
  imports them.
- The `identity/` package has exactly one consumer of these strings
  (`Ed25519Signer.Sign`), which uses the constants directly.
- No other package in the praxis tree constructs JWT claims.

`golangci-lint`'s `goconst` linter provides supplementary detection:
it flags repeated string literals (at min-occurrences=3), which would
catch accidental duplication within a package. However, `goconst` cannot
detect a single inline literal replacing a constant reference — that
failure mode is caught by code review, not CI.

A custom grep-based CI check was considered and rejected as brittle:
it would need to distinguish between the constant declaration site and
inline usage, and the architectural isolation already limits the blast
radius to a single file.

**Rationale.** Phase 5 REVIEW.md OQ2 asked whether these should be
CI-enforced. The answer is "enforced by architecture, supplemented by
lint." The claim keys live in `internal/jwt` and are consumed by a
single call site. Drift requires a code review failure, not a tool
failure — and the scope is narrow enough that review is reliable.

---

## D102 — v0.x breaking-change communication

**Status:** decided
**Summary:** Breaking changes in v0.x are communicated via `CHANGELOG.md`
(automatically) and a GitHub Discussion announcement for changes to
exported interfaces.

**Decision.** During v0.x, breaking changes are expected (seed §9: "any
minor tag may break any public API"). Communication channels:

1. **`CHANGELOG.md`:** release-please automatically creates a "Breaking
   Changes" section from commits with a `BREAKING CHANGE:` footer. This is
   the primary channel.

2. **GitHub Discussion (Announcements category):** for breaking changes to
   exported interfaces (not internal refactors), the maintainer posts a
   brief announcement in the Discussions "Announcements" category explaining
   what changed, why, and how to migrate. This is a courtesy, not a
   guarantee — the CHANGELOG is authoritative.

3. **Migration guide:** if a v0.x minor changes more than three exported
   symbols, a `docs/migration/v0.X-to-v0.Y.md` file is created with
   before/after code snippets.

**Rationale.** v0.x consumers accept breakage risk (seed §9), but silent
breakage is hostile. The three-channel approach (changelog always, Discussion
for interface changes, migration guide for large changes) is proportional to
the project size and the expected consumer count during v0.x.

---

## D103 — Release milestone exit criteria

**Status:** decided
**Summary:** Concrete checklists for v0.1.0, v0.3.0, v0.5.0, v1.0.0.
Full specification in [`06-release-milestones.md`](06-release-milestones.md).

**Decision.** Each release milestone has a checklist that must be satisfied
before the tag is created. The checklists incorporate all decisions from
Phases 1–5 and are the authoritative "definition of done" for each tag.
See [`06-release-milestones.md`](06-release-milestones.md) for the full
specification.

Key gates:
- **v0.1.0:** D10 resolved, `go.mod` with confirmed module path, minimal
  sync path, Anthropic adapter, all null defaults, 85% coverage, CI green.
- **v0.3.0:** All interfaces at v1.0-candidate shape, hooks + filters +
  budget functional, streaming, OpenAI adapter, property-based tests in CI.
- **v0.5.0:** Feature complete, conformance suite green, benchmarks green,
  Ed25519Signer, 85% coverage, ready for production consumer.
- **v1.0.0:** Production consumer attestation, all interfaces frozen, API
  freeze committed, `SECURITY.md` published, RFC process active.

---

## D104 — v0.x interface-change review requirement (seed §14.2)

**Status:** decided
**Summary:** Any v0.x minor release that changes an exported interface
method signature requires a decisions-log entry in
`docs/decisions/v0.x-interface-changes.md` before the release PR is merged.

**Decision.** Seed §14.2 mandates: "every v0.x minor release goes through a
phase-style review with a decisions log entry for any interface change."
This is implemented as follows:

1. **When triggered:** a PR that changes the method signature of any
   exported interface listed in Phase 1 `04-v1-freeze-surface.md` (the 14
   frozen-v1.0 interfaces) — adding a method, removing a method, changing
   a parameter or return type.

2. **What is required:** before the PR is merged, a decisions-log entry is
   added to `docs/decisions/v0.x-interface-changes.md` (created at v0.1.0).
   The entry records:
   - Which interface changed.
   - What the old and new signatures are.
   - Why the change is necessary.
   - Whether any consumer migration is needed.

3. **Not triggered by:** internal refactors, new types or functions that do
   not change existing interfaces, documentation changes, test changes.

4. **Enforcement:** the PR template checkbox "Interface change (requires
   maintainer discussion before merge)" flags these PRs. The maintainer
   verifies the decisions-log entry exists before merging. This is a
   process gate, not an automated CI check — the set of "interface changes"
   is not reliably detectable by grep.

This requirement applies only during v0.x. After v1.0, the frozen-v1.0
commitment (D13) makes interface changes a v2 module-path event, which is
governed by the RFC process (D95).

**Rationale.** Seed §14.2 identifies "velocity pressure during v0.x" as a
risk. The mitigation is a lightweight decisions log for interface changes —
not a full phase-style review (which would be disproportionate for a
single-maintainer project), but a written record that forces the maintainer
to articulate why the change is needed before shipping it. This is
proportional to the project size while satisfying the seed's intent.

---

## D105 — Benchmark baseline cache workflow

**Status:** decided
**Summary:** A `post-merge-bench` workflow runs benchmarks on `main` after
merge and caches the results for PR comparison.

**Decision.** A dedicated `post-merge-bench` GitHub Actions workflow runs
benchmarks on every push to `main`:

```yaml
name: Benchmark Baseline
on:
  push:
    branches: [main]
jobs:
  bench:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
      - run: go test -bench=. -benchmem -count=5 ./... > bench-main.txt
      - uses: actions/cache/save@v4
        with:
          path: bench-main.txt
          key: bench-main-${{ github.sha }}
```

The PR `bench` job fetches the most recent cached baseline:

```yaml
- uses: actions/cache/restore@v4
  with:
    path: bench-main.txt
    key: bench-main-
    restore-keys: bench-main-
```

If no cached baseline exists (e.g., first PR after repo creation), the PR
bench job runs benchmarks without comparison and posts a note instead of
a regression report.

**Rationale.** The reviewer correctly identified that the bench job
referenced a cached baseline without specifying how it was produced. This
decision closes that gap. The `actions/cache/save` approach is the standard
GitHub Actions pattern for cross-workflow data sharing. The baseline is
keyed by commit SHA to ensure it reflects the current state of `main`.

---

## Adopted decisions summary (for `roadmap-status`)

| ID | Title | Status |
|---|---|---|
| D81 | Deprecation window: 2 minors + 6 months | decided |
| D82 | Single `main` branch, no release branches | decided |
| D83 | Conventional commits enforced via commitsar | decided |
| D84 | release-please `go` type with version.go extra-file | decided |
| D85 | CI pipeline: 6 required + 2 informational PR checks + 2 nightly + 1 release | decided |
| D86 | 85% coverage gate on all packages incl. internal/ | decided |
| D87 | Property tests: 10k PR, 100k nightly | decided |
| D88 | LLM conformance: nightly only, budget-capped | decided |
| D89 | D10 tripwire: module path before first go.mod | decided |
| D90 | Bus-factor: design docs + exec specs + onboarding checklist | decided |
| D91 | v1.0.0 gated on production consumer attestation | decided |
| D92 | DCO via probot/dco, no CLA | decided |
| D93 | PR review: 1 approval v0.x; 2 for frozen interfaces post-v1.0 | decided |
| D94 | Squash merge only, 6 required checks | decided |
| D95 | RFC via GitHub Discussions, post-v1.0 only | decided |
| D96 | SECURITY.md: 90-day disclosure, GitHub private reporting, OI-1/OI-2 | decided |
| D97 | SPDX Apache-2.0 header on all .go files, CI-enforced | decided |
| D98 | Lightweight tags + GitHub attestations, no GPG | decided |
| D99 | internal/jwt added to canonical package layout | decided |
| D100 | MetricsRecorderV2 embedding for v1.x metric additions | decided |
| D101 | JWT claim constants in internal/jwt, architectural enforcement | decided |
| D102 | v0.x breaking changes: CHANGELOG + Discussion + migration guide | decided |
| D103 | Release milestone exit criteria in 06-release-milestones.md | decided |
| D104 | v0.x interface changes require decisions-log entry (seed §14.2) | decided |
| D105 | Benchmark baseline cache workflow for PR comparison | decided |
