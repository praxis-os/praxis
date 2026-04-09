// SPDX-License-Identifier: Apache-2.0

// Package main demonstrates Ed25519 identity signing with the praxis orchestrator.
//
// It shows how to:
//   - Generate an Ed25519 key pair
//   - Create an identity signer with options (issuer, key ID, token lifetime)
//   - Wire the signer into an orchestrator
//   - Observe signed identity tokens in invocation results
//   - Decode token claims for inspection
//
// Usage:
//
//	go run ./examples/identity/
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/identity"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
)

func main() {
	// Step 1: Generate an Ed25519 key pair.
	// In production, load the private key from a secrets manager or HSM.
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Generated Ed25519 key pair")

	// Step 2: Create an Ed25519Signer with options.
	// WithIssuer sets the "iss" claim; WithKeyID sets the JWT "kid" header;
	// WithTokenLifetime controls how long tokens are valid.
	signer, err := identity.NewEd25519Signer(privateKey,
		identity.WithIssuer("my-agent-service"),
		identity.WithKeyID("key-2026-04"),
		identity.WithTokenLifetime(30*time.Second),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created Ed25519Signer (issuer=my-agent-service, kid=key-2026-04)")

	// Step 3: Wire the signer into the orchestrator.
	// The orchestrator signs an identity token at the start of every invocation.
	provider := mock.NewSimple("The capital of France is Paris.")
	orch, err := orchestrator.New(
		provider,
		orchestrator.WithDefaultModel("demo-model"),
		orchestrator.WithIdentitySigner(signer),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Step 4: Run a mock invocation and observe the signed identity token.
	result, err := orch.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("What is the capital of France?")}},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nInvocation completed: state=%s\n", result.FinalState)
	fmt.Printf("Signed identity token:\n  %s\n", result.SignedIdentity)

	// Step 5: Decode the token payload to inspect claims.
	// This is a compact JWT (header.payload.signature). We decode the payload
	// portion to show the claims the signer embedded.
	claims, err := decodeJWTPayload(result.SignedIdentity)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decoding token: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nDecoded claims:")
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("  ", "  ")
	fmt.Print("  ")
	_ = enc.Encode(claims)

	// Step 6: Sign a token directly with custom claims.
	// The signer can also be called directly outside the orchestrator loop.
	token, err := signer.Sign(context.Background(), map[string]any{
		"custom_field": "hello-world",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nDirect Sign() token:\n  %s\n", token)
}

// decodeJWTPayload extracts and JSON-decodes the payload from a compact JWT.
func decodeJWTPayload(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected 3 JWT parts, got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return claims, nil
}
