# Phase 7 — Scope and Positioning

**Decisions:** D106, D107, D108, D109
**Cross-references:** Phase 1 D09 / Non-goal 7 (no plugins),
Phase 1 D04 (frozen interface surface), Phase 3
`06-tools-and-invocation-context.md` (`tools.Invoker` frozen shape),
`research-solutions.md` §5 (reuse-vs-build).

---

## 1. Positioning statement

praxis ships **first-class support** for the Model Context Protocol
(MCP) as an **officially supported, separately-versioned Go module**
in the praxis monorepo.

Concretely:

- The core module `github.com/praxis-go/praxis` ships **no** MCP code
  and **no** MCP dependency. Zero-MCP consumers pay zero transitive
  dependency cost.
- A second Go module, `github.com/praxis-go/praxis/mcp`, lives in the
  same repository under a `mcp/` directory with its own `go.mod`. It
  implements `praxis/tools.Invoker` on top of the official MCP Go SDK
  and is consumed by users who opt in with
  `go get github.com/praxis-go/praxis/mcp`.
- The MCP module depends on `praxis/tools`, `praxis/credentials`,
  `praxis/errors`, and `praxis/telemetry` — the existing frozen
  interfaces. It does **not** depend on `praxis/orchestrator` or on
  any other caller-level glue.
- Configuration of which MCP servers to connect to, which transports
  to use, and how to map credentials is a **build-time Go
  construction-time** concern. No runtime server discovery. No plugin
  loading. No reflection.

This positioning is the middle path between "pattern-only docs" (which
fragments the ecosystem and forces every consumer to re-solve
namespacing, credential flow, error translation, and observability)
and "core-module package" (which pollutes every praxis consumer's
dependency graph with the MCP SDK and its transitive deps).

## 2. What "first-class" means

The `praxis/mcp` module is first-class in the following concrete
senses, and only these senses:

1. **Maintained in the same repository** as the core module, by the
   same maintainer set, subject to the same CI pipeline (lint, test,
   banned-identifier grep, SPDX check, coverage gate, govulncheck).
2. **Shipped under the same Apache-2.0 license** and the same
   Contributor Covenant code of conduct.
3. **Documented in the main praxis documentation tree** with a link
   from the top-level README's "Tool integrations" section.
4. **Covered by the v1.0 stability commitment at the module level** —
   its exported API freezes at `praxis/mcp v1.0.0`, which may tag
   independently of the core module under its own semver line. Phase 6
   (D92, D93) covers the "modules in a monorepo release independently"
   pattern; Phase 7 inherits it.
5. **Subject to the decoupling contract.** Banned identifiers do not
   relax inside `mcp/`. No consumer brand names, no hardcoded tenant
   or agent attribute keys, no leakage of the governance-event
   vocabulary.

"First-class" does **not** mean any of the following:

- It does not mean the MCP SDK becomes a dependency of the core
  `praxis` module. The core module's `go.mod` stays untouched by
  Phase 7.
- It does not mean `tools.Invoker`, `ToolCall`, `ToolResult`, or
  `InvocationContext` get new fields or new methods. The Phase 3
  frozen surface is preserved verbatim.
- It does not mean the core runtime becomes MCP-aware. The core
  orchestrator dispatches to `tools.Invoker` and does not know whether
  the invoker routes to in-process code, an HTTP API, or an MCP
  session.
- It does not mean runtime discovery or dynamic server registration.
  The adapter is constructed at `main()` time with a fixed server list.

## 3. What is in scope for Phase 7

The following items are in scope and are addressed in the remaining
Phase 7 deliverables:

1. **Package layout** of the `praxis/mcp` module. (This document +
   03-integration-model.md.)
2. **Adapter API shape** — how consumers construct an `Invoker` that
   fronts one or more MCP servers. (03.)
3. **Tool-name namespacing convention** for the multi-server case.
   (03.)
4. **Credential flow** from `credentials.Resolver` to the MCP session
   auth, including the long-lived-session tension with Phase 5's
   "credentials per tool call" posture. (04.)
5. **Error translation** from MCP JSON-RPC errors and `isError: true`
   tool results into the `errors.TypedError` taxonomy. (03.)
6. **Untrusted-output extension** — confirmation that Phase 5 D77
   covers MCP-sourced content without modification, plus MCP-specific
   guidance for `PostToolFilter` implementors. (04.)
7. **Observability extensions** — span name, attributes, metric label
   additions, cardinality analysis. (03 + 04.)
8. **Trust boundary classification** — the MCP transport edge as a
   Phase 5 trust boundary, `SignedIdentity` propagation policy. (04.)
9. **Budget participation** — how MCP-backed tool calls count against
   the four `budget.Guard` dimensions. (03.)
10. **Transport priority list** for v1.0.0. (This document §5.)
11. **Explicit non-goals** — the list of things Phase 7 declines to
    own at v1.0.0, each with the interface that would exist if the
    non-goal were reversed. (05-non-goals.md.)

## 4. What is out of scope for Phase 7

The following items are explicitly out of scope. Each is either
blocked by another phase, blocked by D09, or deferred post-v1.0.0 as
a documented non-goal (05).

- **praxis as an MCP server.** Exposing praxis invocations themselves
  as an MCP tool endpoint (the "reverse" direction) is not part of
  Phase 7. If there is demand, a future phase decides.
- **Runtime MCP server discovery** (e.g., mDNS, registry lookups).
  Violates D09. Hard non-goal.
