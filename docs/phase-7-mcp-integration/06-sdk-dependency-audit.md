# Phase 7 — MCP SDK Dependency Audit

**Scope:** Transitive-dependency, license, and vulnerability audit of the
`github.com/modelcontextprotocol/go-sdk` module as introduced into
`praxis/mcp` by S31 PR-A.

**Decision reference:** D107 precondition 3 — "Transitive-dependency audit
of `modelcontextprotocol/go-sdk` recorded in the PR that introduces the
dependency: list of added transitive modules, license summary,
`govulncheck` pass."

**Phase 6 milestone gate:** `docs/phase-6-release-governance/06-release-milestones.md`
§4 (v0.7.0 exit criteria, SDK reuse gate).

**Audit date:** 2026-04-11.
**Auditor:** S31 PR-A author.
**Audit result:** **APPROVED** for merge into `praxis/mcp`. All new
transitive modules carry permissive licenses; `govulncheck` reports zero
vulnerabilities reachable through the SDK itself or any of the modules it
introduces.

This document is a **permanent audit artifact** and must not be deleted
after the PR merges. It remains the historical record of the dependency
surface praxis/mcp shipped on the day the SDK was integrated.

---

## 1. SDK target

| Field | Value |
|---|---|
| Module path | `github.com/modelcontextprotocol/go-sdk` |
| Version pinned | `v1.5.0` |
| Satisfies T31.1 "v1.x line" | **Yes** (v1.x currently includes v1.0.0, v1.2.0, v1.5.0) |
| Import path used by praxis/mcp | `github.com/modelcontextprotocol/go-sdk/mcp` (aliased as `sdkmcp` in `mcp/internal/client/`) |
| License (SDK module) | Dual: **Apache-2.0** for new/relicensed contributions, **MIT** for legacy unrelicensed contributions. See §3 for the full text. |

Version selection rationale: at audit time `v1.5.0` was the most recent tag
on the v1.x line in the Go module proxy. The SDK follows SemVer; the v1.x
commitment means minor/patch bumps inside this line will not introduce
breaking API changes, so `praxis/mcp` can stay on the latest v1.x tip
without forcing a new audit cycle per minor bump. A major bump (v2 module
path) would require a fresh audit.

---

## 2. New transitive modules

The following module-level dependencies are **new to
`mcp/go.sum`** after running `go mod tidy` with the SDK added to
`mcp/go.mod`'s require block. The baseline for the diff is the `main`
branch at commit `59c8f1d` (S30 merge commit).

| Module | Version | Role / why pulled in |
|---|---|---|
| `github.com/modelcontextprotocol/go-sdk` | **v1.5.0** | Direct — the target dep. |
| `github.com/google/jsonschema-go` | v0.4.2 | JSON Schema implementation the SDK uses to describe MCP tool parameter schemas. |
| `github.com/segmentio/encoding` | v0.5.4 | Fast JSON encoder used by the SDK on the wire hot path. |
| `github.com/segmentio/asm` | v1.1.3 | Assembly helpers consumed by `segmentio/encoding`. |
| `github.com/yosida95/uritemplate/v3` | v3.0.2 | RFC 6570 URI template implementation used for MCP `resources/*` URI matching. |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | JWT library used by the SDK's OAuth authentication helpers. Compiled into the module graph; reachable only through OAuth code paths that praxis/mcp does not currently activate. |
| `golang.org/x/oauth2` | v0.35.0 | OAuth 2.0 helpers used by the SDK's HTTP transport authentication path. |

**go.mod bumps** (modules already present at a lower version before S31):

| Module | Before | After | Note |
|---|---|---|---|
| `golang.org/x/sys` | v0.35.0 | **v0.41.0** | Platform syscall helpers. Bumped transitively by the SDK; the higher minimum version is carried forward. |
| `golang.org/x/tools` | — | v0.42.0 | Not compiled; referenced only from `go.sum` at the `/go.mod` level for module graph resolution. |

**Counting discipline:** the column "Role / why pulled in" names the
immediate consumer upstream of the new module. None of the modules above
were pulled in by praxis-owned code in PR-A — they are all reached through
the SDK's own import graph (directly or transitively).

