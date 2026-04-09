// SPDX-License-Identifier: Apache-2.0

package jwt

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// decodeToken is a test helper that splits a compact JWT and base64url-decodes
// header and payload into maps. It does NOT verify the signature — that is
// done separately in TestEncode_SignatureVerification.
func decodeToken(t *testing.T, token string) (header, payload map[string]any) {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}

	rawHeader, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if err := json.Unmarshal(rawHeader, &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	return header, payload
}

// generateKey is a test helper that creates a fresh Ed25519 key pair or fails
// the test immediately.
func generateKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	return pub, priv
}

// TestEncode_Header verifies that every token produced by Encode carries the
// fixed header {"alg":"EdDSA","typ":"JWT"}.
func TestEncode_Header(t *testing.T) {
	_, priv := generateKey(t)

	token, err := Encode(Claims{Issuer: "test"}, priv, "")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	header, _ := decodeToken(t, token)

	if got := header["alg"]; got != "EdDSA" {
		t.Errorf("alg = %q, want %q", got, "EdDSA")
	}
	if got := header["typ"]; got != "JWT" {
		t.Errorf("typ = %q, want %q", got, "JWT")
	}
	if len(header) != 2 {
		t.Errorf("header has %d keys, want exactly 2", len(header))
	}
}

// TestEncode_HeaderWithKeyID verifies that a non-empty keyID adds a "kid"
// field to the JOSE header.
func TestEncode_HeaderWithKeyID(t *testing.T) {
	_, priv := generateKey(t)

	token, err := Encode(Claims{Issuer: "test"}, priv, "my-key-id")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	header, _ := decodeToken(t, token)

	if got := header["alg"]; got != "EdDSA" {
		t.Errorf("alg = %q, want %q", got, "EdDSA")
	}
	if got := header["typ"]; got != "JWT" {
		t.Errorf("typ = %q, want %q", got, "JWT")
	}
	if got := header["kid"]; got != "my-key-id" {
		t.Errorf("kid = %q, want %q", got, "my-key-id")
	}
	if len(header) != 3 {
		t.Errorf("header has %d keys, want exactly 3", len(header))
	}
}

// TestEncode_FixedHeaderReused verifies that consecutive calls produce the
// same base64url header segment — confirming the pre-encoded fixedHeader is
// stable.
func TestEncode_FixedHeaderReused(t *testing.T) {
	_, priv := generateKey(t)

	token1, _ := Encode(Claims{Issuer: "a"}, priv, "")
	token2, _ := Encode(Claims{Issuer: "b"}, priv, "")

	h1 := strings.Split(token1, ".")[0]
	h2 := strings.Split(token2, ".")[0]

	if h1 != h2 {
		t.Errorf("header segment changed between calls: %q vs %q", h1, h2)
	}
}

// TestEncode_SignatureVerification verifies that the signature in the produced
// token is valid under the corresponding public key.
func TestEncode_SignatureVerification(t *testing.T) {
	pub, priv := generateKey(t)

	claims := Claims{
		Issuer:       "praxis",
		Subject:      "inv-001",
		InvocationID: "inv-001",
	}
	token, err := Encode(claims, priv, "")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	parts := strings.Split(token, ".")
	signingInput := parts[0] + "." + parts[1]

	rawSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}

	if !ed25519.Verify(pub, []byte(signingInput), rawSig) {
		t.Error("signature verification failed")
	}
}

// TestEncode_RegisteredClaims verifies that all five registered claims are
// serialised with their correct RFC 7519 claim names and values.
func TestEncode_RegisteredClaims(t *testing.T) {
	_, priv := generateKey(t)

	now := time.Unix(1700000000, 0)
	exp := now.Add(time.Hour)

	claims := Claims{
		Issuer:     "issuer-val",
		Subject:    "subject-val",
		Audience:   []string{"audience-val"},
		IssuedAt:   now,
		Expiration: exp,
	}

	token, err := Encode(claims, priv, "")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	_, payload := decodeToken(t, token)

	tests := []struct {
		claim string
		want  any
	}{
		{ClaimIssuer, "issuer-val"},
		{ClaimSubject, "subject-val"},
		{ClaimAudience, "audience-val"},
		// JSON numbers unmarshal as float64.
		{ClaimIssuedAt, float64(now.Unix())},
		{ClaimExpiration, float64(exp.Unix())},
	}

	for _, tc := range tests {
		t.Run(tc.claim, func(t *testing.T) {
			got, ok := payload[tc.claim]
			if !ok {
				t.Fatalf("claim %q missing from payload", tc.claim)
			}
			if got != tc.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tc.want, tc.want)
			}
		})
	}
}

