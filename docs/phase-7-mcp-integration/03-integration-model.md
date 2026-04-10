# Phase 7 — Integration Model

**Decisions:** D110, D111, D112, D113, D114
**Cross-references:** 02-scope-and-positioning.md (module layout),
Phase 3 `06-tools-and-invocation-context.md` (frozen `tools.Invoker`),
Phase 3 `07-errors-and-classifier.md` (error taxonomy),
Phase 4 `02-span-tree.md`, Phase 4 `03-metrics.md` (cardinality),
Phase 5 `04-trust-boundaries.md`.

---

## 1. Module layout

```
praxis/                            # core module (github.com/praxis-go/praxis)
│   go.mod                         # unchanged by Phase 7
│   ...
└── mcp/                           # Phase 7 sub-module
    │   go.mod                     # module github.com/praxis-go/praxis/mcp
    │   go.sum
    │   doc.go                     # package-level documentation
    │   invoker.go                 # Invoker implementation
    │   options.go                 # constructor options
    │   session.go                 # internal session management
    │   namespace.go               # tool-name namespacing
    │   errors.go                  # MCP error → TypedError mapping
    │   observability.go           # span + metric helpers
    │   credentials.go             # credentials.Resolver integration
    │   examples_test.go           # godoc examples
    │   internal/
    │       client/                # thin wrapper over the official MCP SDK
    │       transport/             # stdio + HTTP transport wiring
```

The `mcp/` directory has its own `go.mod` declaring
`module github.com/praxis-go/praxis/mcp`. It requires the parent
module for the frozen interfaces:

```go
require (
    github.com/praxis-go/praxis v0.5.x  // or v1.0.0 once released
    github.com/modelcontextprotocol/go-sdk v1.x
)
```

The core module never requires `praxis/mcp`. The direction of the
dependency is strictly one-way: `mcp → core`, never `core → mcp`.

## 2. Public API shape (D110)

The exported surface is intentionally small: a constructor that
returns a `tools.Invoker`, plus options. There is no new "MCP tool"
type visible to the orchestrator.

