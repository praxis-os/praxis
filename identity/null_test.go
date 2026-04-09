// SPDX-License-Identifier: Apache-2.0

package identity_test

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/identity"
)

func TestNullSigner_ImplementsSigner(_ *testing.T) {
	var _ identity.Signer = identity.NullSigner{}
}

func TestNullSigner_Sign(t *testing.T) {
	tests := []struct {
		claims map[string]any
		name   string
	}{
		{
			name:   "nil claims",
			claims: nil,
		},
		{
			name:   "empty claims",
			claims: map[string]any{},
		},
		{
			name: "populated claims",
			claims: map[string]any{
				"sub": "agent-1",
				"iss": "praxis",
				"iat": int64(1_700_000_000),
			},
		},
	}

	s := identity.NullSigner{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := s.Sign(context.Background(), tt.claims)

			if err != nil {
				t.Errorf("Sign() unexpected error: %v", err)
			}
			if token != "" {
				t.Errorf("Sign() = %q, want empty string", token)
			}
		})
	}
}