// TestEncode_MultiAudience verifies that a multi-element audience is
// serialised as a JSON array.
func TestEncode_MultiAudience(t *testing.T) {
	_, priv := generateKey(t)

	claims := Claims{Audience: []string{"svc-a", "svc-b", "svc-c"}}
	token, err := Encode(claims, priv, "")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	_, payload := decodeToken(t, token)

	raw, ok := payload[ClaimAudience]
	if !ok {
		t.Fatal("aud claim missing")
	}

	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("aud: got %T, want []any", raw)
	}
	if len(arr) != 3 {
		t.Fatalf("aud length = %d, want 3", len(arr))
	}

	want := []string{"svc-a", "svc-b", "svc-c"}
	for i, w := range want {
		if arr[i] != w {
			t.Errorf("aud[%d] = %v, want %v", i, arr[i], w)
		}
	}
}

// TestEncode_PraxisClaims verifies that InvocationID and ParentToken are
// serialised under their praxis-namespaced claim names.
func TestEncode_PraxisClaims(t *testing.T) {
	_, priv := generateKey(t)

	claims := Claims{
		InvocationID: "inv-abc123",
		ToolName:     "tools.Search",
		ParentToken:  "parent.jwt.token",
	}
	token, err := Encode(claims, priv, "")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	_, payload := decodeToken(t, token)

	if got := payload[ClaimInvocationID]; got != "inv-abc123" {
		t.Errorf("%s = %v, want %q", ClaimInvocationID, got, "inv-abc123")
	}
	if got := payload[ClaimToolName]; got != "tools.Search" {
		t.Errorf("%s = %v, want %q", ClaimToolName, got, "tools.Search")
	}
	if got := payload[ClaimParentToken]; got != "parent.jwt.token" {
		t.Errorf("%s = %v, want %q", ClaimParentToken, got, "parent.jwt.token")
	}
}

// TestEncode_ExtraClaimsMerged verifies that Extra claims appear in the
// payload and that Extra keys overwrite named fields on collision.
func TestEncode_ExtraClaimsMerged(t *testing.T) {
	_, priv := generateKey(t)

	claims := Claims{
		Issuer: "original-issuer",
		Extra: map[string]any{
			"custom.claim": "custom-value",
			ClaimIssuer:    "overridden-issuer", // collision: Extra wins
		},
	}

	token, err := Encode(claims, priv, "")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	_, payload := decodeToken(t, token)

	if got := payload["custom.claim"]; got != "custom-value" {
		t.Errorf("custom.claim = %v, want %q", got, "custom-value")
	}
	if got := payload[ClaimIssuer]; got != "overridden-issuer" {
		t.Errorf("iss = %v, want %q (Extra should win on collision)", got, "overridden-issuer")
	}
}

// TestEncode_EmptyClaims verifies that Encode succeeds with a zero-value
// Claims struct and produces a valid three-part JWT.
func TestEncode_EmptyClaims(t *testing.T) {
	_, priv := generateKey(t)

	token, err := Encode(Claims{}, priv, "")
	if err != nil {
		t.Fatalf("Encode with empty claims: %v", err)
	}

	if parts := strings.Split(token, "."); len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}

	_, payload := decodeToken(t, token)

	// With all zero-value fields and no Extra, the payload should be empty.
	if len(payload) != 0 {
		t.Errorf("empty Claims produced non-empty payload: %v", payload)
	}
}

// TestEncode_OmitsZeroTimeFields verifies that a zero-value time.Time for
// Expiration and IssuedAt does not appear in the payload.
func TestEncode_OmitsZeroTimeFields(t *testing.T) {
	_, priv := generateKey(t)

	claims := Claims{Issuer: "test"} // Expiration and IssuedAt are zero
	token, err := Encode(claims, priv, "")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	_, payload := decodeToken(t, token)

	if _, ok := payload[ClaimExpiration]; ok {
		t.Error("exp claim present for zero Expiration, want omitted")
	}
	if _, ok := payload[ClaimIssuedAt]; ok {
		t.Error("iat claim present for zero IssuedAt, want omitted")
	}
}

