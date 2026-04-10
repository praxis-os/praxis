# Phase 7 — Decisions Log

**Phase:** MCP Integration
**Range owned:** D106–D121 (contiguous, 16 decisions)
**Status:** **approved** 2026-04-10 after reviewer pass + in-place amendments; `REVIEW.md` verdict **READY**
**Starting baseline:** `docs/PRAXIS-SEED-CONTEXT.md`,
Phase 1–6 approved artifacts.

Each decision below carries: title, status, rationale, alternatives
considered, and a one-line summary for the `roadmap-status` skill.

---

## Amendment protocol

Follows the protocol documented in
`docs/phase-1-api-scope/01-decisions-log.md#amendment-protocol`.
Adopted decisions are working positions, not immutable commitments.
The decoupling contract in seed §6 is binding and is not subject to
amendment.

---

## D106 — MCP positioning at v1.0.0

**Status:** decided
**Summary:** MCP is a first-class, shipped integration delivered as
a **separately-versioned Go sub-module** at
`github.com/praxis-go/praxis/mcp`. Core `praxis` ships no MCP code
and no MCP dependency.

**Decision.** The praxis repository hosts a second Go module at
`praxis/mcp/` with its own `go.mod`. That module implements
`praxis/tools.Invoker` over the official MCP Go SDK and is
consumed via `go get github.com/praxis-go/praxis/mcp`. The core
module's `go.mod` and dependency graph are untouched by Phase 7.
"First-class" means shipped in the same repo, under the same
license, same CI, same reviewer set, and subject to its own v1.0.0
stability commitment on an independent semver line.

**Rationale.** The three candidate positionings are (a) ignore MCP
and leave it to consumers, (b) ship an in-tree `tools/mcp/`
sub-package sharing the core `go.mod`, and (c) ship a sub-module.
Option (a) is rejected because it fragments the ecosystem and
pushes a correctness-sensitive piece of work (trust-boundary
content, credential flow, namespacing, cardinality-safe metrics) to
every consumer individually — the prior-art evidence from
LangChainGo's community-maintained external MCP adapter shows this
failure mode. Option (b) is rejected because the MCP Go SDK's
transitive dependency graph is non-trivial (JSON schema, HTTP/SSE,
OAuth) and polluting every `praxis` consumer's binary with this cost
is unjustifiable for the substantial fraction who will never use
MCP. Option (c) preserves the core's dependency-lean posture while
giving MCP users a supported, auditable, co-evolving integration.

The sub-module pattern is the dominant idiom for "official
extension" packages in large Go projects. Co-evolution is
retained because the code lives in the same repo and is covered
by the same CI; zero-cost opt-out is retained because non-MCP
consumers do not import the module.

**Release-pipeline dependency (amendment 2026-04-10).** An earlier
revision of this decision claimed that Phase 6's release-please
configuration "already accommodates" multi-module releases. This
was incorrect: D84's release-please manifest has a single
`packages` entry (`"."`). Phase 6's D92/D93 cover DCO and PR
review policy, not pipeline multi-module support. The actual
pipeline amendment required to give `praxis/mcp` an independent
semver line is recorded as **D121** in this log. D106 is
conditional on D121 being applied to Phase 6's CI configuration
before any `praxis/mcp` tag is cut.

**Alternatives considered.** (a) Pattern-only docs — rejected;
ecosystem fragmentation, correctness load, duplicated audit
burden. (b) `tools/mcp/` sub-package inside the core module —
rejected; dependency footprint pollution. (c) Separate repository
— rejected; fragments DX and risks version drift.

---

## D107 — MCP SDK reuse target

**Status:** decided (conditional)
**Summary:** Reuse the official `github.com/modelcontextprotocol/go-sdk`
pinned to `v1.x`. Conditional on license and API-stability
verification before the first `praxis/mcp` `go.mod` commit.

**Decision.** The `praxis/mcp` module imports the official MCP Go
SDK maintained by Anthropic and Google. This is the only candidate
among the three identified in `research-solutions.md` §2 that has
an official-maintenance story from the protocol authors and a v1
stability declaration. The sub-module pins to the `v1.x` line.

**Precondition status (updated 2026-04-10 from verified research).**

| # | Precondition | Status |
|---|---|---|
| 1 | **License verification** — must be Apache-2.0-compatible | ✅ **Verified.** New contributions are Apache-2.0; older MIT contributions being migrated. Both compatible with praxis's Apache-2.0 license. |
| 2 | **API stability statement** — must commit to backward compatibility on the `v1.x` line | ✅ **Verified.** The v1.0.0 release (Sep 30, 2025) explicitly states "no breaking API changes"; v1.5.0 (Apr 7, 2026) has continued the line across 22 tagged releases without breakage. |
| 3 | **Transitive-dependency audit** — measure actual graph before committing | ⚠ **Still open.** Header count is 9 direct/indirect, which is manageable on paper. The critical unknown: does `golang.org/x/oauth2` pull in `google.golang.org/grpc` and/or is `golang.org/x/tools` a production dependency (vs test-only)? Must be verified with `go mod graph` before the first `praxis/mcp` `go.mod` commit. |

**Hard gate.** Precondition 3 is the only remaining reason D107 is
"conditional". If the `go mod graph` check reveals unacceptable
transitive bloat (grpc, large net stack, unrelated tooling), D107
is re-opened and the fallback path in `research-solutions.md` §5.3
— "pattern-only reference implementation in `examples/`" — is
chosen instead.