```go
// Package mcp implements praxis/tools.Invoker over one or more
// Model Context Protocol servers.
//
// An Invoker constructed via New fronts a fixed set of MCP servers
// supplied at construction time. Each server is addressed by a
// caller-chosen logical name; tool calls are routed to the correct
// server via a namespacing convention (§3).
//
// The adapter does not perform runtime MCP server discovery. The
// server set is fixed at construction and is not mutable without
// re-constructing the Invoker. This is a deliberate consequence of
// the core library's "no plugins in v1" commitment (core D09).
//
// Package: github.com/praxis-go/praxis/mcp
package mcp

// Server describes a single MCP server to connect to.
//
// Server values are consumed by New and retained internally for the
// lifetime of the returned Invoker. Each Server's transport opens at
// most one MCP session; parallel tool dispatch to the same server is
// serialised inside the adapter if the underlying MCP client is not
// safe for concurrent use.
type Server struct {
    // LogicalName is the caller-chosen identifier for this server.
    // Used as the left half of the namespaced tool name (§3).
    // Must be non-empty and must match [a-zA-Z0-9_-]{1,64}.
    // Duplicate LogicalNames within a single New call are a
    // construction error.
    LogicalName string

    // Transport selects the MCP transport binding.
    Transport Transport

    // CredentialRef, if non-empty, names a credential that will be
    // resolved via credentials.Resolver and used to authenticate the
    // MCP session (§4 and 04-security-and-credentials.md).
    //
    // Interpretation: stdio transport → injected as an environment
    // variable whose name is given by TransportStdio.CredentialEnv;
    // HTTP transport → injected as a bearer token.
    //
    // An empty CredentialRef means the session opens unauthenticated.
    CredentialRef credentials.CredentialRef
}

// Transport is the sealed interface describing an MCP transport.
// Only the transport types declared in this package are valid.
//
// Sealing is enforced by an unexported sentinel method. Callers
// cannot implement Transport themselves.
type Transport interface {
    mcpTransport() // unexported sentinel — sealed interface
}

// TransportStdio launches the MCP server as a child process and
// communicates over stdio.
type TransportStdio struct {
    // Command is the executable to launch. Must be an absolute path
    // or a name resolvable via exec.LookPath at New time.
    Command string

    // Args are the command-line arguments passed to the child process.
    Args []string

    // Env is a fixed environment variable map. Framework-injected
    // credential values are merged on top; a credential's env var
    // overwrites any key in Env with the same name.
    Env map[string]string

    // CredentialEnv is the environment variable name under which the
    // resolved credential's Value() bytes are placed before launch.
    // Ignored if the server's CredentialRef is empty.
    CredentialEnv string
}

// TransportHTTP connects to an MCP server over Streamable HTTP (which
// the official SDK abstracts over SSE as well).
type TransportHTTP struct {
    // URL is the fully-qualified MCP endpoint URL.
    URL string

    // Header is a map of fixed HTTP headers set on every request.
    // The adapter adds an Authorization header derived from the
    // resolved credential, if any; callers must not pre-set
    // Authorization in this map.
    Header map[string]string
}

// Option configures the Invoker returned by New.
type Option func(*config)

// WithResolver injects the credentials.Resolver used by the adapter
// to fetch each Server's credential. Defaults to the NullResolver,
// which errors on any Fetch call — acceptable only for servers with
// empty CredentialRefs.
func WithResolver(r credentials.Resolver) Option

// WithMetricsRecorder injects the telemetry.MetricsRecorder used by
// the adapter to record MCP-specific metrics. Defaults to the
// NullMetricsRecorder. Callers typically pass the same recorder they
// pass to AgentOrchestrator construction.
func WithMetricsRecorder(r telemetry.MetricsRecorder) Option

// WithTracerProvider injects the OTel TracerProvider used for
// MCP-transport child spans. Defaults to the no-op tracer provider.
func WithTracerProvider(tp trace.TracerProvider) Option

// WithMaxResponseBytes caps the maximum size of an MCP tool
// response that the adapter will buffer before returning. A
// response that exceeds the cap is rejected with
// ErrorKindTool/ToolSubKindServerError. Default: 16 MiB.
// See D112 amendment for rationale.
func WithMaxResponseBytes(n int64) Option

// New constructs an Invoker that fronts the given MCP servers.
//
// New opens each server's transport eagerly (stdio: spawns the child
// process; HTTP: negotiates capabilities via handshake) and returns
// an error if any server fails to open. Partial openings are cleaned
// up before returning.
//
// The returned Invoker implements praxis/tools.Invoker and is safe
// for concurrent use. The caller is responsible for calling Close on
// it when done; Close tears down all MCP sessions and releases
// resources.
func New(ctx context.Context, servers []Server, opts ...Option) (Invoker, error)

// Invoker is the public handle returned by New. It implements
// praxis/tools.Invoker and adds a Close method for session teardown.
type Invoker interface {
    tools.Invoker
    io.Closer
}
```

Key points:

- **`Transport` is a sealed interface.** Adding a new transport type
  (e.g., WebSocket) is a source-level change in the `praxis/mcp`
  module. This is deliberate: it prevents callers from implementing
  their own transports, which would break the adapter's audit story.
- **`New` takes a slice of `Server` values**, not a varargs or a
  builder pattern. Reason: the server set is the whole configuration
  story; one slice is sufficient, and a varargs form would make
  iteration-based construction awkward.
- **`Option` is a functional option type** matching the rest of the
  praxis library's convention (Phase 3 D12). Callers who wire a
  full-featured deployment use `WithResolver`,
  `WithMetricsRecorder`, `WithTracerProvider`.
- **`Invoker` (mcp package) embeds `tools.Invoker`** and adds only
  `Close`. No new methods. This preserves the Phase 3 frozen
  `tools.Invoker` surface verbatim.

**D110** — The public `praxis/mcp` API surface consists of
`Server`, `Transport` (sealed), `TransportStdio`, `TransportHTTP`,
`Option`, `WithResolver`, `WithMetricsRecorder`,
`WithTracerProvider`, `WithMaxResponseBytes` (added via the D112
amendment), `New`, and `Invoker`. No other exported identifiers.
No amendment to core praxis packages.

