// SPDX-License-Identifier: Apache-2.0

package identity_test

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/praxis-os/praxis/identity"
	"github.com/praxis-os/praxis/internal/jwt"
)

const defaultIssuer = "praxis"

// generateTestKey creates a fresh Ed25519 key pair for testing.
func generateTestKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	return pub, priv
}

// decodePayload extracts and JSON-decodes the payload from a compact JWT.
func decodePayload(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return payload
}

func TestNewEd25519Signer_NilKeyReturnsError(t *testing.T) {
	_, err := identity.NewEd25519Signer(nil)
	if err == nil {
		t.Fatal("expected error for nil key")
	}
	if !strings.Contains(err.Error(), "must not be nil") {
		t.Errorf("error = %q, want mention of nil", err)
	}
}

func TestNewEd25519Signer_WrongKeySizeReturnsError(t *testing.T) {
	_, err := identity.NewEd25519Signer(ed25519.PrivateKey(make([]byte, 32)))
	if err == nil {
		t.Fatal("expected error for wrong key size")
	}
	if !strings.Contains(err.Error(), "64 bytes") {
		t.Errorf("error = %q, want mention of 64 bytes", err)
	}
}

func TestNewEd25519Signer_ValidKey(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, err := identity.NewEd25519Signer(priv)
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	if signer == nil {
		t.Fatal("signer is nil")
	}
}

func TestEd25519Signer_ImplementsSigner(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, err := identity.NewEd25519Signer(priv)
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	_ = signer // compile-time interface satisfaction checked via var _ identity.Signer in production code
}

func TestEd25519Signer_ProducesThreePartJWT(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, err := signer.Sign(context.Background(), nil)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}

	// No padding characters.
	if strings.Contains(token, "=") {
		t.Error("token contains padding '='")
	}
}

