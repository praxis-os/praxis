# Phase 7 — Research: Go MCP SDK Ecosystem

> **Scope.** Pre-design scan for Phase 7 "MCP Integration". Catalogues the
> Go MCP SDK landscape, summarises prior art for MCP-bridge patterns in
> other agent frameworks, records the protocol facts that bind the design,
> and gives a reuse-vs-build recommendation.
>
> **Method.** Synthesis from public sources as of 2026-04-10. Initial
> draft written under time pressure from prior knowledge when the
> first solution-researcher agent stalled; subsequently replaced by the
> researcher's verified output. Remaining unverified claims are
> explicitly marked `[unverified]`.

---

## 1. Need

praxis needs a clear stance on how its build-time `tools.Invoker` seam
can be wired to external MCP servers. Before Phase 7 can issue
decisions, the team must know which Go MCP SDKs exist, how mature they
are, what licenses they carry, how many transitive dependencies they
add, and what patterns comparable agent orchestrators use to bridge
MCP into their generic tool abstraction. This research surfaces that
prior art so Phase 7 decision-making is grounded in what exists rather
than assumptions.

---

## 2. Go MCP SDKs — detailed evaluation

### 2.1 `github.com/modelcontextprotocol/go-sdk`

- **What it is:** the official, jointly-maintained Go SDK for the
  Model Context Protocol, administered by the Go team and Anthropic.
- **URL:** <https://github.com/modelcontextprotocol/go-sdk>
- **License:** Apache-2.0 for all new contributions; older
  MIT-licensed contributions are being migrated under a consent
  process. pkg.go.dev reports Apache-2.0 / CC-BY-4.0 / MIT. Both
  Apache-2.0 and MIT are compatible with praxis's Apache-2.0 license.
- **Latest version:** v1.5.0 (April 7, 2026), following v1.0.0
  (September 30, 2025 — the stability milestone).
- **Release cadence:** active; 22 tagged releases, sustained 2025–2026
  cadence.
- **Maturity:** production. Maintainers explicitly committed to "no
  breaking API changes" at v1.0.0. 4,300+ GitHub stars, 401 forks,
  58+ contributors.
- **Go version requirement:** tracks the two most recently supported
  Go releases. Given the v1.5.0 release date, the `go` directive is
  likely `go 1.24` or `go 1.25` `[unverified]`. praxis is on
  `go 1.26.0` so this is a non-issue.
- **MCP spec version targeted:** v1.0.0+ targets the **2025-11-25**
  spec; v1.4.0+ adds experimental client-side OAuth (Step-Up
  Authorization extension).
- **Transports supported:** `StdioTransport`, `CommandTransport`
  (subprocess), `SSEClientTransport`, `SSEServerTransport`,
  `StreamableClientTransport`, `StreamableHTTPHandler`,
  **`InMemoryTransport`** (testing). Full transport matrix.
- **Direction:** full bidirectional — both MCP client and MCP server
  implementations.
- **Transitive dependency count:** 9 direct/indirect in go.mod:
  - Direct: `github.com/golang-jwt/jwt/v5`, `github.com/google/go-cmp`,
    `github.com/google/jsonschema-go`, `github.com/segmentio/encoding`,
    `github.com/yosida95/uritemplate/v3`, `golang.org/x/oauth2`,
    `golang.org/x/tools`.
  - Indirect: `github.com/segmentio/asm`, `golang.org/x/sys`.
  - **Total: 9 unique entries.** praxis currently has 11 direct/
    indirect entries.
  - **Concern:** `golang.org/x/oauth2` may pull in `golang.org/x/net`
    and `google.golang.org/grpc` transitively. This requires a
    `go mod graph` check before any commit. `golang.org/x/tools` is
    similarly heavy and may be a test-only or production dependency
    (to be verified).
- **API stability:** v1.x frozen per maintainer commitment.
- **Strengths:** official joint Anthropic + Go team stewardship;
  idiomatic use of Go generics (`AddTool[In, Out any]`, `iter.Seq`
  iterators); all MCP transports in one import; clean layered
  architecture (Transport → Connection → Session → Application);
  native `log/slog` use (aligns with praxis); JSON schema inference
  from Go types via `google/jsonschema-go`; used in production at
  Google.