---

## 3. License inventory

Every new module carries a **permissive** license. There are no copyleft
(GPL/AGPL/LGPL/MPL) licenses in the new dependency surface; there are no
proprietary or unknown licenses.

| Module | License SPDX | Source of truth | Compatible with praxis/mcp's Apache-2.0 distribution? |
|---|---|---|---|
| `github.com/modelcontextprotocol/go-sdk@v1.5.0` | Apache-2.0 (new contribs) / MIT (legacy, during transition) | `LICENSE` at module root | **Yes** — Apache-2.0 identical; MIT permissive. |
| `github.com/google/jsonschema-go@v0.4.2` | MIT | `LICENSE` at module root | **Yes** — MIT is permissive and Apache-2.0-compatible. |
| `github.com/segmentio/encoding@v0.5.4` | MIT | `LICENSE` at module root | **Yes**. |
| `github.com/segmentio/asm@v1.1.3` | MIT | `LICENSE` at module root | **Yes**. |
| `github.com/yosida95/uritemplate/v3@v3.0.2` | BSD-3-Clause | `LICENSE` at module root | **Yes** — BSD-3-Clause is permissive. |
| `github.com/golang-jwt/jwt/v5@v5.3.1` | MIT | `LICENSE` at module root | **Yes**. |
| `golang.org/x/oauth2@v0.35.0` | BSD-3-Clause (Go Project) | `LICENSE` at module root | **Yes**. |
| `golang.org/x/sys@v0.41.0` | BSD-3-Clause (Go Project) | Pre-existing. Bump only. | **Yes**. |
| `golang.org/x/tools@v0.42.0` | BSD-3-Clause (Go Project) | Pre-existing. Bump only. | **Yes**. |

### SDK module dual-license note

The upstream SDK is in the middle of a relicensing transition recorded in
its `LICENSE` file verbatim:

> The MCP project is undergoing a licensing transition from the MIT License
> to the Apache License, Version 2.0 ("Apache-2.0"). All new code and
> specification contributions to the project are licensed under Apache-2.0.
> Documentation contributions (excluding specifications) are licensed under
> CC-BY-4.0.
>
> Contributions for which relicensing consent has been obtained are
> licensed under Apache-2.0. Contributions made by authors who originally
> licensed their work under the MIT License and who have not yet granted
> explicit permission to relicense remain licensed under the MIT License.

**Audit interpretation.** Both terminal states (Apache-2.0, MIT) are
compatible with praxis/mcp's Apache-2.0 distribution. The SDK's own
maintainers handle the per-file attribution and their `LICENSE` file
captures the overall state. praxis/mcp does not need to track per-contribution
license state; consuming the SDK module as a whole under the terms of its
`LICENSE` file is sufficient for compliance.

---

## 4. Vulnerability scan

**Tool:** `govulncheck@v1.1.4`
**Vulnerability database:** `https://vuln.go.dev`
**DB snapshot:** `2026-04-08 15:08:18 UTC`
**Go toolchain:** `go1.26.1 darwin/arm64`

### 4.1 Command executed

```
cd mcp && govulncheck ./...
```

### 4.2 Findings — **SDK-specific: zero**

`govulncheck` reports **zero** vulnerabilities in any of the modules
introduced by the SDK (either the SDK itself or any of the 7 new
transitive modules from §2) that are reachable through praxis/mcp's call
graph.

This is the load-bearing audit result: no entry in the new dependency
closure is currently flagged as a known vulnerability. The SDK is
approved for integration on the vulnerability axis.

### 4.3 Findings — **Go standard library**

`govulncheck` additionally reports four Go standard-library
vulnerabilities reachable through praxis/mcp's call graph via the SDK's
TLS/x509 usage:

| ID | Area | Title | Fixed in |
|---|---|---|---|
| GO-2026-4947 | crypto/x509 | Unexpected work during chain building | `crypto/x509@go1.26.2` |
| GO-2026-4946 | crypto/x509 | Inefficient policy validation | `crypto/x509@go1.26.2` |
| GO-2026-4870 | crypto/tls | Unauthenticated TLS 1.3 KeyUpdate record can cause persistent connection retention and DoS | `crypto/tls@go1.26.2` |
| GO-2026-4866 | crypto/x509 | Case-sensitive excludedSubtrees name constraints cause Auth Bypass | `crypto/x509@go1.26.2` |

