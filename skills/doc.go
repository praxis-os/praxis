// SPDX-License-Identifier: Apache-2.0

// Package skills loads SKILL.md bundles and composes their instructions
// into a praxis [orchestrator.Orchestrator] system prompt.
//
// A skill bundle is a filesystem directory containing a SKILL.md file with
// YAML frontmatter (name, description, and optional fields) followed by a
// Markdown instruction body. The loader parses the frontmatter, validates
// required fields, preserves unrecognised fields as extensions, and returns
// an immutable [*Skill] value.
//
// # Loading
//
// Use [Open] for hermetic / testable loading from an [fs.FS], or [Load] as
// a convenience wrapper that reads from the host filesystem:
//
//	sk, warnings, err := skills.Load("./bundles/code-reviewer")
//
// # Composition
//
// Wire loaded skills into the orchestrator at construction time with
// [WithSkill]. Skill instruction fragments are injected into the system
// prompt in the order [WithSkill] is called:
//
//	orch, err := orchestrator.New(
//	    provider,
//	    orchestrator.WithDefaultModel("claude-sonnet-4-6"),
//	    skills.WithSkill(sk),
//	)
//
// Duplicate skill names cause a panic at construction time (D127).
//
// # Non-goals
//
// This package does not download bundles, discover them at runtime,
// hot-reload, execute scripts, or sandbox tool calls (D133).
// See the Phase 8 decisions log for the full non-goals catalogue.
//
// Stability: stable-v0.x-candidate (D134).
package skills
