// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/praxis-os/praxis/credentials"
)

// authorizationHeader is the canonical HTTP header name used for
// bearer token delivery by the Streamable HTTP transport. Declared
// as a constant so the "pre-set Authorization" rejection rule and
// the round-tripper injection site are anchored to the same
// string.
const authorizationHeader = "Authorization"

// BuildHTTPTransport builds a configured
// [*sdkmcp.StreamableClientTransport] from the caller's HTTP
// transport specification, wiring an
// [*http.Client] whose [http.RoundTripper] injects both the
// caller's fixed Header map and an `Authorization: Bearer <token>`
// header derived from the supplied credential bytes on every
// outgoing request.
//
// The function is deterministic and does NOT open a session — the
// caller (the session pool in praxis/mcp/new.go) passes the
// returned transport to [sdkmcp.Client.Connect], which performs
// the MCP handshake over HTTP.
//
// # Validation rules
//
// endpoint must be:
//
//   - Non-empty.
//   - Parseable as a URL via [net/url.Parse].
//   - Carry an `http` or `https` scheme. File, gopher, ftp, and
//     similar schemes are rejected — Phase 7 D108 commits to
//     Streamable HTTP only.
//
// header must NOT contain a pre-set `Authorization` key (case
// insensitive). The adapter derives that header from
// [Server.CredentialRef] via the resolver; accepting a
// caller-supplied value would force a choice between the two
// sources that would be indistinguishable at runtime from a
// misconfiguration. Rejecting the ambiguity at construction time
// is the safe design.
//
// # Credential lifecycle (T31.3 for the HTTP path)
//
// cred is the caller-owned fresh byte slice containing the
// bearer token material. If cred is non-empty, BuildHTTPTransport:
//
//  1. Converts the slice to an immutable Go string via
//     `string(cred)`, materialising a fresh copy.
//  2. Captures that string in the returned round-tripper's
//     closure-like struct field.
//  3. Zeros the caller-owned byte slice via
//     [credentials.ZeroBytes].
//
// After BuildHTTPTransport returns, the only in-process copy of
// the credential material is the immutable Go string held by the
// round-tripper struct, which lives for the duration of the MCP
// session and is released when the session closes. This is the
// HTTP path's half of the **OI-MCP-1** residual risk described in
// the package godoc of github.com/praxis-os/praxis/mcp.
//
// If cred is empty, no bearer is injected: the transport is built
// for an unauthenticated session. The returned http.Client's
// round-tripper still injects the caller's custom headers.
//
// # Errors
//
// All failures return typed [praxiserrors.SystemError] with
// Kind() == ErrorKindSystem. Construction-time failures are
// framework/configuration errors and do not translate to
// ErrorKindTool sub-kinds at this layer (the Phase 7 error
// translation catalogue applies to runtime session/tool-call
// errors, not builder-time rejection).
func BuildHTTPTransport(endpoint string, header map[string]string, cred []byte) (*sdkmcp.StreamableClientTransport, error) {
	if endpoint == "" {
		return nil, systemError("transport: TransportHTTP.URL is empty; a Streamable HTTP endpoint URL is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, systemError(fmt.Sprintf(
			"transport: TransportHTTP.URL %q is not parseable: %v",
			endpoint, err,
		))
	}
	switch parsed.Scheme {
	case "http", "https":
		// accepted
	case "":
		return nil, systemError(fmt.Sprintf(
			"transport: TransportHTTP.URL %q has no scheme; an absolute http:// or https:// URL is required",
			endpoint,
		))
	default:
		return nil, systemError(fmt.Sprintf(
			"transport: TransportHTTP.URL %q has unsupported scheme %q; only http and https are permitted (D108)",
			endpoint, parsed.Scheme,
		))
	}

	canonicalHeader := make(http.Header, len(header))
	for k, v := range header {
		if strings.EqualFold(k, authorizationHeader) {
			return nil, systemError(fmt.Sprintf(
				"transport: TransportHTTP.Header must not pre-set %q; the adapter derives this header from Server.CredentialRef",
				authorizationHeader,
			))
		}
		canonicalHeader.Add(k, v)
	}

	var bearer string
	if len(cred) > 0 {
		// The `string(cred)` conversion allocates a fresh immutable
		// Go string; after this statement the caller's byte slice
		// is no longer referenced by the string. Zeroing the byte
		// slice leaves the string intact.
		bearer = string(cred)
		credentials.ZeroBytes(cred)
	}

	client := &http.Client{
		Transport: &headerInjectingRoundTripper{
			base:    http.DefaultTransport,
			headers: canonicalHeader,
			bearer:  bearer,
		},
	}

	return &sdkmcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: client,
	}, nil
}

// headerInjectingRoundTripper is the [http.RoundTripper]
// implementation used by the HTTP transport to inject the caller's
// fixed headers and the credential-derived Authorization header on
// every outgoing request produced by the SDK's Streamable HTTP
// transport.
//
// The round-tripper clones each incoming request before mutating
// its header so the SDK's own request state is never modified in
// place — idempotent behaviour even if the SDK issues retry
// requests that share an underlying *http.Request value.
//
// # Header merge semantics
//
// For each key/value pair in the caller's headers map:
//
//   - If the request does NOT already carry a value for the key,
//     the round-tripper adds it via [http.Header.Add].
//   - If the request already carries a value (set by the SDK),
//     the round-tripper ALSO adds the caller's value, preserving
//     both — http.Header.Add appends, not replaces. This matches
//     the [net/http] convention for headers like `Accept` or
//     `Cookie` that legitimately repeat.
//
// The Authorization header is always set via
// [http.Header.Set] (not Add), so the adapter's value wins over
// any value the SDK might have placed there — though the SDK's
// default Streamable HTTP transport does not set Authorization
// itself; the header path is entirely the adapter's.
type headerInjectingRoundTripper struct {
	base    http.RoundTripper
	headers http.Header
	bearer  string // empty if no credential is configured
}

// RoundTrip satisfies [http.RoundTripper]. It clones the incoming
// request, applies the configured header merge, then delegates to
// the wrapped base round-tripper.
func (rt *headerInjectingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request and its Header map so we never mutate the
	// SDK's state. req.Clone uses context for propagation.
	req2 := req.Clone(req.Context())

	for k, vs := range rt.headers {
		for _, v := range vs {
			req2.Header.Add(k, v)
		}
	}
	if rt.bearer != "" {
		req2.Header.Set(authorizationHeader, "Bearer "+rt.bearer)
	}

	return rt.base.RoundTrip(req2)
}
