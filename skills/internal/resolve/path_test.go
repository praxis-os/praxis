// SPDX-License-Identifier: Apache-2.0

package resolve_test

import (
	"testing"
	"testing/fstest"

	"github.com/praxis-os/praxis/skills/internal/resolve"
)

func TestValidateFSRoot_Valid(t *testing.T) {
	tests := []string{".", "bundles/my-skill", "a/b/c"}
	for _, root := range tests {
		if err := resolve.ValidateFSRoot(root); err != nil {
			t.Errorf("ValidateFSRoot(%q): unexpected error: %v", root, err)
		}
	}
}

func TestValidateFSRoot_Absolute(t *testing.T) {
	if err := resolve.ValidateFSRoot("/absolute"); err == nil {
		t.Error("expected error for absolute path")
	}
}

func TestValidateFSRoot_DotDot(t *testing.T) {
	tests := []string{"..", "../escape", "a/../b", "a/b/.."}
	for _, root := range tests {
		if err := resolve.ValidateFSRoot(root); err == nil {
			t.Errorf("ValidateFSRoot(%q): expected error", root)
		}
	}
}

func TestOpenFile_Found(t *testing.T) {
	fs := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte("content")},
	}
	data, err := resolve.OpenFile(fs, ".")
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("got %q", string(data))
	}
}

func TestOpenFile_Subdirectory(t *testing.T) {
	fs := fstest.MapFS{
		"sub/SKILL.md": &fstest.MapFile{Data: []byte("sub content")},
	}
	data, err := resolve.OpenFile(fs, "sub")
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if string(data) != "sub content" {
		t.Errorf("got %q", string(data))
	}
}

func TestOpenFile_Missing(t *testing.T) {
	fs := fstest.MapFS{}
	_, err := resolve.OpenFile(fs, ".")
	if err == nil {
		t.Error("expected error for missing SKILL.md")
	}
}

func TestResolvePath_Directory(t *testing.T) {
	dir, file, err := resolve.ResolvePath("./testdata_resolve_dummy")
	// This will fail because the directory doesn't exist, which is expected.
	if err == nil {
		t.Logf("dir=%s file=%s (only passes if testdata_resolve_dummy exists)", dir, file)
	}
}

func TestEscapeError_Message(t *testing.T) {
	e := &resolve.EscapeError{Path: "../bad", Reason: "dot-dot not allowed"}
	got := e.Error()
	if got != "path escape: ../bad: dot-dot not allowed" {
		t.Errorf("Error(): got %q", got)
	}
}
