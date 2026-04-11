# Phase 7 — Non-goals

**Decisions:** D120
**Cross-references:** Phase 1 `03-non-goals.md` (Non-goal 7 /
no plugins), 02-scope-and-positioning.md §4 (out-of-scope list),
03-integration-model.md, 04-security-and-credentials.md.

---

Every non-goal below is binding for `praxis/mcp v1.0.0`. Reversing
any of them requires a new planning phase (or a Phase 7 amendment
decision) and explicit impact on the core library's Non-goal 7 /
D09 commitment.

For each non-goal, the entry lists the interface that would exist
if the non-goal were reversed — matching the Phase 1 non-goals
convention and giving future reviewers a concrete reference for
rejecting feature requests on charter grounds.

---

## Non-goal 7.1 — No runtime MCP server discovery

**Commitment.** The `praxis/mcp` adapter will not ship any runtime
discovery mechanism: no mDNS, no registry lookup, no directory
service, no file-system watcher that auto-loads new servers, no
hot-reload of the server list. The server set is fixed at `New`
time.

**Interface that would exist if reversed.** `mcp.Discovery`,
`mcp.ServerRegistry`, or a `WithDiscovery(d Discovery)` option on
`New`.

**Rationale.** Runtime discovery is exactly the shape of the
"plugin system" that Phase 1 D09 / Non-goal 7 rejects. Even if the
discovery source is static (a file on disk), the lifecycle of
"server appeared / server disappeared" is a runtime extension event
that the framework cannot freeze under the v1.0 stability commitment.
Consumers who need runtime server management build their own wrapping
invoker that re-constructs the MCP adapter on config change.

---

## Non-goal 7.2 — No praxis-as-MCP-server

**Commitment.** The `praxis/mcp` module will not ship a server-side
implementation that exposes a praxis `AgentOrchestrator` as an MCP
tool endpoint. Phase 7 is one-way: praxis is an MCP **client** that
calls remote MCP servers. The reverse direction (making praxis itself
an MCP server) is not part of v1.0.0.

**Interface that would exist if reversed.** `mcp.ServerHandler`,
`mcp.Expose(orchestrator AgentOrchestrator) Handler`, and a
sub-package `praxis/mcp/server`.

