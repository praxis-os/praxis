// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"os/exec"
	"strings"
	"testing"
)

// TestInjectEnvCredentialHappyPath asserts the three core
// invariants of the helper:
//
//  1. The `envName=<token>` entry is appended to cmd.Env.
//  2. Pre-existing entries in cmd.Env are preserved in their
//     original positions.
//  3. The caller-supplied cred []byte is zeroed in place after
//     the call — the T31.3 adapter-owned buffer contract.
func TestInjectEnvCredentialHappyPath(t *testing.T) {
	t.Parallel()

	cmd := &exec.Cmd{
		Env: []string{"PRE_EXISTING=keep"},
	}
	cred := []byte("super-secret-bearer-token-xyz")
	credCopy := append([]byte(nil), cred...) // for later "was it zeroed" check

	if err := InjectEnvCredential(cmd, "MCP_TOKEN", cred); err != nil {
		t.Fatalf("InjectEnvCredential: %v", err)
	}

	// Env contains pre-existing entry + new entry, in that order.
	if len(cmd.Env) != 2 {
		t.Fatalf("len(cmd.Env) = %d, want 2", len(cmd.Env))
	}
	if cmd.Env[0] != "PRE_EXISTING=keep" {
		t.Errorf("cmd.Env[0] = %q, want %q (pre-existing entry mutated)", cmd.Env[0], "PRE_EXISTING=keep")
	}
	want := "MCP_TOKEN=" + string(credCopy)
	if cmd.Env[1] != want {
		t.Errorf("cmd.Env[1] = %q, want %q", cmd.Env[1], want)
	}

	// The caller's cred slice must be all-zero after the call.
	// This is the T31.3 adapter-owned buffer invariant.
	for i, b := range cred {
		if b != 0 {
			t.Errorf("cred[%d] = 0x%02x, want 0 (T31.3 buffer-zeroing violated)", i, b)
			break
		}
	}
}

// TestInjectEnvCredentialEmptyEnvName pins the validation rule
// that an empty envName is a typed system error.
func TestInjectEnvCredentialEmptyEnvName(t *testing.T) {
	t.Parallel()

	cmd := &exec.Cmd{}
	cred := []byte("whatever")

	if err := InjectEnvCredential(cmd, "", cred); err == nil {
		t.Fatal("expected error on empty envName, got nil")
	} else {
		assertSystemError(t, err, "credential env var name is empty")
	}
	// cred must NOT be zeroed on validation failure — the caller
	// retains the buffer for a retry or a logged error path.
	if string(cred) != "whatever" {
		t.Errorf("cred mutated on error: got %q, want %q (caller's buffer must survive validation failures)",
			string(cred), "whatever")
	}
}

// TestInjectEnvCredentialEmptyCred pins the validation rule that
// an empty cred slice is a typed system error. This case is
// unreachable from the S30 New validator (which rejects empty
// CredentialRef upstream), but the helper is defensive.
func TestInjectEnvCredentialEmptyCred(t *testing.T) {
	t.Parallel()

	cmd := &exec.Cmd{}

	if err := InjectEnvCredential(cmd, "MCP_TOKEN", nil); err == nil {
		t.Fatal("expected error on nil cred, got nil")
	} else {
		assertSystemError(t, err, "credential bytes", "empty")
	}
	if err := InjectEnvCredential(cmd, "MCP_TOKEN", []byte{}); err == nil {
		t.Fatal("expected error on empty cred, got nil")
	} else {
		assertSystemError(t, err, "credential bytes", "empty")
	}
}

// TestInjectEnvCredentialStringCopyIsIndependent pins the
// practical consequence of the Go string conversion semantics:
// after `string(cred)` materialises an immutable copy, the cred
// backing array can be mutated freely without affecting the
// string held by cmd.Env. This is what makes the inline zeroing
// correct (if strings shared the backing array, zeroing cred
// would corrupt cmd.Env).
//
// The test builds the env entry manually, mutates cred, and
// verifies the env entry is unchanged. If this assertion ever
// fires, it means the Go runtime has changed its string
// conversion semantics in a way that would invalidate the
// OI-MCP-1 residual risk analysis.
func TestInjectEnvCredentialStringCopyIsIndependent(t *testing.T) {
	t.Parallel()

	cred := []byte("original-secret")
	// Capture what InjectEnvCredential's conversion would produce.
	capturedEntry := "MCP_TOKEN=" + string(cred)
	// Manually mutate cred; credentials.ZeroBytes would do the same.
	for i := range cred {
		cred[i] = 0
	}
	// The captured string must be untouched.
	if !strings.HasSuffix(capturedEntry, "original-secret") {
		t.Errorf("captured entry = %q; Go string conversion appears to share backing array with source []byte", capturedEntry)
	}
}
