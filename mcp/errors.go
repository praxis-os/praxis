// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	praxiserrors "github.com/praxis-os/praxis/errors"
)

// classifyCallToolError maps an error returned by
// [sdkmcp.ClientSession.CallTool] into a [praxiserrors.ToolSubKind]
// per the D113 translation table.
//
// The classifier runs only on the mid-session dispatch path: by
// the time [invoker.Invoke] calls it, the session has been open
// since [New] and the error, if any, represents a runtime failure
// on an already-established connection. Handshake failures
// surface at construction time via [openSessions] as typed
// [praxiserrors.SystemError] values and never reach this function.
//
// # Classification order (first match wins)
//
//  1. Context cancellation or deadline exceeded → [praxiserrors.ToolSubKindNetwork].
//     Cancellation is treated as a transport-layer interruption:
//     the call never completed, the session is still live from
//     the adapter's standpoint, and the orchestrator's own
//     classifier decides retry semantics via the normal Phase 3
//     D44 path.
//  2. SDK transport sentinels ([sdkmcp.ErrConnectionClosed]) and
//     stdlib network / syscall sentinels (`*net.Error`, `io.EOF`,
//     `io.ErrUnexpectedEOF`, `syscall.ECONNREFUSED`, and friends) →
//     [praxiserrors.ToolSubKindNetwork]. These cover the
//     "transport disconnect / I/O failure" row of the D113 table.
//  3. [sdkmcp.ErrSessionMissing] → [praxiserrors.ToolSubKindCircuitOpen].
//     The MCP server has terminated the session per §2.5.3 of the
//     Streamable HTTP transport spec; the only recovery is a
//     fresh session open with a fresh credential fetch, which
//     matches the "session circuit-broken" + "HTTP 401/403 on
//     established session" rows of D113.
//  4. HTTP status sniffing on the error message
//     ([extractHTTPStatusSignal]):
//     "Unauthorized" / "Forbidden" → [praxiserrors.ToolSubKindCircuitOpen]
//     (mid-session auth revocation — D113 amendment 2026-04-10),
//     "Too Many Requests" → [praxiserrors.ToolSubKindNetwork]
//     (transient rate-limit — D113 amendment 2026-04-10).
//  5. TLS error patterns (`tls:` / `x509:` prefixes) →
//     [praxiserrors.ToolSubKindNetwork]. The Go stdlib does not
//     export a single TLS-error type, so pattern matching on the
//     documented message prefixes is the most portable option;
//     the stdlib TLS error messages are effectively API-stable at
//     this point.
//  6. `*json.SyntaxError` / `*json.UnmarshalTypeError` →
//     [praxiserrors.ToolSubKindSchemaViolation]. These arise when
//     the SDK fails to decode the wire response because the MCP
//     server produced a payload that does not match the expected
//     `CallToolResult` shape.
//  7. Default → [praxiserrors.ToolSubKindServerError]. D113
//     collapses every JSON-RPC protocol code (−32700…−32603) and
//     every server-defined code (−32000…−32099) to `ServerError`,
//     and the same applies to any unclassified error the SDK
//     surfaces from a tool-level failure. The default is
//     deliberately generous: a miss here is safer than a
//     misclassification.
//
// # Known limitations
//
// The HTTP status sniffing step inspects the error message rather
// than a typed error from the SDK because the `go-sdk@v1.5.0`
// Streamable HTTP transport wraps non-2xx responses using
// `fmt.Errorf("%s: %v", requestSummary, http.StatusText(resp.StatusCode))`
// (see `mcp/streamable.go` in that release), which does not
// produce a typed error that [errors.As] could match. Callers
// with stricter classification requirements can wrap the
// returned `tools.Invoker` and re-classify before the result
// reaches the orchestrator. Upstream-friendly alternatives
// (a typed `HTTPStatusError` in the SDK) are tracked outside
// this module.
//
// The classifier always returns a non-empty [praxiserrors.ToolSubKind]
// for a non-nil input. It returns the empty sub-kind only when
// called with a nil error, which is a programmer-misuse case
// that the single call site inside [invoker.Invoke] never
// triggers.
func classifyCallToolError(err error) praxiserrors.ToolSubKind {
	if err == nil {
		return ""
	}

	// 1. Context cancellation / deadline
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return praxiserrors.ToolSubKindNetwork
	}

	// 2. Transport disconnect / I/O failure
	if isNetworkError(err) {
		return praxiserrors.ToolSubKindNetwork
	}

	// 3. Session-gone → circuit open
	if errors.Is(err, sdkmcp.ErrSessionMissing) {
		return praxiserrors.ToolSubKindCircuitOpen
	}

	// 4. HTTP status signals
	if sub, ok := classifyHTTPStatusSignal(err); ok {
		return sub
	}

	// 5. TLS errors (pattern-matched)
	if looksLikeTLSError(err) {
		return praxiserrors.ToolSubKindNetwork
	}

	// 6. Decode-time schema violations
	var jsonSyntaxErr *json.SyntaxError
	if errors.As(err, &jsonSyntaxErr) {
		return praxiserrors.ToolSubKindSchemaViolation
	}
	var jsonTypeErr *json.UnmarshalTypeError
	if errors.As(err, &jsonTypeErr) {
		return praxiserrors.ToolSubKindSchemaViolation
	}

	// 7. Default: server-side error (covers JSON-RPC protocol and
	//    server-defined codes, plus any unclassified SDK error).
	return praxiserrors.ToolSubKindServerError
}

