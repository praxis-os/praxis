// SPDX-License-Identifier: Apache-2.0

// Package frontmatter extracts and validates YAML frontmatter from SKILL.md content.
package frontmatter

import (
	"bytes"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// MaxFileSize is the maximum SKILL.md file size (256 KiB).
const MaxFileSize = 256 * 1024

// maxAnchorExpansion is the YAML anchor expansion limit (64 KiB, D124).
const maxAnchorExpansion = 64 * 1024

// namePattern validates skill names (D123).
var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// recognisedFields is the set of frontmatter keys with typed accessors.
var recognisedFields = map[string]bool{
	"name":          true,
	"description":   true,
	"license":       true,
	"compatibility": true,
	"metadata":      true,
	"allowed-tools": true,
}

// ErrorKind classifies frontmatter parse errors so callers can map them
// to the appropriate SkillSubKind without string matching.
type ErrorKind int

const (
	// ErrKindMalformed covers structural issues: missing delimiters, bad YAML syntax.
	ErrKindMalformed ErrorKind = iota
	// ErrKindInvalidField covers missing required fields and field validation failures.
	ErrKindInvalidField
)

// ParseError is a typed error returned by [Parse].
type ParseError struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

func (e *ParseError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *ParseError) Unwrap() error { return e.Cause }

func malformed(msg string) *ParseError {
	return &ParseError{Kind: ErrKindMalformed, Message: msg}
}

func malformedWrap(msg string, cause error) *ParseError {
	return &ParseError{Kind: ErrKindMalformed, Message: msg, Cause: cause}
}

func invalidField(msg string) *ParseError {
	return &ParseError{Kind: ErrKindInvalidField, Message: msg}
}

// Warning is a non-fatal diagnostic from parsing.
type Warning struct {
	Kind    string // "extension_field" or "empty_instructions"
	Field   string // frontmatter key, or ""
	Message string
}

// Result holds the validated output of a successful parse.
type Result struct {
	Name          string
	Description   string
	License       string
	Compatibility string
	Metadata      map[string]any
	AllowedTools  []string
	Instructions  string
	Extensions    map[string]any
	Warnings      []Warning
}

// Parse extracts frontmatter and body from raw SKILL.md content.
// Returns a [*ParseError] on failure with a classified [ErrorKind].
func Parse(data []byte) (*Result, error) {
	if len(data) > MaxFileSize {
		return nil, malformed(fmt.Sprintf("file exceeds maximum size of %d bytes", MaxFileSize))
	}

	frontmatterYAML, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err // already a *ParseError
	}

	// YAML bomb mitigation (D124): reject if raw frontmatter exceeds
	// anchor expansion limit. Since yaml.v3 expands anchors during decode,
	// we bound the raw input size as a conservative proxy. Frontmatter
	// larger than 64 KiB is almost certainly an anchor bomb or
	// unreasonable content.
	if len(frontmatterYAML) > maxAnchorExpansion {
		return nil, malformed(fmt.Sprintf("frontmatter exceeds %d byte anchor expansion limit", maxAnchorExpansion))
	}

	// Parse YAML into raw map.
	raw := make(map[string]any)
	decoder := yaml.NewDecoder(bytes.NewReader(frontmatterYAML))
	decoder.KnownFields(false)
	if err := decoder.Decode(&raw); err != nil {
		return nil, malformedWrap("YAML parse error", err)
	}

	r := &Result{}

	// Required: name
	nameVal, ok := raw["name"]
	if !ok {
		return nil, invalidField("required field \"name\" is missing")
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return nil, invalidField("field \"name\" must be a non-empty string")
	}
	if !namePattern.MatchString(name) {
		return nil, invalidField(fmt.Sprintf("field \"name\" value %q does not match pattern %s", name, namePattern.String()))
	}
	r.Name = name

	// Required: description
	descVal, ok := raw["description"]
	if !ok {
		return nil, invalidField("required field \"description\" is missing")
	}
	desc, ok := descVal.(string)
	if !ok || desc == "" {
		return nil, invalidField("field \"description\" must be a non-empty string")
	}
	r.Description = desc

	// Optional: license (string)
	if v, ok := raw["license"]; ok {
		if s, ok := v.(string); ok {
			r.License = s
		} else {
			return nil, invalidField("field \"license\" must be a string")
		}
	}

	// Optional: compatibility (string)
	if v, ok := raw["compatibility"]; ok {
		if s, ok := v.(string); ok {
			r.Compatibility = s
		} else {
			return nil, invalidField("field \"compatibility\" must be a string")
		}
	}

	// Optional: metadata (map)
	if v, ok := raw["metadata"]; ok {
		if m, ok := v.(map[string]any); ok {
			r.Metadata = m
		} else {
			return nil, invalidField("field \"metadata\" must be a YAML mapping")
		}
	}

	// Optional: allowed-tools ([]string)
	if v, ok := raw["allowed-tools"]; ok {
		slice, ok := v.([]any)
		if !ok {
			return nil, invalidField("field \"allowed-tools\" must be a YAML sequence of strings")
		}
		tools := make([]string, 0, len(slice))
		for i, item := range slice {
			s, ok := item.(string)
			if !ok {
				return nil, invalidField(fmt.Sprintf("field \"allowed-tools[%d]\" must be a string", i))
			}
			tools = append(tools, s)
		}
		r.AllowedTools = tools
	}

	// Collect extension fields (unrecognised keys).
	for key, val := range raw {
		if !recognisedFields[key] {
			if r.Extensions == nil {
				r.Extensions = make(map[string]any)
			}
			r.Extensions[key] = val
			r.Warnings = append(r.Warnings, Warning{
				Kind:    "extension_field",
				Field:   key,
				Message: fmt.Sprintf("unrecognised frontmatter field %q preserved in Extensions()", key),
			})
		}
	}

	// Instructions: the markdown body after frontmatter.
	r.Instructions = string(body)
	if r.Instructions == "" {
		r.Warnings = append(r.Warnings, Warning{
			Kind:    "empty_instructions",
			Message: "SKILL.md body is empty; Instructions() will return \"\"",
		})
	}

	return r, nil
}