// TestEncode_OmitsEmptyStringFields verifies that empty-string fields (Issuer,
// Subject, InvocationID, ParentToken) are omitted from the payload.
func TestEncode_OmitsEmptyStringFields(t *testing.T) {
	_, priv := generateKey(t)

	claims := Claims{
		IssuedAt: time.Unix(1700000000, 0), // one non-empty field to confirm payload is non-trivial
	}
	token, err := Encode(claims, priv, "")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	_, payload := decodeToken(t, token)

	for _, name := range []string{ClaimIssuer, ClaimSubject, ClaimAudience, ClaimInvocationID, ClaimToolName, ClaimParentToken} {
		if _, ok := payload[name]; ok {
			t.Errorf("claim %q present for zero/empty value, want omitted", name)
		}
	}
}

// TestEncode_NoPadding verifies that no base64 padding characters ('=') appear
// anywhere in the token, per RFC 7515 §2.
func TestEncode_NoPadding(t *testing.T) {
	_, priv := generateKey(t)

	token, err := Encode(Claims{Issuer: "praxis", Subject: "test"}, priv, "")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	if strings.Contains(token, "=") {
		t.Errorf("token contains padding '=': %q", token)
	}
}

// TestEncode_DifferentKeysProduceDifferentSignatures verifies that two
// different private keys produce different signature segments for the same
// claims.
func TestEncode_DifferentKeysProduceDifferentSignatures(t *testing.T) {
	_, priv1 := generateKey(t)
	_, priv2 := generateKey(t)

	claims := Claims{Issuer: "same", Subject: "same"}

	t1, err := Encode(claims, priv1, "")
	if err != nil {
		t.Fatalf("Encode key1: %v", err)
	}
	t2, err := Encode(claims, priv2, "")
	if err != nil {
		t.Fatalf("Encode key2: %v", err)
	}

	sig1 := strings.Split(t1, ".")[2]
	sig2 := strings.Split(t2, ".")[2]

	if sig1 == sig2 {
		t.Error("different keys produced identical signatures")
	}
}

// TestEncode_SignatureInvalidWithWrongKey verifies that verifying the signature
// with a different public key fails.
func TestEncode_SignatureInvalidWithWrongKey(t *testing.T) {
	_, priv := generateKey(t)
	wrongPub, _ := generateKey(t)

	token, err := Encode(Claims{Issuer: "praxis"}, priv, "")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	parts := strings.Split(token, ".")
	rawSig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	signingInput := parts[0] + "." + parts[1]

	if ed25519.Verify(wrongPub, []byte(signingInput), rawSig) {
		t.Error("signature verified with wrong public key — should have failed")
	}
}

// TestClaimConstants verifies the exact string values of exported claim
// constants so a refactor cannot silently change wire-format names.
func TestClaimConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"ClaimIssuer", "iss"},
		{"ClaimSubject", "sub"},
		{"ClaimAudience", "aud"},
		{"ClaimExpiration", "exp"},
		{"ClaimIssuedAt", "iat"},
		{"ClaimInvocationID", "praxis.invocation_id"},
		{"ClaimToolName", "praxis.tool_name"},
		{"ClaimParentToken", "praxis.parent_token"},
	}
	actuals := map[string]string{
		"ClaimIssuer":       ClaimIssuer,
		"ClaimSubject":      ClaimSubject,
		"ClaimAudience":     ClaimAudience,
		"ClaimExpiration":   ClaimExpiration,
		"ClaimIssuedAt":     ClaimIssuedAt,
		"ClaimInvocationID": ClaimInvocationID,
		"ClaimToolName":     ClaimToolName,
		"ClaimParentToken":  ClaimParentToken,
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := actuals[tc.name]; got != tc.value {
				t.Errorf("%s = %q, want %q", tc.name, got, tc.value)
			}
		})
	}
}

// BenchmarkEncode measures the cost of a full Encode call with a realistic
// claims set. Run with: go test -bench=. -benchmem ./internal/jwt/
func BenchmarkEncode(b *testing.B) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		b.Fatalf("generate key: %v", err)
	}

	now := time.Now()
	claims := Claims{
		Issuer:       "praxis",
		Subject:      "inv-bench-001",
		Audience:     []string{"downstream-svc"},
		IssuedAt:     now,
		Expiration:   now.Add(5 * time.Minute),
		InvocationID: "bench-invocation-id-0001",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := Encode(claims, priv, ""); err != nil {
			b.Fatalf("Encode: %v", err)
		}
	}
}