## 3. Tool namespacing convention (D111)

When a single `Invoker` fronts multiple MCP servers, tool-name
collision is a real hazard (two servers both expose a tool called
`search`). The adapter imposes a namespacing convention at
session-open time:

**Rule.** The adapter exposes each MCP tool to the LLM using the name
`{LogicalName}__{mcpToolName}`. The delimiter is a **double
underscore** (`__`).

Example: a server with `LogicalName = "github"` exposing a tool named
`list_issues` surfaces to the LLM as `github__list_issues`.

### 3.1 Why double underscore

- **Unambiguous.** Most MCP tool names use single underscores; a
  double underscore is unlikely to appear in a server-defined tool
  name and is therefore robust as a delimiter.
- **LLM-safe.** The major LLM providers (Anthropic, OpenAI) accept
  tool names matching `^[a-zA-Z0-9_-]{1,64}$`. Double underscore is
  within this set.
- **Reversible.** The adapter can uniquely split a routed tool name
  back into `(logicalName, mcpToolName)` at dispatch time.
- **Familiar.** The `__` convention is used by langchain's Python
  `MultiServerMCPClient` and by several community MCP bridges. It is
  the closest thing to a de-facto standard in the ecosystem.

### 3.2 Collision handling

If two servers expose the same `mcpToolName`, the namespaced form
still differentiates them (`s1__foo` vs `s2__foo`). If the LLM is
given two servers with the same `LogicalName`, construction fails —
duplicate `LogicalName`s within a single `New` call are a validation
error.

If an MCP server exposes a tool whose raw name already contains `__`,
the adapter treats the full raw name opaquely and splits on the
**leftmost** `__` to recover `(logicalName, tail)` at dispatch; the
tail is passed to the MCP server verbatim. This preserves correctness
in the pathological case at the cost of a potentially awkward public
name.

### 3.3 Caller override

**Not supported in v1.0.0.** The convention is fixed. A caller who
wants a different convention builds its own adapter using this one as
a template. Reason: adapter-level customisation of tool names is a
surface-area expansion that does not carry its weight for v1.0.

**D111** — The tool-namespacing convention is
`{LogicalName}__{mcpToolName}`, fixed at the adapter level, not
caller-configurable in v1.0.0.

## 4. Budget participation (D112)

MCP-backed tool calls participate in `budget.Guard` identically to any
other `tools.Invoker` dispatch:

| Dimension | How MCP calls contribute |
|---|---|
| `wall_clock` | Measured as the elapsed time from `Invoker.Invoke` entry to return. MCP transport latency, network time, and server-side execution all count. |
| `tool_calls` | Incremented by 1 per `Invoker.Invoke` call, regardless of the MCP server or tool name. |
| `tokens` | **Not affected by MCP calls.** MCP tool calls do not consume LLM tokens directly. Any tokens generated by an LLM response that includes MCP-sourced content are counted at the `LLMCall` accounting site as normal. |
| `cost` | **Not affected by MCP calls directly.** MCP calls have no per-call micro-dollar cost in the `PriceProvider` model because praxis does not price tool calls. Consumers who want to price MCP usage implement their own accounting in the `LifecycleEventEmitter`. |

**No new budget dimension.** Phase 7 rejects adding a "transport
latency" or "bytes over wire" dimension because:

1. The core `budget.Guard` interface is frozen at v1.0 (Phase 3 D04).
   Adding a dimension is a breaking change.
2. The existing `wall_clock` and `tool_calls` dimensions already cap
   the two obvious MCP cost axes.
3. Adding a cost dimension for MCP would contradict D08's per-invocation
   snapshot pricing model — MCP pricing is nowhere defined by the
   protocol and would have to be caller-supplied, which is exactly
   what `StaticPriceProvider` already enables for LLM calls.

**D112** — MCP calls participate in `wall_clock` and `tool_calls`
budget dimensions via the existing `tools.Invoker` accounting path.
No new budget dimension is added for v1.0.0.