- **Weaknesses:** `golang.org/x/oauth2` + `golang.org/x/tools` may
  stress praxis's dep budget — transitive check required; the SDK
  covers both client and server (praxis only needs client; Go has no
  tree-shaking); not yet widely battle-tested as a library
  dependency (most users build MCP servers, not bridges).

### 2.2 `github.com/mark3labs/mcp-go`

- **What it is:** community-authored Go MCP implementation, the most
  widely-adopted pre-official SDK, used by Eino (ByteDance).
- **URL:** <https://github.com/mark3labs/mcp-go>
- **License:** MIT — compatible with Apache-2.0.
- **Latest version:** v0.47.1+ (active as of research date); pre-v1.0.
- **Release cadence:** very high — multiple releases per month through
  2025–2026.
- **Maturity:** beta/production-quality core, pre-v1.0, no API
  stability commitment.
- **Go version requirement:** `go 1.23.0` per go.mod (confirmed).
- **MCP spec version targeted:** 2025-11-25 with backward compatibility
  to 2025-06-18, 2025-03-26, and 2024-11-05.
- **Transports:** stdio, HTTP/SSE, Streamable HTTP with spec-compliant
  SSE fallback on 4xx initialize.
- **Direction:** server-focused historically, includes client package
  (`mcp-go/client`).
- **Transitive dependency count:** ~7 total (3 direct + 4 indirect,
  some test-only). Lightest of the three candidates.
- **API stability:** pre-v1.0 — no stability guarantee.
- **Strengths:** lightest dependency footprint; Go 1.23 compatible
  (matches praxis minimum); 8,600+ stars (most community-adopted Go
  MCP library); active release cadence tracks spec evolution quickly;
  used by Eino in production.
- **Weaknesses:** pre-v1.0 with no API stability commitment — praxis
  v1.0 depending on a pre-v1.0 library is a governance risk;
  community-maintained (individual + Mark III Labs), higher bus-factor
  risk; server-side origin, client ergonomics less polished than the
  official SDK.

### 2.3 `github.com/metoro-io/mcp-golang`

- **What it is:** early community Go MCP SDK by the Metoro
  observability company.
- **URL:** <https://github.com/metoro-io/mcp-golang>
- **License:** `[unverified]` — the official go-sdk README
  acknowledges it as inspiration but the research pass could not
  confirm the exact license.
- **Latest version:** v0.8.0 (January 22, 2025).
- **Release cadence:** stalled — no releases after January 2025
  (15+ months of inactivity as of 2026-04-10).
- **Maturity:** experimental / effectively abandoned.
- **Go version requirement:** `go 1.21`.
- **MCP spec version targeted:** 2024-11-05 only; has not tracked
  subsequent spec evolution.
- **Transports:** stdio only `[unverified]`; no evidence of HTTP/SSE
  support in recent releases.
- **Transitive dependency count:** 4 direct + 24 indirect — heaviest of
  the three (`pkg/errors`, `tidwall/sjson`, `invopop/jsonschema`).
- **Verdict:** not suitable as a dependency; effectively superseded by
  the official SDK.

### 2.4 `golang.org/x/tools/internal/mcp` (noted for completeness)

This package exists but lives under `internal/`, so it is not
importable by external packages. It represents Go tooling's internal
MCP usage (e.g., for gopls). Not a candidate.

### 2.5 SDK comparison matrix

