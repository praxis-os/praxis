// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/mcp/internal/client"
	internaltransport "github.com/praxis-os/praxis/mcp/internal/transport"
	"github.com/praxis-os/praxis/tools"
)

// MaxServers is the upper bound on the number of MCP [Server]
// entries a single [Invoker] may front.
//
// The cap is enforced at construction time by [New]: a call with
// more than [MaxServers] entries fails fast with a typed system
// error. Callers whose deployment requires a larger fleet partition
// their servers across multiple Invokers.
//
// The cap is 32, matching the Phase 7 D115 cardinality contract for
// the `praxis.mcp.server` span attribute and the `server` metric
// label. Raising this constant without first widening the D115
// cardinality budget would silently break the Phase 4 D60 metric
// cardinality boundary.
const MaxServers = 32

// logicalNameRegexp matches permitted [Server.LogicalName] values:
// non-empty, bounded length, ASCII alphanumeric plus hyphen and
// underscore. The upper length bound is 64 characters, matching the
// conservative intersection of major LLM provider tool-name
// alphabets.
//
// The regex is compiled once at package init so validation is
// allocation-free on the hot path of [New].
var logicalNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// errInvokerClosed is the sentinel-prefix text used by [invoker.Invoke]
// when a dispatch arrives after [invoker.Close] has already run.
// Tests match on the stable prefix, so the value is a package-level
// constant rather than an inlined literal.
const errInvokerClosed = "mcp: Invoker is closed"

// errUnknownTool is the sentinel-prefix text returned by
// [invoker.Invoke] when the composed tool name is not present in the
// routing table. It is returned via ToolResult.Err as an
// ErrorKindTool/ToolSubKindServerError so the LLM can observe the
// classification and self-correct.
const errUnknownTool = "mcp: unknown tool"

