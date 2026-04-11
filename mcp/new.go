// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"regexp"

	praxiserrors "github.com/praxis-os/praxis/errors"
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
// New validates every Server in the input slice (see
// [validateServers]) before applying the supplied options. If any
// validation rule fails, New returns a nil Invoker and a typed error
// that satisfies [praxiserrors.TypedError] with
// [praxiserrors.ErrorKindSystem] — the adapter treats construction
// errors as framework/configuration failures, never as tool errors.
//
// The caller's servers slice is copied internally; mutations to the
// original slice after New returns do not affect the Invoker. The
// options are applied in order against a fresh [config] seeded with
// defaults from [defaultConfig], so later options observe earlier
// mutations.
//
// In this build, New returns a stub Invoker whose [tools.Invoker.Invoke]
// method reports `ErrorKindSystem` with a "runtime not yet implemented"
// message. The MCP SDK integration lands in S31+ and replaces the
// stub's internals without touching the public Invoker surface.
// Construction-time validation is fully wired and covered by tests.
//
// The ctx parameter is reserved for future use: the eventual
// implementation will use it to govern eager session opening,
// credential resolution, and handshake deadlines (see Phase 7 §8).
// S30 does not open any session, so ctx is currently unused.
//
// Stability: stable-v0.x-candidate. The New signature freezes at
// praxis/mcp v1.0.0.
func New(ctx context.Context, servers []Server, opts ...Option) (Invoker, error) {
	_ = ctx // reserved for S31+ session opening

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

	return &invoker{cfg: cfg, servers: pinned}, nil
}

// invoker is the unexported concrete implementation of [Invoker]
// returned by [New]. Its public contract is exposed exclusively
// through the [Invoker] interface.
//
// In S30 the adapter body is a stub: [Invoke] returns a typed system
// error and [Close] is a no-op. S31+ replaces the internals with the
// real MCP SDK integration without mutating the struct's observable
// behaviour beyond making Invoke route to a live session.
type invoker struct {
	cfg     config
	servers []Server // pinned at construction, never mutated after New
}

// errAdapterNotYetImplemented is the sentinel error body used by the
// stub [invoker.Invoke] until the real SDK integration lands.
// Exposing it as a file-local const keeps the message consistent and
// allows tests to match on a stable prefix.
const errAdapterNotYetImplemented = "mcp: adapter runtime not yet implemented in this build (S30 stub)"

// Invoke is the S30 stub for the [tools.Invoker] contract.
//
// The returned [tools.ToolResult] carries [tools.ToolStatusError]
// and a typed [praxiserrors.SystemError] carrying
// [praxiserrors.ErrorKindSystem]. This lets early callers wire the
// Invoker into their orchestrator to exercise construction and
// shutdown paths without needing a live MCP server, while still
// producing a well-formed tool result the orchestrator can classify.
//
// A non-nil framework error is NOT returned: per the
// [tools.Invoker] godoc, tool-level failures travel through
// [tools.ToolResult.Err] with an appropriate [tools.ToolStatus],
// and a non-nil return error is reserved for broken-invoker
// signalling. The stub is intentionally well-behaved so that
// orchestrator-level error handling can be unit-tested against a
// realistic adapter shape before S31 lands.
func (i *invoker) Invoke(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	return tools.ToolResult{
		Status:  tools.ToolStatusError,
		Content: "",
		Err:     praxiserrors.NewSystemError(errAdapterNotYetImplemented, nil),
		CallID:  call.CallID,
	}, nil
}

// Close is the S30 stub for the [io.Closer] contract.
//
// No sessions are opened by the S30 stub, so teardown has nothing to
// release. Close is idempotent and always returns nil. S31+ will
// replace this body with real session teardown (child-process
// termination for stdio, transport close for HTTP).
func (i *invoker) Close() error {
	return nil
}

// Compile-time interface-satisfaction check. If the invoker struct
// ever drops a method required by the public [Invoker] interface,
// this line fails to compile, which is the desired behaviour.
var _ Invoker = (*invoker)(nil)