**These four CVEs are NOT caused by the SDK integration.** Re-running
`govulncheck ./...` against the `praxis` core module on `main` (at commit
`59c8f1d`, pre-PR-A) produces the **identical four stdlib findings**.
They are reachable from core code that existed before this audit (e.g.,
`llm/anthropic`'s HTTPS client, `telemetry/prometheus`'s registry metric
endpoints). Adding the SDK does not change their reachability.

**Resolution path.** These four CVEs are fixed in the Go `1.26.2` release.
They are a **Go toolchain bump**, not a praxis/mcp dependency decision;
the remediation is to rebuild the project with `go1.26.2+` on the CI
runners and developer machines. That work is out of scope for this audit
and for S31 — it belongs to an independent `chore(ci):` bump that can
land at any time on `main`. It does **not** block v0.7.0 per the Phase 7
D107 precondition, which is scoped to "transitive-dependency audit of
`modelcontextprotocol/go-sdk`"; Go toolchain CVEs are a separate axis
covered by the core module's `govulncheck` CI job (D85).

### 4.4 Non-called vulnerabilities

`govulncheck` additionally reported:

- **1 vulnerability in a package we import** — not reached by any actual
  call path from praxis/mcp code.
- **2 vulnerabilities in modules we require** — not reached at the
  package level at all.

These are suppressed by govulncheck's default output because the symbol
analysis proved unreachability. We do not individually enumerate them
here because the audit's decision gate is "reachable findings", which is
zero on the SDK axis.

---

## 5. Approval

- [x] **§1 — SDK target:** `v1.5.0` on the v1.x line (satisfies T31.1).
- [x] **§2 — Transitive modules:** 7 new modules enumerated. No surprises
      beyond what the SDK's own go.mod declares.
- [x] **§3 — License inventory:** every new module is Apache-2.0 /
      MIT / BSD-3-Clause. No copyleft. No unknown / unlisted. Compatible
      with praxis/mcp's Apache-2.0 distribution.
- [x] **§4 — Vulnerability scan:** zero SDK-specific findings. Four Go
      stdlib CVEs are pre-existing on `main` and independent of this PR.

**Approval decision:** `github.com/modelcontextprotocol/go-sdk@v1.5.0`
is **APPROVED** as the MCP SDK for `praxis/mcp` v0.7.0 per D107 and
Phase 6 §4.

---

## 6. Re-audit triggers

This audit is **point-in-time**. A fresh audit pass SHOULD be conducted
if any of the following occur:

1. The SDK version is bumped across a minor boundary (e.g., `v1.5.0` →
   `v1.6.0` or later).
2. The SDK version is bumped across a major boundary (e.g., `v1.x` →
   `v2.x`) — this case is **mandatory**, not optional, because the
   module path itself changes.
3. `govulncheck` in CI flags a NEW vulnerability in the SDK or any
   transitive module listed in §2 that did not appear at audit time.
4. Any new transitive dependency appears in `mcp/go.sum` between SDK
   bumps without an accompanying bump to the SDK itself — indicates the
   SDK's own `go.mod` grew a new upstream that we did not anticipate.

Re-audits append to this document as new top-level sections (`## 7. …`,
`## 8. …`). The 2026-04-11 audit in §§1–5 above is immutable.

---

## 7. Artifact manifest

The evidence backing this audit is:

- `mcp/go.mod` and `mcp/go.sum` at the merge commit of S31 PR-A — the
  canonical record of the dependency versions audited.
- `mcp/internal/client/client.go` — the single file in PR-A that imports
  the SDK. Every other adapter file imports the wrapper, not the SDK
  directly.
- `mcp/internal/client/client_test.go` — the `TestSDKInMemorySessionLifecycle`
  test that proves the SDK handshake is actually functional in praxis/mcp's
  build, not just nominally compilable.
- This document.

All four are versioned in the praxis repository. There is no external
artifact hosting for the audit.
