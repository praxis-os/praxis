// SPDX-License-Identifier: Apache-2.0

package skills

import (
	"fmt"

	"github.com/praxis-os/praxis/errors"
)

// SkillSubKind classifies the specific failure mode of a skill bundle load.
type SkillSubKind string

const (
	// SkillSubKindMissing indicates that the SKILL.md file was not found.
	SkillSubKindMissing SkillSubKind = "skill_bundle_missing"

	// SkillSubKindMalformedYAML indicates that the YAML frontmatter could
	// not be parsed.
	SkillSubKindMalformedYAML SkillSubKind = "skill_bundle_malformed_yaml"

	// SkillSubKindInvalidField indicates that a required field is missing
	// or a field value fails validation (e.g. name regex).
	SkillSubKindInvalidField SkillSubKind = "skill_bundle_invalid_field"

	// SkillSubKindPathEscape indicates that the bundle path attempts to
	// escape the root directory.
	SkillSubKindPathEscape SkillSubKind = "skill_bundle_path_escape"
)

// LoadError is a typed error returned by [Open] and [Load] when a skill
// bundle cannot be loaded. It implements [errors.TypedError].
type LoadError struct {
	// Bundle is the path or root that was being loaded.
	Bundle string

	// SubKind classifies the specific failure mode.
	SubKind SkillSubKind

	// Cause is the underlying error, if any.
	Cause error
}

// Error formats the load error as: "skills: <subkind>: bundle=<path>: <cause>".
func (e *LoadError) Error() string {
	base := fmt.Sprintf("skills: %s: bundle=%s", e.SubKind, e.Bundle)
	if e.Cause != nil {
		return base + ": " + e.Cause.Error()
	}
	return base
}

// Unwrap returns the underlying cause.
func (e *LoadError) Unwrap() error { return e.Cause }

// Kind maps the sub-kind to the appropriate [errors.ErrorKind].
// Missing and path-escape errors are system errors (caller misconfiguration);
// malformed YAML and invalid fields are tool-class errors (bad bundle content).
func (e *LoadError) Kind() errors.ErrorKind {
	switch e.SubKind {
	case SkillSubKindMissing, SkillSubKindPathEscape:
		return errors.ErrorKindSystem
	default:
		return errors.ErrorKindTool
	}
}

// HTTPStatusCode returns 400 (Bad Request) for all load errors.
func (e *LoadError) HTTPStatusCode() int { return 400 }

// Verify interface compliance at compile time.
var _ errors.TypedError = (*LoadError)(nil)