**Two facts that strengthen D107 against alternative SDKs.** The
research pass confirmed (a) that `mark3labs/mcp-go` remains
pre-v1.0 with no stability commitment, which is disqualifying for
a praxis v1.0 dependency; and (b) that `metoro-io/mcp-golang` has
been inactive since January 2025 and does not track the current
2025-11-25 spec. Neither is a viable alternative even if
precondition 3 fails — the fallback on failure is pattern-only,
not a different SDK.

**Rationale.** The alternatives (`mark3labs/mcp-go`,
`metoro-io/mcp-golang`) are capable community SDKs but carry a
maintenance risk: the protocol itself evolves under Anthropic's
stewardship, and an SDK that is not maintained by the protocol
authors has a structural lag. The official SDK is the lowest-risk
long-term commitment. See `research-solutions.md` §2.4 for the
full comparison table.

**Alternatives considered.** (a) `mark3labs/mcp-go` — inspired by
its API shape but rejected as the primary reuse target due to
maintenance-alignment risk. (b) `metoro-io/mcp-golang` — rejected,
less ecosystem traction. (c) Reimplement MCP from JSON-RPC
primitives — rejected, spec surface too large to justify
reinvention when an official SDK exists.

---

## D108 — Transport priority for v1.0.0

**Status:** decided
**Summary:** `praxis/mcp v1.0.0` ships stdio and Streamable HTTP.
SSE is folded into the HTTP implementation (the SDK abstracts
over them). No other transports.

**Decision.** The `praxis/mcp v1.0.0` module supports exactly two
user-visible transports: `TransportStdio` (for locally-installed
MCP server binaries) and `TransportHTTP` (for remote MCP servers
over Streamable HTTP, which the official SDK implements alongside
SSE). A caller who wants pure SSE uses `TransportHTTP` — the SDK
decides which HTTP sub-binding to use based on server capabilities.

WebSocket, Unix sockets, and custom binary transports are out of
scope for v1.0.0. They enter the v1.x backlog only with a concrete
consumer request.

**Rationale.** stdio and Streamable HTTP are the table-stakes
transport set for "first-class MCP support":

- stdio is the default packaging for MCP servers shipped as local
  binaries (which is most of the community ecosystem as of
  2026-04). Without it the adapter is not credible.
- Streamable HTTP is the default transport for remote MCP
  servers and is the 2025 addition that subsumes the older SSE
  binding.

Shipping fewer transports is not credible. Shipping more is
unnecessary surface area.

**Alternatives considered.** (a) stdio only for v1.0 — rejected;
excludes remote MCP servers, which are the majority of
enterprise use cases. (b) HTTP only — rejected; excludes the
dominant transport for local tools. (c) All three (stdio, HTTP,
SSE as distinct types) — rejected; the SDK abstracts SSE into HTTP
and exposing them as separate types leaks SDK-internal detail.

---

## D109 — D09 / Non-goal 7 re-confirmation

**Status:** decided
**Summary:** Re-confirmed. `praxis/mcp` is build-time Go interface
composition. No runtime plugin loading, no dynamic discovery, no
reflection-based extension.

**Decision.** The `praxis/mcp` module does not weaken Phase 1 D09
or Non-goal 7. All of the following are confirmed:

- The adapter is imported at build time by explicit Go `import`.
  Consumers who do not `import "github.com/praxis-go/praxis/mcp"`
  do not ship any MCP code.
- The server set that an `Invoker` fronts is fixed at `New` time
  and is not mutable for the `Invoker`'s lifetime.
- There is no `plugin.Open` call, no WASM host, no reflection
  registry, no dynamic type lookup.
- There is no runtime discovery mechanism: no mDNS, no directory
  lookup, no file-watcher-driven hot reload.

**Rationale.** The "no plugins in v1" commitment is the load-bearing
stability anchor for the core interface surface. Phase 7 must
extend praxis's reach into the MCP ecosystem without loosening it.
Build-time Go interface composition is sufficient: consumers compose
their MCP server list in their own `main()` and pass it to
`mcp.New`, which returns a `tools.Invoker` they then pass to the
core orchestrator. This is exactly the extension model D09 permits.

**Alternatives considered.** (a) Optional runtime-discovery option
as a WithDiscovery functional option — rejected; the option's
mere existence violates D09. (b) Build-tag gated dynamic loader —
rejected for the same reason.

**Forward record.** Any future RFC proposing runtime MCP server
discovery in v2+ must address this decision explicitly.

---

## D110 — Public API surface of `praxis/mcp`

**Status:** decided
**Summary:** Minimal exported surface: `Server`, sealed `Transport`
(with `TransportStdio` and `TransportHTTP`), `Option`,
`WithResolver`/`WithMetricsRecorder`/`WithTracerProvider`, `New`,
`Invoker` (embedding `tools.Invoker` + `io.Closer`). Nothing else.

**Decision.** The full public surface of the `praxis/mcp` module
at v1.0.0 is:

- Types: `Server`, `Transport` (sealed interface),
  `TransportStdio`, `TransportHTTP`, `Option`, `Invoker`.
- Functions: `New`, `WithResolver`, `WithMetricsRecorder`,
  `WithTracerProvider`.

No other exported identifiers. No new type amendments to core
praxis packages. `tools.Invoker`, `ToolCall`, `ToolResult`, and
`InvocationContext` are preserved verbatim.

