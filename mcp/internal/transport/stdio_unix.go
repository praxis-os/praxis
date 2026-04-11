// SPDX-License-Identifier: Apache-2.0

//go:build unix

package transport

import (
	"os/exec"
	"syscall"
)

// applyProcessIsolation installs Unix-only process-group isolation
// on the given [*exec.Cmd].
//
// Setpgid=true instructs the kernel to place the child process in
// its own process group at fork time rather than inheriting the
// parent's. This has three effects that together satisfy the
// Phase 7 D119 stdio hardening requirements:
//
//  1. A signal (e.g., SIGINT from Ctrl-C on an interactive shell)
//     delivered to the parent's process group is NOT automatically
//     propagated to the child MCP server. The adapter owns the
//     child's lifecycle and decides when to signal it.
//  2. If the child spawns grandchildren, they inherit the child's
//     process group. When the adapter eventually signals the child
//     to terminate, a kill to the child's process group (via
//     SIGTERM or SIGKILL on -pgid) will reach the whole tree
//     deterministically.
//  3. The child cannot, via process-group tricks, accidentally
//     terminate the parent's process group: it is not part of it.
//
// This file is compiled only under the `unix` build constraint,
// which covers Linux, macOS, BSD variants, Solaris, and illumos.
// The non-Unix counterpart [applyProcessIsolation] lives in
// stdio_other.go as a no-op; process-group semantics do not apply
// on Windows or Plan 9, and the adapter relies on the host OS's
// own parent-child lifecycle guarantees on those platforms.
func applyProcessIsolation(cmd *exec.Cmd) {
	attr := cmd.SysProcAttr
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	attr.Setpgid = true
	cmd.SysProcAttr = attr
}

// procIsolationSetpgid reports whether the given command has
// Unix process-group isolation applied — i.e., whether its
// [syscall.SysProcAttr.Setpgid] field is true. Tests use it as a
// build-tag-safe inspection helper that does not require platform-
// specific struct access in the test file itself.
func procIsolationSetpgid(cmd *exec.Cmd) bool {
	return cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid
}
