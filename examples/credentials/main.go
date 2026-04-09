// SPDX-License-Identifier: Apache-2.0

// Package main demonstrates how to implement a custom [credentials.Resolver]
// and wire it into the praxis orchestrator.
//
// It shows two resolver implementations:
//   - EnvResolver reads credentials from environment variables
//   - StaticResolver holds a fixed in-memory mapping for tests/demos
//
// It also demonstrates the zero-on-close lifecycle: [credentials.Credential.Close]
// overwrites the backing bytes with zeros so sensitive material does not linger
// in the process heap.
//
// Usage:
//
//	go run ./examples/credentials/
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
)

// EnvResolver is a [credentials.Resolver] that reads named credentials from
// environment variables.
type EnvResolver struct{}

// Fetch retrieves the environment variable named by name.
func (EnvResolver) Fetch(_ context.Context, name string) (credentials.Credential, error) {
	val := os.Getenv(name)
	if val == "" {
		return credentials.Credential{}, fmt.Errorf("credentials: env var %q is not set", name)
	}
	b := make([]byte, len(val))
	copy(b, val)
	return credentials.Credential{Value: b}, nil
}

// StaticResolver is a [credentials.Resolver] that returns credentials from a
// fixed in-memory map. Safe for concurrent use (read-only after construction).
type StaticResolver struct {
	secrets map[string]string
}

// NewStaticResolver creates a StaticResolver with the supplied key-value pairs.
func NewStaticResolver(secrets map[string]string) StaticResolver {
	m := make(map[string]string, len(secrets))
	for k, v := range secrets {
		m[k] = v
	}
	return StaticResolver{secrets: m}
}

// Fetch returns the credential stored under name.
func (r StaticResolver) Fetch(_ context.Context, name string) (credentials.Credential, error) {
	val, ok := r.secrets[name]
	if !ok {
		return credentials.Credential{}, fmt.Errorf("credentials: %q not found", name)
	}
	b := make([]byte, len(val))
	copy(b, val)
	return credentials.Credential{Value: b}, nil
}

func main() {
	ctx := context.Background()

	// --- EnvResolver demonstration ---
	fmt.Println("=== EnvResolver ===")
	os.Setenv("DEMO_API_KEY", "sk-demo-key-abc123") //nolint:errcheck

	envResolver := EnvResolver{}
	cred, err := envResolver.Fetch(ctx, "DEMO_API_KEY")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Fetched: %d bytes\n", len(cred.Value))

	// Close zeroes the credential bytes.
	cred.Close()
	fmt.Printf("  After Close(): all zeros = %v\n", allZero(cred.Value))

	// Error path: unset variable.
	_, err = envResolver.Fetch(ctx, "UNSET_VARIABLE")
	fmt.Printf("  Unset var error: %v\n\n", err)

	// --- StaticResolver + orchestrator ---
	fmt.Println("=== StaticResolver + Orchestrator ===")

	resolver := NewStaticResolver(map[string]string{
		"provider-key": "sk-mock-key-for-demo",
	})

	// Verify fetch + close lifecycle.
	apiKey, err := resolver.Fetch(ctx, "provider-key")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Fetched provider-key: %d bytes\n", len(apiKey.Value))
	apiKey.Close()
	fmt.Printf("  After Close(): all zeros = %v\n", allZero(apiKey.Value))

	// Wire resolver into orchestrator.
	provider := mock.NewSimple("Hello from mock provider.")
	orch, err := orchestrator.New(
		provider,
		orchestrator.WithDefaultModel("demo-model"),
		orchestrator.WithCredentialResolver(resolver),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	result, err := orch.Invoke(ctx, praxis.InvocationRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("Demo credential lifecycle.")}},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Invocation state: %v\n", result.FinalState)
}

func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}
