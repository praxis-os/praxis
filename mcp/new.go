// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	praxiserrors "github.com/praxis-os/praxis/errors"
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
// In this build the returned Invoker's [tools.Invoker.Invoke] method
// is a stub that reports `ErrorKindSystem`: sessions are open but
// tool-name routing through them (namespacing decode, server
// lookup, SDK CallTool dispatch) arrives in S32. Early callers can
// wire the Invoker into their orchestrator to exercise construction,
// shutdown, credential flow, and error-path classification before
// S32 lands.
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

	return &invoker{cfg: cfg, servers: pinned, sessions: sessions}, nil
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
// A nil or empty slice returns nil.
func closeSessions(sessions []*sdkmcp.ClientSession) error {
	var errs []error
	for i := len(sessions) - 1; i >= 0; i-- {
		if err := sessions[i].Close(); err != nil {
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
// are index-aligned. [Close] tears every session down in reverse
// order, aggregating errors via [errors.Join].
//
// [Invoke] is still a stub in S31 PR-B: sessions are open but
// tool-name routing through them arrives in S32.
type invoker struct {
	cfg      config
	servers  []Server                // pinned at construction, never mutated after New
	sessions []*sdkmcp.ClientSession // index-aligned with servers, opened eagerly by New
}

// errAdapterRoutingNotWired is the sentinel error body used by the
// S31 PR-B stub [invoker.Invoke] until the tool-name routing layer
// lands in S32. Exposing it as a file-local const keeps the message
// consistent and allows tests to match on a stable prefix.
const errAdapterRoutingNotWired = "mcp: session is open but tool-name routing is not yet wired in this build (S32 stub)"

// Invoke is the S31 PR-B stub for the [tools.Invoker] contract.
//
// In this build every MCP session owned by the invoker is already
// open and ready to dispatch tool calls — what is missing is the
// namespacing-decode layer that would turn an incoming
// `{LogicalName}__{mcpToolName}` call into a
// (session, raw mcp tool name) tuple. That layer arrives in S32.
//
// The returned [tools.ToolResult] carries [tools.ToolStatusError]
// and a typed [praxiserrors.SystemError] with
// [praxiserrors.ErrorKindSystem]. A nil framework error is returned
// per the [tools.Invoker] godoc contract (framework errors are
// reserved for broken-invoker signalling; tool-level failures flow
// through ToolResult.Err).
//
// Tests that wire the S31 PR-B Invoker into an orchestrator can
// observe this stub and classify it deterministically. S32 will
// replace the body of this method with real dispatch.
func (i *invoker) Invoke(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	return tools.ToolResult{
		Status:  tools.ToolStatusError,
		Content: "",
		Err:     praxiserrors.NewSystemError(errAdapterRoutingNotWired, nil),
		CallID:  call.CallID,
	}, nil
}

// Close tears down every MCP session owned by this invoker.
//
// Sessions are closed in reverse order of opening (LIFO). Errors
// from individual session closes are collected and returned as a
// single joined error via [errors.Join]; a session that fails to
// close does not prevent sibling sessions from being closed. If
// every session closes cleanly, Close returns nil.
//
// Close is safe to call more than once: the second call iterates
// over an already-drained session slice and returns nil. The
// underlying [sdkmcp.ClientSession.Close] method, per the go-sdk
// v1.5.0 source, is not documented as idempotent, so this invoker
// clears its own sessions slice after the first call to guarantee
// the documented [Invoker] idempotency contract.
func (i *invoker) Close() error {
	err := closeSessions(i.sessions)
	i.sessions = nil
	return err
}

// Compile-time interface-satisfaction check. If the invoker struct
// ever drops a method required by the public [Invoker] interface,
// this line fails to compile, which is the desired behaviour.
var _ Invoker = (*invoker)(nil)
