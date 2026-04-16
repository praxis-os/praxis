// SPDX-License-Identifier: Apache-2.0

package resolve_test

import (
	"os"
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
	dir := t.TempDir()

	gotDir, gotFile, err := resolve.ResolvePath(dir)
	if err != nil {
		t.Fatalf("ResolvePath(%q): %v", dir, err)
	}
	if gotDir != dir {
		t.Errorf("dir = %q, want %q", gotDir, dir)
	}
	wantFile := dir + "/" + resolve.SkillFileName
	if gotFile != wantFile {
		t.Errorf("file = %q, want %q", gotFile, wantFile)
	}
}

func TestResolvePath_File(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/SKILL.md"
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	gotDir, gotFile, err := resolve.ResolvePath(file)
	if err != nil {
		t.Fatalf("ResolvePath(%q): %v", file, err)
	}
	if gotDir != dir {
		t.Errorf("dir = %q, want %q", gotDir, dir)
	}
	if gotFile != file {
		t.Errorf("file = %q, want %q", gotFile, file)
	}
}

func TestResolvePath_NotExist(t *testing.T) {
	_, _, err := resolve.ResolvePath("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestEscapeError_Message(t *testing.T) {
	e := &resolve.EscapeError{Path: "../bad", Reason: "dot-dot not allowed"}
	got := e.Error()
	if got != "path escape: ../bad: dot-dot not allowed" {
		t.Errorf("Error(): got %q", got)
	}
}