## 5. Error translation (D113)

The adapter maps MCP errors into the praxis error taxonomy at two
distinct layers.

### 5.1 Tool-level errors (`isError: true` in the result)

When an MCP `tools/call` response carries `isError: true`, the tool
ran and reported a semantic failure. This is **not** a transport
failure — the tool simply couldn't do what the LLM asked (e.g., "file
not found", "row rejected by schema").

**Mapping.** The adapter returns:

```go
tools.ToolResult{
    Status:  tools.ToolStatusError,
    Content: <flattened content text — §6>,
    Err:     &mcpToolError{
        code:     <server-reported code if any>,
        message:  <server-reported message>,
        toolName: call.Name,       // namespaced form
    },
    CallID:  call.CallID,
}
```

Where `mcpToolError` is a private concrete type in the `mcp` package
that implements `errors.TypedError` with `Kind() ==
errors.ErrorKindTool` and `SubKind() == errors.ToolSubKindServerError`.
The orchestrator's classifier (Phase 3 D44) sees the `ErrorKindTool`
classification via `errors.As` and treats the result as a non-retryable
tool failure that is injected back into the conversation.

### 5.2 Protocol / transport errors (JSON-RPC error object)

When the MCP transport or the MCP protocol itself errors (JSON-RPC
`-32000` range or a transport disconnect), the call did not produce a
meaningful `isError` result. The adapter maps these to the
`tools.ToolResult{Status: tools.ToolStatusError}` with an error that
has `ErrorKindTool` and the appropriate sub-kind:

| Condition | `ErrorKind` | `ToolSubKind` |
|---|---|---|
| Transport-level disconnect / I/O error | `ErrorKindTool` | `ToolSubKindNetwork` |
| JSON-RPC error `-32700` … `-32603` (protocol errors) | `ErrorKindTool` | `ToolSubKindServerError` |
| JSON-RPC error `-32000` … `-32099` (server-defined) | `ErrorKindTool` | `ToolSubKindServerError` |
| Schema validation failure of the response | `ErrorKindTool` | `ToolSubKindSchemaViolation` |
| Session-level circuit-broken (e.g., repeated handshake failures) | `ErrorKindTool` | `ToolSubKindCircuitOpen` |

The adapter does not synthesise `TransientLLMError` or
`PermanentLLMError`: MCP is not an LLM call. The adapter does not
synthesise `SystemError` either: MCP failures are tool failures by
classification.

### 5.3 Result status semantics

The adapter returns `ToolResult.Status` values consistent with the
Phase 3 `ToolStatus` enum:

| Outcome | `ToolResult.Status` |
|---|---|
| Tool ran and succeeded (`isError: false`) | `ToolStatusSuccess` |
| Tool ran and reported failure (`isError: true`) | `ToolStatusError` |
| Tool name not recognised by any configured server | `ToolStatusNotImplemented` |
| Transport or protocol failure | `ToolStatusError` |

The adapter does **not** return `ToolStatusDenied` in v1.0.0.
Adapter-local policy is a non-goal (see 05-non-goals.md §7.9), so
there is no denial path inside the MCP adapter. Callers who want
denial semantics for MCP calls implement a `PolicyHook` at the
orchestrator layer; the orchestrator maps policy denials through
its existing contract, which is unchanged by Phase 7.

**D113** — The adapter maps MCP tool-level errors to
`ToolStatusError` with `ErrorKindTool`/`ToolSubKindServerError`; maps
MCP transport-level errors to the `ToolSubKind` taxonomy above; and
never synthesises `SystemError` or LLM-class errors from MCP failures.

## 6. Content flattening (D114)

The MCP tool-call response carries a `content[]` array of blocks;
`tools.ToolResult.Content` is a single `string`. The adapter flattens
as follows:

1. Filter the content array to text blocks only.
2. Concatenate the text-block values with a single `\n\n` separator.
3. If the filtered array is empty (the response is purely image or
   resource-reference blocks), set `Content` to the empty string and
   set `Status` to `ToolStatusSuccess`. The `PostToolFilter` receives
   the empty content and can decide how to handle it.

