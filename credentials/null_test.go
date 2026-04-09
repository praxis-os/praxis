// SPDX-License-Identifier: Apache-2.0

package credentials_test

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis/credentials"
)

func TestNullResolver_ImplementsResolver(_ *testing.T) {
	var _ credentials.Resolver = credentials.NullResolver{}
}

func TestCredential_Close_ZeroesValue(t *testing.T) {
	secret := []byte("api-key-secret-1234") //nolint:gosec // G101: test data
	cred := credentials.Credential{Value: secret}
	cred.Close()

	// Value must be nil after Close.
	if cred.Value != nil {
		t.Error("Value is non-nil after Close()")
	}

	// Original backing array must be zeroed.
	for i, b := range secret {
		if b != 0 {
			t.Errorf("secret[%d] = 0x%X, want 0x00 (backing array not zeroed)", i, b)
		}
	}
}

func TestCredential_Close_NilValue(t *testing.T) {
	cred := credentials.Credential{}
	// Must not panic.
	cred.Close()

	if cred.Value != nil {
		t.Error("Value is non-nil after Close() on zero Credential")
	}
}

func TestCredential_Close_Idempotent(t *testing.T) {
	cred := credentials.Credential{Value: []byte("secret")} //nolint:gosec // G101: test data
	cred.Close()
	// Second call must not panic.
	cred.Close()

	if cred.Value != nil {
		t.Error("Value is non-nil after double Close()")
	}
}

func TestNullResolver_Fetch(t *testing.T) {
	tests := []struct {
		name        string
		credName    string
		wantInError string
	}{
		{
			name:        "named credential returns error",
			credName:    "ANTHROPIC_API_KEY",   //nolint:gosec // G101: test data, not real credentials
			wantInError: "ANTHROPIC_API_KEY",
		},
		{
			name:        "empty name returns error",
			credName:    "",
			wantInError: "no resolver configured",
		},
		{
			name:        "error contains no resolver message",
			credName:    "some-secret",
			wantInError: "no resolver configured",
		},
	}

	r := credentials.NullResolver{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred, err := r.Fetch(context.Background(), tt.credName)

			if err == nil {
				t.Fatal("Fetch() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantInError) {
				t.Errorf("Fetch() error = %q, want it to contain %q", err.Error(), tt.wantInError)
			}
			if len(cred.Value) != 0 {
				t.Errorf("Fetch() returned non-empty credential value: %v", cred.Value)
			}
		})
	}
}
