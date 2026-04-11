// SPDX-License-Identifier: Apache-2.0

package mcp

import "testing"

// TestTransportSealed asserts that the only two valid [Transport]
// implementations are [TransportStdio] and [TransportHTTP]. The check
// is a compile-time static assertion embedded in transport.go; this
// test exists so regression coverage of the sealing contract is
// discoverable in the test binary.
func TestTransportSealed(t *testing.T) {
	t.Parallel()

	// Direct satisfaction checks. If a future edit to transport.go
	// accidentally drops the isMCPTransport method from either
	// concrete type, these lines fail to compile — which is the
	// desired behaviour.
	var _ Transport = TransportStdio{}
	var _ Transport = TransportHTTP{}

	// Value-level round-trip through the Transport interface. This
	// catches accidental pointer-receiver slips (a pointer receiver
	// would still satisfy the interface but would change how the
	// adapter stores Server values).
	cases := []struct {
		name string
		t    Transport
	}{
		{name: "stdio", t: TransportStdio{Command: "mcp-server"}},
		{name: "http", t: TransportHTTP{URL: "https://example.test/mcp"}},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.t == nil {
				t.Fatalf("transport %s: unexpected nil", c.name)
			}
			// Invoke the sealing marker to make sure the type
			// actually implements it (not just structurally, but via
			// the method set). This is a no-op at runtime.
			c.t.isMCPTransport()
		})
	}
}

// TestTransportStdioZeroValue documents that a zero [TransportStdio]
// is a valid Go value — it satisfies [Transport] — but is not a valid
// configuration: it will be rejected by the [New] validator in the
// commit that introduces it. This test pins the zero-value contract
// so a later refactor does not accidentally add required fields via
// panics at construction time.
func TestTransportStdioZeroValue(t *testing.T) {
	t.Parallel()

	var stdio TransportStdio
	if stdio.Command != "" {
		t.Errorf("zero Command: got %q, want empty", stdio.Command)
	}
	if stdio.Args != nil {
		t.Errorf("zero Args: got %v, want nil", stdio.Args)
	}
	if stdio.Env != nil {
		t.Errorf("zero Env: got %v, want nil", stdio.Env)
	}
	if stdio.CredentialEnv != "" {
		t.Errorf("zero CredentialEnv: got %q, want empty", stdio.CredentialEnv)
	}
}

// TestTransportHTTPZeroValue mirrors [TestTransportStdioZeroValue]
// for the HTTP transport.
func TestTransportHTTPZeroValue(t *testing.T) {
	t.Parallel()

	var http TransportHTTP
	if http.URL != "" {
		t.Errorf("zero URL: got %q, want empty", http.URL)
	}
	if http.Header != nil {
		t.Errorf("zero Header: got %v, want nil", http.Header)
	}
}
