// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestBuildHTTPTransportHappyPath exercises the full builder flow
// against an httptest server that inspects incoming request
// headers. Verifies:
//
//  1. The resulting transport's HTTPClient is wired with the
//     header-injecting round-tripper.
//  2. The caller's Header map values appear on outgoing requests.
//  3. An `Authorization: Bearer <token>` header derived from the
//     credential is present on outgoing requests.
//  4. The credential []byte is zeroed by the builder.
//
// The test deliberately uses an httptest.Server rather than an
// sdkmcp.Connect flow so the round-tripper is exercised
// independently of the SDK's handshake path (which is covered by
// the SDK's own tests and by the S39 integration suite).
func TestBuildHTTPTransportHappyPath(t *testing.T) {
	t.Parallel()

	// Handler captures the first request's headers and responds
	// with a trivial 200 OK.
	var captured http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(srv.Close)

	header := map[string]string{
		"X-Praxis-Test": "value-1",
	}
	cred := []byte("bearer-token-xyz")
	credCopy := append([]byte(nil), cred...)

	transport, err := BuildHTTPTransport(srv.URL, header, cred)
	if err != nil {
		t.Fatalf("BuildHTTPTransport: %v", err)
	}
	if transport == nil {
		t.Fatal("BuildHTTPTransport returned nil transport with nil error")
	}
	if transport.Endpoint != srv.URL {
		t.Errorf("transport.Endpoint = %q, want %q", transport.Endpoint, srv.URL)
	}
	if transport.HTTPClient == nil {
		t.Fatal("transport.HTTPClient is nil; round-tripper not wired")
	}

	// Exercise the round-tripper with a real HTTP request. The
	// httptest server captures the resulting headers for
	// inspection.
	req, err := http.NewRequestWithContext(context.Background(),http.MethodPost, srv.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := transport.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("HTTPClient.Do: %v", err)
	}
	_ = resp.Body.Close()

	if got := captured.Get("X-Praxis-Test"); got != "value-1" {
		t.Errorf("captured X-Praxis-Test = %q, want %q", got, "value-1")
	}
	wantAuth := "Bearer " + string(credCopy)
	if got := captured.Get(authorizationHeader); got != wantAuth {
		t.Errorf("captured Authorization = %q, want %q", got, wantAuth)
	}

	// The credential []byte must have been zeroed by BuildHTTPTransport.
	for i, b := range cred {
		if b != 0 {
			t.Errorf("cred[%d] = 0x%02x, want 0 (T31.3 buffer-zeroing violated for HTTP path)", i, b)
			break
		}
	}
}

// TestBuildHTTPTransportNoCredential covers the unauthenticated
// path: nil or empty cred means no Authorization header should be
// injected.
func TestBuildHTTPTransportNoCredential(t *testing.T) {
	t.Parallel()

	var captured http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	transport, err := BuildHTTPTransport(srv.URL, nil, nil)
	if err != nil {
		t.Fatalf("BuildHTTPTransport: %v", err)
	}

	req, _ := http.NewRequestWithContext(context.Background(),http.MethodGet, srv.URL, nil)
	resp, err := transport.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("HTTPClient.Do: %v", err)
	}
	_ = resp.Body.Close()

	if got := captured.Get(authorizationHeader); got != "" {
		t.Errorf("captured Authorization = %q, want empty (unauthenticated session)", got)
	}
}

// TestBuildHTTPTransportValidation drives the rejection matrix
// for invalid inputs. Every case is a typed SystemError.
func TestBuildHTTPTransportValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		endpoint     string
		header       map[string]string
		wantFragment string
	}{
		{
			name:         "empty endpoint",
			endpoint:     "",
			wantFragment: "URL is empty",
		},
		{
			name:         "no scheme",
			endpoint:     "example.test/mcp",
			wantFragment: "has no scheme",
		},
		{
			name:         "unsupported scheme ftp",
			endpoint:     "ftp://example.test/mcp",
			wantFragment: "unsupported scheme",
		},
		{
			name:         "unsupported scheme file",
			endpoint:     "file:///etc/passwd",
			wantFragment: "unsupported scheme",
		},
		{
			name:         "malformed URL percent encoding",
			endpoint:     "http://example.test/mcp%zz",
			wantFragment: "not parseable",
		},
		{
			name:         "pre-set Authorization header",
			endpoint:     "https://example.test/mcp",
			header:       map[string]string{"Authorization": "Bearer caller-owned"},
			wantFragment: "must not pre-set",
		},
		{
			name:         "pre-set Authorization case-insensitive",
			endpoint:     "https://example.test/mcp",
			header:       map[string]string{"authorization": "Bearer caller-owned"},
			wantFragment: "must not pre-set",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			transport, err := BuildHTTPTransport(c.endpoint, c.header, nil)
			if transport != nil {
				t.Errorf("expected nil transport on validation failure, got %v", transport)
			}
			assertSystemError(t, err, c.wantFragment)
		})
	}
}

// TestHeaderInjectingRoundTripperClonesRequest asserts that the
// round-tripper does NOT mutate the caller-supplied *http.Request
// in place. A second consecutive call with the same request
// object must observe the same initial state every time.
func TestHeaderInjectingRoundTripperClonesRequest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	transport, err := BuildHTTPTransport(srv.URL, map[string]string{"X-Inject": "injected"}, []byte("tok"))
	if err != nil {
		t.Fatalf("BuildHTTPTransport: %v", err)
	}

	req, _ := http.NewRequestWithContext(context.Background(),http.MethodGet, srv.URL, nil)
	// Caller's request starts with no headers.
	if len(req.Header) != 0 {
		t.Fatalf("test precondition: req.Header should be empty, got %v", req.Header)
	}

	resp, err := transport.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("first Do: %v", err)
	}
	_ = resp.Body.Close()

	// After the round-trip, the caller's req.Header MUST still be
	// empty — the round-tripper cloned the request before mutating.
	if len(req.Header) != 0 {
		t.Errorf("req.Header mutated by round-tripper: %v", req.Header)
	}
}
