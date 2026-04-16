// SPDX-License-Identifier: Apache-2.0

package frontmatter_test

import (
	"testing"

	"github.com/praxis-os/praxis/skills/internal/frontmatter"
)

var minimalBundle = []byte(`---
name: bench-skill
description: A benchmark skill.
---

Be helpful.
`)

var fullBundle = []byte(`---
name: code-reviewer
description: |
  Reviews staged changes for correctness, test coverage, and style.
license: Apache-2.0
compatibility: "claude-sonnet-4-6, claude-opus-4-6"
metadata:
  author: team-platform
  maintainer: platform@example.com
allowed-tools:
  - read_file
  - grep
  - run_tests
  - write_file
---

You are a code reviewer. When invoked, read the currently staged
changes and produce a review organised by severity:

1. **Blocker** — bugs, regressions, security issues.
2. **Important** — missing tests, missing documentation, inconsistent style.
3. **Nitpick** — cosmetic suggestions.

Always end with a one-line "READY / BLOCK" verdict.
`)

// BenchmarkParse_Minimal measures parse cost for a trivial bundle
// (name + description + short body).
func BenchmarkParse_Minimal(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := frontmatter.Parse(minimalBundle)
		if err != nil {
			b.Fatalf("Parse: %v", err)
		}
	}
}

// BenchmarkParse_Full measures parse cost for a realistic bundle with
// all optional fields (license, compatibility, metadata, allowed-tools)
// and a multi-paragraph body.
func BenchmarkParse_Full(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := frontmatter.Parse(fullBundle)
		if err != nil {
			b.Fatalf("Parse: %v", err)
		}
	}
}
