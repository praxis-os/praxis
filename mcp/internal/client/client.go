// SPDX-License-Identifier: Apache-2.0

// Package client is the praxis/mcp internal wrapper around the
// official Model Context Protocol Go SDK
// (github.com/modelcontextprotocol/go-sdk/mcp).
//
// The wrapper is deliberately thin: it exists only so that every
// import of the upstream SDK is isolated to a single Go package
// inside the praxis/mcp module. That isolation enables two things:
//
//  1. The package name collision between the upstream SDK (which
//     also exports its top-level types from a package literally
//     named `mcp`) and the praxis/mcp module is resolved in exactly
//     one place via an import alias, instead of being sprinkled
//     across every transport and session file. The adapter's
//     own code sees a single, praxis-flavoured API.
//  2. A future SDK swap (e.g., if upstream cuts a v2 module path
//     or we replace the SDK with an in-house client) is a
//     source-level change in this package only. Callers in the
//     rest of the adapter observe a stable internal contract.
//
// This package must not be imported from outside
// github.com/praxis-os/praxis/mcp; the `internal/` prefix enforces
// that at the compile level.
//
// # S31 PR-A scope (this file)
//
// PR-A of S31 only proves the SDK integrates into the module build
// graph: this package exports a [NewClient] constructor that wraps
// [sdkmcp.NewClient] with praxis-side identity strings. Session
// opening, transport wiring, and the real adapter dispatch path
// arrive in S31 PR-B (stdio + HTTP implementations) and S32 (tool
// namespacing + session routing).
package client

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/praxis-os/praxis/mcp/internal/version"
)

// implementationName is the value the adapter reports to MCP
// servers as its `Implementation.Name`. Per the MCP specification
// this is a free-form identifier; we use the full Go module path so
// upstream servers can trace connections back to this adapter in
// their logs unambiguously.
const implementationName = "github.com/praxis-os/praxis/mcp"

// NewClient constructs a fresh upstream MCP client configured with
// the praxis adapter's identity.
//
// The returned [sdkmcp.Client] is ready to Connect over any
// Transport the caller supplies. PR-A does not call Connect from
// production code; the real session-opening path arrives in PR-B
// when the stdio and HTTP transports are wired.
//
// Returned errors come directly from the upstream SDK and are not
// translated. Callers in adapter code MUST translate them into the
// praxis errors taxonomy (errors.ErrorKindTool sub-kinds) before
// surfacing them to orchestrator consumers.
func NewClient() *sdkmcp.Client {
	return sdkmcp.NewClient(
		&sdkmcp.Implementation{
			Name:    implementationName,
			Version: version.Version,
		},
		nil, // ClientOptions: leave at upstream default for PR-A.
	)
}