- **Dynamic tool registration.** The adapter's tool catalogue is the
  union of tools advertised by the configured servers at session
  open; mid-session catalogue mutation is treated as a session-reset
  event, not as a "tool registry update" API.
- **Implementation code.** Phase 7 is a design phase. The actual
  `praxis/mcp` module is implemented post-phase.
- **Bundled MCP server implementations** (e.g., a praxis-branded
  filesystem server). Out of charter — praxis is an invocation kernel,
  not a tool marketplace.
- **Amendment of the Phase 5 credential contract.** The Phase 5
  credential lifecycle is not re-opened. Phase 7 accommodates it via
  an adapter-local session-credential pattern; it does not propose a
  new `credentials.Resolver` shape.
- **Amendment of `tools.Invoker` / `ToolCall` / `ToolResult` /
  `InvocationContext`.** Phase 3 frozen surface is preserved verbatim.

## 5. Transport priority for v1.0.0

The MCP specification defines three transport bindings (see
`research-solutions.md` §3.1). Phase 7 commits to the following
priority for `praxis/mcp v1.0.0`:

| Transport | v1.0.0 commitment | Rationale |
|---|---|---|
| **stdio** | **Required.** Ships in v1.0.0. | Dominant transport for locally-installed MCP servers; simplest trust model (no network; credentials via env); table-stakes for any "first-class" claim. |
| **Streamable HTTP** | **Required.** Ships in v1.0.0. | Dominant transport for remote MCP servers; without it the adapter is not credible for production use; the 2025 spec addition. |
| **SSE (standalone)** | **Folded into HTTP.** Not a separate commitment. | The official SDK abstracts over SSE and Streamable HTTP; callers do not choose between them at praxis's layer. If the SDK drops SSE support, praxis follows. |

Other transports (WebSocket, Unix sockets, custom binary) are out of
scope for v1.0.0 and enter the v1.x backlog only with a specific
consumer request.

## 6. Why not "pattern-only"

An obvious alternative is to ship no code at all and instead document
a worked example of implementing `tools.Invoker` over an MCP client.
Phase 7 rejects this for the following specific reasons:

1. **Ecosystem fragmentation risk.** Without a canonical adapter,
   every consumer invents its own namespacing convention, credential
   wiring, and error translation. Cross-consumer tool sharing
   degrades. The prior art in LangChainGo (where MCP support is a
   community-maintained external adapter) demonstrates this failure
   mode.
2. **Correctness load.** Getting the `PostToolFilter` hand-off right,
   the error classification right, and the cardinality-safe metric
   labels right is non-trivial. Shipping an audited reference
   implementation is much cheaper than auditing N re-implementations.
3. **First-consumer obligation.** praxis's first production consumer
   will almost certainly need MCP bridging. Moving the work outside
   the framework just moves the audit burden to the first consumer's
   integration layer, where it is less reviewable.
4. **Cost of the fix is low.** The `praxis/mcp` module is a thin
   adapter (the real work is in the official MCP Go SDK). Isolating
   it in a sub-module pays the pattern-only benefits (zero dependency
   cost for non-MCP users) without the pattern-only drawbacks.

"Pattern-only + in-tree reference adapter" collapses into the chosen
positioning once the reference adapter is polished enough to ship,
which it is.

## 7. Why not "core package"

An equally obvious alternative is a `tools/mcp/` sub-package of the
core `praxis` module, sharing the core `go.mod`. Phase 7 rejects this
for exactly one reason: **dependency footprint**.

The core `praxis` module's appeal rests on a small, audited
dependency graph (Phase 5 D73, stdlib-favoured posture). The MCP Go
SDK is not a lightweight dependency — it pulls in a JSON-schema
validation library, an HTTP/SSE plumbing layer, and OAuth support.
Forcing every praxis consumer to compile those into their binary, or
to run `govulncheck` over them, is an unjustifiable cost for the
~50% of consumers who will never use MCP.

The sub-module pattern avoids this cost cleanly. The code lives in
the same repo, under the same CI, with the same reviewer set, and is
released from the same monorepo — but consumers who do not import it
pay nothing.

## 8. Decisions (summary)

| ID | Subject | Outcome |
|---|---|---|
| D106 | MCP positioning at v1.0.0 | First-class, shipped as a separately-versioned Go sub-module |
| D107 | MCP SDK reuse target | Official `modelcontextprotocol/go-sdk` (pinned to `v1.x`), subject to license/Apache-2.0 verification |
| D108 | Transport priority for v1.0.0 | stdio + Streamable HTTP (SSE folded into HTTP) |
| D109 | D09 / Non-goal 7 re-confirmation | No runtime discovery, no plugin loading, no dynamic registration — build-time Go interface composition only |

Full decision text in `01-decisions-log.md`.

## 9. Downstream obligations

Phase 7 is complete only after these downstream impacts are
acknowledged:

- **Phase 6** must record the `praxis/mcp` sub-module in the release
  manifest. Its release cadence is independent of the core module but
  covered by the same release-please pipeline (D92, D93). This is a
  Phase 6 **observation**, not an amendment — Phase 6's multi-module
  monorepo decisions already accommodate it.
- **Phase 8 (Skills)** consumes Phase 7's outputs for its
  namespacing-convention and credential-flow decisions. The Phase 8
  activation gate is Phase 7 approval.
- The banned-identifier CI job's deny-list is not changed by Phase 7.
  The `praxis/mcp` module is subject to the existing deny-list
  verbatim.

---

**Next:** `03-integration-model.md` specifies the adapter API shape,
namespacing, error translation, budget flow, and observability
extensions.