| Criterion | `modelcontextprotocol/go-sdk` | `mark3labs/mcp-go` | `metoro-io/mcp-golang` |
|---|---|---|---|
| Fit (as bridge client library) | High — full client, idiomatic Go | Medium-High — works but server-first | Low — stalled, missing transports |
| License | Apache-2.0 + MIT (compatible) | MIT (compatible) | `[unverified]` |
| Maturity | Production (v1.0.0 Sep 2025, v1.5.0 Apr 2026) | Beta/production-quality, pre-v1.0 | Abandoned (no 2025 releases) |
| Dep footprint | ~9 entries; oauth2+tools are heavy | ~7 entries; very light | ~28 entries; heaviest |
| API stability | v1.x frozen | Pre-v1.0, no stability commitment | N/A (abandoned) |
| Go version | 1.24+ likely `[unverified]` | 1.23 (confirmed) | 1.21 |
| Spec version | 2025-11-25 (current) | 2025-11-25 (current) | 2024-11-05 only |
| Transports | All (stdio, SSE, Streamable HTTP, InMemory) | All (stdio, SSE, Streamable HTTP) | Stdio only |
| Maintainer | Official (Anthropic + Google) | Community (individual) | Community (Metoro) |
| Ergonomics vs praxis style | High — generics, slog, clean layering | Medium — functional, less idiomatic | Low |

---

## 3. Prior art: MCP-bridge patterns in agent orchestrators

### 3.1 Eino (ByteDance / CloudWeGo)

Eino uses `mark3labs/mcp-go` directly as its MCP client library. The
integration creates `MCPTool` wrappers that implement Eino's generic
`Tool` interface, backed by an MCP client session. Tool discovery
happens at bridge construction time via `tools/list`, and each
discovered MCP tool becomes a separate `Tool` implementation. The
server+tool name mapping is consumer-managed (not standardised by
Eino). Credential passing is not abstracted — callers configure the
MCP client transport directly before passing it to the bridge
constructor. This is essentially the pattern praxis would mirror: a
thin struct that implements `tools.Invoker` by translating `ToolCall`
into an MCP `tools/call` RPC.

Source: <https://www.cloudwego.io/docs/eino/ecosystem_integration/tool/tool_mcp/>

### 3.2 LangChainGo

No native MCP support in core `tmc/langchaingo`. A third-party
adapter (`github.com/i2y/langchaingo-mcp-adapter`) wraps an MCP
client and exposes discovered tools as LangChainGo `tools.Tool`
implementations. Same pattern as Eino: connect → `tools/list` → one
`Tool` per MCP tool → each `Tool.Call()` delegates to `tools/call`.
Credential passing and name scoping are adapter-consumer concerns.
Not in-tree, not official — exactly the ecosystem-fragmentation
failure mode Phase 7 wants to avoid.

Source: <https://github.com/i2y/langchaingo-mcp-adapter>

### 3.3 Google ADK for Go

ADK for Go (released November 2025) has first-class MCP support in
its documentation. ADK helps you "both use and consume MCP tools in
your agents" and includes out-of-the-box support via MCP Toolbox. The
underlying SDK is not confirmed in research (ADK is maintained by
Google, which also co-maintains the official go-sdk; the official
SDK is the likely choice `[unverified]`). The pattern is the same:
MCP tools surfaced as ADK's generic `Tool` type.

Source: <https://google.github.io/adk-docs/mcp/>

### 3.4 OpenAI Agents SDK (Python — prior art only)

The Python SDK treats MCP servers as first-class tool sources. Each
MCP server connection is an `MCPServer` object; when an agent runs,
the SDK calls `tools/list` on each server and adds results to the
agent's tool set. Tool names stay flat (no cross-server namespacing)
— collision resolution is the consumer's problem. Credentials are
passed at `MCPServer` construction time, never through the tool
invocation path. Error translation: `isError: true` results are
surfaced to the LLM as tool errors (not raised as Python exceptions),
mirroring the MCP spec's intent. This is the most well-documented
pattern and the most relevant prior art for praxis's design.

Source: <https://openai.github.io/openai-agents-python/mcp/>

### 3.5 Pattern summary

Every framework reviewed follows the same structural pattern:

1. A bridge struct holds an MCP client session (long-lived connection
   to one server).
2. At bridge construction, call `tools/list` to discover tools.
3. Each discovered tool becomes a callable behind the framework's
   generic tool interface.
4. On invocation, translate the generic `ToolCall` into `tools/call`
   RPC; translate the response back.
5. Credentials are injected at transport construction time, before
   the bridge is created.
6. Tool naming: flat by default (MCP tool name as-is), with no
   standard cross-server namespacing convention.

