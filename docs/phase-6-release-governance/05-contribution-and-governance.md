# Phase 6 — Contribution and Governance

**Related decisions:** D90 (bus-factor), D92 (DCO), D93 (PR review), D94
(branch protection), D95 (RFC process), D96 (SECURITY.md).

---

## 1. Developer Certificate of Origin (D92)

Contributors attest to their right to submit code via the DCO (v1.1).

### How it works

Every commit must include a `Signed-off-by:` line matching the commit author:

```
Signed-off-by: Jane Developer <jane@example.com>
```

This is added automatically with `git commit -s`.

### Enforcement

The `probot/dco` GitHub App is installed at the org level. It is a required
status check on PRs targeting `main`. A PR with unsigned commits cannot be
merged.

### Remediation

If a contributor forgets `-s`:
1. Amend the commit: `git commit --amend -s` + force push.
2. Or: add a new commit with `Signed-off-by:` to cover the previous ones
   (probot/dco's remediation mode accepts this).

### DCO file

A `DCO` file at the repository root contains the full text of the Developer
Certificate of Origin v1.1.

---

## 2. Code of Conduct

praxis adopts the **Contributor Covenant v2.1** as its code of conduct.

- The full text is in `CODE_OF_CONDUCT.md` at the repository root.
- Enforcement contact: the project maintainer(s), reachable via the email
  address listed in `CODE_OF_CONDUCT.md`.
- Scope: all project spaces (issues, PRs, Discussions, releases).
- Escalation: if the report concerns a maintainer, the reporter should
  contact the enforcement contact listed in the Contributor Covenant
  template (or a neutral third party if the project grows to have one).

---

## 3. CONTRIBUTING.md Outline

The `CONTRIBUTING.md` file at the repository root covers:

### 3.1 Before You Start

- Read the README and at least one example.
- Check the non-goals list
  ([`docs/phase-1-api-scope/03-non-goals.md`](../phase-1-api-scope/03-non-goals.md))
  before proposing a feature — feature requests for non-goal items are closed
  with a redirect.
- Opening an issue before a large PR is appreciated but not required for
  small fixes.

### 3.2 Dev Setup

- Go 1.23 or later.
- `golangci-lint` (install via `go install` or Homebrew).
- Run `make check` to validate your setup (runs lint + test + banned-grep +
  spdx-check).
- Run `make bench` to run benchmarks (optional for most PRs).

### 3.3 Commit Messages

Conventional commits required. Format: `<type>(<scope>): <description>`.
Include `Signed-off-by:` via `git commit -s`.

Reference to commit type table and examples (see
[`02-release-process.md`](02-release-process.md) §1).

### 3.4 Pull Requests

- Keep PRs focused on one thing.
- All CI checks must pass before review.
- Use the PR template (see §5 below).
- For changes to a `frozen-v1.0` interface (after v1.0 ships), two
  maintainer approvals are required.
- For all other changes, one approval is sufficient.

### 3.5 Where to Ask Questions

- **Usage questions:** GitHub Discussions (Q&A category).
- **Bug reports:** GitHub Issues with the bug report template.
- **Feature proposals (post-v1.0):** GitHub Discussions (RFC category).
- Do not use issues for general questions.

### 3.6 Adding a New LLM Adapter

For contributors adding a third-party adapter (e.g., `llm/gemini`):

1. Create a new package under `llm/<provider>/`.
2. Implement the `llm.Provider` interface.
3. Run the shared conformance suite: `go test ./llm/conformance/... -provider=gemini`.
4. Add a provider name constant (e.g., `gemini.ProviderName`).
5. Add at least one example in `examples/`.
6. The conformance suite must pass for the PR to be merged.

### 3.7 Maintainer Onboarding (D90)

For new maintainers:

1. Read `docs/PRAXIS-SEED-CONTEXT.md` and all six phase directories.
2. Run `make check` and `make bench` locally.
3. Create a test release via release-please dry-run.
4. Review the GitHub Actions workflows in `.github/workflows/`.
5. Confirm access to: GitHub org admin, API key secrets for CI,
   release-please token.
6. Review `SECURITY.md` and confirm the disclosure contact info is current.

---

## 4. PR Review Policy (D93)

| Phase | Change type | Required approvals |
|---|---|---|
| v0.x | Any change | 1 maintainer |
| v0.x | Exported symbol change (sole maintainer) | 1 + 24-hour waiting period |
| v1.0+ | Internal code, tests, docs | 1 maintainer |
| v1.0+ | `frozen-v1.0` interface change | 2 maintainers |

Self-merge is permitted for the sole maintainer during v0.x with the
24-hour cooling-off period for exported-symbol changes. Self-merge of
interface-changing PRs is not permitted after a second maintainer is
onboarded.

---

## 5. Issue and PR Templates

### 5.1 Bug Report Template (`.github/ISSUE_TEMPLATE/bug_report.md`)

```markdown
---
name: Bug Report
about: Report a bug in praxis
labels: bug
---

**What happened?**
<!-- One or two sentences: what did you observe vs. what you expected. -->

**Minimal reproduction**
<!-- Paste the smallest Go snippet that triggers the bug, or link to a
     public repo. If you cannot reproduce it outside your codebase,
     describe the inputs and outputs. -->

**Environment**
- praxis version:
- Go version:
- OS:

**Error output**
<!-- Paste the full error string or stack trace. Do not truncate. -->

**What have you tried?**
<!-- Optional: steps you took to diagnose or work around the issue. -->
```

### 5.2 Feature Request Template (`.github/ISSUE_TEMPLATE/feature_request.md`)

```markdown
---
name: Feature Request
about: Propose a new feature
labels: enhancement
---

**What problem are you trying to solve?**
<!-- Describe the use case in concrete terms. -->

**Is this within praxis scope?**
<!-- Check docs/phase-1-api-scope/03-non-goals.md before filing.
     Feature requests for non-goal items will be closed with a redirect. -->

**Proposed interface (optional)**
<!-- If you have a concrete Go interface or function signature, show it. -->

**Alternatives you have considered**
<!-- What do you do today? Why is that insufficient? -->
```

### 5.3 PR Template (`.github/pull_request_template.md`)

```markdown
## What does this PR do?
<!-- One sentence. -->

## Type of change
- [ ] Bug fix (no interface change)
- [ ] New feature (no interface change)
- [ ] Interface change (requires maintainer discussion before merge)
- [ ] Documentation or example only

## Checklist
- [ ] `make check` passes (lint, test, banned-grep, spdx-check)
- [ ] Godoc added or updated for any new exported symbols
- [ ] No `CHANGELOG.md` edit needed (release-please handles this)

## Related issues
Closes #XXX
```

---

## 6. Branch Protection Rules (D94)

Applied to `main`:

| Setting | Value |
|---|---|
| Merge strategy | Squash merge only |
| Required status checks | `lint`, `test`, `commitsar`, `banned-grep`, `spdx-check`, `dco` |
| Dismiss stale reviews | Enabled |
| Require branches up to date | Enabled |
| Force pushes | Disabled |
| Branch deletion | Disabled |
| Required reviewers (v0.x) | 0 (self-merge permitted) |
| Required reviewers (v1.0+) | 1 (2 for frozen-interface PRs) |

---

## 7. RFC Process (D95)

### When

RFCs are valid **only after v1.0 ships**. Before v1.0, feature proposals
are regular GitHub issues.

### What requires an RFC

- Changes to a `frozen-v1.0` interface.
- Addition of a new public interface to the v1.x surface.
- Removal of a non-goal restriction (i.e., bringing something into scope
  that Phase 1 explicitly excluded).

### Template

Filed as a GitHub Discussion in the "RFC" category:

```
## Motivation
What problem does this solve? Include a concrete use case.

## Proposed Interface
Go code showing the new or changed interface.

## Alternatives Considered
At least one alternative approach.
```

### Acceptance

1. Maintainer reviews and invites community discussion (minimum 14 days).
2. Maintainer closes with a pinned comment:
   - **ACCEPTED:** tracking issue opened and linked.
   - **REJECTED:** rationale documented, with reference to the relevant
     design decision or non-goal.

---

## 8. SECURITY.md Content (D96)

### 8.1 Reporting Channel

Security vulnerabilities are reported via GitHub's private vulnerability
reporting feature (repository Settings > Security > Advisories). This
provides encrypted communication without requiring PGP key management.

### 8.2 Response Timeline

| Step | Target |
|---|---|
| Acknowledgement | 48 hours |
| Triage and severity assessment | 7 days |
| Fix or mitigation | 90 days |
| Public disclosure | After fix release, or after 90 days |

### 8.3 Known Limitations

**OI-1 — Private key in-memory lifetime.** The `ed25519.PrivateKey` held
by `Ed25519Signer` is not zeroed on garbage collection. The Go runtime
does not provide a mechanism to hook into GC for memory clearing. Callers
with strict key hygiene requirements should use a KMS/HSM-backed
`identity.Signer` implementation that delegates signing to hardware and
never holds the private key in user-space memory. See Phase 5
`03-identity-signing.md` §5.5.

**OI-2 — Enricher attribute log-injection vector.** Caller-provided
`AttributeEnricher` values are included in OTel spans and lifecycle events.
The framework's `RedactingHandler` redacts by key pattern (D58, D79) but
cannot redact by value content. If an enricher value contains sensitive
data (e.g., a PII field embedded in a tenant ID), the framework cannot
detect or redact it. Callers must ensure enricher values are safe for
export to their observability backend. See Phase 5
`04-trust-boundaries.md` §5.

### 8.4 Scope

`SECURITY.md` covers the praxis library. Security issues in caller-provided
implementations (custom hooks, resolvers, signers, filters) are the
caller's responsibility.

---

## 9. Bus-Factor Mitigation (D90)

The project mitigates bus-factor risk through three mechanisms:

1. **Design documentation.** Six planning phases with 101 decisions, each
   carrying rationale and alternatives-considered. A new maintainer reads
   the decision logs to understand *why*, not just *what*.

2. **Executable specifications.** Property-based state machine tests, LLM
   conformance suite, banned-identifier grep, coverage gate, and SPDX
   checks collectively serve as a machine-verifiable specification. A new
   maintainer who passes all CI checks has high confidence they have not
   broken the contract.

3. **Maintainer onboarding checklist** (§3.7 above). Target: a second
   maintainer can ship a release within one week of reading the onboarding
   guide.

The project does not mandate a minimum maintainer count. Adding maintainers
is desirable but outside the project's control. The mitigation is legibility,
not headcount.
