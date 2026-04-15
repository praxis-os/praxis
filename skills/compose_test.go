// SPDX-License-Identifier: Apache-2.0

package skills_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/praxis-os/praxis/skills"
)

func loadTestSkill(t *testing.T, name, desc, body string) *skills.Skill {
	t.Helper()
	content := "---\nname: " + name + "\ndescription: " + desc + "\n---\n\n" + body
	fs := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte(content)},
	}
	sk, _, err := skills.Open(fs, ".")
	if err != nil {
		t.Fatalf("loadTestSkill(%q): %v", name, err)
	}
	return sk
}

func TestWithSkill_DuplicateName_Panics(t *testing.T) {
	skA := loadTestSkill(t, "same-name", "First", "Body A")
	skB := loadTestSkill(t, "same-name", "Second", "Body B")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate skill name")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string", r)
		}
		if !strings.Contains(msg, "same-name") {
			t.Errorf("panic message %q should contain skill name", msg)
		}
	}()

	// This should panic during orchestrator construction.
	_ = skills.ComposedInstructions("base", skA, skB)
}

func TestComposedInstructions_NoSkills(t *testing.T) {
	got := skills.ComposedInstructions("base prompt")
	if got != "base prompt" {
		t.Errorf("got %q, want %q", got, "base prompt")
	}
}

func TestComposedInstructions_SingleSkill(t *testing.T) {
	sk := loadTestSkill(t, "reviewer", "Review code", "Check everything.")
	got := skills.ComposedInstructions("You are helpful.", sk)
	want := "You are helpful.\n\n--- Skills ---\n\nCheck everything."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestComposedInstructions_MultiSkill_OrderPreserved(t *testing.T) {
	skA := loadTestSkill(t, "skill-a", "First", "Fragment A")
	skB := loadTestSkill(t, "skill-b", "Second", "Fragment B")
	got := skills.ComposedInstructions("base", skA, skB)

	// Verify order: Fragment A before Fragment B.
	idxA := strings.Index(got, "Fragment A")
	idxB := strings.Index(got, "Fragment B")
	if idxA < 0 || idxB < 0 {
		t.Fatalf("missing fragments in: %q", got)
	}
	if idxA >= idxB {
		t.Errorf("Fragment A (idx=%d) should appear before Fragment B (idx=%d)", idxA, idxB)
	}

	// Verify separator.
	if !strings.Contains(got, "--- Skills ---") {
		t.Error("missing separator")
	}
}