**None of the surveyed frameworks have invented a different pattern.**
The seam is always the generic "tool" interface. This strongly
validates praxis's choice to wrap the MCP adapter behind
`tools.Invoker` — and also means Phase 7's explicit namespacing
convention (D111) is a genuine advance over the state of the art, not
just a local choice.

---

## 4. MCP protocol facts relevant to integration design

### 4.1 Spec versions and stability

Current stable MCP spec: **2025-11-25**. Prior versions (2024-11-05,
2025-03-26, 2025-06-18) remain specified for backward compatibility.
The spec is governed by Anthropic and openly iterated; it is not an
ISO or IETF standard but has wide industry adoption (Anthropic,
Google, OpenAI, Microsoft).

Source: <https://modelcontextprotocol.io/specification/2025-11-25>

### 4.2 Transport model

| Transport | Status | Auth posture |
|---|---|---|
| **stdio** | Universal, original, recommended for local servers | No transport auth — credentials from env per spec |
| **HTTP/SSE** | Deprecated in 2025-06-18 spec; still widely deployed | HTTP bearer / OAuth 2.1 |
| **Streamable HTTP** | Current preferred HTTP transport (2025-06-18+) | HTTP bearer / OAuth 2.1, including Step-Up Authorization (2025-11-25) |

### 4.3 Authentication model

- **stdio:** "implementations SHOULD NOT follow the HTTP auth spec
  and instead retrieve credentials from the environment" — env vars,
  process inheritance.
- **HTTP transports:** OAuth 2.1. Protected MCP servers act as OAuth
  2.1 resource servers; clients present
  `Authorization: Bearer <token>`. Step-up authorization (403 + new
  scope list) added in 2025-11-25. Dynamic Client Registration
  supported. Custom auth strategies are permitted at SHOULD level.
- The spec does **not** mandate how credentials are provisioned —
  only how they are presented to the transport.

Source: <https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization>

### 4.4 Error object shapes

**JSON-RPC layer:**

```json
{ "jsonrpc": "2.0", "id": 3, "error": { "code": -32602, "message": "..." } }
```

Standard JSON-RPC codes apply (`-32700` parse error, `-32600` invalid
request, `-32601` method not found, `-32602` invalid params, `-32603`
internal error).

**Tool-level layer:** tool execution errors SHOULD be returned as
`isError: true` in the result, **not** as JSON-RPC error responses.
This is a distinct error channel from the JSON-RPC layer and informs
Phase 7's D113 mapping.

### 4.5 Tool call response shape

```json
{ "content": [{"type": "text", "text": "..."}], "isError": false }
```

- `content` is an array of typed blocks: `text`, `image`, `audio`,
  `resource_link`, `resource` (embedded).
- `isError: true` signals a tool execution error (the LLM should see
  this and may self-correct); distinct from JSON-RPC errors.
- `structuredContent` (optional, 2025-11-25+): JSON object for typed
  structured results.

Source: <https://modelcontextprotocol.io/specification/2025-11-25/server/tools>

### 4.6 Session and connection lifecycle

MCP uses stateful, long-lived connections. A connection involves an
initialization handshake (`initialize` request → `initialized`
notification) that negotiates capabilities. Once initialised, the
session remains open until either side closes it. This is not a
per-call protocol — connection setup cost is paid once.

### 4.7 Capability negotiation

On `initialize`, the client sends `ClientCapabilities` and receives
`ServerCapabilities`. The client learns which features the server
supports (tools, resources, prompts, sampling, logging, etc.) before
making feature-specific calls. A client MUST NOT call `tools/list`
if the server did not advertise the `tools` capability.

### 4.8 Dynamic tool list mutation (`tools/list_changed`)

`tools/list_changed` notifications mean tool definitions can change
after initial discovery. This is a spec-level "rug pull" vector and a
real design concern for any bridge that caches tools at session open.
None of the surveyed bridge implementations handle this at all —
they capture the tool list once and ignore subsequent notifications.
Phase 7 must choose a posture explicitly (see Open Question 6).

### 4.9 Security considerations called out by the spec

1. Tool annotations (descriptions, metadata) are untrusted unless
   from a trusted server — malicious servers can poison descriptions.