**Rationale.** Small surface = small freeze. The adapter's job is
to satisfy `tools.Invoker`; its constructor and configuration
surface should be just large enough to do that and no larger. The
sealed `Transport` interface prevents caller-supplied transports,
which would break the adapter's audit story.

**Alternatives considered.** (a) Expose the underlying MCP session
handle for caller introspection — rejected; leaks SDK detail and
expands the freeze surface. (b) Provide a `Builder` type instead
of functional options — rejected; inconsistent with the rest of
praxis's API idiom. (c) Allow third-party transports via an
exported `Transport` method set — rejected; makes the trust model
unauditable.

---

## D111 — Tool namespacing convention

**Status:** decided
**Summary:** Fixed `{LogicalName}__{mcpToolName}` convention with
double-underscore delimiter. Not caller-configurable in v1.0.0.

**Decision.** When the adapter fronts multiple MCP servers, the
tool name exposed to the LLM (and routed by `tools.Invoker`) is
`{LogicalName}__{mcpToolName}`. The delimiter is a double
underscore. The `LogicalName` is the caller-chosen name from
`Server.LogicalName`; the `mcpToolName` is the raw tool name
advertised by the MCP server at session handshake.

**`LogicalName` validation (amendment 2026-04-10):** `LogicalName`
must match the regex `^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$` **and**
must not contain two consecutive underscores anywhere. The
"no `__` in LogicalName" rule is a hard validation at `New` time;
construction fails if any `Server.LogicalName` contains `__`.
This prohibition closes a silent-dispatch hazard: without it,
`LogicalName = "foo__bar"` fronting a tool `baz` produces the
namespaced form `foo__bar__baz`, whose leftmost split recovers
`(foo, bar__baz)` — routing the call to a server named `"foo"`,
not `"foo__bar"`. With the prohibition, the leftmost-split
recovery of server-defined tool names containing `__`
(e.g., `bar__baz`) remains correct because only the server side
can legitimately contain `__`.

The convention is fixed at the adapter level and not
caller-configurable in v1.0.0.

**Rationale.** Three properties converge on `__`:

1. **LLM-safe.** Both major provider tool-name regexes
   (`^[a-zA-Z0-9_-]{1,64}$`) admit it.
2. **Robust.** Double-underscore is rare inside server-defined
   tool names (most use single underscores or camelCase), so the
   delimiter collision rate is low.
3. **Precedented.** Python langchain's `MultiServerMCPClient` and
   several community Go bridges already use `__` as the de-facto
   namespacing separator.

Fixing the convention at v1.0.0 prevents the adapter's public
surface from accumulating a `NamingStrategy` interface that would
later have to be frozen. Consumers with strong naming preferences
build their own adapter.

**Alternatives considered.** (a) Period (`.`) — rejected;
disallowed by some provider tool-name regexes. (b) Slash (`/`) —
rejected; same regex issue. (c) Caller-configurable convention via
`WithNamingStrategy` — rejected; unnecessary v1.0 surface
expansion. (d) No namespacing, collision-last-writer-wins —
rejected; silent correctness hazard in multi-server deployments.

---

## D112 — Budget participation

**Status:** decided
**Summary:** MCP calls participate in the existing `wall_clock`
and `tool_calls` budget dimensions via the standard `tools.Invoker`
accounting path. No new budget dimension.

**Decision.** The core orchestrator's existing budget accounting
treats each `Invoker.Invoke` dispatch as one tool call and
measures its duration. The MCP adapter inherits this accounting
verbatim; MCP calls count against `tool_calls` (incremented by
one) and `wall_clock` (incremented by the call duration). Tokens
and cost are not affected by MCP calls directly — MCP operations
do not consume LLM tokens, and praxis does not price tool calls.

Phase 7 does **not** introduce a "transport bytes" or "MCP
server-side latency" budget dimension.

**Rationale.** `budget.Guard` is frozen at v1.0 (Phase 3 D04).
Adding a dimension is a breaking change. The existing
`wall_clock` and `tool_calls` dimensions already cap the two
obvious cost axes for MCP calls. A cost dimension would require a
pricing source that the MCP spec does not define; consumers who
want to price MCP usage implement their own
`LifecycleEventEmitter` and aggregate from there.

**Alternatives considered.** (a) New `mcp_transport_bytes`
dimension — rejected; breaks the freeze and has no agreed
pricing source. (b) Extend the `cost` dimension with a
caller-supplied per-call fee — rejected; contradicts D08's
per-invocation snapshot pricing model for LLM calls.

**Known gap: streaming response size (amendment 2026-04-10).**
MCP's Streamable HTTP transport permits the server to push
multiple content chunks over an open HTTP stream. `wall_clock`
and `tool_calls` do not bound the **size** of a streamed
response. A pathological server could stream an unbounded
response that exhausts adapter memory before `wall_clock`
fires. The adapter **must** enforce a maximum-response-size
limit at the transport boundary, with a default of **16 MiB**
per tool call. Responses that exceed the limit fail with
`ToolSubKindServerError` (classified as a server policy
violation). A `WithMaxResponseBytes(n int64) Option` is added
to the v1.0.0 public surface (amending D110 §5 of the options
list); the default is applied if the option is not supplied.
This is **not** a new budget dimension — it is an
adapter-local resource guard that operates outside the
`budget.Guard` model.

---

## D113 — Error translation

**Status:** decided
**Summary:** MCP tool-level errors (`isError: true`) and
transport-level errors both map to `ErrorKindTool` with
appropriate `ToolSubKind`. No new error kinds are introduced.

