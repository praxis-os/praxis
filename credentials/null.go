// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"context"
	"fmt"
)

// Credential holds resolved credential material returned by a [Resolver].
//
// Callers must treat the Value slice as immutable and must not retain
// references beyond the scope of the call that received it. Implementations
// that wish to enforce zero-on-close semantics should overwrite Value before
// releasing the backing memory.
type Credential struct {
	// Value holds the raw credential bytes (e.g., an API key, bearer token,
	// or PEM-encoded certificate).
	Value []byte
}

// Resolver fetches named credentials at invocation time.
//
// The orchestrator calls Fetch when it needs a credential (e.g., a provider
// API key). Implementations may retrieve credentials from environment
// variables, a secrets manager, a vault, or any other store.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type Resolver interface {
	// Fetch retrieves the credential identified by name. Returns a non-nil
	// error if the credential cannot be resolved.
	Fetch(ctx context.Context, name string) (Credential, error)
}

// Compile-time interface check.
var _ Resolver = NullResolver{}

// NullResolver is a [Resolver] that always returns an error. Used as the
// safe default when no credential resolver is configured.
//
// Returning an error (rather than an empty credential) prevents accidental
// unauthenticated calls to providers.
type NullResolver struct{}

// Fetch always returns an error indicating that no resolver has been configured.
func (NullResolver) Fetch(_ context.Context, name string) (Credential, error) {
	return Credential{}, fmt.Errorf("credentials: no resolver configured (requested %q)", name)
}