2. Tool result content is untrusted — servers can embed prompt
   injection payloads.
3. Clients SHOULD show tool inputs to users before calling the server
   (data exfiltration risk).
4. Clients MUST validate tool results before passing them to the LLM.
5. `tools/list_changed` notifications enable mid-session tool-set
   mutation — a "rug pull" attack vector.

These align directly with praxis's existing Phase 5 model:
`ToolResult.Content` is already untrusted by contract, and
`PostToolFilter` runs before content reaches the LLM.

Source: <https://modelcontextprotocol.io/specification/2025-11-25/server/tools>,
<https://simonwillison.net/2025/Apr/9/mcp-prompt-injection/>

---

## 5. Recommendation

### 5.1 `metoro-io/mcp-golang` — **Reject**

Abandoned, missing transports, heaviest dep footprint, unverified
license. No further evaluation.

### 5.2 `mark3labs/mcp-go` — **Inspire (do not depend)**

Pre-v1.0 with no API stability guarantee creates a governance problem:
praxis v1.0 would carry a hard dependency on a library with no
stability commitment. The high release cadence that makes it
attractive is also what makes it risky as a dependency. The pattern
it establishes (thin bridge struct, per-server client session, one
`Invoker` per server) is the right model to mirror. Eino's use of it
confirms the viability, but Eino does not share praxis's stability
requirements.

### 5.3 `modelcontextprotocol/go-sdk` — **Reuse (conditional)**

**Recommended approach for the `praxis/mcp` sub-module.** Dependency
footprint is manageable on paper: the SDK brings 9 entries against
praxis's current 11. The critical unknown is whether
`golang.org/x/oauth2` pulls in `google.golang.org/grpc` transitively
(which would be unacceptable). This must be verified with
`go mod graph` before committing. If the transitive graph stays
clean, the official SDK is the correct choice:

- v1.x API freeze means the praxis adapter will not break on SDK
  minor releases.
- Joint Anthropic + Google maintenance provides long-term assurance.
- All transports (including `InMemoryTransport` for testing) are
  covered in one import.
- Idiomatic use of Go generics and `log/slog` aligns with praxis
  design principles.
- Apache-2.0 new-contribution license is clean for praxis's own
  Apache-2.0 license.

If the transitive graph is unacceptable, the fallback is
**pattern-only**: no in-tree dependency, a documented reference
implementation showing how to wire an MCP client behind
`tools.Invoker`. The reference implementation lives in `examples/`
or docs, not in an importable module. This fallback is the re-open
path for D106/D107 if the dep audit fails.

### 5.4 `InMemoryTransport` eliminates the need for a praxis-side fake

The official SDK already provides `InMemoryTransport`, a built-in
test double for MCP sessions. This substantially simplifies the
Phase 7 testability story: the `internal/transport/fake/`
sub-package proposed in earlier phase drafts can be replaced by
direct use of the SDK's `InMemoryTransport` in test code. See
03-integration-model.md §8a Testability (testability note amendment
2026-04-10).

### 5.5 Minimum transport set

stdio is table-stakes (all local MCP servers, lowest attack surface).
Streamable HTTP is the current spec-preferred remote transport.
HTTP/SSE is deprecated but still widely deployed; the SDK abstracts
it behind the same `StreamableClientTransport` handshake fallback
used by the ecosystem. Phase 7 D108 ships stdio + Streamable HTTP
(with SSE handled transparently by the SDK layer) and does not
expose SSE as a distinct caller-visible transport.

---

## 6. Open questions for Phase 7

These are the items that could not be resolved without deeper source
verification and must be answered during phase execution before any
code ships.

1. **Does `golang.org/x/oauth2` pull in `google.golang.org/grpc`
   (or other heavy transitive deps) in `modelcontextprotocol/go-sdk`
   v1.5.0?** Must be verified with `go mod graph` before the first
   `praxis/mcp` `go.mod` commit. This is the single biggest unknown
   for the "reuse" decision and a formal D107 precondition.
2. **Is `golang.org/x/tools` in the SDK's go.mod a production
   dependency or test-only?** If test-only, the impact on praxis
   consumers is zero.