Non-text blocks (image, audio, resource references) are **discarded**
at the adapter boundary for v1.0.0. This is a lossy projection — it
is the subject of a future amendment if consumers need multi-modal
tool output.

### 6.1 Why flattening and not JSON

An alternative is to JSON-encode the full MCP content array into
`ToolResult.Content`. Phase 7 rejects this because:

- The LLM receives `ToolResult.Content` back in the conversation
  turn. Raw JSON is a worse affordance than concatenated text for the
  LLM's comprehension.
- Consumers who need full-fidelity access to the MCP response
  construct their own `tools.Invoker` that wraps the adapter and
  handles the content array directly.
- The Phase 5 `PostToolFilter` contract (D77) assumes `Content` is
  treatable as free-form text for prompt-injection detection. A JSON
  encoding would change that assumption without giving filter
  implementors a clear new contract.

**D114** — MCP content arrays are flattened to text-only, newline-
separated content in `ToolResult.Content`. Non-text blocks are
discarded. This is a lossy projection accepted for v1.0.0.

## 7. Observability extensions

### 7.1 Spans

The adapter introduces **one new child span** per MCP tool call:

- **Name:** `praxis.mcp.toolcall`
- **Parent:** the current OTel span at `Invoker.Invoke` entry, which
  is the `praxis.toolcall` span created by the core orchestrator
  (Phase 4 §3.3). The MCP span is a child of the core tool-call
  span, giving the trace a 2-level-deep hierarchy rooted at the
  orchestrator.
- **Opened at:** `Invoker.Invoke` entry.
- **Closed at:** `Invoker.Invoke` exit.

**Attributes:**

| Attribute key | Type | Cardinality | Notes |
|---|---|---|---|
| `praxis.mcp.server` | string | Bounded by configured server count | The `LogicalName` from `Server.LogicalName`; stable per Invoker |
| `praxis.mcp.tool` | string | Bounded by union of per-server tool names | The raw MCP tool name (right half of the namespaced form) |
| `praxis.mcp.transport` | string | 2 | `"stdio"` or `"http"` |
| `praxis.mcp.jsonrpc_code` | int | Bounded by MCP error code set | Set only on protocol-level errors |
| `praxis.mcp.is_error` | bool | 2 | Mirrors the MCP response `isError` flag |

The adapter calls `span.RecordError` on the `praxis.mcp.toolcall`
span when the call fails at the transport level. Tool-level
`isError: true` results do **not** call `RecordError` — they are
deliberate semantic failures, not exceptions, and the core orchestrator's
span already carries the `tool_status=error` attribute.

The existing `praxis.toolcall` span (Phase 4 D53) is **not** modified
by Phase 7. It continues to carry `praxis.tool_name` as the
namespaced form (`github__list_issues`), which is what the LLM saw
and what the orchestrator routed on.

### 7.2 Metrics (D115, cardinality-bounded)

The adapter adds **two** new metrics, both consumed via a narrow
`MetricsRecorderV2`-style extension interface:

```go
// mcp.MetricsRecorder is a standalone optional interface the
// adapter uses to record MCP-specific metrics. It lives in the
// praxis/mcp module and does NOT embed telemetry.MetricsRecorder.
//
// Callers pass a telemetry.MetricsRecorder through the
// WithMetricsRecorder option on praxis/mcp.New. The adapter then
// type-asserts the passed recorder against mcp.MetricsRecorder.
// If the assertion succeeds, the adapter uses the extension
// methods to record MCP-specific metrics. If it fails, MCP
// metrics are dropped silently — the core metrics recorded by
// the orchestrator through the core telemetry.MetricsRecorder
// are unaffected.
//
// This is NOT the D100 MetricsRecorderV2 pattern, which uses
// embedding. This is a separate-interface + type-assertion
// pattern, chosen because the MCP metric methods live in a
// different package than the core interface and embedding
// across modules would require the core to know about the
// extension. See D115 for the full rationale.
//
// Package: github.com/praxis-go/praxis/mcp
type MetricsRecorder interface {
    RecordMCPCall(server, transport, status string, duration time.Duration)
    RecordMCPTransportError(server, transport, kind string)
}
```

