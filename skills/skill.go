// SPDX-License-Identifier: Apache-2.0

package skills

// Skill is an immutable, opaque representation of a loaded SKILL.md bundle.
//
// All accessor methods return zero values for absent optional fields.
// The struct is safe to share across goroutines after construction.
type Skill struct {
	name         string
	description  string
	license      string
	compatibility string
	metadata     map[string]any
	allowedTools []string
	instructions string
	extensions   map[string]any
}

// Name returns the skill's unique identifier.
// Always non-empty and matches ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$.
func (s *Skill) Name() string { return s.name }

// Description returns the human-readable skill description.
// Always non-empty.
func (s *Skill) Description() string { return s.description }

// License returns the SPDX identifier or free-text license statement.
// Returns "" if absent.
func (s *Skill) License() string { return s.license }

// Compatibility returns the free-text compatibility statement.
// Returns "" if absent.
func (s *Skill) Compatibility() string { return s.compatibility }

// Metadata returns the author-defined metadata mapping.
// Returns nil if absent.
func (s *Skill) Metadata() map[string]any { return s.metadata }

// AllowedTools returns the list of pre-approved tool names.
// Returns nil if absent.
func (s *Skill) AllowedTools() []string { return s.allowedTools }

// Instructions returns the non-frontmatter Markdown body of SKILL.md,
// captured verbatim. Returns "" if the body is empty.
func (s *Skill) Instructions() string { return s.instructions }

// Extensions returns unrecognised frontmatter fields preserved under
// their original keys. Returns nil if only recognised fields were present.
func (s *Skill) Extensions() map[string]any { return s.extensions }
