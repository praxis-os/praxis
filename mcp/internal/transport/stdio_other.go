// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package transport

import "os/exec"

// applyProcessIsolation is the non-Unix fallback for the
// Unix-only process-group isolation helper. Process groups are a
// Unix concept; on Windows, Plan 9, WASI, and any other
// non-Unix target the Go runtime supports, there is no direct
// equivalent and this function is a no-op.
//
// Callers on non-Unix platforms rely on whatever parent-child
// lifecycle guarantees the host OS provides. On Windows, for
// example, a process spawned without an explicit Job Object
// still terminates when the parent exits under most shells; that
// is not a praxis-level guarantee but an OS-level convention.
// Hardening Windows child-process containment (e.g., via Job
// Objects) is out of scope for v1.0.0 — Phase 7 D108 commits to
// stdio + Streamable HTTP but Phase 7 does not mandate Windows
// parity for stdio process-group semantics.
//
// This file is compiled only when the `unix` build constraint
// does NOT match. The Unix counterpart lives in stdio_unix.go.
func applyProcessIsolation(_ *exec.Cmd) {
	// intentional no-op on non-Unix platforms
}

// procIsolationSetpgid is the non-Unix companion to the helper
// in stdio_unix.go. On non-Unix targets there is no process-group
// concept, so this function always reports false. Test code
// guards its call sites with a runtime.GOOS check and skips
// before invoking the helper, so this body is never reached in
// practice on the platforms it is compiled for.
func procIsolationSetpgid(_ *exec.Cmd) bool {
	return false
}