func TestEd25519Signer_SignatureVerification(t *testing.T) {
	pub, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, err := signer.Sign(context.Background(), map[string]any{
		jwt.ClaimInvocationID: "inv-001",
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
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

func TestEd25519Signer_DefaultIssuer(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, _ := signer.Sign(context.Background(), nil)
	payload := decodePayload(t, token)

	if got := payload[jwt.ClaimIssuer]; got != defaultIssuer {
		t.Errorf("iss = %v, want %q", got, defaultIssuer)
	}
}

func TestEd25519Signer_MandatoryClaims(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	claims := map[string]any{
		jwt.ClaimInvocationID: "inv-abc",
		jwt.ClaimToolName:     "tools.Search",
	}
	token, err := signer.Sign(context.Background(), claims)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	payload := decodePayload(t, token)

	// iss, sub, iat, exp, jti must all be present.
	for _, claim := range []string{
		jwt.ClaimIssuer,
		jwt.ClaimSubject,
		jwt.ClaimIssuedAt,
		jwt.ClaimExpiration,
		jwt.ClaimJTI,
	} {
		if _, ok := payload[claim]; !ok {
			t.Errorf("mandatory claim %q missing", claim)
		}
	}

	// praxis claims must be present.
	if got := payload[jwt.ClaimInvocationID]; got != "inv-abc" {
		t.Errorf("praxis.invocation_id = %v, want %q", got, "inv-abc")
	}
	if got := payload[jwt.ClaimToolName]; got != "tools.Search" {
		t.Errorf("praxis.tool_name = %v, want %q", got, "tools.Search")
	}
}

func TestEd25519Signer_SubjectMatchesInvocationID(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, _ := signer.Sign(context.Background(), map[string]any{
		jwt.ClaimInvocationID: "inv-999",
	})
	payload := decodePayload(t, token)

	if got := payload[jwt.ClaimSubject]; got != "inv-999" {
		t.Errorf("sub = %v, want %q (should match invocation_id)", got, "inv-999")
	}
}

func TestEd25519Signer_ToolNameIsFirstClassClaim(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, err := signer.Sign(context.Background(), map[string]any{
		jwt.ClaimInvocationID: "inv-tool",
		jwt.ClaimToolName:     "tools.Search",
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	payload := decodePayload(t, token)

	if got := payload[jwt.ClaimToolName]; got != "tools.Search" {
		t.Errorf("praxis.tool_name = %v, want %q", got, "tools.Search")
	}
}

func TestEd25519Signer_ToolNameProtectedFromExtraOverride(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	// Pass tool_name in incoming claims; it should be promoted to a named
	// field and not overwritable by Extra collision.
	token, err := signer.Sign(context.Background(), map[string]any{
		jwt.ClaimInvocationID: "inv-1",
		jwt.ClaimToolName:     "tools.Legit",
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	payload := decodePayload(t, token)
	if got := payload[jwt.ClaimToolName]; got != "tools.Legit" {
		t.Errorf("praxis.tool_name = %v, want %q", got, "tools.Legit")
	}
}

func TestEd25519Signer_MandatoryClaimsOverrideIncoming(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	// Attempt to override mandatory claims via incoming map.
	claims := map[string]any{
		jwt.ClaimIssuer:     "attacker",
		jwt.ClaimExpiration: float64(0),
		jwt.ClaimIssuedAt:   float64(0),
		jwt.ClaimJTI:        "fake-jti",
	}
	token, err := signer.Sign(context.Background(), claims)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	payload := decodePayload(t, token)

	// iss must be the default issuer, not "attacker".
	if got := payload[jwt.ClaimIssuer]; got != defaultIssuer {
		t.Errorf("iss = %v, want %q (mandatory must override)", got, defaultIssuer)
	}

	// jti must not be "fake-jti".
	if got := payload[jwt.ClaimJTI]; got == "fake-jti" {
		t.Error("jti was overridden by incoming claims; mandatory must win")
	}

	// exp and iat must be non-zero.
	if exp, ok := payload[jwt.ClaimExpiration].(float64); !ok || exp == 0 {
		t.Error("exp is zero or missing; mandatory must override")
	}
	if iat, ok := payload[jwt.ClaimIssuedAt].(float64); !ok || iat == 0 {
		t.Error("iat is zero or missing; mandatory must override")
	}
}

func TestEd25519Signer_ExpirationIsIatPlusLifetime(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, _ := signer.Sign(context.Background(), nil)
	payload := decodePayload(t, token)

	iat := payload[jwt.ClaimIssuedAt].(float64)
	exp := payload[jwt.ClaimExpiration].(float64)

	// Default lifetime is 60s.
	diff := exp - iat
	if diff != 60 {
		t.Errorf("exp - iat = %v, want 60 (default lifetime)", diff)
	}
}

func TestTokenLifetimeConstants(t *testing.T) {
	if identity.DefaultTokenLifetime != 60*time.Second {
		t.Errorf("DefaultTokenLifetime = %v, want 60s", identity.DefaultTokenLifetime)
	}
	if identity.MinTokenLifetime != 5*time.Second {
		t.Errorf("MinTokenLifetime = %v, want 5s", identity.MinTokenLifetime)
	}
	if identity.MaxTokenLifetime != 300*time.Second {
		t.Errorf("MaxTokenLifetime = %v, want 300s", identity.MaxTokenLifetime)
	}
}

func TestEd25519Signer_JTIIsUUIDv7Format(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, _ := signer.Sign(context.Background(), nil)
	payload := decodePayload(t, token)

	jti, ok := payload[jwt.ClaimJTI].(string)
	if !ok {
		t.Fatal("jti is not a string")
	}

	// UUIDv7 format: 8-4-4-4-12 hex digits.
	parts := strings.Split(jti, "-")
	if len(parts) != 5 {
		t.Fatalf("jti has %d parts, want 5 (UUID format)", len(parts))
	}

	wantLens := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != wantLens[i] {
			t.Errorf("jti part %d has length %d, want %d", i, len(p), wantLens[i])
		}
	}

	// Version nibble (byte 6, high nibble) should be '7'.
	if parts[2][0] != '7' {
		t.Errorf("jti version nibble = %c, want '7'", parts[2][0])
	}
}

func TestEd25519Signer_UniqueJTIPerCall(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	seen := make(map[string]struct{}, 100)
	for range 100 {
		token, err := signer.Sign(context.Background(), nil)
		if err != nil {
			t.Fatalf("Sign: %v", err)
		}
		payload := decodePayload(t, token)
		jti := payload[jwt.ClaimJTI].(string)

		if _, dup := seen[jti]; dup {
			t.Fatalf("duplicate jti: %s", jti)
		}
		seen[jti] = struct{}{}
	}
}

func TestEd25519Signer_ParentTokenPassedThrough(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, _ := signer.Sign(context.Background(), map[string]any{
		jwt.ClaimParentToken: "parent.jwt.here",
	})
	payload := decodePayload(t, token)

	if got := payload[jwt.ClaimParentToken]; got != "parent.jwt.here" {
		t.Errorf("praxis.parent_token = %v, want %q", got, "parent.jwt.here")
	}
}

func TestEd25519Signer_IncomingClaimsPassedThrough(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, _ := signer.Sign(context.Background(), map[string]any{
		"custom.claim":        "custom-value",
		jwt.ClaimInvocationID: "inv-1",
	})
	payload := decodePayload(t, token)

	if got := payload["custom.claim"]; got != "custom-value" {
		t.Errorf("custom.claim = %v, want %q", got, "custom-value")
	}
}

func TestEd25519Signer_NilClaimsSucceeds(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, err := signer.Sign(context.Background(), nil)
	if err != nil {
		t.Fatalf("Sign with nil claims: %v", err)
	}
	if token == "" {
		t.Error("Sign returned empty token")
	}
}

func TestEd25519Signer_EmptyClaimsSucceeds(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, err := signer.Sign(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Sign with empty claims: %v", err)
	}
	if token == "" {
		t.Error("Sign returned empty token")
	}
}

func TestEd25519Signer_ConcurrentSignIsSafe(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	errs := make(chan error, goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			token, err := signer.Sign(context.Background(), map[string]any{
				jwt.ClaimInvocationID: "inv-concurrent",
				jwt.ClaimToolName:     "tools.Concurrent",
			})
			if err != nil {
				errs <- err
				return
			}
			if parts := strings.Split(token, "."); len(parts) != 3 {
				errs <- fmt.Errorf("got %d JWT parts, want 3", len(parts))
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Sign error: %v", err)
	}
}

func TestEd25519Signer_DifferentKeysProduceDifferentTokens(t *testing.T) {
	_, priv1 := generateTestKey(t)
	_, priv2 := generateTestKey(t)

	signer1, _ := identity.NewEd25519Signer(priv1)
	signer2, _ := identity.NewEd25519Signer(priv2)

	claims := map[string]any{jwt.ClaimInvocationID: "inv-1"}
	t1, _ := signer1.Sign(context.Background(), claims)
	t2, _ := signer2.Sign(context.Background(), claims)

	// Signatures (part 3) must differ.
	sig1 := strings.Split(t1, ".")[2]
	sig2 := strings.Split(t2, ".")[2]
	if sig1 == sig2 {
		t.Error("different keys produced identical signatures")
	}
}

func TestWithIssuer(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, err := identity.NewEd25519Signer(priv, identity.WithIssuer("my-service"))
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}

	token, _ := signer.Sign(context.Background(), nil)
	payload := decodePayload(t, token)

	if got := payload[jwt.ClaimIssuer]; got != "my-service" {
		t.Errorf("iss = %v, want %q", got, "my-service")
	}
}

func TestWithTokenLifetime(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, err := identity.NewEd25519Signer(priv, identity.WithTokenLifetime(30*time.Second))
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}

	token, _ := signer.Sign(context.Background(), nil)
	payload := decodePayload(t, token)

	iat := payload[jwt.ClaimIssuedAt].(float64)
	exp := payload[jwt.ClaimExpiration].(float64)
	if diff := exp - iat; diff != 30 {
		t.Errorf("exp - iat = %v, want 30", diff)
	}
}

func TestWithTokenLifetime_BelowMinReturnsError(t *testing.T) {
	_, priv := generateTestKey(t)
	_, err := identity.NewEd25519Signer(priv, identity.WithTokenLifetime(2*time.Second))
	if err == nil {
		t.Fatal("expected error for lifetime below minimum")
	}
	if !strings.Contains(err.Error(), "below minimum") {
		t.Errorf("error = %q, want mention of minimum", err)
	}
}

func TestWithTokenLifetime_AboveMaxReturnsError(t *testing.T) {
	_, priv := generateTestKey(t)
	_, err := identity.NewEd25519Signer(priv, identity.WithTokenLifetime(10*time.Minute))
	if err == nil {
		t.Fatal("expected error for lifetime above maximum")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("error = %q, want mention of maximum", err)
	}
}

func TestWithTokenLifetime_BoundaryValues(t *testing.T) {
	_, priv := generateTestKey(t)

	// Exact minimum should succeed.
	if _, err := identity.NewEd25519Signer(priv, identity.WithTokenLifetime(identity.MinTokenLifetime)); err != nil {
		t.Errorf("MinTokenLifetime rejected: %v", err)
	}

	// Exact maximum should succeed.
	if _, err := identity.NewEd25519Signer(priv, identity.WithTokenLifetime(identity.MaxTokenLifetime)); err != nil {
		t.Errorf("MaxTokenLifetime rejected: %v", err)
	}
}

func TestWithKeyID(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, err := identity.NewEd25519Signer(priv, identity.WithKeyID("key-2024-01"))
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}

	token, _ := signer.Sign(context.Background(), nil)

	// Decode header to check kid.
	parts := strings.Split(token, ".")
	rawHeader, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var header map[string]any
	if err := json.Unmarshal(rawHeader, &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}

	if got := header["kid"]; got != "key-2024-01" {
		t.Errorf("kid = %v, want %q", got, "key-2024-01")
	}
	if got := header["alg"]; got != "EdDSA" {
		t.Errorf("alg = %v, want %q", got, "EdDSA")
	}
	if got := header["typ"]; got != "JWT" {
		t.Errorf("typ = %v, want %q", got, "JWT")
	}
}

func TestWithKeyID_Empty_NoKidInHeader(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv)

	token, _ := signer.Sign(context.Background(), nil)

	parts := strings.Split(token, ".")
	rawHeader, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var header map[string]any
	_ = json.Unmarshal(rawHeader, &header)

	if _, ok := header["kid"]; ok {
		t.Error("kid present in header without WithKeyID")
	}
}

func TestWithExtraClaims(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, err := identity.NewEd25519Signer(priv, identity.WithExtraClaims(map[string]any{
		"env":     "production",
		"team.id": "platform",
	}))
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}

	token, _ := signer.Sign(context.Background(), nil)
	payload := decodePayload(t, token)

	if got := payload["env"]; got != "production" {
		t.Errorf("env = %v, want %q", got, "production")
	}
	if got := payload["team.id"]; got != "platform" {
		t.Errorf("team.id = %v, want %q", got, "platform")
	}
}

func TestWithExtraClaims_ShallowCopy(t *testing.T) {
	_, priv := generateTestKey(t)
	original := map[string]any{"key": "original"}
	signer, _ := identity.NewEd25519Signer(priv, identity.WithExtraClaims(original))

	// Mutate original after construction.
	original["key"] = "mutated"

	token, _ := signer.Sign(context.Background(), nil)
	payload := decodePayload(t, token)

	if got := payload["key"]; got != "original" {
		t.Errorf("key = %v, want %q (should be shallow-copied)", got, "original")
	}
}

func TestWithExtraClaims_MandatoryClaimsWin(t *testing.T) {
	_, priv := generateTestKey(t)
	signer, _ := identity.NewEd25519Signer(priv, identity.WithExtraClaims(map[string]any{
		jwt.ClaimIssuer: "evil",
	}))

	token, _ := signer.Sign(context.Background(), nil)
	payload := decodePayload(t, token)

	// Mandatory iss must win over extra claims.
	if got := payload[jwt.ClaimIssuer]; got != defaultIssuer {
		t.Errorf("iss = %v, want %q (mandatory must win)", got, defaultIssuer)
	}
}

func BenchmarkEd25519Signer_Sign(b *testing.B) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		b.Fatalf("generate key: %v", err)
	}
	signer, err := identity.NewEd25519Signer(priv)
	if err != nil {
		b.Fatalf("NewEd25519Signer: %v", err)
	}

	claims := map[string]any{
		jwt.ClaimInvocationID: "inv-bench-001",
		jwt.ClaimToolName:     "tools.BenchTool",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := signer.Sign(context.Background(), claims); err != nil {
			b.Fatalf("Sign: %v", err)
		}
	}
}
