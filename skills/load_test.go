// SPDX-License-Identifier: Apache-2.0

package skills_test

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/skills"
)

func TestOpen_ValidBundle(t *testing.T) {
	fs := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte(`---
name: test-skill
description: A test skill.
license: MIT
---

Be helpful.
`)},
	}

	sk, warnings, err := skills.Open(fs, ".")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if sk.Name() != "test-skill" {
		t.Errorf("Name: got %q, want %q", sk.Name(), "test-skill")
	}
	if sk.Description() != "A test skill." {
		t.Errorf("Description: got %q, want %q", sk.Description(), "A test skill.")
	}
	if sk.License() != "MIT" {
		t.Errorf("License: got %q, want %q", sk.License(), "MIT")
	}
	if sk.Instructions() == "" {
		t.Error("Instructions should not be empty")
	}
	if !strings.HasPrefix(sk.Instructions(), "Be helpful.") {
		t.Errorf("Instructions: got %q, want prefix %q", sk.Instructions(), "Be helpful.")
	}
}

func TestOpen_SubdirectoryRoot(t *testing.T) {
	fs := fstest.MapFS{
		"bundles/my-skill/SKILL.md": &fstest.MapFile{Data: []byte(`---
name: sub-skill
description: In subdirectory.
---

Do stuff.
`)},
	}

	sk, _, err := skills.Open(fs, "bundles/my-skill")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if sk.Name() != "sub-skill" {
		t.Errorf("Name: got %q, want %q", sk.Name(), "sub-skill")
	}
}

func TestOpen_AllOptionalFields(t *testing.T) {
	fs := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte(`---
name: full-skill
description: All fields present.
license: Apache-2.0
compatibility: claude-sonnet-4-6
metadata:
  author: test
  version: "1.0"
allowed-tools:
  - read_file
  - write_file
---

Instructions here.
`)},
	}

	sk, warnings, err := skills.Open(fs, ".")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if sk.License() != "Apache-2.0" {
		t.Errorf("License: got %q", sk.License())
	}
	if sk.Compatibility() != "claude-sonnet-4-6" {
		t.Errorf("Compatibility: got %q", sk.Compatibility())
	}
	if sk.Metadata() == nil {
		t.Fatal("Metadata: got nil")
	}
	if sk.Metadata()["author"] != "test" {
		t.Errorf("Metadata[author]: got %v", sk.Metadata()["author"])
	}
	tools := sk.AllowedTools()
	if len(tools) != 2 || tools[0] != "read_file" || tools[1] != "write_file" {
		t.Errorf("AllowedTools: got %v", tools)
	}
}

func TestOpen_ExtensionFields(t *testing.T) {
	fs := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte(`---
name: ext-skill
description: Has extension fields.
version: "2.0"
custom_field: custom_value
---

Body.
`)},
	}

	sk, warnings, err := skills.Open(fs, ".")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Should have 2 extension warnings (version + custom_field).
	extWarnings := 0
	for _, w := range warnings {
		if w.Kind == skills.WarnExtensionField {
			extWarnings++
		}
	}
	if extWarnings != 2 {
		t.Errorf("extension warnings: got %d, want 2", extWarnings)
	}

	ext := sk.Extensions()
	if ext == nil {
		t.Fatal("Extensions: got nil")
	}
	if ext["version"] != "2.0" {
		t.Errorf("Extensions[version]: got %v", ext["version"])
	}
	if ext["custom_field"] != "custom_value" {
		t.Errorf("Extensions[custom_field]: got %v", ext["custom_field"])
	}
}

func TestOpen_EmptyBody_Warning(t *testing.T) {
	fs := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte(`---
name: no-body
description: Empty body.
---
`)},
	}

	sk, warnings, err := skills.Open(fs, ".")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if sk.Instructions() != "" {
		t.Errorf("Instructions: got %q, want empty", sk.Instructions())
	}

	hasEmptyWarning := false
	for _, w := range warnings {
		if w.Kind == skills.WarnEmptyInstructions {
			hasEmptyWarning = true
		}
	}
	if !hasEmptyWarning {
		t.Error("expected WarnEmptyInstructions warning")
	}
}

func TestOpen_MissingFile(t *testing.T) {
	fs := fstest.MapFS{}
	_, _, err := skills.Open(fs, ".")
	if err == nil {
		t.Fatal("expected error for missing SKILL.md")
	}
	var le *skills.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error type: got %T, want *LoadError", err)
	}
	if le.SubKind != skills.SkillSubKindMissing {
		t.Errorf("SubKind: got %q, want %q", le.SubKind, skills.SkillSubKindMissing)
	}
}

func TestOpen_PathEscape(t *testing.T) {
	fs := fstest.MapFS{}
	_, _, err := skills.Open(fs, "../escape")
	if err == nil {
		t.Fatal("expected error for path escape")
	}
	var le *skills.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error type: got %T, want *LoadError", err)
	}
	if le.SubKind != skills.SkillSubKindPathEscape {
		t.Errorf("SubKind: got %q, want %q", le.SubKind, skills.SkillSubKindPathEscape)
	}
}

func TestOpen_AbsolutePath(t *testing.T) {
	fs := fstest.MapFS{}
	_, _, err := skills.Open(fs, "/absolute/path")
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
	var le *skills.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error type: got %T, want *LoadError", err)
	}
	if le.SubKind != skills.SkillSubKindPathEscape {
		t.Errorf("SubKind: got %q, want %q", le.SubKind, skills.SkillSubKindPathEscape)
	}
}

