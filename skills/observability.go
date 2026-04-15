// SPDX-License-Identifier: Apache-2.0

package skills

// MetricsRecorder is an optional interface for recording skill-loading
// metrics. Callers opt in via type assertion on their observability
// pipeline.
//
// The status parameter is "success" on successful load, or a
// [SkillSubKind] string value on failure. Skill names MUST NOT be used
// as metric labels (D130 — bounded cardinality).
//
// Suggested Prometheus counter: praxis_skills_loaded_total{status="..."}
//
// Stability: experimental (D134).
type MetricsRecorder interface {
	RecordSkillLoaded(skillName string, status string)
}
