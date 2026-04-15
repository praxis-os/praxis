// SPDX-License-Identifier: Apache-2.0

package skills

import (
	"errors"
	"io/fs"
	"os"

	"github.com/praxis-os/praxis/skills/internal/frontmatter"
	"github.com/praxis-os/praxis/skills/internal/resolve"
)

// Open loads a skill bundle from the given [fs.FS] at root.
// root must be a relative path without ".." components; it is joined
// with "SKILL.md" to locate the bundle file.
//
// Returns the loaded [*Skill], any non-fatal [SkillWarning] values,
// and a [*LoadError] on failure.
func Open(fsys fs.FS, root string) (*Skill, []SkillWarning, error) {
	if err := resolve.ValidateFSRoot(root); err != nil {
		return nil, nil, &LoadError{
			Bundle:  root,
			SubKind: SkillSubKindPathEscape,
			Cause:   err,
		}
	}

	data, err := resolve.OpenFile(fsys, root)
	if err != nil {
		return nil, nil, &LoadError{
			Bundle:  root,
			SubKind: SkillSubKindMissing,
			Cause:   err,
		}
	}

	return parseAndBuild(root, data)
}

// Load loads a skill bundle from the host filesystem.
// path may point to a directory (SKILL.md is appended) or directly to
// a SKILL.md file.
//
// Returns the loaded [*Skill], any non-fatal [SkillWarning] values,
// and a [*LoadError] on failure.
func Load(path string) (*Skill, []SkillWarning, error) {
	dir, file, err := resolve.ResolvePath(path)
	if err != nil {
		return nil, nil, &LoadError{
			Bundle:  path,
			SubKind: SkillSubKindMissing,
			Cause:   err,
		}
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, &LoadError{
			Bundle:  dir,
			SubKind: SkillSubKindMissing,
			Cause:   err,
		}
	}

	return parseAndBuild(dir, data)
}

// parseAndBuild parses raw SKILL.md content and constructs a Skill.
func parseAndBuild(bundle string, data []byte) (*Skill, []SkillWarning, error) {
	result, err := frontmatter.Parse(data)
	if err != nil {
		// Map typed ParseError to the appropriate SkillSubKind.
		subKind := SkillSubKindMalformedYAML
		var pe *frontmatter.ParseError
		if errors.As(err, &pe) {
			switch pe.Kind {
			case frontmatter.ErrKindInvalidField:
				subKind = SkillSubKindInvalidField
			case frontmatter.ErrKindMalformed:
				subKind = SkillSubKindMalformedYAML
			}
		}
		return nil, nil, &LoadError{
			Bundle:  bundle,
			SubKind: subKind,
			Cause:   err,
		}
	}

	sk := &Skill{
		name:          result.Name,
		description:   result.Description,
		license:       result.License,
		compatibility: result.Compatibility,
		metadata:      result.Metadata,
		allowedTools:  result.AllowedTools,
		instructions:  result.Instructions,
		extensions:    result.Extensions,
	}

	var warnings []SkillWarning
	for _, w := range result.Warnings {
		warnings = append(warnings, SkillWarning{
			Kind:    WarnKind(w.Kind),
			Field:   w.Field,
			Message: w.Message,
		})
	}

	return sk, warnings, nil
}