func TestOpen_MalformedYAML(t *testing.T) {
	fs := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte(`---
name: [broken yaml
description: bad
---

Body.
`)},
	}
	_, _, err := skills.Open(fs, ".")
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	var le *skills.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error type: got %T, want *LoadError", err)
	}
	if le.SubKind != skills.SkillSubKindMalformedYAML {
		t.Errorf("SubKind: got %q, want %q", le.SubKind, skills.SkillSubKindMalformedYAML)
	}
}

func TestOpen_MissingRequiredName(t *testing.T) {
	fs := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte(`---
description: No name field.
---

Body.
`)},
	}
	_, _, err := skills.Open(fs, ".")
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	var le *skills.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error type: got %T, want *LoadError", err)
	}
	if le.SubKind != skills.SkillSubKindInvalidField {
		t.Errorf("SubKind: got %q, want %q", le.SubKind, skills.SkillSubKindInvalidField)
	}
}

func TestOpen_BadNameRegex(t *testing.T) {
	fs := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte(`---
name: "has spaces"
description: Bad name.
---

Body.
`)},
	}
	_, _, err := skills.Open(fs, ".")
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
	var le *skills.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error type: got %T, want *LoadError", err)
	}
	if le.SubKind != skills.SkillSubKindInvalidField {
		t.Errorf("SubKind: got %q, want %q", le.SubKind, skills.SkillSubKindInvalidField)
	}
}

func TestOpen_NoFrontmatterDelimiter(t *testing.T) {
	fs := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte(`Just plain text, no frontmatter.`)},
	}
	_, _, err := skills.Open(fs, ".")
	if err == nil {
		t.Fatal("expected error for missing frontmatter delimiter")
	}
	var le *skills.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error type: got %T, want *LoadError", err)
	}
	if le.SubKind != skills.SkillSubKindMalformedYAML {
		t.Errorf("SubKind: got %q, want %q", le.SubKind, skills.SkillSubKindMalformedYAML)
	}
}

func TestLoad_ValidBundle(t *testing.T) {
	sk, warnings, err := skills.Load("./testdata/valid-bundle")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if sk.Name() != "code-reviewer" {
		t.Errorf("Name: got %q, want %q", sk.Name(), "code-reviewer")
	}
	if sk.License() != "Apache-2.0" {
		t.Errorf("License: got %q", sk.License())
	}
	if sk.Instructions() == "" {
		t.Error("Instructions should not be empty")
	}
}

func TestLoad_MinimalBundle(t *testing.T) {
	sk, warnings, err := skills.Load("./testdata/minimal-bundle")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if sk.Name() != "minimal" {
		t.Errorf("Name: got %q", sk.Name())
	}
	if sk.License() != "" {
		t.Errorf("License should be empty, got %q", sk.License())
	}
}

func TestLoad_DirectFilePathValid(t *testing.T) {
	sk, _, err := skills.Load("./testdata/valid-bundle/SKILL.md")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sk.Name() != "code-reviewer" {
		t.Errorf("Name: got %q", sk.Name())
	}
}

func TestLoad_NonexistentPath(t *testing.T) {
	_, _, err := skills.Load("./testdata/does-not-exist")
	if err == nil {
		t.Fatal("expected error")
	}
	var le *skills.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error type: got %T, want *LoadError", err)
	}
	if le.SubKind != skills.SkillSubKindMissing {
		t.Errorf("SubKind: got %q", le.SubKind)
	}
}

// --- TypedError compliance tests ---

func TestLoadError_TypedError_System(t *testing.T) {
	tests := []skills.SkillSubKind{
		skills.SkillSubKindMissing,
		skills.SkillSubKindPathEscape,
	}
	for _, sk := range tests {
		le := &skills.LoadError{Bundle: "test", SubKind: sk}
		var typed praxiserrors.TypedError = le
		if typed.Kind() != praxiserrors.ErrorKindSystem {
			t.Errorf("%s: Kind() = %v, want System", sk, typed.Kind())
		}
		if typed.HTTPStatusCode() != 400 {
			t.Errorf("%s: HTTPStatusCode() = %d, want 400", sk, typed.HTTPStatusCode())
		}
	}
}

func TestLoadError_TypedError_Tool(t *testing.T) {
	tests := []skills.SkillSubKind{
		skills.SkillSubKindMalformedYAML,
		skills.SkillSubKindInvalidField,
	}
	for _, sk := range tests {
		le := &skills.LoadError{Bundle: "test", SubKind: sk}
		var typed praxiserrors.TypedError = le
		if typed.Kind() != praxiserrors.ErrorKindTool {
			t.Errorf("%s: Kind() = %v, want Tool", sk, typed.Kind())
		}
	}
}

func TestLoadError_ErrorFormat(t *testing.T) {
	le := &skills.LoadError{
		Bundle:  "/path/to/bundle",
		SubKind: skills.SkillSubKindMissing,
		Cause:   errors.New("file not found"),
	}
	got := le.Error()
	if !strings.Contains(got, "skills:") || !strings.Contains(got, "skill_bundle_missing") || !strings.Contains(got, "/path/to/bundle") || !strings.Contains(got, "file not found") {
		t.Errorf("Error(): got %q, expected all of: skills:, skill_bundle_missing, path, cause", got)
	}
}

func TestLoadError_ErrorFormat_NilCause(t *testing.T) {
	le := &skills.LoadError{
		Bundle:  "test",
		SubKind: skills.SkillSubKindPathEscape,
	}
	got := le.Error()
	want := "skills: skill_bundle_path_escape: bundle=test"
	if got != want {
		t.Errorf("Error(): got %q, want %q", got, want)
	}
}

func TestLoadError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	le := &skills.LoadError{Bundle: "test", SubKind: skills.SkillSubKindMissing, Cause: cause}
	if le.Unwrap() != cause {
		t.Error("Unwrap() did not return cause")
	}
}