**Decision.** The adapter translates MCP outcomes as follows:

| MCP outcome | `ToolResult.Status` | `errors.Kind` | `ToolSubKind` |
|---|---|---|---|
| `isError: false` success | `ToolStatusSuccess` | n/a | n/a |
| `isError: true` tool result | `ToolStatusError` | `ErrorKindTool` | `ToolSubKindServerError` |
| JSON-RPC protocol error `-32700`…`-32603` | `ToolStatusError` | `ErrorKindTool` | `ToolSubKindServerError` |
| JSON-RPC server-defined error `-32000`…`-32099` | `ToolStatusError` | `ErrorKindTool` | `ToolSubKindServerError` |
| Transport disconnect / I/O failure | `ToolStatusError` | `ErrorKindTool` | `ToolSubKindNetwork` |
| Response schema violation | `ToolStatusError` | `ErrorKindTool` | `ToolSubKindSchemaViolation` |
| Session circuit-broken | `ToolStatusError` | `ErrorKindTool` | `ToolSubKindCircuitOpen` |
| HTTP 401 / 403 on an established session (expired or revoked credential) | `ToolStatusError` | `ErrorKindTool` | `ToolSubKindCircuitOpen` |
| HTTP 429 (rate-limited by MCP server) | `ToolStatusError` | `ErrorKindTool` | `ToolSubKindNetwork` |
| MCP capability-negotiation handshake timeout (single attempt) | `ToolStatusError` | `ErrorKindTool` | `ToolSubKindNetwork` |
| TLS handshake failure (expired certificate, hostname mismatch, untrusted CA) | `ToolStatusError` | `ErrorKindTool` | `ToolSubKindNetwork` |
| Response exceeds the adapter's `MaxResponseBytes` limit (see D112 amendment) | `ToolStatusError` | `ErrorKindTool` | `ToolSubKindServerError` |

The adapter never synthesises `TransientLLMError`,
`PermanentLLMError`, or `SystemError` from MCP failures. It never
introduces a new `ErrorKind` value.

**Why HTTP 401/403 maps to `ToolSubKindCircuitOpen` (amendment
2026-04-10):** a 401/403 on an already-established session
indicates the session's credential is no longer valid. Since
v1.0.0 does not ship credential refresh (Non-goal 7.6), the
session is effectively non-recoverable without external
intervention. Mapping to `ToolSubKindCircuitOpen` causes the
adapter to enter its cool-down window, and the next call after
cool-down triggers a fresh session open with a fresh
`Resolver.Fetch` call — which is the only recovery path
available without credential refresh. This is distinct from the
"repeated handshake failures" `CircuitOpen` case; both map to
the same sub-kind because they both require the same recovery
action (session teardown + re-open).

**Why HTTP 429 maps to `ToolSubKindNetwork` (amendment
2026-04-10):** 429 is semantically transient. Mapping to
`Network` lets the caller's classifier-level retry policy (if
any) treat it consistently with other transient transport
failures. The adapter does not itself retry on 429 — retry is
the caller's decision via the normal `Classifier` path.

**Rationale.** MCP failures are, by construction, tool failures
from the orchestrator's standpoint — the invocation's control flow
is identical whether the tool ran locally or through MCP. Mapping
to `ErrorKindTool` keeps the orchestrator's classifier logic
(Phase 3 D44) unchanged. The existing `ToolSubKind` enum already
covers the MCP failure modes with `ServerError`, `Network`,
`SchemaViolation`, and `CircuitOpen` — no new sub-kind is needed.

**Alternatives considered.** (a) Introduce `ErrorKindMCPTransport`
as a new typed error — rejected; expands the frozen error taxonomy
and forces every Classifier consumer to add a new case. (b) Map
transport errors to `SystemError` — rejected; `SystemError` is for
framework bugs, not for recoverable remote-call failures.

---

## D114 — Content flattening

**Status:** decided
**Summary:** MCP tool-result content arrays are flattened to
text-only, newline-joined output in `ToolResult.Content`. Non-text
blocks are discarded at the adapter boundary.

**Decision.** On MCP tool-call success, the adapter:

1. Filters the response's `content[]` array to text blocks.
2. Joins the text blocks with `"\n\n"` into a single string.
3. Sets `ToolResult.Content` to the joined string.
4. Discards image, audio, and resource-reference blocks.

If the filtered array is empty, `Content` is set to the empty
string and `Status` is `ToolStatusSuccess`.

**Contract note for `PostToolFilter` implementors (amendment
2026-04-10):** the combination
`Status == ToolStatusSuccess && Content == ""` is a **new valid
combination** introduced by this decision that the Phase 3
`ToolResult` documentation does not explicitly call out. It
arises when an MCP tool succeeds and returns only non-text
content (images, audio, resources). `PostToolFilter`
implementors must not treat this combination as a framework
bug or as a "denied" signal — it is a legitimate "tool ran
successfully but produced no text" outcome. The adapter's
godoc and the Phase 5 filter-implementor guidance are both
updated to document this.

**Rationale.** `tools.ToolResult.Content` is frozen at `string` in
Phase 3 D40 and Phase 1 D04. A structured content array is a
breaking change to the core interface surface. Flattening is an
honest lossy projection with a documented information-loss
boundary; a post-v1 `ToolResultV2` amendment can introduce a
structured content field.

Joining with newlines (rather than JSON-encoding) optimises for
the LLM's comprehension of the result when it is fed back into
the conversation turn. JSON encoding would make
`PostToolFilter`'s prompt-injection detection less effective,
because injection payloads hidden inside nested JSON fields
would be harder to match against text patterns.

