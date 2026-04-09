// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/identity"
	"github.com/praxis-os/praxis/internal/jwt"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
)

func decodeJWTPayload(t *testing.T, token string) map[string]any {
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

func TestIdentityChaining_SignedIdentityOnResult(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := identity.NewEd25519Signer(priv)
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}

	p := mock.NewSimple("hello")
	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithIdentitySigner(signer),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Fatalf("FinalState: want Completed, got %v", result.FinalState)
	}

	// Result should carry a signed identity token.
	if result.SignedIdentity == "" {
		t.Fatal("SignedIdentity is empty, expected a signed JWT")
	}

	// Token should have 3 parts.
	parts := strings.Split(result.SignedIdentity, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts, want 3", len(parts))
	}

	// Payload should contain the invocation ID.
	payload := decodeJWTPayload(t, result.SignedIdentity)
	if _, ok := payload[jwt.ClaimInvocationID]; !ok {
		t.Error("praxis.invocation_id missing from token payload")
	}

	// No parent_token for root invocation.
	if _, ok := payload[jwt.ClaimParentToken]; ok {
		t.Error("praxis.parent_token should not be present for root invocation")
	}
}

func TestIdentityChaining_ParentTokenPassedThrough(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := identity.NewEd25519Signer(priv)
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}

	p := mock.NewSimple("hello")
	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithIdentitySigner(signer),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Simulate a nested invocation with a parent token.
	outerToken := "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.outer.sig"
	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages:    userMsg("hi"),
		ParentToken: outerToken,
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	// Token payload should contain parent_token.
	payload := decodeJWTPayload(t, result.SignedIdentity)
	if got := payload[jwt.ClaimParentToken]; got != outerToken {
		t.Errorf("praxis.parent_token = %v, want %q", got, outerToken)
	}
}

func TestIdentityChaining_InvocationIDOnResult(t *testing.T) {
	p := mock.NewSimple("hello")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if result.InvocationID == "" {
		t.Error("InvocationID is empty")
	}
	if !strings.HasPrefix(result.InvocationID, "inv-") {
		t.Errorf("InvocationID = %q, want inv- prefix", result.InvocationID)
	}
}

func TestIdentityChaining_NullSigner_EmptySignedIdentity(t *testing.T) {
	p := mock.NewSimple("hello")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	// NullSigner produces empty token.
	if result.SignedIdentity != "" {
		t.Errorf("SignedIdentity = %q, want empty for NullSigner", result.SignedIdentity)
	}
}
