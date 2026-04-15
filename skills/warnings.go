// SPDX-License-Identifier: Apache-2.0

package skills

// WarnKind classifies non-fatal diagnostics emitted during skill loading.
type WarnKind string

const (
	// WarnExtensionField indicates that the frontmatter contains an
	// unrecognised field, preserved in [Skill.Extensions].
	WarnExtensionField WarnKind = "extension_field"

	// WarnEmptyInstructions indicates that the SKILL.md body (after
	// frontmatter) is empty.
	WarnEmptyInstructions WarnKind = "empty_instructions"
)

// SkillWarning is a non-fatal diagnostic produced during skill loading.
type SkillWarning struct {
	// Kind classifies the warning.
	Kind WarnKind

	// Field is the frontmatter key that triggered the warning, or ""
	// for structural warnings like empty instructions.
	Field string

	// Message is a human-readable description.
	Message string
}

// String formats the warning for logging.
func (w SkillWarning) String() string {
	if w.Field != "" {
		return string(w.Kind) + ": " + w.Field + ": " + w.Message
	}
	return string(w.Kind) + ": " + w.Message
}
