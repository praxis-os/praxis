// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/praxis-os/praxis/internal/jwt"
)

// SignerOption configures an [Ed25519Signer] created by [NewEd25519Signer].
type SignerOption func(*ed25519Signer) error

// WithIssuer overrides the default "praxis" issuer claim (iss) in tokens
// produced by the signer. Production callers should set a meaningful issuer
// (e.g., service name or environment identifier).
func WithIssuer(iss string) SignerOption {
	return func(s *ed25519Signer) error {
		s.issuer = iss
		return nil
	}
}

// WithTokenLifetime sets the duration for which each signed token is valid.
// The lifetime is measured from the time of the Sign call (iat) to
// expiration (exp = iat + lifetime).
//
// Default: 60 seconds. Minimum: 5 seconds. Maximum: 300 seconds.
// Out-of-range values are rejected with an error.
func WithTokenLifetime(d time.Duration) SignerOption {
	return func(s *ed25519Signer) error {
		if err := validateTokenLifetime(d); err != nil {
			return err
		}
		s.tokenLifetime = d
		return nil
	}
}

// WithKeyID sets the "kid" (key ID) JOSE header field on tokens produced
// by the signer (per D74). Verifiers use kid to select the correct public
// key when multiple keys are in rotation.
func WithKeyID(kid string) SignerOption {
	return func(s *ed25519Signer) error {
		s.keyID = kid
		return nil
	}
}

// WithExtraClaims adds caller-defined claims to every token produced by the
// signer. Claims are merged into the payload before signing.
//
// If a caller claim key collides with a mandatory registered or custom
// claim key (e.g., "iss", "sub", "praxis.invocation_id"), the mandatory
// claim wins and the caller claim for that key is silently dropped.
//
// The claims map is shallow-copied at construction time; mutations to the
// original map after calling WithExtraClaims have no effect.
func WithExtraClaims(claims map[string]any) SignerOption {
	return func(s *ed25519Signer) error {
		s.extraClaims = make(map[string]any, len(claims))
		for k, v := range claims {
			s.extraClaims[k] = v
		}
		return nil
	}
}

// ed25519Signer implements Signer using Ed25519-signed JWTs.
type ed25519Signer struct {
	extraClaims    map[string]any
	issuer         string
	keyID          string
	cachedKidHeader string // pre-encoded base64url header with kid, computed at construction
	key            ed25519.PrivateKey
	audience       []string
	tokenLifetime  time.Duration
}

// ed25519KeySize is the expected length (in bytes) of an Ed25519 private key.
const ed25519KeySize = ed25519.PrivateKeySize // 64

// claimNamedKeys is the set of JWT claim keys that map to named fields on
// jwt.Claims. Used to filter Extra claims without allocating a map per Sign call.
var claimNamedKeys = map[string]struct{}{
	jwt.ClaimIssuer: {}, jwt.ClaimSubject: {}, jwt.ClaimAudience: {},
	jwt.ClaimExpiration: {}, jwt.ClaimIssuedAt: {}, jwt.ClaimJTI: {},
	jwt.ClaimInvocationID: {}, jwt.ClaimToolName: {}, jwt.ClaimParentToken: {},
}

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

	// Pre-compute the kid header at construction time so Encode
	// doesn't rebuild and base64url-encode it on every Sign call.
	if s.keyID != "" {
		s.cachedKidHeader = jwt.EncodeKidHeader(s.keyID)
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

	// Build Extra lazily: only allocate when non-standard claims exist.
	// Keys that map to named fields on jwt.Claims are stripped to prevent
	// Extra from overwriting them in marshalPayload.
	unknownCount := 0
	for k := range claims {
		if _, named := claimNamedKeys[k]; !named {
			unknownCount++
		}
	}

	var extra map[string]any
	if unknownCount+len(s.extraClaims) > 0 {
		extra = make(map[string]any, unknownCount+len(s.extraClaims))
		for k, v := range claims {
			if _, named := claimNamedKeys[k]; !named {
				extra[k] = v
			}
		}
		for k, v := range s.extraClaims {
			if _, named := claimNamedKeys[k]; !named {
				extra[k] = v
			}
		}
	}

	jwtClaims := jwt.Claims{
		Issuer:       s.issuer,
		Subject:      invocationID,
		Audience:     s.audience,
		IssuedAt:     now,
		Expiration:   now.Add(s.tokenLifetime),
		InvocationID: invocationID,
		ToolName:     toolName,
		ParentToken:  parentToken,
		JTI:          jti,
		Extra:        extra,
	}

	header := jwt.FixedHeader()
	if s.cachedKidHeader != "" {
		header = s.cachedKidHeader
	}
	return jwt.EncodeWithHeader(jwtClaims, s.key, header)
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
	binary.BigEndian.PutUint16(uuid[0:2], uint16(ms>>32)) //nolint:gosec // 48-bit timestamp fits after shift
	binary.BigEndian.PutUint32(uuid[2:6], uint32(ms))     //nolint:gosec // lower 32 bits of 48-bit timestamp

	// Bytes 6–15: random, then set version and variant.
	if _, err := rand.Read(uuid[6:]); err != nil {
		return "", err
	}

	// Version 7: high nibble of byte 6 = 0111.
	uuid[6] = (uuid[6] & 0x0F) | 0x70

	// Variant 10: high 2 bits of byte 8 = 10.
	uuid[8] = (uuid[8] & 0x3F) | 0x80

	// Format as xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx using stack-allocated
	// buffer to avoid fmt.Sprintf overhead.
	var buf [36]byte
	hex.Encode(buf[0:8], uuid[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], uuid[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], uuid[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], uuid[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], uuid[10:16])
	return string(buf[:]), nil
}
