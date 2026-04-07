// SPDX-License-Identifier: Apache-2.0

package credentials_test

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis/credentials"
)

func TestNullResolver_ImplementsResolver(t *testing.T) {
	var _ credentials.Resolver = credentials.NullResolver{}
}

func TestNullResolver_Fetch(t *testing.T) {
	tests := []struct {
		name        string
		credName    string
		wantInError string
	}{
		{
			name:        "named credential returns error",
			credName:    "ANTHROPIC_API_KEY",
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
