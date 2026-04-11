// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"fmt"
	"os/exec"
	"path/filepath"

	praxiserrors "github.com/praxis-os/praxis/errors"
)

// BuildCommand builds an [*exec.Cmd] configured from the caller's
// stdio transport specification.
//
// The function is deterministic and does NOT start the process —
// the caller (the session pool in praxis/mcp/new.go) is responsible
// for passing the returned *exec.Cmd to an
// [sdkmcp.CommandTransport], which calls Start when the SDK opens
// the session.
//
// # Absolute command resolution (T31.2, D119)
//
// commandPath is resolved via [exec.LookPath] at call time. The
// resolved absolute path is stored on the returned *exec.Cmd.Path
// field, so any later PATH environment changes — between the call
// to BuildCommand and the eventual session open — cannot redirect
// the binary the adapter invokes. This is the concrete mechanism
// behind Phase 7 D119's "PATH changes between New and Invoke do
// not affect resolution" requirement.
//
// If commandPath cannot be resolved — either because it is empty,
// or because exec.LookPath does not find a matching executable on
// the current PATH — BuildCommand returns a typed
// [praxiserrors.SystemError] with Kind() == ErrorKindSystem.
//
// # File descriptor hygiene (T31.4)
//
// The returned *exec.Cmd has its [exec.Cmd.ExtraFiles] field
// explicitly set to nil so no file descriptors owned by the parent
// process can leak into the child beyond the three standard streams
// (stdin, stdout, stderr) — and even stderr is left to whatever the
// SDK wires it to during session open (usually os.Stderr, inherited).
//
// # Process isolation (T31.4, D119)
//
// The process is placed in its own process group on Unix systems
// via [applyProcessIsolation], which sets
// [syscall.SysProcAttr.Setpgid] on [exec.Cmd.SysProcAttr]. On
// non-Unix platforms the helper is a no-op — process-group
// semantics do not apply — and the adapter relies on whatever
// parent-child lifecycle guarantees the host OS provides. The
// build-tagged helper files live alongside this file.
//
// # Environment
//
// env is merged verbatim into the child's environment. No
// inheritance of the parent's environment happens at this layer:
// if env is nil or empty, the child inherits nothing beyond what
// the OS would set by default. Callers that want the parent's
// environment to propagate must construct env themselves from
// [os.Environ] before calling BuildCommand.
//
// Credential material is NOT injected by this function. The
// separate credential-injection helper [InjectEnvCredential] (also
// in this package) owns the byte-buffer lifecycle for the
// resolved credential, per Phase 7 D117 and the T31.3 zeroing
// contract.
//
// # Empty args
//
// A nil or empty args slice is permitted and means the child is
// invoked with the resolved path as argv[0] only. This matches the
// semantics of [exec.Command].
func BuildCommand(commandPath string, args []string, env map[string]string) (*exec.Cmd, error) {
	if commandPath == "" {
		return nil, systemError("transport: TransportStdio.Command is empty; a command path is required")
	}

	resolved, err := exec.LookPath(commandPath)
	if err != nil {
		return nil, systemError(fmt.Sprintf(
			"transport: exec.LookPath(%q): %v; command must be an absolute path or a name resolvable on PATH at New time",
			commandPath, err,
		))
	}
	// Defence in depth: LookPath on an absolute path returns the
	// path unchanged; on a bare name it returns an absolute path.
	// But a pathological PATH could still resolve to a relative
	// entry. Reject anything non-absolute explicitly.
	if !filepath.IsAbs(resolved) {
		return nil, systemError(fmt.Sprintf(
			"transport: exec.LookPath(%q) returned non-absolute path %q; refusing to launch",
			commandPath, resolved,
		))
	}

	cmd := &exec.Cmd{
		Path: resolved,
		Args: buildArgv(resolved, args),
		Env:  flattenEnv(env),
		// ExtraFiles is explicitly nil: no parent-owned file
		// descriptors beyond stdin/stdout/stderr are handed to the
		// child process. This is a T31.4 hardening invariant.
		ExtraFiles: nil,
	}
	applyProcessIsolation(cmd)

	return cmd, nil
}

// buildArgv constructs the argv slice for an [*exec.Cmd]. The first
// element must be the resolved program path (matching the Unix
// execv convention the Go runtime uses on all platforms), and the
// remaining elements are the caller-supplied arguments verbatim.
func buildArgv(resolved string, args []string) []string {
	argv := make([]string, 0, 1+len(args))
	argv = append(argv, resolved)
	argv = append(argv, args...)
	return argv
}

// flattenEnv turns the caller-supplied map[string]string environment
// into the `KEY=VALUE` string slice shape expected by
// [exec.Cmd.Env]. A nil or empty map is returned as a nil slice,
// which the Go runtime interprets as "child inherits nothing".
//
// The order of entries in the returned slice is not deterministic
// (Go map iteration order), but [exec.Cmd.Env]'s semantics do not
// depend on ordering: the kernel sees the set as a whole. Tests
// that assert on specific env entries should use set-based
// assertions, not slice-index comparisons.
func flattenEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

// systemError is the local mirror of praxis/mcp's own systemError
// helper (which cannot be imported here due to the cycle rule
// documented in doc.go). Both call through to
// [praxiserrors.NewSystemError].
func systemError(message string) error {
	return praxiserrors.NewSystemError(message, nil)
}