**Alternatives considered.** (a) JSON-encode the content array —
rejected; worse LLM affordance and weaker filter story. (b) Return
the first text block and discard all others — rejected; loses
information for multi-block responses. (c) Amend `ToolResult` to
carry a structured content slice — rejected; breaks the v1.0
freeze.

---

## D115 — MCP-specific metrics

**Status:** decided
**Summary:** The adapter ships three bounded-cardinality metrics
via an optional `mcp.MetricsRecorder` extension interface.
Construction fails if more than 32 distinct server `LogicalName`s
are passed to `New`.

**Decision.** The adapter introduces three new metrics and a
companion optional interface:

```go
type MetricsRecorder interface {
    RecordMCPCall(server, transport, status string, duration time.Duration)
    RecordMCPTransportError(server, transport, kind string)
}
```

| Metric | Type | Labels |
|---|---|---|
| `praxis_mcp_calls_total` | Counter | `server`, `transport`, `status` |
| `praxis_mcp_call_duration_seconds` | Histogram | `server`, `transport`, `status` |
| `praxis_mcp_transport_errors_total` | Counter | `server`, `transport`, `kind` |

Label cardinality is bounded: `server` by the configured
`LogicalName` set with a hard cap of **32** enforced at `New`
time; `transport` to the fixed `{stdio, http}` set; `status` to
the fixed `{ok, error, denied}` set; `kind` to the fixed
`{network, protocol, schema, circuit_open, handshake}` set.

The core `telemetry.MetricsRecorder` interface is not modified.

**Detection mechanism (amendment 2026-04-10).** The adapter
accepts a `telemetry.MetricsRecorder` via `WithMetricsRecorder`
and, at construction time, type-asserts the passed recorder
against the separate `mcp.MetricsRecorder` interface:

```go
// Simplified:
coreRec := opt.metricsRecorder // telemetry.MetricsRecorder
mcpRec, hasMCP := coreRec.(mcp.MetricsRecorder)
```

If the type assertion succeeds, the adapter records MCP-specific
metrics through `mcpRec`. If it fails, MCP-specific metrics are
silently dropped — the core metrics (`praxis_tool_calls_total`
etc.) continue to be recorded through the core
`telemetry.MetricsRecorder` by the orchestrator's normal
accounting path.

**This is not the D100 `MetricsRecorderV2` pattern.** D100
defines an embedding-based extension for the core
`telemetry.MetricsRecorder` interface (`MetricsRecorderV2`
embeds `MetricsRecorder`). The `mcp.MetricsRecorder` defined
here is a **separate, standalone interface in a different
package** that does not embed `telemetry.MetricsRecorder`.
Callers who want MCP metrics must either (a) pass a recorder
that implements both interfaces directly, or (b) pass an
adapter type that wraps a core recorder and adds MCP methods.
Previous revisions of this decision described the pattern as
"consistent with D100"; that description was misleading and is
withdrawn.

Callers who pass a recorder implementing only
`telemetry.MetricsRecorder` (including
`telemetry.NullMetricsRecorder`) silently drop MCP-specific
metrics. The adapter's godoc calls this out explicitly so
operators are not surprised when the MCP dashboards stay
empty.

**Rationale.** Adding MCP metrics as new label values on the
existing `praxis_tool_calls_total` metric would risk cardinality
explosion via the N × M `tool_name × server` combination. A
dedicated metric set with bounded labels is cleaner and respects
Phase 4 D57 / D60. The 32-server cap is a guardrail, not a hard
architectural limit — it matches the realistic upper bound of
simultaneously wired MCP servers in any deployment we can envision.

**Alternatives considered.** (a) Fold MCP calls into the core
`praxis_tool_calls_total` with a new `server` label — rejected;
cardinality explosion. (b) No MCP-specific metrics — rejected;
operators lose the ability to per-server SLO or alert. (c) Expose
the cap via an option — rejected; unnecessary knob.

---

## D116 — Trust classification of the MCP transport edge

**Status:** decided
**Summary:** The MCP transport edge is a Phase 5 trust boundary.
MCP-sourced content is classified as untrusted by the existing
Phase 5 D77 contract without modification. No new filter, hook,
or trust tier is added.

**Decision.** The MCP transport edge — the boundary between the
praxis process and any MCP server — is classified as a trust
boundary. `ToolResult.Content` produced by MCP tools is untrusted
by contract and must pass through `PostToolFilter` before being
injected into the conversation history. This is the existing
Phase 5 D77 contract applied verbatim; Phase 7 does not add a new
filter interface, a new trust tier, or a new hook phase.

**Rationale.** Ranking trust levels across tool sources invites
false-hierarchy risks (consumers who mark some tools as "trusted"
and skip filtering miss injections from compromised local
binaries). Content-based filtering is strictly better than
source-based filtering for the prompt-injection threat. The
framework cannot certify MCP server trustworthiness — that is a
consumer-side decision. Phase 5's uniform "all tool output is
untrusted" model handles MCP cleanly.

**Alternatives considered.** (a) Tag MCP content with `source="mcp"`
in a new `ToolResult` field — rejected; breaks the v1.0 freeze and
introduces false-hierarchy risk. (b) Ship an MCP-specific
`PostToolFilter` that runs inside the adapter — rejected;
duplicates the caller-supplied filter layer and fragments the
policy story.

---

