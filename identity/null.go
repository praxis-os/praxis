// SPDX-License-Identifier: Apache-2.0

package identity

import "context"

// Signer produces a signed identity token for an invocation.
//
// The orchestrator calls Sign at the start of each invocation. The resulting
// token (e.g., an Ed25519-signed JWT) can be attached to outbound requests so
// downstream systems can verify the caller's identity and confirm the payload
// was not tampered with.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type Signer interface {
	// Sign produces a signed token from the given claims map. The token
	// format is implementation-defined (commonly a JWT compact serialization).
	//
	// Returns an empty string and nil error when signing is disabled
	// ([NullSigner]).
	Sign(ctx context.Context, claims map[string]any) (string, error)
}

// Compile-time interface check.
var _ Signer = NullSigner{}

// NullSigner is a [Signer] that returns an empty token without performing any
// cryptographic operation. Used as the default when identity signing is
// disabled or not yet configured.
//
// Callers that require signed tokens must replace NullSigner with a real
// implementation before sending invocations to systems that enforce identity
// verification.
type NullSigner struct{}

// Sign returns an empty string and nil error for all inputs.
func (NullSigner) Sign(_ context.Context, _ map[string]any) (string, error) {
	return "", nil
}
