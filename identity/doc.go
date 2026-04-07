// SPDX-License-Identifier: Apache-2.0

// Package identity defines the invocation identity signing interface and
// its null implementation.
//
// [Signer] produces a signed token (e.g., an Ed25519 JWT) for each
// invocation so downstream systems can verify the caller's identity and
// confirm the token was not tampered with.
//
// [NullSigner] returns an empty token string without error. Used when
// identity signing is disabled or not yet configured.
//
// Stability: frozen-v1.0.
package identity
