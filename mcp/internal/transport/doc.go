// SPDX-License-Identifier: Apache-2.0

// Package transport holds the praxis-internal builders that turn a
// caller-supplied praxis/mcp Transport specification into an
// SDK-native [sdkmcp.Transport] value ready for
// [sdkmcp.Client.Connect].
//
// The package is strictly internal (enforced by the `internal/`
// prefix) and exports only the two builder functions consumed by
// praxis/mcp's session pool:
//
//   - [BuildCommand] turns a [praxis/mcp.TransportStdio] spec
//     (passed as individual primitive fields to avoid a parent-import
//     cycle) into an [*exec.Cmd] configured with process isolation
//     and no leaked file descriptors, suitable for wrapping in
//     [sdkmcp.CommandTransport].
//
//   - [BuildHTTPTransport] turns a [praxis/mcp.TransportHTTP] spec
//     into a [*sdkmcp.StreamableClientTransport] configured with an
//     [*http.Client] whose round-tripper injects the bearer token
//     from the caller-supplied credential material on every request.
//
// Neither builder opens a session. Session opening happens in
// praxis/mcp's `new.go` after a builder returns successfully — the
// two steps are kept separate so that credential-buffer zeroing
// (Phase 7 D117) can happen at the exact moment after the session
// is open and the adapter no longer needs the raw bytes.
//
// Both builders return typed errors that satisfy
// [praxiserrors.TypedError] with [praxiserrors.ErrorKindSystem].
// Construction-time failures are framework/configuration errors,
// never tool errors; the praxis error taxonomy maps them to
// ErrorKindSystem at this layer.
//
// # Import discipline
//
// This package MUST NOT import github.com/praxis-os/praxis/mcp:
// that would create a cycle since praxis/mcp imports this package.
// The builder functions take primitive field values instead of the
// parent's spec types; praxis/mcp/new.go unpacks Server values and
// passes individual fields down.
package transport