// splitFrontmatter separates YAML frontmatter from body.
// Supports both "---" and "..." as the closing delimiter (YAML spec).
func splitFrontmatter(data []byte) (frontmatter, body []byte, err error) {
	const openMarker = "---"

	// Must start with ---
	if !bytes.HasPrefix(data, []byte(openMarker)) {
		return nil, nil, malformed("SKILL.md must start with \"---\" frontmatter delimiter")
	}

	// Find end of opening --- line.
	rest := data[len(openMarker):]
	idx := bytes.IndexByte(rest, '\n')
	if idx < 0 {
		return nil, nil, malformed("no closing frontmatter delimiter found")
	}
	rest = rest[idx+1:]

	// Find closing delimiter: --- or ...
	closeIdx := -1
	closeLen := 0
	for _, marker := range []string{"\n---", "\n..."} {
		ci := bytes.Index(rest, []byte(marker))
		if ci >= 0 && (closeIdx < 0 || ci < closeIdx) {
			closeIdx = ci
			closeLen = len(marker)
		}
	}

	if closeIdx < 0 {
		// Check if file ends with a delimiter without trailing newline.
		for _, marker := range []string{"---", "..."} {
			if bytes.HasSuffix(rest, []byte(marker)) {
				return rest[:len(rest)-len(marker)], nil, nil
			}
		}
		return nil, nil, malformed("no closing frontmatter delimiter found")
	}

	fm := rest[:closeIdx]
	afterClose := rest[closeIdx+closeLen:]

	// Skip the rest of the closing delimiter line.
	nlIdx := bytes.IndexByte(afterClose, '\n')
	if nlIdx < 0 {
		// No content after closing delimiter.
		return fm, nil, nil
	}
	body = afterClose[nlIdx+1:]

	// Trim at most one leading newline from body (conventional separator).
	// Preserves intentional blank lines per D128 verbatim semantics.
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}

	return fm, body, nil
}
