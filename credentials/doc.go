// SPDX-License-Identifier: Apache-2.0

// Package credentials defines the credential resolution interface and its
// null implementation.
//
// [Resolver] fetches named credentials at invocation time. The orchestrator
// uses a Resolver to obtain provider API keys and other secrets without
// holding them in memory at startup.
//
// [NullResolver] always returns an error, signalling that no credential
// source has been configured. This is the safe default — it prevents
// accidental unauthenticated calls rather than silently returning empty
// values.
//
// Stability: frozen-v1.0.
package credentials
