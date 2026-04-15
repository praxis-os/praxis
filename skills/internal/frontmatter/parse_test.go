// SPDX-License-Identifier: Apache-2.0

package frontmatter_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/praxis-os/praxis/skills/internal/frontmatter"
)

func validSkillMD(body string) []byte {
	return []byte("---\nname: test\ndescription: A test.\n---\n\n" + body)
}

func TestParse_ValidMinimal(t *testing.T) {
	r, err := frontmatter.Parse(validSkillMD("Do stuff."))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Name != "test" {
		t.Errorf("Name: got %q", r.Name)
	}
	if r.Description != "A test." {
		t.Errorf("Description: got %q", r.Description)
	}
	if !strings.HasPrefix(r.Instructions, "Do stuff.") {
		t.Errorf("Instructions: got %q", r.Instructions)
	}
}

func TestParse_AllOptionalFields(t *testing.T) {
	data := []byte(`---
name: full
description: Full skill.
license: MIT
compatibility: claude-sonnet-4-6
metadata:
  key: value
allowed-tools:
  - tool_a
  - tool_b
---

Body.
`)
	r, err := frontmatter.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.License != "MIT" {
		t.Errorf("License: got %q", r.License)
	}
	if r.Compatibility != "claude-sonnet-4-6" {
		t.Errorf("Compatibility: got %q", r.Compatibility)
	}
	if r.Metadata == nil || r.Metadata["key"] != "value" {
		t.Errorf("Metadata: got %v", r.Metadata)
	}
	if len(r.AllowedTools) != 2 || r.AllowedTools[0] != "tool_a" {
		t.Errorf("AllowedTools: got %v", r.AllowedTools)
	}
}

func TestParse_ExtensionFields(t *testing.T) {
	data := []byte("---\nname: ext\ndescription: Has extras.\nversion: \"1.0\"\ncustom: foo\n---\n\nBody.\n")
	r, err := frontmatter.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Extensions == nil {
		t.Fatal("Extensions: got nil")
	}
	if r.Extensions["version"] != "1.0" {
		t.Errorf("Extensions[version]: got %v", r.Extensions["version"])
	}
	extCount := 0
	for _, w := range r.Warnings {
		if w.Kind == "extension_field" {
			extCount++
		}
	}
	if extCount != 2 {
		t.Errorf("extension warnings: got %d, want 2", extCount)
	}
}

func TestParse_EmptyBody_Warning(t *testing.T) {
	data := []byte("---\nname: empty\ndescription: No body.\n---\n")
	r, err := frontmatter.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Instructions != "" {
		t.Errorf("Instructions: got %q", r.Instructions)
	}
	hasWarning := false
	for _, w := range r.Warnings {
		if w.Kind == "empty_instructions" {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Error("expected empty_instructions warning")
	}
}

// --- Error classification tests ---

func assertParseErrorKind(t *testing.T, data []byte, wantKind frontmatter.ErrorKind, desc string) {
	t.Helper()
	_, err := frontmatter.Parse(data)
	if err == nil {
		t.Fatalf("%s: expected error", desc)
	}
	var pe *frontmatter.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("%s: got %T, want *ParseError", desc, err)
	}
	if pe.Kind != wantKind {
		t.Errorf("%s: Kind = %d, want %d", desc, pe.Kind, wantKind)
	}
}

func TestParse_NoFrontmatter_Malformed(t *testing.T) {
	assertParseErrorKind(t, []byte("plain text"), frontmatter.ErrKindMalformed, "no frontmatter")
}