## D117 — Credential flow for long-lived MCP sessions

**Status:** decided
**Summary:** The adapter fetches credentials via `credentials.Resolver`
on the first `Invoke` call to each server, uses them to open the
session, and calls `Credential.Close()` immediately after session
establishment. Subsequent calls reuse the session without
re-fetching. Soft-cancel rules from D69 apply to the fetch.

**Decision.** The adapter's credential lifecycle:

1. At `New` time: no credential fetch. Only validates that
   `CredentialRef` values are well-formed.
2. On the first `Invoke` call routed to a given server:
   - Calls `Resolver.Fetch(ctx, server.CredentialRef)` inside the
     tool-call goroutine.
   - Uses `Credential.Value()` to authenticate the MCP session
     (bearer header for HTTP, env var for stdio).
   - Calls `Credential.Close()` immediately after the session is
     established.
3. Subsequent calls to the same server reuse the open session.
   No re-fetch, no credential-handle retention beyond the first
   call.
4. On session tear-down (`Invoker.Close`, circuit-open cool-down,
   process exit), the session closes; no credential to zero at
   this point.

The first-call fetch uses the Phase 5 D69 soft-cancel
context-derivation rule verbatim. If the first-call handshake does
not complete within the 500 ms grace window, the adapter abandons
the session open and returns
`ToolResult{Status: ToolStatusError, Err: ErrorKindTool/ToolSubKindNetwork}`.

**Rationale.** MCP sessions are long-lived, and the Phase 5
"credentials fetched per tool call" posture is in tension with
session-based authentication. The resolution preserves the Phase
5 structural invariants — zero-on-close, no-credential-in-shared-state,
soft-cancel fetch semantics — at the cost of an acknowledged
imperfect zeroing boundary: once the credential has been passed
to the MCP SDK for bearer-header or env-var construction, the
SDK's internal cache is outside praxis's zeroing guarantee. This
matches the Phase 5 D67 §4.3 "acceptable risk" statement.

The alternative — fetching credentials on **every** MCP call —
would require tearing down and re-opening the MCP session per
call, making the adapter latency-prohibitive and defeating the
session-oriented design of the protocol.

**Alternatives considered.** (a) Fetch credentials per call —
rejected; unacceptable latency, incompatible with session-based
MCP auth. (b) Fetch at `New` time — rejected; credential is held
for the full `Invoker` lifetime with no per-call resolver
observation, weakening the Phase 5 audit story. (c) Retain the
`Credential` handle and `Close()` only at `Invoker.Close` time —
rejected; violates Phase 5 zero-on-close by holding the secret
indefinitely.

**Known residual risk.** The SDK-owned copy of the credential is
not zeroed by praxis. Consumers with strict in-memory-secret
requirements are advised in godoc to use short-lived credentials
or KMS-backed proxy tokens.

**Accepted deviation from Phase 5 §3.2 (amendment 2026-04-10):**
Phase 5 `02-credential-lifecycle.md` §3.2 states the credential
goroutine-scope isolation invariant: "The `Credential` value is
used only within the goroutine that received it from
`Resolver.Fetch`. It is not passed to other goroutines." This
invariant is **breached** by the HTTP transport path in the
MCP adapter: the bearer-token string is handed to the underlying
HTTP client library, which maintains connection pools and
keep-alive goroutines that may read the token during
connection-reuse and re-authentication operations. The breach
is structural and unavoidable — any HTTP client that supports
connection reuse will read auth headers from a background
goroutine.

This is a separate concern from the D67 zero-on-close boundary
discussed above. D67 governs when the credential bytes are
erased; Phase 5 §3.2 governs which goroutines can observe them
while they are still live. The MCP adapter breaches the
**goroutine isolation** invariant for HTTP transport only; the
stdio path does not breach it because the credential bytes are
consumed inside the spawning goroutine and never cross a
goroutine boundary within the praxis process.

The breach is accepted as an architectural consequence of
supporting HTTP-backed MCP sessions. It is documented as a
known deviation in the adapter's godoc, classified at the same
"acceptable risk" tier as the D67 §4.3 statement, and is
recoverable post-v1.0 only if MCP moves to a request-scoped auth
model or praxis adopts a KMS-backed proxy-token pattern at
the `credentials.Resolver` layer (a caller decision, not a
framework change). No new Phase 5 decision is opened by this
acknowledgement — the deviation is recorded here and referenced
forward by any future Phase 5 or Phase 7 amendment.

**Credential refresh.** Not supported in v1.0.0 (D120 Non-goal
7.6). Expiring credentials fail the session with a transport auth
error, triggering the circuit-open cool-down; the next call
re-fetches. v1.x may add explicit refresh support via an optional
interface.

---

## D118 — `SignedIdentity` propagation policy

**Status:** decided
**Summary:** `tools.InvocationContext.SignedIdentity` is **not**
forwarded to MCP servers by the v1.0.0 adapter. Consumers who
need identity-chain propagation build a wrapping invoker
themselves.

**Decision.** The v1.0.0 `praxis/mcp` adapter does not:

- Add the signed-identity JWT to any MCP HTTP header.
- Add the JWT to any stdio environment variable.
- Add the JWT to any MCP JSON-RPC request field.
- Log the JWT at any level.
- Include the JWT in any metric label.
- Include the JWT in any span attribute.

The adapter is required to neither read nor write `SignedIdentity`
except to the extent needed to prove it is not being forwarded.

**Rationale.**