| Metric | Type | Labels | Cardinality |
|---|---|---|---|
| `praxis_mcp_calls_total` | Counter | `server`, `transport`, `status` | ≈ (servers × 2 × 3) |
| `praxis_mcp_call_duration_seconds` | Histogram | `server`, `transport`, `status` | same |
| `praxis_mcp_transport_errors_total` | Counter | `server`, `transport`, `kind` | ≈ (servers × 2 × 5) |

Label domains:

- **`server`**: values are the `LogicalName`s configured at `New`.
  Bounded by the configured server count. Callers MUST NOT configure
  an unbounded number of servers; the adapter documents a soft cap of
  32 distinct `LogicalName`s in godoc. **Cardinality contract enforced
  at construction**: if more than 32 distinct `LogicalName`s are
  passed to `New`, construction fails with an error.
- **`transport`**: `"stdio"` or `"http"`. Fixed 2-value set.
- **`status`**: `"ok"`, `"error"`, `"denied"`. Mirrors the core
  `status` label for tool calls (Phase 4 §2.3).
- **`kind`** (transport errors only): `"network"`, `"protocol"`,
  `"schema"`, `"circuit_open"`, `"handshake"`. Fixed 5-value set.

**Worst-case total new time series:** 32 × 2 × 3 × 2 (calls counter
+ duration histogram buckets) + 32 × 2 × 5 = 384 + 320 = **~704**.
Well within the Phase 4 cardinality budget (Phase 4 §6 worst-case
was 1,032 series for the whole core).

**The MCP metrics do not appear on the core `tools.Invoker`-level
metric labels.** The core `praxis_tool_calls_total` metric sees the
namespaced form as its `tool_name` label; to avoid an N×M cardinality
explosion (core_tool_name × server), the adapter counts MCP calls on
its own metrics instead and relies on the core metric for
server-independent tool aggregation.

**D115** — The adapter ships three bounded-cardinality metrics
(`praxis_mcp_calls_total`, `praxis_mcp_call_duration_seconds`,
`praxis_mcp_transport_errors_total`) via an `mcp.MetricsRecorder`
optional interface. The core `telemetry.MetricsRecorder` is not
modified. Construction fails if more than 32 distinct server
`LogicalName`s are configured.

## 8. Concurrency and session pooling

MCP sessions are long-lived. Under parallel tool dispatch (Phase 2
D24), the core orchestrator may issue multiple concurrent
`Invoker.Invoke` calls for a single invocation. The adapter must
handle this correctly regardless of whether the underlying MCP client
is safe for concurrent use.

**Contract.** The adapter owns one session per `Server` at a time.
Calls to the same server are serialised through a per-server
mutex if the official SDK's client is not documented as
concurrency-safe at the method level. If the SDK is concurrency-safe,
the mutex is elided (config option `internal/client` module decides
at build time, not runtime).

Calls to **different** servers proceed concurrently. Parallel tool
dispatch across multiple servers is the common case and the one that
must be fast.

**Session health.** If an MCP session enters a broken state
(repeated handshake failures, transport disconnect), the adapter
marks the session as "circuit-broken" for a fixed cool-down window
(default: 30 seconds). Subsequent calls to that server fail-fast
with `ToolSubKindCircuitOpen`. After the cool-down, the next call
attempts a reconnect.

This mirrors the existing `ToolSubKind` taxonomy without introducing
any new error-classification concept.

## 8a. Testability

The sealed `Transport` interface (§2) prevents consumer-supplied
transports in production, but the adapter's own unit tests need a
way to exercise the credential lifecycle, error mapping, and
content flattening paths without spawning real child processes or
running live HTTP servers. The adapter ships an internal fake
transport to satisfy this requirement.

**SDK-provided `InMemoryTransport` (preferred).** The official
`modelcontextprotocol/go-sdk` ships an `InMemoryTransport` type
intended for testing — a full in-process MCP session double that
speaks the real protocol. The `praxis/mcp` module's unit tests use
`InMemoryTransport` directly as the test substrate; no praxis-owned
fake is required for the normal happy-path, error-mapping, and
content-flattening tests. (This simplification was surfaced by the
verified research pass dated 2026-04-10; earlier drafts specified a
praxis-owned `internal/transport/fake/` which is now redundant.)