func TestParse_MalformedYAML(t *testing.T) {
	data := []byte("---\nname: [broken\n---\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindMalformed, "malformed YAML")
}

func TestParse_MissingName_InvalidField(t *testing.T) {
	data := []byte("---\ndescription: No name.\n---\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindInvalidField, "missing name")
}

func TestParse_EmptyName_InvalidField(t *testing.T) {
	data := []byte("---\nname: \"\"\ndescription: Empty name.\n---\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindInvalidField, "empty name")
}

func TestParse_BadNameRegex_InvalidField(t *testing.T) {
	data := []byte("---\nname: \"has spaces\"\ndescription: Bad.\n---\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindInvalidField, "bad name regex")
}

func TestParse_MissingDescription_InvalidField(t *testing.T) {
	data := []byte("---\nname: ok\n---\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindInvalidField, "missing description")
}

func TestParse_BadLicenseType_InvalidField(t *testing.T) {
	data := []byte("---\nname: ok\ndescription: Ok.\nlicense: [not, a, string]\n---\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindInvalidField, "bad license type")
}

func TestParse_BadMetadataType_InvalidField(t *testing.T) {
	data := []byte("---\nname: ok\ndescription: Ok.\nmetadata: not_a_map\n---\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindInvalidField, "bad metadata type")
}

func TestParse_BadAllowedToolsType_InvalidField(t *testing.T) {
	data := []byte("---\nname: ok\ndescription: Ok.\nallowed-tools: not_a_list\n---\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindInvalidField, "bad allowed-tools type")
}

func TestParse_BadAllowedToolsElement_InvalidField(t *testing.T) {
	data := []byte("---\nname: ok\ndescription: Ok.\nallowed-tools:\n  - 42\n---\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindInvalidField, "bad allowed-tools element")
}

// --- Delimiter tests ---

func TestParse_DotsClosingDelimiter(t *testing.T) {
	data := []byte("---\nname: dots\ndescription: Uses dots.\n...\n\nBody after dots.\n")
	r, err := frontmatter.Parse(data)
	if err != nil {
		t.Fatalf("Parse with ... delimiter: %v", err)
	}
	if r.Name != "dots" {
		t.Errorf("Name: got %q", r.Name)
	}
	if !strings.HasPrefix(r.Instructions, "Body after dots.") {
		t.Errorf("Instructions: got %q", r.Instructions)
	}
}

func TestParse_NoClosingDelimiter_Malformed(t *testing.T) {
	data := []byte("---\nname: broken\ndescription: No close.\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindMalformed, "no closing delimiter")
}

// --- Size limit tests ---

func TestParse_ExceedsMaxFileSize(t *testing.T) {
	data := make([]byte, frontmatter.MaxFileSize+1)
	copy(data, "---\nname: big\ndescription: Too big.\n---\n\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindMalformed, "exceeds max file size")
}

func TestParse_ExceedsAnchorExpansionLimit(t *testing.T) {
	// Build frontmatter that exceeds 64 KiB.
	var b strings.Builder
	b.WriteString("---\nname: bomb\ndescription: YAML bomb test.\n")
	// Add enough content to exceed 64 KiB of frontmatter.
	filler := strings.Repeat("x", 1024)
	for b.Len() < 65*1024 {
		b.WriteString("filler_" + filler + ": value\n")
	}
	b.WriteString("---\n\nBody.\n")
	assertParseErrorKind(t, []byte(b.String()), frontmatter.ErrKindMalformed, "anchor expansion limit")
}

// --- Verbatim body tests ---

func TestParse_PreservesBodyVerbatim(t *testing.T) {
	data := []byte("---\nname: verbatim\ndescription: Test.\n---\n\n\nLine after blank.\n")
	r, err := frontmatter.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Body should start with \nLine (one leading newline trimmed, one preserved).
	if !strings.HasPrefix(r.Instructions, "\nLine after blank.") {
		t.Errorf("Instructions not verbatim: got %q", r.Instructions)
	}
}

func TestParse_SingleNewlineTrimmed(t *testing.T) {
	data := []byte("---\nname: trim\ndescription: Test.\n---\n\nBody.\n")
	r, err := frontmatter.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.HasPrefix(r.Instructions, "Body.") {
		t.Errorf("Instructions: got %q", r.Instructions)
	}
}

// --- Name pattern edge cases ---

func TestParse_NameMaxLength(t *testing.T) {
	// 64 chars: 1 leading + 63 trailing = valid
	name := "a" + strings.Repeat("b", 63)
	data := []byte("---\nname: " + name + "\ndescription: Max length.\n---\n\nBody.\n")
	r, err := frontmatter.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Name != name {
		t.Errorf("Name: got %q", r.Name)
	}
}

func TestParse_NameTooLong(t *testing.T) {
	// 65 chars: exceeds pattern
	name := "a" + strings.Repeat("b", 64)
	data := []byte("---\nname: " + name + "\ndescription: Too long.\n---\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindInvalidField, "name too long")
}

func TestParse_NameWithHyphensUnderscores(t *testing.T) {
	data := []byte("---\nname: my-skill_v2\ndescription: Valid name.\n---\n\nBody.\n")
	r, err := frontmatter.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Name != "my-skill_v2" {
		t.Errorf("Name: got %q", r.Name)
	}
}

func TestParse_NameStartsWithDigit(t *testing.T) {
	data := []byte("---\nname: 2fast\ndescription: Starts with digit.\n---\n\nBody.\n")
	r, err := frontmatter.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Name != "2fast" {
		t.Errorf("Name: got %q", r.Name)
	}
}

func TestParse_NameStartsWithHyphen(t *testing.T) {
	data := []byte("---\nname: -invalid\ndescription: Bad start.\n---\n\nBody.\n")
	assertParseErrorKind(t, data, frontmatter.ErrKindInvalidField, "name starts with hyphen")
}
