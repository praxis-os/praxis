// SPDX-License-Identifier: Apache-2.0

package mcp

import "testing"

// TestServerZeroValue pins the zero-value shape of [Server]. The zero
// value is a valid Go value (all fields empty), but is rejected by
// the [New] validator — this test documents the contract so a later
// refactor cannot silently add required fields or hidden state.
func TestServerZeroValue(t *testing.T) {
	t.Parallel()

	var s Server
	if s.LogicalName != "" {
		t.Errorf("zero LogicalName: got %q, want empty", s.LogicalName)
	}
	if s.Transport != nil {
		t.Errorf("zero Transport: got %v, want nil", s.Transport)
	}
	if s.CredentialRef != "" {
		t.Errorf("zero CredentialRef: got %q, want empty", s.CredentialRef)
	}
}

// TestCredentialRefIsStringAlias asserts that [CredentialRef] is a
// type alias for string, not a distinct type. An alias permits
// implicit conversion in both directions so callers can pass string
// literals verbatim where a CredentialRef is expected and vice-versa.
// If this test fails to compile, CredentialRef has been promoted to
// a named type and the aliased-string contract documented in
// server.go is broken.
func TestCredentialRefIsStringAlias(t *testing.T) {
	t.Parallel()

	// string → CredentialRef assignment without conversion.
	var ref CredentialRef = "my-secret"
	// CredentialRef → string assignment without conversion.
	var s string = ref
	if s != "my-secret" {
		t.Errorf("alias round-trip: got %q, want %q", s, "my-secret")
	}
}

// TestServerFieldCompositions is a compile-time smoke test that each
// legal Server configuration is constructible via a composite literal.
// It documents the permitted combinations without asserting any
// runtime behaviour (which is the responsibility of the New-time
// validator tests in commit 5).
func TestServerFieldCompositions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		s    Server
	}{
		{
			name: "stdio no credential",
			s: Server{
				LogicalName: "fs",
				Transport:   TransportStdio{Command: "mcp-fs-server"},
			},
		},
		{
			name: "stdio with credential env",
			s: Server{
				LogicalName:   "fs_auth",
				Transport:     TransportStdio{Command: "mcp-fs-server", CredentialEnv: "MCP_TOKEN"},
				CredentialRef: "fs-token",
			},
		},
		{
			name: "http no credential",
			s: Server{
				LogicalName: "github",
				Transport:   TransportHTTP{URL: "https://example.test/mcp"},
			},
		},
		{
			name: "http with credential",
			s: Server{
				LogicalName:   "github_auth",
				Transport:     TransportHTTP{URL: "https://example.test/mcp"},
				CredentialRef: "github-token",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.s.LogicalName == "" {
				t.Fatal("smoke case forgot to set LogicalName")
			}
			if c.s.Transport == nil {
				t.Fatal("smoke case forgot to set Transport")
			}
		})
	}
}
