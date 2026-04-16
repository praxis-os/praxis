// SPDX-License-Identifier: Apache-2.0

package skills

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis/llm"
)

func TestNopProvider_CompletePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from nopProvider.Complete")
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "Complete") {
			t.Errorf("unexpected panic value: %v", r)
		}
	}()
	var p nopProvider
	p.Complete(context.Background(), llm.LLMRequest{}) //nolint:errcheck
}

func TestNopProvider_StreamPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from nopProvider.Stream")
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "Stream") {
			t.Errorf("unexpected panic value: %v", r)
		}
	}()
	var p nopProvider
	p.Stream(context.Background(), llm.LLMRequest{}) //nolint:errcheck
}

func TestNopProvider_Accessors(t *testing.T) {
	var p nopProvider
	if got := p.Name(); got != "nop" {
		t.Errorf("Name() = %q, want %q", got, "nop")
	}
	if p.SupportsParallelToolCalls() {
		t.Error("SupportsParallelToolCalls() = true, want false")
	}
	caps := p.Capabilities()
	if caps.MaxContextTokens != 0 || caps.SupportsStreaming {
		t.Errorf("Capabilities() = %+v, want zero value", caps)
	}
}

func TestSkillWarning_String(t *testing.T) {
	tests := []struct {
		name string
		warn SkillWarning
		want string
	}{
		{
			name: "with field",
			warn: SkillWarning{Kind: WarnExtensionField, Field: "x-custom", Message: "unknown field"},
			want: "extension_field: x-custom: unknown field",
		},
		{
			name: "without field",
			warn: SkillWarning{Kind: WarnEmptyInstructions, Message: "body is empty"},
			want: "empty_instructions: body is empty",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.warn.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