// New constructs an [Invoker] that fronts the given MCP servers.
//
// New runs the construction pipeline in three stages:
//
//  1. **Validation.** Every Server in the input slice is checked
//     against the rules in [validateServers]. A failing rule returns
//     a typed [praxiserrors.SystemError] with
//     [praxiserrors.ErrorKindSystem] — construction errors are
//     framework/configuration failures, never tool errors.
//  2. **Option binding.** A fresh [config] is seeded with defaults
//     from [defaultConfig] and the supplied options are applied in
//     order. Nil options in the variadic list are silently ignored,
//     matching the default-safe posture documented on each `With*`
//     constructor.
//  3. **Eager session opening.** For each server, New resolves the
//     credential (if [Server.CredentialRef] is set), builds the
//     SDK-native [sdkmcp.Transport] via the internal transport
//     builders, constructs a fresh MCP client, and calls
//     [sdkmcp.Client.Connect]. The returned [*sdkmcp.ClientSession]
//     is stored inside the [Invoker]. If any server fails to open,
//     every previously-opened session is closed before New returns
//     the error — callers never receive a partially-constructed
//     Invoker. This matches the Phase 7
//     `03-integration-model.md §2` "partial openings are cleaned up"
//     contract.
//
// The caller's servers slice is copied internally before session
// opening. Mutations to the original slice after New returns do not
// affect the Invoker (D110 construction-time binding).
//
// # Credential lifecycle
//
// For every server with a non-empty [Server.CredentialRef], New:
//
//   - Calls the configured [credentials.Resolver.Fetch] with ctx.
//   - Copies the resolver-returned [credentials.Credential.Value]
//     into a fresh adapter-owned byte slice.
//   - Calls [credentials.Credential.Close] on the resolver-returned
//     handle, zeroing the resolver-owned buffer.
//   - Passes the adapter-owned copy to the internal transport
//     builder ([internaltransport.InjectEnvCredential] for stdio,
//     [internaltransport.BuildHTTPTransport] for HTTP), which zeros
//     the copy after materialising the Go string used by the SDK.
//
// See the "Known limitation OI-MCP-1" section of the package godoc
// for the residual-risk boundary that the Go language's immutable
// string semantics place on this flow.
//
// # Tool-call routing
//
// After the session-opening stage New walks every open session,
// enumerates its advertised tools via the SDK iterator
// `session.Tools`, and builds a routing table (composed name →
// (session, raw name)) plus a cached [llm.ToolDefinition] slice.
// The routing table is the single source of truth for
// [tools.Invoker.Invoke] dispatch; the cached definitions back the
// [Invoker.Definitions] accessor so callers can thread MCP tool
// schemas into `llm.Request.Tools` without re-enumerating anything
// at runtime. Composition follows the D111 rule
// `{LogicalName}__{mcpToolName}`; collisions across servers are a
// typed [praxiserrors.SystemError] (not a panic — see Phase 4 D43).
// A router-construction failure at this stage runs the same
// partial-openings rollback as an earlier connect failure: every
// opened session is closed before New returns the error.
//
// Stability: stable-v0.x-candidate. The New signature freezes at
// praxis/mcp v1.0.0.
func New(ctx context.Context, servers []Server, opts ...Option) (Invoker, error) {
	if err := validateServers(servers); err != nil {
		return nil, err
	}

	cfg := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	// Pin the server slice: copy so caller-side mutations after New
	// returns cannot affect the Invoker's configuration. D110 requires
	// construction-time binding; no runtime registration.
	pinned := make([]Server, len(servers))
	copy(pinned, servers)

	// Production uses [openSessions]; tests can override via the
	// unexported withSessionOpener option to inject an in-memory
	// substrate. Nil in every production code path.
	opener := cfg.opener
	if opener == nil {
		opener = openSessions
	}
	sessions, err := opener(ctx, cfg, pinned)
	if err != nil {
		return nil, err
	}

	rt, err := buildRouter(ctx, pinned, sessions)
	if err != nil {
		// A router build failure after sessions are up must run the
		// same LIFO teardown path as any other partial-openings
		// failure, otherwise New would leak the eager transports it
		// just spawned. The rollback result is folded into the
		// returned error via wrapOpenFailure so callers still see
		// the offending server index; we use -1 / "" sentinels here
		// because the router operates on the whole set, not a single
		// server.
		rollbackErr := closeSessions(sessions)
		return nil, wrapRouterFailure(err, rollbackErr)
	}

	return &invoker{
		cfg:      cfg,
		servers:  pinned,
		sessions: sessions,
		router:   rt,
	}, nil
}

// wrapRouterFailure wraps a buildRouter error with an optional
// rollback error from closeSessions, using the same
// joined-error-in-message convention as [wrapOpenFailure]. The
// buildRouter layer already produces typed SystemError messages
// that carry the offending server index, so this helper only has
// to thread rollback context through without duplicating the
// per-server annotation.
func wrapRouterFailure(buildErr, rollbackErr error) error {
	if rollbackErr == nil {
		return buildErr
	}
	return systemError(fmt.Sprintf(
		"%v; rollback encountered errors: %v", buildErr, rollbackErr,
	))
}