3. **Exact `go` directive in the SDK's go.mod** (Go 1.24? 1.25?).
   praxis at 1.26 is fine either way, but this should be confirmed.
4. **Written stability statement on the SDK's v1 line** beyond the
   v1.0.0 release notes' "no breaking API changes" commitment — is
   there a STABILITY.md or equivalent?
5. **MCP server tool-set mutability handling (`tools/list_changed`).**
   Phase 7 must decide whether the adapter ignores the notification
   (simple; tool list is captured at session open only), reacts by
   closing and re-opening the session (safer but disruptive), or
   exposes a notification channel to callers (new API surface, not
   covered by D110). None of the surveyed bridges handle this.
   Recommended default: ignore + document; revisit post-v1.0 if
   consumer demand emerges.
6. **SDK concurrency guarantee.** Is a single MCP client session
   safe for concurrent `tools/call` requests under praxis's parallel
   tool dispatch (Phase 2 D24)? If not, the adapter must serialise
   or pool. The per-server mutex fallback in 03-integration-model.md
   §8 is correct but imposes a real cost on same-server parallel
   dispatch.
7. **`SignedIdentity` propagation to MCP servers.** Decided as "not
   forwarded" in D118 on trust-boundary grounds; this research
   pass confirms no MCP spec field would naturally carry it. D118
   stands.
8. **Soft-cancel mid-call session survival.** If praxis's 500 ms
   soft-cancel window (Phase 2 D21) closes an in-flight MCP request,
   is the session recoverable or must it be torn down? Addressed
   partially in 04-security-and-credentials.md §2.3 (first-call case)
   but not for subsequent calls on an already-open session.

---

## 7. Sources consulted

All citations are public URLs.

- MCP specification — <https://modelcontextprotocol.io/>
- MCP specification 2025-11-25 — <https://modelcontextprotocol.io/specification/2025-11-25>
- MCP authorization — <https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization>
- MCP tools — <https://modelcontextprotocol.io/specification/2025-11-25/server/tools>
- `modelcontextprotocol/go-sdk` — <https://github.com/modelcontextprotocol/go-sdk>
- `modelcontextprotocol/go-sdk` on pkg.go.dev — <https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk>
- `modelcontextprotocol/go-sdk` mcp package — <https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp>
- `modelcontextprotocol/go-sdk` v1.0.0 release — <https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.0.0>
- `mark3labs/mcp-go` — <https://github.com/mark3labs/mcp-go>
- `mark3labs/mcp-go` on pkg.go.dev — <https://pkg.go.dev/github.com/mark3labs/mcp-go>
- `metoro-io/mcp-golang` — <https://github.com/metoro-io/mcp-golang>
- Eino MCP integration — <https://www.cloudwego.io/docs/eino/ecosystem_integration/tool/tool_mcp/>
- LangChainGo MCP adapter — <https://github.com/i2y/langchaingo-mcp-adapter>
- Google ADK MCP — <https://google.github.io/adk-docs/mcp/>
- Announcing ADK for Go — <https://developers.googleblog.com/announcing-the-agent-development-kit-for-go-build-powerful-ai-agents-with-your-favorite-languages/>
- OpenAI Agents SDK MCP — <https://openai.github.io/openai-agents-python/mcp/>
- Simon Willison: MCP prompt injection — <https://simonwillison.net/2025/Apr/9/mcp-prompt-injection/>
- MCP security vulnerabilities — <https://www.practical-devsecops.com/mcp-security-vulnerabilities/>
- MCP authentication — <https://stackoverflow.blog/2026/01/21/is-that-allowed-authentication-and-authorization-in-model-context-protocol/>
- MCP Go SDK design discussion — <https://github.com/orgs/modelcontextprotocol/discussions/364>

---

**Document status.** Consolidated from the solution-researcher
agent's verified output plus Phase 7 planning notes, 2026-04-10.
The initial draft (written from prior knowledge when the first
researcher invocation stalled) has been replaced by this verified
version. Every `[unverified]` marker is a deferred verification
obligation that must be resolved before the `praxis/mcp` `go.mod`
is committed.