// isNetworkError tests whether err represents a transport-layer
// network failure: SDK-exported connection-closed sentinel, any
// `net.Error` in the chain, one of the well-known stdlib network
// syscalls, or an `io.EOF`/`io.ErrUnexpectedEOF` unexpected on a
// JSON-RPC response. The helper is extracted from
// [classifyCallToolError] so unit tests can exercise the
// predicate in isolation — the classifier itself has a
// first-match-wins structure that would otherwise be awkward to
// test a single branch at a time.
func isNetworkError(err error) bool {
	if errors.Is(err, sdkmcp.ErrConnectionClosed) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	for _, sentinel := range []error{
		syscall.ECONNREFUSED,
		syscall.ECONNRESET,
		syscall.ETIMEDOUT,
		syscall.EPIPE,
		io.EOF,
		io.ErrUnexpectedEOF,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

// classifyHTTPStatusSignal inspects the error message for known
// HTTP status text produced by the go-sdk Streamable HTTP
// transport and, if recognised, returns the corresponding
// [praxiserrors.ToolSubKind] per D113.
//
// The SDK wraps non-2xx responses with
// `fmt.Errorf("%s: %v", requestSummary, http.StatusText(resp.StatusCode))`,
// so the status surfaces as its human-readable text
// (`"Unauthorized"`, `"Forbidden"`, `"Too Many Requests"`).
// Matching on the text is less fragile than it looks: the
// strings are constants in the Go stdlib's `net/http` package
// and are effectively API-stable.
//
// The boolean result distinguishes "classified" from
// "not an HTTP status signal" so the classifier caller can
// continue through the subsequent branches on a miss.
func classifyHTTPStatusSignal(err error) (praxiserrors.ToolSubKind, bool) {
	msg := err.Error()

	// Order matters: a 401 Unauthorized response body might include
	// the word "Forbidden" inside the payload, but the SDK wraps the
	// wire error with `http.StatusText(resp.StatusCode)` at the very
	// end of the message, so we can use a suffix-anchored contains
	// check for reliability.
	switch {
	case containsWord(msg, "Unauthorized"),
		containsWord(msg, "Forbidden"):
		return praxiserrors.ToolSubKindCircuitOpen, true
	case containsWord(msg, "Too Many Requests"):
		return praxiserrors.ToolSubKindNetwork, true
	}
	return "", false
}

// containsWord returns true when msg contains word as a
// whitespace- or colon- delimited token. It avoids false
// positives from substrings embedded inside longer words (for
// example, a tool called "AuthorizedHandler" would not match
// "Unauthorized"). The delimiter set matches the punctuation the
// go-sdk error formatter emits around `http.StatusText` values.
func containsWord(msg, word string) bool {
	idx := strings.Index(msg, word)
	if idx < 0 {
		return false
	}
	// Check left boundary.
	if idx > 0 {
		prev := msg[idx-1]
		if !isWordBoundary(prev) {
			return false
		}
	}
	// Check right boundary.
	end := idx + len(word)
	if end < len(msg) {
		next := msg[end]
		if !isWordBoundary(next) {
			return false
		}
	}
	return true
}

// isWordBoundary returns true for bytes that delimit a word
// token in an HTTP-status-carrying SDK error message. The SDK's
// `fmt.Errorf("%s: %v", summary, http.StatusText(code))` emits
// a ": " before the status text and the status text ends the
// string, so colon, space, and the zero byte (end-of-string
// surrogate used by [containsWord]) are the only boundaries
// that matter in practice. Punctuation is added defensively.
func isWordBoundary(b byte) bool {
	switch b {
	case ' ', ':', ',', '.', ';', '(', ')', '[', ']', '{', '}', '"', '\'', '\t', '\n', '\r':
		return true
	}
	return false
}

// looksLikeTLSError reports whether err's message starts with a
// well-known TLS/X.509 stdlib prefix. The Go standard library
// emits TLS handshake and certificate verification errors with
// stable `tls:` or `x509:` message prefixes that are effectively
// API-stable; no single exported type covers both layers, so
// prefix matching is the most portable classification path.
// Examples caught by this predicate:
//
//   - `"tls: failed to verify certificate: ..."` (handshake failure)
//   - `"x509: certificate signed by unknown authority"` (trust-store miss)
//   - `"x509: certificate has expired or is not yet valid: ..."`
func looksLikeTLSError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "tls: ") || strings.Contains(msg, "x509: ")
}
