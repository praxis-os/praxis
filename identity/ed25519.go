// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/praxis-os/praxis/internal/jwt"
)

// SignerOption configures an [Ed25519Signer] created by [NewEd25519Signer].
type SignerOption func(*ed25519Signer) error

// ed25519Signer implements Signer using Ed25519-signed JWTs.
type ed25519Signer struct {
	key           ed25519.PrivateKey
	issuer        string
	audience      []string
	keyID         string
	tokenLifetime time.Duration
	extraClaims   map[string]any
}

// ed25519KeySize is the expected length (in bytes) of an Ed25519 private key.
const ed25519KeySize = ed25519.PrivateKeySize // 64

// Token lifetime constants per D72.

// DefaultTokenLifetime is the token lifetime used when no WithTokenLifetime
// option is provided (60 seconds).
const DefaultTokenLifetime = 60 * time.Second

// MinTokenLifetime is the minimum allowed token lifetime (5 seconds).
// Values below this are rejected at construction time.
const MinTokenLifetime = 5 * time.Second

// MaxTokenLifetime is the maximum allowed token lifetime (300 seconds).
// Values above this are rejected at construction time.
const MaxTokenLifetime = 300 * time.Second

// NewEd25519Signer returns a [Signer] that produces Ed25519-signed JWTs.
//
// key must be a valid [crypto/ed25519.PrivateKey] (64 bytes).
// NewEd25519Signer returns a non-nil error if key is nil, incorrectly sized,
// or if any option is invalid.
//
// The returned Signer is safe for concurrent use. Multiple goroutines may
// call Sign simultaneously.
//
// Token construction uses only stdlib packages: crypto/ed25519,
// encoding/json, encoding/base64, crypto/rand (for jti). No external JWT
// library is imported.
//
// Algorithm: EdDSA (Ed25519). JOSE header: {"alg":"EdDSA","typ":"JWT"}.
func NewEd25519Signer(key ed25519.PrivateKey, opts ...SignerOption) (Signer, error) {
	if key == nil {
		return nil, fmt.Errorf("identity: ed25519 private key must not be nil")
	}
	if len(key) != ed25519KeySize {
		return nil, fmt.Errorf("identity: ed25519 private key must be %d bytes, got %d", ed25519KeySize, len(key))
	}

	s := &ed25519Signer{
		key:           key,
		issuer:        "praxis",
		tokenLifetime: DefaultTokenLifetime,
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("identity: signer option: %w", err)
		}
	}

	return s, nil
}

// Sign produces a signed JWT from the given claims map.
//
// The signer sets mandatory claims (iss, sub, iat, exp, jti) which override
// any values for those keys in the incoming claims map. The "sub" claim is
// set to the value of the "praxis.invocation_id" key from the incoming
// claims, if present.
//
// Additional caller-configured claims (via WithExtraClaims, once available)
// are merged after incoming claims but before mandatory claims, so mandatory
// claims always win.
func (s *ed25519Signer) Sign(_ context.Context, claims map[string]any) (string, error) {
	now := time.Now()

	jti, err := generateUUIDv7(now)
	if err != nil {
		return "", fmt.Errorf("identity: generate jti: %w", err)
	}

	// Extract invocation ID for the sub claim.
	var invocationID string
	if id, ok := claims[jwt.ClaimInvocationID].(string); ok {
		invocationID = id
	}

	// Extract tool name.
	var toolName string
	if tn, ok := claims[jwt.ClaimToolName].(string); ok {
		toolName = tn
	}

	// Extract parent token if present.
	var parentToken string
	if pt, ok := claims[jwt.ClaimParentToken].(string); ok {
		parentToken = pt
	}

	// Build Extra: incoming claims → configured extra.
	// Mandatory claim keys set as named fields on jwt.Claims are stripped
	// to prevent Extra from overwriting them in marshalPayload.
	extra := make(map[string]any, len(claims)+len(s.extraClaims)+1)

	for k, v := range claims {
		extra[k] = v
	}
	for k, v := range s.extraClaims {
		extra[k] = v
	}

	// Remove keys that are set as named fields on jwt.Claims.
	delete(extra, jwt.ClaimIssuer)
	delete(extra, jwt.ClaimSubject)
	delete(extra, jwt.ClaimAudience)
	delete(extra, jwt.ClaimExpiration)
	delete(extra, jwt.ClaimIssuedAt)
	delete(extra, jwt.ClaimInvocationID)
	delete(extra, jwt.ClaimToolName)
	delete(extra, jwt.ClaimParentToken)

	// jti is added after cleanup — it is signer-generated and must always
	// be present, overriding any incoming value.
	extra[jwt.ClaimJTI] = jti

	jwtClaims := jwt.Claims{
		Issuer:       s.issuer,
		Subject:      invocationID,
		Audience:     s.audience,
		IssuedAt:     now,
		Expiration:   now.Add(s.tokenLifetime),
		InvocationID: invocationID,
		ToolName:     toolName,
		ParentToken:  parentToken,
		Extra:        extra,
	}

	return jwt.Encode(jwtClaims, s.key)
}

// validateTokenLifetime checks that d is within [MinTokenLifetime, MaxTokenLifetime].
func validateTokenLifetime(d time.Duration) error {
	if d < MinTokenLifetime {
		return fmt.Errorf("token lifetime %v is below minimum %v", d, MinTokenLifetime)
	}
	if d > MaxTokenLifetime {
		return fmt.Errorf("token lifetime %v exceeds maximum %v", d, MaxTokenLifetime)
	}
	return nil
}

// generateUUIDv7 produces a UUIDv7 string using the given timestamp for the
// 48-bit millisecond prefix and crypto/rand for the remaining random bits.
//
// Format: xxxxxxxx-xxxx-7xxx-yxxx-xxxxxxxxxxxx
// where version=7 and variant bits are set per RFC 9562.
func generateUUIDv7(now time.Time) (string, error) {
	var uuid [16]byte

	// Bytes 0–5: 48-bit big-endian Unix millisecond timestamp.
	ms := uint64(now.UnixMilli())
	binary.BigEndian.PutUint16(uuid[0:2], uint16(ms>>32))
	binary.BigEndian.PutUint32(uuid[2:6], uint32(ms))

	// Bytes 6–15: random, then set version and variant.
	if _, err := rand.Read(uuid[6:]); err != nil {
		return "", err
	}

	// Version 7: high nibble of byte 6 = 0111.
	uuid[6] = (uuid[6] & 0x0F) | 0x70

	// Variant 10: high 2 bits of byte 8 = 10.
	uuid[8] = (uuid[8] & 0x3F) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}