// openSessions is the eager session-opening loop called by [New].
// It is extracted from New so white-box tests can exercise the
// partial-failure rollback path without driving construction
// through the full validation and option-binding machinery.
//
// For each server, openSessions:
//
//  1. Fetches the credential if [Server.CredentialRef] is set,
//     producing an adapter-owned byte slice via a fresh copy. The
//     resolver-owned [credentials.Credential] is Closed before the
//     adapter-owned copy is passed to the transport builder.
//  2. Builds the SDK-native transport via [buildSDKTransport].
//  3. Constructs a fresh [*sdkmcp.Client] from the internal wrapper.
//  4. Calls [sdkmcp.Client.Connect] with the supplied ctx.
//
// On the first failure at any stage, openSessions closes every
// previously-opened session via their individual Close methods,
// collects any resulting errors into the returned error via
// [errors.Join], and returns. The returned error is always a typed
// [praxiserrors.SystemError] — connect failures are construction
// failures at this layer, not tool errors.
func openSessions(ctx context.Context, cfg config, servers []Server) ([]*sdkmcp.ClientSession, error) {
	sessions := make([]*sdkmcp.ClientSession, 0, len(servers))

	for i, s := range servers {
		sdkTransport, err := buildSDKTransport(ctx, cfg, s)
		if err != nil {
			rollbackErr := closeSessions(sessions)
			return nil, wrapOpenFailure(i, s.LogicalName, err, rollbackErr)
		}

		mcpClient := client.NewClient()
		session, err := mcpClient.Connect(ctx, sdkTransport, nil)
		if err != nil {
			rollbackErr := closeSessions(sessions)
			return nil, wrapOpenFailure(i, s.LogicalName, err, rollbackErr)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// buildSDKTransport dispatches on the concrete type of
// [Server.Transport] and returns an [sdkmcp.Transport] ready for
// [sdkmcp.Client.Connect]. It is the single production call site
// where the sealed [Transport] interface is decomposed — every
// other adapter file treats Transport as opaque.
//
// Credential handling is interleaved here because the stdio and
// HTTP paths differ in where the credential material flows:
//
//   - stdio: the fresh adapter-owned credential byte slice is
//     passed into [internaltransport.InjectEnvCredential], which
//     appends the env entry to the built *exec.Cmd.Env and zeros
//     the buffer.
//   - HTTP: the credential byte slice is passed into
//     [internaltransport.BuildHTTPTransport], which captures the
//     bearer string in its round-tripper and zeros the buffer.
//
// If [Server.CredentialRef] is empty, no credential is resolved
// and the builders run with a nil/empty buffer, producing an
// unauthenticated session.
func buildSDKTransport(ctx context.Context, cfg config, s Server) (sdkmcp.Transport, error) {
	// Resolve credential once, if configured. The adapter-owned
	// copy returned here is zeroed by the transport builder below
	// after it materialises the Go string it needs.
	var cred []byte
	if s.CredentialRef != "" {
		fetched, err := cfg.resolver.Fetch(ctx, s.CredentialRef)
		if err != nil {
			return nil, systemError(fmt.Sprintf(
				"resolver.Fetch(%q): %v", s.CredentialRef, err,
			))
		}
		// The resolver-owned buffer is Closed here so only our
		// adapter-owned copy survives into the builder. fetched.Close
		// is idempotent per credentials.Credential.Close godoc.
		cred = append([]byte(nil), fetched.Value...)
		fetched.Close()
		if len(cred) == 0 {
			return nil, systemError(fmt.Sprintf(
				"resolver.Fetch(%q) returned empty credential material",
				s.CredentialRef,
			))
		}
	}

	switch t := s.Transport.(type) {
	case TransportStdio:
		cmd, err := internaltransport.BuildCommand(t.Command, t.Args, t.Env)
		if err != nil {
			return nil, err
		}
		if cred != nil {
			if err := internaltransport.InjectEnvCredential(cmd, t.CredentialEnv, cred); err != nil {
				return nil, err
			}
		}
		return &sdkmcp.CommandTransport{Command: cmd}, nil

	case TransportHTTP:
		return internaltransport.BuildHTTPTransport(t.URL, t.Header, cred)

	default:
		// Unreachable: the sealed Transport interface admits only
		// the two concrete types above. If a future refactor adds
		// a third variant, this default arm points at the exact
		// place that needs updating.
		return nil, systemError(fmt.Sprintf(
			"buildSDKTransport: unknown Transport type %T (sealed interface violation?)", t,
		))
	}
}

// closeSessions closes every session in the slice in reverse
// order (LIFO), collecting non-nil errors into a single joined
// error via [errors.Join]. Reverse order matches the intuition
// that sessions opened later might depend on earlier ones (not
// actually true at this layer, but a harmless ordering choice
// that keeps teardown predictable in tests).
//
// Nil entries in the slice are tolerated and skipped without
// touching them — test hooks in `testing_internal_test.go` produce
// such slices to exercise the Close path without materialising
// real MCP sessions. Production [openSessions] never returns nil
// entries.
//
// A nil or empty slice returns nil.
func closeSessions(sessions []*sdkmcp.ClientSession) error {
	var errs []error
	for i := len(sessions) - 1; i >= 0; i-- {
		session := sessions[i]
		if session == nil {
			continue
		}
		if err := session.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// wrapOpenFailure wraps a connect-loop failure with the offending
// server index and logical name, and optionally reports a
// non-nil rollback error (from closeSessions) as a joined error.
// The returned value is always a typed
// [praxiserrors.SystemError] satisfying the praxis error taxonomy.
func wrapOpenFailure(index int, logicalName string, openErr, rollbackErr error) error {
	msg := fmt.Sprintf("servers[%d] (%q): open session: %v", index, logicalName, openErr)
	if rollbackErr != nil {
		msg = fmt.Sprintf("%s; rollback encountered errors: %v", msg, rollbackErr)
	}
	return systemError(msg)
}

// invoker is the unexported concrete implementation of [Invoker]
// returned by [New]. Its public contract is exposed exclusively
// through the [Invoker] interface.
//
// # Lifecycle
//
// Construction (New) populates `sessions` with one live
// [*sdkmcp.ClientSession] per Server in `servers`; the two slices
// are index-aligned. `router` is built from those sessions at the
// same construction step and never mutated afterwards. [Close]
// tears every session down in reverse order, aggregating errors
// via [errors.Join], and flips the `closed` flag so subsequent
// [Invoke] calls fail fast instead of racing with teardown.
//
// # Concurrency
//
// The `mu` mutex guards the `closed` flag and the read path into
// `sessions` / `router`. [Invoke] takes a read lock so concurrent
// dispatches proceed in parallel; [Close] takes a write lock and
// blocks any new Invoke until teardown is done. In-flight Invoke
// calls observed before Close acquired the write lock are allowed
// to complete — the SDK serialises or parallelises its own
// session access per its concurrency contract.
type invoker struct {
	cfg      config
	servers  []Server                // pinned at construction, never mutated after New
	sessions []*sdkmcp.ClientSession // index-aligned with servers, opened eagerly by New
	router   *router                 // built eagerly by New, read-only thereafter

	mu     sync.RWMutex
	closed bool
}

// Invoke dispatches a single tool call through one of the MCP
// sessions owned by this Invoker.
//
// The routing flow is:
//
//  1. Take a read lock on `mu` and check the `closed` flag — a
//     post-Close dispatch returns a framework error so broken
//     orchestrator wiring is surfaced immediately.
//  2. Look up `call.Name` in the router. A miss is reported
//     through `ToolResult.Err` as an
//     `ErrorKindTool/ToolSubKindServerError`, not as a framework
//     error: unknown-tool is an LLM/configuration concern, not a
//     broken-invoker signal. The LLM can observe the classification
//     and self-correct on the next turn.
//  3. Parse `call.ArgumentsJSON` into a generic `map[string]any`
//     and hand it to the SDK as `CallToolParams.Arguments`. Empty
//     or nil ArgumentsJSON becomes a nil `Arguments` value, which
//     the SDK marshals as omitted per JSON-RPC rules. A malformed
//     ArgumentsJSON yields `ToolSubKindSchemaViolation`.
//  4. Invoke [sdkmcp.ClientSession.CallTool]; an SDK error is
//     routed through [classifyCallToolError] (D113 translation
//     table) which maps it to one of
//     `Network` / `CircuitOpen` / `SchemaViolation` / `ServerError`
//     per the mid-session classification rules. The full branch
//     list lives on [classifyCallToolError] godoc; handshake
//     failures never reach Invoke because they surface at
//     construction time through [openSessions].
//  5. Flatten the `*sdkmcp.TextContent` blocks in the SDK result
//     with [flattenTextContent]: text blocks joined with `\n\n`
//     per D114, non-text blocks (image, audio, resource) silently
//     dropped. The empty-content / all-non-text case yields
//     `Content == ""`, which is still a valid `ToolStatusSuccess`
//     per the D114 amendment 2026-04-10 contract note for
//     PostToolFilter implementors.
//  6. If `result.IsError` is set by the server, the flattened
//     text is returned with `ToolStatusError` and an
//     `ErrorKindTool/ToolSubKindServerError` wrapper (server-side
//     tool error), matching the MCP spec's "errors go in the
//     Content field with IsError=true" contract and D113's
//     `isError: true` row.
//
// A nil framework error is returned in every tool-level case,
// matching the [tools.Invoker] godoc contract; a non-nil framework
// error is reserved for "this Invoker is broken and should not be
// called again" signalling (closed-post-Close above).
func (i *invoker) Invoke(ctx context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	i.mu.RLock()
	closed := i.closed
	rt := i.router
	sessions := i.sessions
	i.mu.RUnlock()

	if closed {
		return tools.ToolResult{
			Status: tools.ToolStatusError,
			CallID: call.CallID,
			Err:    praxiserrors.NewSystemError(errInvokerClosed, nil),
		}, praxiserrors.NewSystemError(errInvokerClosed, nil)
	}

	route, ok := rt.lookup(call.Name)
	if !ok {
		return tools.ToolResult{
			Status: tools.ToolStatusError,
			CallID: call.CallID,
			Err: praxiserrors.NewToolError(
				call.Name, call.CallID, praxiserrors.ToolSubKindServerError,
				fmt.Errorf("%s %q (leftmost %q-split not present in routing table; see D111)", errUnknownTool, call.Name, logicalNameSeparator),
			),
		}, nil
	}

	if route.sessionIdx < 0 || route.sessionIdx >= len(sessions) {
		// Defensive: the router built this route from the same
		// sessions slice, so an out-of-range index would be an
		// internal bug. Surface it as a framework error so tests
		// see it deterministically rather than crashing.
		return tools.ToolResult{
				Status: tools.ToolStatusError,
				CallID: call.CallID,
			}, praxiserrors.NewSystemError(fmt.Sprintf(
				"mcp: internal routing invariant violated: sessionIdx=%d out of range [0,%d)",
				route.sessionIdx, len(sessions),
			), nil)
	}

	if sessions[route.sessionIdx] == nil {
		// Production [openSessions] never returns nil entries; a
		// nil here means a test hook installed a placeholder slice
		// via [withSessionOpener] + [nullSessionOpener] and then
		// attempted real dispatch through it. Surface the misuse
		// as a framework error so the test sees it deterministically.
		return tools.ToolResult{
				Status: tools.ToolStatusError,
				CallID: call.CallID,
			}, praxiserrors.NewSystemError(fmt.Sprintf(
				"mcp: internal routing invariant violated: sessions[%d] is nil (test hook misuse?)",
				route.sessionIdx,
			), nil)
	}

	var args any
	if len(call.ArgumentsJSON) > 0 {
		var parsed any
		if err := json.Unmarshal(call.ArgumentsJSON, &parsed); err != nil {
			return tools.ToolResult{
				Status: tools.ToolStatusError,
				CallID: call.CallID,
				Err: praxiserrors.NewToolError(
					call.Name, call.CallID, praxiserrors.ToolSubKindSchemaViolation, err,
				),
			}, nil
		}
		args = parsed
	}

	session := sessions[route.sessionIdx]
	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      route.rawName,
		Arguments: args,
	})
	if err != nil {
		// Translate the SDK-returned error into the praxis error
		// taxonomy. classifyCallToolError implements the D113
		// mapping table: SDK transport sentinels become Network,
		// session-gone becomes CircuitOpen, HTTP 401/403 become
		// CircuitOpen, 429 becomes Network, and so on. Every
		// unclassified error collapses to ServerError per D113's
		// "JSON-RPC protocol / server-defined codes → ServerError"
		// uniform rule.
		subKind := classifyCallToolError(err)
		return tools.ToolResult{
			Status: tools.ToolStatusError,
			CallID: call.CallID,
			Err: praxiserrors.NewToolError(
				call.Name, call.CallID, subKind, err,
			),
		}, nil
	}

	// MaxResponseBytes guard (D112 amendment / T33.7). The
	// adapter-level cap is a resource-consumption guard against a
	// runaway MCP server that returns gigabytes of tool output.
	// The check runs before flattenTextContent so the rejection
	// does not have to allocate the joined string, and the
	// returned Content is empty — the caller has asked for a hard
	// cap, so handing back a truncated payload would be worse
	// than a clean rejection.
	if cap := i.cfg.maxResponseBytes; cap > 0 {
		if actual := estimateResponseBytes(result.Content); actual > cap {
			return tools.ToolResult{
				Status: tools.ToolStatusError,
				CallID: call.CallID,
				Err: praxiserrors.NewToolError(
					call.Name, call.CallID, praxiserrors.ToolSubKindServerError,
					fmt.Errorf("mcp: response exceeds MaxResponseBytes: actual=%d cap=%d (D112; see WithMaxResponseBytes)", actual, cap),
				),
			}, nil
		}
	}

	content := flattenTextContent(result.Content)

	if result.IsError {
		// Per D113, an MCP tool result with IsError=true is a
		// server-reported tool-level failure. Phase 7 maps it to
		// ErrorKindTool + ToolSubKindServerError and preserves
		// the flattened text content so PostToolFilter implementors
		// can still inspect the response payload (often the
		// server-side error message, per the MCP spec's "errors
		// go in the Content field with IsError=true" contract).
		return tools.ToolResult{
			Status:  tools.ToolStatusError,
			Content: content,
			CallID:  call.CallID,
			Err: praxiserrors.NewToolError(
				call.Name, call.CallID, praxiserrors.ToolSubKindServerError,
				fmt.Errorf("mcp server reported tool error; see ToolResult.Content"),
			),
		}, nil
	}

	return tools.ToolResult{
		Status:  tools.ToolStatusSuccess,
		Content: content,
		CallID:  call.CallID,
	}, nil
}

// flattenTextContent joins every [*sdkmcp.TextContent] block in
// the SDK result with a `\n\n` separator, preserving server-side
// order, and drops every other content variant (image, audio,
// resource, tool-use, tool-result). An empty or nil slice — or a
// slice that contains only non-text blocks — yields `""`.
//
// This is the adapter's implementation of D114. Design points:
//
//   - The separator is a **double newline** (`\n\n`), not a single
//     `\n`. Double newlines read as paragraph breaks in the LLM
//     prompt context, which preserves the server-side block
//     boundaries without forcing the LLM to infer them from a
//     wall of text. The jira task T33.1 description uses `\n`
//     colloquially; D114 is the authoritative source for the
//     exact separator.
//   - Non-text blocks are **silently dropped**. Phase 7 rejects
//     encoding images or audio as text at the adapter boundary
//     (D114 §rationale); callers who need access to non-text
//     blocks must implement their own `tools.Invoker` wrapper.
//   - A `Content` slice of zero text blocks yields the empty
//     string. Per D114 §amendment 2026-04-10, the combination
//     `Status == ToolStatusSuccess && Content == ""` is a
//     **valid** outcome that Phase 3 `PostToolFilter`
//     implementors must not treat as a denial or framework bug.
//     The adapter's invoker godoc and the `flattenTextContent`
//     call site both rely on this invariant.
//   - Server-side order is preserved. Two separate `TextContent`
//     blocks appear in the flattened output in the same order
//     they arrived on the wire, separated by the `\n\n` joiner.
func flattenTextContent(blocks []sdkmcp.Content) string {
	if len(blocks) == 0 {
		return ""
	}
	var parts []string
	for _, block := range blocks {
		if tc, ok := block.(*sdkmcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// estimateResponseBytes approximates the total byte footprint of
// the SDK-decoded content slice returned by a successful
// [sdkmcp.ClientSession.CallTool]. It is the measurement primitive
// used by the MaxResponseBytes guard (D112 amendment / T33.7).
//
// The estimate sums the "large-payload" fields of the three
// Content variants that can plausibly carry hundreds of KiB to
// MiB of data:
//
//   - [*sdkmcp.TextContent]: the length of the `Text` string in
//     bytes.
//   - [*sdkmcp.ImageContent]: the length of the decoded `Data`
//     byte slice. The SDK delivers the image payload already
//     base64-decoded into this field, so `len(Data)` is the
//     authoritative raw-bytes count.
//   - [*sdkmcp.AudioContent]: same treatment as ImageContent.
//
// Resource-link metadata (URIs, names, descriptions) and embedded
// resource metadata are deliberately NOT counted: they are
// structurally small (each field is bounded by MCP's own
// transport limits) and the point of the guard is catching
// runaway payloads, not micro-accounting every block kind.
//
// The estimate is a lower bound: it ignores protocol framing,
// JSON overhead, and the handful of small metadata fields on
// each block. The practical effect is that the cap rejects
// payloads whose "true" byte cost is somewhat higher than the
// configured cap — always on the safe side of the guard.
//
// The function is pure and cheap: it iterates the slice once
// and does a handful of integer additions. It does not allocate.
func estimateResponseBytes(blocks []sdkmcp.Content) int64 {
	var total int64
	for _, block := range blocks {
		switch v := block.(type) {
		case *sdkmcp.TextContent:
			total += int64(len(v.Text))
		case *sdkmcp.ImageContent:
			total += int64(len(v.Data))
		case *sdkmcp.AudioContent:
			total += int64(len(v.Data))
		}
	}
	return total
}

// Definitions returns the composed tool definitions produced at
// [New] time by the router. The slice is owned by the Invoker and
// must not be mutated by the caller; see the [Invoker.Definitions]
// godoc on the public interface for the stability contract.
func (i *invoker) Definitions() []llm.ToolDefinition {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.router == nil {
		return nil
	}
	return i.router.defs
}

// Close tears down every MCP session owned by this invoker.
//
// Sessions are closed in reverse order of opening (LIFO). Errors
// from individual session closes are collected and returned as a
// single joined error via [errors.Join]; a session that fails to
// close does not prevent sibling sessions from being closed. If
// every session closes cleanly, Close returns nil.
//
// Close is safe to call more than once: after the first call the
// `closed` flag is set and subsequent calls return nil without
// re-draining the session slice. The underlying
// [sdkmcp.ClientSession.Close] method, per the go-sdk v1.5.0
// source, is not documented as idempotent, so this invoker clears
// its own sessions slice after the first call to guarantee the
// documented [Invoker] idempotency contract.
//
// Close also flips the `closed` flag under the write lock so that
// any post-Close [Invoke] call sees the closed state and returns
// a framework error immediately rather than racing with teardown.
func (i *invoker) Close() error {
	i.mu.Lock()
	if i.closed {
		i.mu.Unlock()
		return nil
	}
	i.closed = true
	sessions := i.sessions
	i.sessions = nil
	i.mu.Unlock()

	return closeSessions(sessions)
}

// Compile-time interface-satisfaction check. If the invoker struct
// ever drops a method required by the public [Invoker] interface,
// this line fails to compile, which is the desired behaviour.
var _ Invoker = (*invoker)(nil)