**Rationale.** Exposing an agent as an MCP tool has a different
design shape: it requires protocol-level implementation of the
server half of the spec, a different transport story (HTTP listener,
stdio pipe ownership), and a different threat model (praxis is now
the untrusted source from the remote caller's standpoint). These
are distinct enough to warrant their own design phase if demand
emerges. Shipping both directions in v1.0.0 doubles the surface
area without evidence that the demand is there.

---

## Non-goal 7.3 — No dynamic tool registration mid-session, and `tools/list_changed` notifications are ignored

**Commitment.** The adapter does not expose an API for adding or
removing tools mid-session. The MCP spec permits servers to emit
`tools/list_changed` notifications when their tool catalogue
mutates after `initialize`; the praxis adapter **ignores** these
notifications in v1.0.0. The tool catalogue observed at session
open is the one used for the lifetime of the session. If a caller
needs to react to catalogue drift, the caller tears down the
`Invoker` and constructs a new one, forcing a fresh session open
and a fresh catalogue read.

No `AddTool` / `RemoveTool` API is exposed. The adapter's public
surface is fixed at the list of servers passed to `New`.

**Rationale for ignoring `tools/list_changed` (amendment
2026-04-10 from verified research).** None of the surveyed bridge
implementations (Eino, LangChainGo-MCP-adapter, Python
langchain-MCP, OpenAI Agents SDK, Google ADK) handle
`tools/list_changed` at all — they all capture the tool list once
at session open and ignore subsequent notifications. Shipping the
same default in praxis keeps the adapter's observable surface
stable (tool-name set is fixed per `Invoker`, metric labels are
stable, span attributes are stable) and matches the ecosystem
baseline. Supporting the notification would require either a new
callback on `mcp.New` (expanding the D110 freeze surface) or a
synchronous tool-list refresh on every `Invoke` call (adding an
RPC round-trip to the hot path). Neither is justified for v1.0.0.

A consumer that specifically needs dynamic tool-set mutation can
treat it as a v1.x backlog item — adding it non-breakingly via a
new `WithListChangedHandler(fn func(Server))` option is possible
within the functional-options convention without modifying the
frozen shape.

**Interface that would exist if reversed.** `mcp.Invoker.AddTool`,
`mcp.Invoker.RemoveTool`, or an `mcp.ToolRegistry` abstraction.

**Rationale.** Exposing dynamic tool registration would require the
framework to take a position on tool-name stability, metric-label
stability, and span-attribute cardinality for a moving target. The
existing "re-open the session on catalogue change" strategy is
simpler and preserves the Phase 4 cardinality guarantees.

---

## Non-goal 7.4 — No multi-modal content preservation

**Commitment.** v1.0.0 flattens MCP tool responses to text-only and
discards image, audio, and resource-reference content blocks (see
03-integration-model.md §6). The adapter does not preserve
multi-modal content in `ToolResult.Content`.

**Interface that would exist if reversed.** Either an extension of
`tools.ToolResult` with a `Content []ContentBlock` field (breaking
the v1.0 freeze), or an MCP-specific adapter that returns a new
`mcp.ToolResult` type instead of `tools.ToolResult`.

**Rationale.** `tools.ToolResult.Content` is frozen at `string` in
Phase 3 D40 and Phase 1 D04. Expanding it to a structured content
array is a breaking change that cannot be made in v1.x. The
v1.0.0 flattening is an honest projection with a documented
information-loss boundary. A post-v1 phase can revisit this by
introducing a `ToolResultV2` with a structured content field.

---

## Non-goal 7.5 — No bundled MCP server implementations

**Commitment.** The `praxis/mcp` module ships no reference MCP server
(no filesystem server, no HTTP-fetch server, no shell-exec server,
no "praxis demo" server). The module is strictly a **client
adapter**.

**Interface that would exist if reversed.** Sub-packages such as
`praxis/mcp/servers/fs`, `praxis/mcp/servers/http`, etc.

**Rationale.** praxis is an invocation kernel, not a tool
marketplace. Bundling servers would expand the module's scope
dramatically, introduce server-side security obligations (sandboxing,
input validation, output truncation), and put praxis in competition
with the growing MCP server ecosystem. Consumers who need an MCP
server install one of the many community-maintained servers.

---

## Non-goal 7.6 — No credential refresh in v1.0.0

**Commitment.** v1.0.0 does not provide an automatic credential-
refresh path for long-lived MCP sessions. Expiring credentials are
detected via transport-level auth failure and handled by the
circuit-open / cool-down / reconnect path
(04-security-and-credentials.md §2.4).

**Interface that would exist if reversed.** `mcp.CredentialRefresher`
interface with a `Refresh(ctx, CredentialRef) (Credential, error)`
method, plus a refresh-scheduling option on `New`.

**Rationale.** Credential refresh is correct to want but
disproportionate to ship in v1.0.0. The existing circuit-open path
gives correct-if-latency-spiky behaviour. v1.x may add refresh
support via an optional interface without a breaking change.

---

## Non-goal 7.7 — No custom `http.Client` or `http.RoundTripper` option

**Commitment.** The v1.0.0 adapter does not expose a
`WithHTTPClient` or `WithRoundTripper` option. HTTP transport uses
the official MCP Go SDK's default HTTP client with Go's default TLS
verification.

**Interface that would exist if reversed.** `WithHTTPClient(c *http.Client) Option`.

**Rationale.** Exposing the HTTP client introduces a configuration
surface that interacts non-trivially with the MCP SDK's own
connection pooling, timeout handling, and reconnect logic. Shipping
it in v1.0.0 without a clear cross-product test matrix is risky.
v1.x can add this option if a concrete consumer need emerges (e.g.,
TLS pinning, mTLS, proxy configuration).

---

## Non-goal 7.8 — No `SignedIdentity` forwarding to MCP servers

**Commitment.** The v1.0.0 adapter does not forward
`tools.InvocationContext.SignedIdentity` to MCP servers via any
transport mechanism (header, JSON-RPC field, env var). See D118.

**Interface that would exist if reversed.** A
`WithForwardSignedIdentity(headerName string) Option` or
equivalent.

**Rationale.** Fully documented in
`04-security-and-credentials.md` §3. Summary: the MCP spec does
not standardise agent-identity propagation, forwarding to an
external process is a credential-disclosure risk, and consumers
who need identity chaining build a wrapping invoker in ~20 lines
of their own code.

---

## Non-goal 7.9 — No adapter-level policy/denial hook

**Commitment.** The `praxis/mcp` adapter does not expose a
`WithPolicyHook` or equivalent option for per-MCP-call policy
evaluation. Policy evaluation lives at the existing Phase 3 seams:
`PolicyHook` for lifecycle phases and `PostToolFilter` for tool
output. The adapter does not shadow these.

**Interface that would exist if reversed.** An `mcp.Policy` with a
`Check(call ToolCall, server Server) Decision` method.

**Rationale.** Phase 3's hook and filter model is the single point
of policy enforcement in praxis. Adding a second policy layer inside
the adapter would confuse the composability story (where does
denial come from? the hook or the adapter?) and would fragment
consumer policy code. Consumers who want MCP-specific policy logic
wire it into their `PolicyHook` or `PostToolFilter` using the
`call.Name` (which carries the namespaced MCP tool name).

---

## Non-goal 7.10 — No runtime MCP SDK version switching

**Commitment.** Each tagged `praxis/mcp` release is pinned to a
specific minor version of the official MCP Go SDK. Runtime
selection of SDK version (e.g., "use SDK 1.2 for server A and SDK
1.3 for server B") is out of scope.

**Interface that would exist if reversed.** A build-tag or
runtime-option mechanism for SDK version selection.

**Rationale.** Go has no idiomatic way to depend on two versions
of the same module in one binary. The module-version alignment
is a release-time constraint, handled by bumping `praxis/mcp`'s
`go.mod` and releasing a new `praxis/mcp` tag.

---

## D120 — Phase 7 non-goals binding

**Status:** decided
**Summary:** The ten non-goals above are binding for
`praxis/mcp v1.0.0`. Any amendment requires an explicit decision
record in Phase 7 or a successor phase.

**Rationale.** The non-goals list is the primary reviewer tool for
rejecting feature requests on charter grounds without per-issue
re-litigation, matching the convention established in Phase 1
Non-goals.

**Alternatives considered.** (a) No non-goals list, rely on scope
statement — rejected; scope statements are too narrative to cite in
PR review. (b) Merge non-goals into 02-scope-and-positioning.md —
rejected; non-goals deserve their own file matching the Phase 1
layout and are referenced by future phases independently.

---

## Non-goal audit

Every proposed v1.0.0 `praxis/mcp` feature is checked against the
above list. If a proposed feature would implement any of the
"interface that would exist if reversed" entries, the proposal is
rejected unless this document is amended first.

The list is expected to grow, not shrink, during implementation as
tempting adjacent features are declined. New non-goals may be added
by amendment; reversing an existing non-goal requires a documented
amendment with its own decision ID.

---

## Relationship to Phase 1 Non-goals

Phase 7 does **not** add to the core `praxis` Non-goals list (Phase 1
`03-non-goals.md`). Phase 1 Non-goal 7 (no plugins) is preserved
verbatim; the `praxis/mcp` module is build-time Go interface
composition, which is explicitly permitted by D09.

The Phase 7 non-goals above are **scoped to the `praxis/mcp`
module** and do not constrain the core library or any other future
sub-module.