**Supplemental praxis-owned fake (only if needed).** If
`InMemoryTransport` proves insufficient for specific failure-mode
tests — e.g., asserting that the credential byte buffer is zeroed
after `Cmd.Start` returns (a praxis-level invariant, not an SDK
concern) — the adapter MAY add a minimal `internal/transport/fake/`
sub-package that:

- Implements the internal transport contract consumed by
  `internal/client/`.
- Records the exact bytes written by the adapter (so
  credential-lifecycle tests can assert that the credential
  buffer is zeroed after `Cmd.Start` / session-open returns).
- Returns scripted `tools/call` responses, including
  `isError: true` branches, JSON-RPC error objects, TLS
  handshake failures, and `MaxResponseBytes` overruns.
- Is **not** exported from the public `praxis/mcp` API. The
  module's `internal/` prefix prevents external import, which
  satisfies the sealed-transport guarantee while giving the
  module's own CI a working unit-test target.

Consumer projects that need their own MCP test doubles build
them by wrapping the public `Invoker` type (which is easily
mocked as an interface) rather than the sealed `Transport`
interface. A worked example is shipped in the module's
`examples_test.go`.

**Coverage target.** The D86 85 % coverage gate applies to the
`praxis/mcp` module. The fake-transport pattern is load-bearing
for hitting the gate: without it, the credential lifecycle,
error mapping, and content-flattening tests would require either
a live MCP server in CI (too brittle) or substantial mocking of
the SDK (which fights the reuse posture of D107).

## 9. Decoupling contract compliance

The `praxis/mcp` module is subject to the same banned-identifier grep
as the core module. This document and the other Phase 7 artifacts
must not contain:

- Consumer product or organisation names.
- Hardcoded `org.id`, `agent.id`, `user.id`, `tenant.id` attribute
  keys.
- Governance-event vocabulary.
- Milestone or decision IDs from other repositories.

The MCP adapter's span and metric keys (`praxis.mcp.*`,
`praxis_mcp_*`) live under the framework's own namespace. They do
not leak any consumer-specific identifier.

The adapter reads `tools.InvocationContext.Metadata` (the
caller-controlled map) verbatim and may propagate specific keys to
MCP as transport headers only if the caller's configuration opts in
via a future `WithMetadataHeaders` option. v1.0.0 does not ship this
option — metadata does not cross the MCP transport edge in v1.0.0.

## 10. Decisions (summary)

| ID | Subject | Outcome |
|---|---|---|
| D110 | Public API surface | `Server`, sealed `Transport` + `TransportStdio` + `TransportHTTP`, `Option`, `WithResolver`/`WithMetricsRecorder`/`WithTracerProvider`/`WithMaxResponseBytes`, `New`, `Invoker io.Closer` |
| D111 | Tool namespacing | `{LogicalName}__{mcpToolName}` with double-underscore delimiter, fixed; `__` prohibited inside `LogicalName` |
| D112 | Budget participation | Existing `wall_clock` + `tool_calls` only; no new dimension; adapter enforces `MaxResponseBytes` (default 16 MiB) as a resource guard |
| D113 | Error translation | MCP → `ErrorKindTool` sub-kinds including OAuth 401/403, HTTP 429, handshake timeout, TLS failure; no new error kinds |
| D114 | Content flattening | Text blocks only, newline-joined; non-text discarded for v1.0.0; `Success + empty Content` is a valid new combination |
| D115 | MCP-specific metrics | Three metrics via standalone `mcp.MetricsRecorder` interface detected by type assertion (not D100 embedding); construction enforces 32-server cap |

Full decision text in `01-decisions-log.md`.

---

**Next:** `04-security-and-credentials.md` addresses credential flow,
trust boundary classification, `SignedIdentity` propagation policy,
and the Phase 5 contract extension for MCP-sourced content.