1. **Spec silence.** MCP does not standardise agent-identity
   propagation as distinct from session authentication. Shipping
   a praxis-specific convention would create a de-facto standard
   the project has no authority to set.
2. **Credential disclosure risk.** An MCP server is an external
   process praxis does not necessarily control. Forwarding a
   bearer JWT to it is a credential-disclosure risk that should
   require explicit caller opt-in.
3. **Defence in depth.** Phase 5 D79's `RedactingHandler`
   deny-list includes `praxis.signed_identity` and `_jwt` suffix
   patterns specifically to catch accidental JWT logging.
   Forwarding the JWT through the adapter is exactly the kind of
   accidental exposure the deny-list is supposed to backstop.
4. **Escape hatch is cheap.** Consumers who do want identity
   forwarding build a ~20-line wrapping `tools.Invoker` that
   reads `SignedIdentity` from `InvocationContext` and injects it
   through whatever transport mechanism their threat model
   permits.

**Alternatives considered.** (a) `WithForwardSignedIdentity(headerName string)`
option — rejected; would make "forward an auth JWT to an
external process" a one-line opt-in, lowering the guardrail below
safe levels. (b) Automatic forwarding to HTTP transports only —
rejected; still external-process exposure. (c) Forward only when
the server has a matching `X-Praxis-Identity` capability — rejected;
no such capability exists in the MCP spec.

---

## D119 — stdio transport hardening requirements

**Status:** decided
**Summary:** stdio transport must resolve `Command` to an absolute
path at `New` time (cached), deliver credentials via a
privately-owned byte buffer that is zeroed after `Cmd.Start`,
redirect stdio to pipes (not inherit), pass no extra file
descriptors, and use process-group isolation.

**Decision.** When implementing `TransportStdio`, the adapter
must satisfy the following concrete requirements:

1. **Absolute command resolution at `New` time.** `Command` is
   resolved via `exec.LookPath` at `New` time and the resolved
   absolute path is stored internally. All subsequent child-process
   launches (including after a circuit-open cool-down) use the
   cached absolute path. `$PATH` is not re-consulted.
2. **Credential delivery via owned byte buffer.** The resolved
   credential's `Value()` bytes are copied into a buffer owned by
   the adapter. The env var string is constructed from this buffer
   in a narrow scope; the buffer is zeroed (via
   `credentials.ZeroBytes`) immediately after `Cmd.Start`
   returns. The adapter does not concatenate the credential byte
   slice into a Go string at any point prior to `Cmd.Start`
   (because Go strings are immutable and cannot be zeroed).
3. **Stdio redirection to pipes.** `Cmd.Stdin`, `Cmd.Stdout`,
   `Cmd.Stderr` are set to pipes owned by the adapter. The child
   process does not inherit the parent's stdio.
4. **No extra file descriptors.** `Cmd.ExtraFiles` is nil.
5. **Process-group isolation.** On Unix, `Cmd.SysProcAttr.Setpgid
   = true` so that `Invoker.Close` can kill the child as a group.
   On other platforms, the closest equivalent is used.
6. **SIGPIPE handling (amendment 2026-04-10).** Writes to the
   child's stdin pipe must handle `EPIPE` cleanly. The adapter
   wraps all pipe writes in a handler that converts `EPIPE` /
   `io.ErrClosedPipe` into a transport-level error
   (`ToolSubKindNetwork`) instead of allowing Go's default
   SIGPIPE-on-write behaviour to propagate to the praxis process.
   On Linux, Go already installs a SIGPIPE handler for the main
   goroutine's stdio but not for pipes created via `os/exec`;
   the adapter therefore handles `EPIPE` at every write site.
7. **Child resource constraints (amendment 2026-04-10).** The
   adapter does not impose OS-level resource limits
   (`setrlimit`, cgroups) on the child process in v1.0.0 —
   doing so cleanly across platforms is out of scope. The
   godoc warns operators that a misbehaving MCP server binary
   can exhaust the parent process's file-descriptor table or
   memory if it leaks descriptors or holds large buffers, and
   recommends running untrusted MCP binaries under an external
   supervisor (systemd, launchd, container) that enforces
   resource limits. A v1.x amendment may add a
   `WithChildRLimits(rl RLimits) Option` once a portable API
   shape is agreed.

**Rationale.** Each requirement closes a specific threat:

- **Absolute path caching** closes a TOCTOU window where an
  attacker could write a malicious binary to a `$PATH`-earlier
  directory between the first and a later launch.
- **Owned byte buffer** preserves the Phase 5 D67 zero-on-close
  invariant up to the `Cmd.Start` boundary; without this, the
  credential would leak into an immutable Go string for the
  full lifetime of `Cmd.Env`.
- **Stdio redirection** prevents the child from writing to the
  praxis operator's terminal or consuming praxis's stdin.
- **No extra FDs** prevents the child from inheriting sensitive
  file descriptors (e.g., a vault unix socket the parent holds).
- **Process-group isolation** ensures `Invoker.Close` terminates
  the child deterministically, preventing orphan-process leaks.

**Alternatives considered.** (a) Trust `exec.LookPath` at every
launch — rejected; TOCTOU. (b) Construct env var via string
concatenation — rejected; violates zero-on-close. (c) Inherit
parent's stdio — rejected; privilege leak. (d) No process group —
rejected; child process leak on orchestrator crash.

---

## D120 — Phase 7 non-goals binding

**Status:** decided
**Summary:** The ten non-goals in `05-non-goals.md` are binding
for `praxis/mcp v1.0.0`. Amendment requires an explicit decision
record.

