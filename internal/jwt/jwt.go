// SPDX-License-Identifier: Apache-2.0

// Package jwt provides a minimal, stdlib-only JWT encoder for EdDSA (Ed25519)
// signed tokens.
//
// Only JWT creation (encoding + signing) is supported. Decoding and
// verification are intentionally out of scope for v0.5.0.
//
// The fixed JWT header is {"alg":"EdDSA","typ":"JWT"} as required by RFC 8037.
// Base64url encoding uses no padding per RFC 7515 §2.
//
// This package is internal to praxis and is not part of the public API.
package jwt

import (
	"crypto"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Claim name constants for all fields emitted by this package.
// Callers should use these constants rather than raw strings to avoid typos
// and to remain stable if the claim names are ever renegotiated.
const (
	// ClaimIssuer is the "iss" registered claim (RFC 7519 §4.1.1).
	ClaimIssuer = "iss"

	// ClaimSubject is the "sub" registered claim (RFC 7519 §4.1.2).
	ClaimSubject = "sub"

	// ClaimAudience is the "aud" registered claim (RFC 7519 §4.1.3).
	ClaimAudience = "aud"

	// ClaimExpiration is the "exp" registered claim (RFC 7519 §4.1.4).
	// Value is a NumericDate (Unix seconds, int64).
	ClaimExpiration = "exp"

	// ClaimIssuedAt is the "iat" registered claim (RFC 7519 §4.1.6).
	// Value is a NumericDate (Unix seconds, int64).
	ClaimIssuedAt = "iat"

	// ClaimInvocationID is the praxis-specific invocation identifier.
	ClaimInvocationID = "praxis.invocation_id"

	// ClaimParentToken is the praxis-specific parent token reference.
	// Set when the current invocation was spawned by a parent invocation.
	ClaimParentToken = "praxis.parent_token"

	// ClaimJTI is the "jti" registered claim (RFC 7519 §4.1.7).
	// Value is a unique identifier for the token (typically UUIDv7).
	ClaimJTI = "jti"

	// ClaimToolName is the praxis-specific tool name claim.
	// Identifies the tool for which this identity token was issued.
	ClaimToolName = "praxis.tool_name"
)

// fixedHeader is the pre-encoded base64url representation of
// {"alg":"EdDSA","typ":"JWT"}.
// Computed once at init time; reused for every call to Encode.
var fixedHeader string

func init() {
	h := map[string]string{"alg": "EdDSA", "typ": "JWT"}
	raw, err := json.Marshal(h)
	if err != nil {
		// json.Marshal of a plain map[string]string never fails.
		panic(fmt.Sprintf("jwt: failed to marshal fixed header: %v", err))
	}
	fixedHeader = base64url(raw)
}

// Claims holds the standard and praxis-specific JWT claims for an invocation
// token.
//
// All time fields (Expiration, IssuedAt) are encoded as NumericDate values
// (Unix seconds, int64) per RFC 7519 §2. A zero value for a time field means
// the claim is omitted from the token.
//
// Extra holds arbitrary additional claims. Keys in Extra must not shadow the
// registered claim names (iss, sub, aud, exp, iat) or the praxis-namespaced
// claims; if they do, the Extra value takes precedence in the serialised
// payload.
type Claims struct {
	// Issuer identifies the principal that issued the token (RFC 7519 §4.1.1).
	Issuer string

	// Subject identifies the principal that is the subject of the token
	// (RFC 7519 §4.1.2).
	Subject string

	// Audience identifies the recipients for which the token is intended
	// (RFC 7519 §4.1.3). A single-element audience is serialised as a string;
	// multi-element as a JSON array, per common JWT library convention.
	Audience []string

	// Expiration is the time after which the token MUST NOT be accepted
	// (RFC 7519 §4.1.4). Zero means the claim is omitted.
	Expiration time.Time

	// IssuedAt is the time at which the token was issued
	// (RFC 7519 §4.1.6). Zero means the claim is omitted.
	IssuedAt time.Time

	// InvocationID is the praxis invocation identifier
	// (claim name: "praxis.invocation_id").
	InvocationID string

	// ToolName is the praxis-specific tool name claim identifying the tool
	// for which this identity token was issued
	// (claim name: "praxis.tool_name").
	ToolName string

	// ParentToken is the signed token of the parent invocation, if any
	// (claim name: "praxis.parent_token").
	ParentToken string

	// JTI is the "jti" registered claim (RFC 7519 §4.1.7).
	// Unique token identifier, typically UUIDv7.
	JTI string

	// Extra holds arbitrary additional claims merged into the payload.
	// Keys take precedence over the named fields above when a collision occurs.
	Extra map[string]any
}

// Encode signs claims with key and returns a compact JWT string of the form
// "base64url(header).base64url(payload).base64url(signature)".
//
// The algorithm is always EdDSA (Ed25519). The header contains
// {"alg":"EdDSA","typ":"JWT"} and, if keyID is non-empty, a "kid" field
// for verifier key selection (per D74).
//
// An error is returned if JSON marshalling of the payload fails or if
// ed25519.Sign returns an error (the latter is only possible with a malformed
// key).
// EncodeKidHeader returns the base64url-encoded JWT header for the given keyID.
// Callers that sign many tokens with the same keyID should call this once at
// construction and pass the result to EncodeWithHeader to avoid per-call overhead.
func EncodeKidHeader(keyID string) string {
	if !strings.ContainsAny(keyID, `"\`) {
		return base64url([]byte(`{"alg":"EdDSA","kid":"` + keyID + `","typ":"JWT"}`))
	}
	h := map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": keyID}
	raw, _ := json.Marshal(h) // map[string]string marshal never fails
	return base64url(raw)
}

// Encode signs claims with key and returns a compact JWT string of the form
// "base64url(header).base64url(payload).base64url(signature)".
//
// If keyID is non-empty, a kid header is computed per call. For better
// performance with a stable keyID, pre-compute with EncodeKidHeader and
// use EncodeWithHeader instead.
func Encode(claims Claims, key ed25519.PrivateKey, keyID string) (string, error) {
	header := fixedHeader
	if keyID != "" {
		header = EncodeKidHeader(keyID)
	}
	return EncodeWithHeader(claims, key, header)
}

// EncodeWithHeader signs claims with key using a pre-computed base64url header.
// Use fixedHeader (no kid) or a cached EncodeKidHeader result.
func EncodeWithHeader(claims Claims, key ed25519.PrivateKey, header string) (string, error) {
	payload, err := marshalPayload(claims)
	if err != nil {
		return "", fmt.Errorf("jwt: marshal payload: %w", err)
	}

	encodedPayload := base64url(payload)
	signingInput := header + "." + encodedPayload

	// Ed25519 requires crypto.Hash(0) (no pre-hashing). Using crypto.Hash(0)
	// directly avoids a heap allocation for &ed25519.Options{}.
	sig, err := key.Sign(nil, []byte(signingInput), crypto.Hash(0))
	if err != nil {
		return "", fmt.Errorf("jwt: sign: %w", err)
	}

	return signingInput + "." + base64url(sig), nil
}

// FixedHeader returns the pre-encoded base64url header without a kid field.
func FixedHeader() string { return fixedHeader }

// payloadStruct is used for struct-based JSON encoding on the common path
// (no Extra claims). Struct marshaling avoids map allocation and reflection-
// heavy json.mapEncoder, saving ~3 allocs per token.
type payloadStruct struct {
	Issuer       string `json:"iss,omitempty"`
	Subject      string `json:"sub,omitempty"`
	Audience     any    `json:"aud,omitempty"`
	Expiration   int64  `json:"exp,omitempty"`
	IssuedAt     int64  `json:"iat,omitempty"`
	InvocationID string `json:"praxis.invocation_id,omitempty"`
	ToolName     string `json:"praxis.tool_name,omitempty"`
	ParentToken  string `json:"praxis.parent_token,omitempty"`
	JTI          string `json:"jti,omitempty"`
}

// marshalPayload converts Claims into a JSON byte slice suitable for base64url
// encoding. When no Extra claims are present, uses struct-based encoding to
// avoid map allocation and reflection overhead. When Extra claims exist, falls
// back to map-based encoding where Extra keys overwrite named claims on collision.
func marshalPayload(c Claims) ([]byte, error) {
	if len(c.Extra) == 0 {
		return marshalPayloadStruct(c)
	}
	return marshalPayloadMap(c)
}

// marshalPayloadStruct encodes claims using a typed struct (fast path).
func marshalPayloadStruct(c Claims) ([]byte, error) {
	p := payloadStruct{
		Issuer:       c.Issuer,
		Subject:      c.Subject,
		InvocationID: c.InvocationID,
		ToolName:     c.ToolName,
		ParentToken:  c.ParentToken,
		JTI:          c.JTI,
	}
	switch len(c.Audience) {
	case 1:
		p.Audience = c.Audience[0]
	default:
		if len(c.Audience) > 0 {
			p.Audience = c.Audience
		}
	}
	if !c.Expiration.IsZero() {
		p.Expiration = c.Expiration.Unix()
	}
	if !c.IssuedAt.IsZero() {
		p.IssuedAt = c.IssuedAt.Unix()
	}
	return json.Marshal(p)
}

// marshalPayloadMap encodes claims using a dynamic map (fallback for Extra claims).
func marshalPayloadMap(c Claims) ([]byte, error) {
	m := make(map[string]any, 7+len(c.Extra))

	if c.Issuer != "" {
		m[ClaimIssuer] = c.Issuer
	}
	if c.Subject != "" {
		m[ClaimSubject] = c.Subject
	}
	switch len(c.Audience) {
	case 0:
		// omit
	case 1:
		m[ClaimAudience] = c.Audience[0]
	default:
		m[ClaimAudience] = c.Audience
	}
	if !c.Expiration.IsZero() {
		m[ClaimExpiration] = c.Expiration.Unix()
	}
	if !c.IssuedAt.IsZero() {
		m[ClaimIssuedAt] = c.IssuedAt.Unix()
	}
	if c.InvocationID != "" {
		m[ClaimInvocationID] = c.InvocationID
	}
	if c.ToolName != "" {
		m[ClaimToolName] = c.ToolName
	}
	if c.ParentToken != "" {
		m[ClaimParentToken] = c.ParentToken
	}
	if c.JTI != "" {
		m[ClaimJTI] = c.JTI
	}

	for k, v := range c.Extra {
		m[k] = v
	}

	return json.Marshal(m)
}

// base64url encodes src using base64 URL encoding without padding (RFC 7515 §2).
func base64url(src []byte) string {
	return base64.RawURLEncoding.EncodeToString(src)
}