**Decision.** The non-goals catalogued in
`docs/phase-7-mcp-integration/05-non-goals.md` (§7.1 through §7.10)
are binding for `praxis/mcp v1.0.0`. They are:

1. No runtime MCP server discovery.
2. No praxis-as-MCP-server.
3. No dynamic tool registration mid-session.
4. No multi-modal content preservation.
5. No bundled MCP server implementations.
6. No credential refresh.
7. No custom `http.Client` / `RoundTripper` option.
8. No `SignedIdentity` forwarding.
9. No adapter-level policy / denial hook.
10. No runtime MCP SDK version switching.

**Rationale.** The catalogue gives future reviewers a concrete
reference for rejecting feature requests on charter grounds,
matching the Phase 1 Non-goals convention. See `05-non-goals.md`
for full per-item rationale, "interface that would exist if
reversed", and reversal cost.

**Alternatives considered.** (a) Merge into
`02-scope-and-positioning.md` — rejected; non-goals deserve their
own file for independent linkability from later phases. (b) Omit
the list, rely on scope statement — rejected; too narrative for
PR-review citation.

---

## D121 — Phase 6 release-pipeline amendment for the `praxis/mcp` sub-module

**Status:** decided
**Summary:** Phase 7 records a specific amendment obligation against
Phase 6 D84: the release-please configuration must grow a second
`packages` entry for `mcp/` before any `praxis/mcp` tag is cut.
This decision is the binding reference for that obligation.

**Decision.** Before the first `praxis/mcp` release commit lands on
`main`, the release-please manifest at
`.github/release-please-config.json` (per D84) must be extended
from the single-package form:

```json
{
  "packages": {
    ".": { ... }
  }
}
```

to the two-package form:

```json
{
  "packages": {
    ".": { ... existing core config ... },
    "mcp": {
      "release-type": "go",
      "bump-minor-pre-major": true,
      "bump-patch-for-minor-pre-major": false,
      "always-update": true,
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
      "extra-files": ["mcp/internal/version/version.go"]
    }
  }
}
```

and a corresponding `.github/release-please-manifest.json` must
track two keys:

```json
{
  ".": "0.5.0",
  "mcp": "0.0.0"
}
```

Conventional commits scoped to the `praxis/mcp` module must
carry a path-prefixed scope (`feat(mcp): ...`,
`fix(mcp): ...`) so that release-please routes them to the
correct package. This convention is recorded here; the
`commitsar` configuration (D84) accepts it without modification
because it uses a free-form scope field.

**This is an amendment obligation against Phase 6, not an
automatic amendment.** Phase 6's decision log (D84) is not
retroactively edited. The Phase 6 `01-decisions-log.md`
Amendment Protocol requires that amendments live in the phase
that discovers the need. This decision (D121) is that record.
When the release pipeline is actually updated in the repository,
the corresponding `release-please` configuration commit cites
D121 in its commit message.

**Rationale.** The reviewer pass on Phase 7 surfaced a factual
error in the original D106 rationale: "Phase 6's monorepo release
pipeline already accommodates multi-module releases." It does
not. D84's manifest contains a single `.` package entry, and no
Phase 6 decision adds a second entry. Without this amendment,
the `praxis/mcp` sub-module would never receive an independent
tag and the "separately-versioned" commitment in D106 would be
unsupported. D121 makes the pipeline obligation explicit so that
no implementer can ship `praxis/mcp` under the assumption that
the pipeline is already in place.

**Alternatives considered.** (a) Retroactively edit D84 — rejected
by the Phase 1 Amendment Protocol; the original decision stays
intact and amendments are recorded in the phase that discovers
them. (b) Defer the pipeline obligation to the implementation
phase without a Phase 7 record — rejected; the "first-class
sub-module" claim in D106 would then be architecturally
unsupported at Phase 7 close, which the reviewer correctly
flagged as a blocker.

**Cross-reference.** D106 rationale amendment (2026-04-10)
points at this decision.

---

## Adopted decisions summary (for `roadmap-status`)

| ID | Title | Status |
|---|---|---|
| D106 | MCP positioning: sub-module at `praxis/mcp` | decided |
| D107 | MCP SDK reuse target: official `modelcontextprotocol/go-sdk` | decided (conditional on license / stability verification) |
| D108 | Transport priority: stdio + Streamable HTTP for v1.0.0 | decided |
| D109 | D09 / Non-goal 7 re-confirmation: build-time only | decided |
| D110 | `praxis/mcp` public API surface (minimal) | decided |
| D111 | Tool namespacing: `{logicalName}__{mcpToolName}` | decided |
| D112 | Budget participation via existing dimensions | decided |
| D113 | Error translation to `ErrorKindTool` + sub-kinds | decided |
| D114 | Content flattening (text-only, newline-joined) | decided |
| D115 | MCP metrics via optional extension interface, 32-server cap | decided |
| D116 | MCP transport edge = Phase 5 trust boundary, no new tier | decided |
| D117 | Credential flow: first-call fetch + session-reuse | decided |
| D118 | No `SignedIdentity` forwarding in v1.0.0 | decided |
| D119 | stdio transport hardening requirements | decided |
| D120 | Phase 7 non-goals binding | decided |
| D121 | Phase 6 release-pipeline amendment obligation for `praxis/mcp` | decided |

**Range closed.** D106–D121 (16 decisions, contiguous).
Phase 8 (Skills Integration) opens D122 onwards.
