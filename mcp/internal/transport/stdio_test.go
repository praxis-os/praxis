// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	praxiserrors "github.com/praxis-os/praxis/errors"
)

// knownExecutable returns the name of a binary that is (almost)
// universally available on the host as a PATH-resolvable name. On
// POSIX systems this is `sh` — mandated by POSIX at /bin/sh and
// always in PATH. On non-POSIX targets the test skips with a
// t.Skip() call.
func knownExecutable(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skipf("no portable PATH-resolvable command on %s; BuildCommand semantics covered by Unix-only subtests", runtime.GOOS)
	}
	return "sh"
}

// assertSystemError matches the praxis/mcp package-level helper:
// asserts err is a typed system error containing each wanted
// fragment. Duplicated here to keep the internal package
// self-contained — tests in internal/transport cannot import
// helpers from the parent mcp package without a cycle.
func assertSystemError(t *testing.T, err error, wantFragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected typed system error, got nil")
	}
	var typed praxiserrors.TypedError
	if !errors.As(err, &typed) {
		t.Fatalf("error is not a TypedError: %T: %v", err, err)
	}
	if typed.Kind() != praxiserrors.ErrorKindSystem {
		t.Fatalf("error Kind() = %q, want %q", typed.Kind(), praxiserrors.ErrorKindSystem)
	}
	msg := err.Error()
	for _, frag := range wantFragments {
		if !strings.Contains(msg, frag) {
			t.Errorf("error message missing fragment %q; got: %s", frag, msg)
		}
	}
}

// TestBuildCommandLookPathResolution exercises the happy path:
// a PATH-resolvable command is translated into an absolute-path
// *exec.Cmd. The resolved path must be absolute and must exist on
// the filesystem (verified via exec.LookPath being the same tool
// Go's runtime uses internally).
func TestBuildCommandLookPathResolution(t *testing.T) {
	t.Parallel()

	name := knownExecutable(t)

	cmd, err := BuildCommand(name, []string{"-c", "exit 0"}, nil)
	if err != nil {
		t.Fatalf("BuildCommand(%q): %v", name, err)
	}
	if !filepath.IsAbs(cmd.Path) {
		t.Errorf("cmd.Path = %q; want absolute path", cmd.Path)
	}
	// Sanity check: the argv[0] entry must match the resolved path
	// (the Unix execv convention the Go runtime uses on all
	// platforms).
	if got := cmd.Args[0]; got != cmd.Path {
		t.Errorf("cmd.Args[0] = %q, want %q", got, cmd.Path)
	}
	// Caller-supplied args must be appended after argv[0] in order.
	if len(cmd.Args) != 3 || cmd.Args[1] != "-c" || cmd.Args[2] != "exit 0" {
		t.Errorf("cmd.Args = %q, want [path, -c, exit 0]", cmd.Args)
	}
	// ExtraFiles must be explicitly nil — no parent-owned fds
	// leak into the child beyond stdin/stdout/stderr.
	if cmd.ExtraFiles != nil {
		t.Errorf("cmd.ExtraFiles = %v, want nil (T31.4 fd hygiene)", cmd.ExtraFiles)
	}
}

// TestBuildCommandAbsolutePathPassthrough asserts that an absolute
// path is accepted and preserved by exec.LookPath's absolute-path
// shortcut. The test resolves the knownExecutable via LookPath once
// externally, then passes the absolute path back to BuildCommand.
func TestBuildCommandAbsolutePathPassthrough(t *testing.T) {
	t.Parallel()

	name := knownExecutable(t)
	abs, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("exec.LookPath(%q) failed during test setup: %v", name, err)
	}

	cmd, err := BuildCommand(abs, nil, nil)
	if err != nil {
		t.Fatalf("BuildCommand(%q): %v", abs, err)
	}
	if cmd.Path != abs {
		t.Errorf("cmd.Path = %q, want %q (absolute-path passthrough)", cmd.Path, abs)
	}
}

// TestBuildCommandRejectsEmptyCommand pins the validation rule that
// an empty Command is a typed system error.
func TestBuildCommandRejectsEmptyCommand(t *testing.T) {
	t.Parallel()

	cmd, err := BuildCommand("", nil, nil)
	if cmd != nil {
		t.Errorf("expected nil cmd on validation failure, got %v", cmd)
	}
	assertSystemError(t, err, "Command is empty")
}

// TestBuildCommandRejectsUnresolvableCommand pins the LookPath
// failure path. The fake name is chosen to be highly unlikely to
// exist on any host PATH.
func TestBuildCommandRejectsUnresolvableCommand(t *testing.T) {
	t.Parallel()

	fake := "praxis-mcp-definitely-not-a-real-binary-" + t.Name()
	cmd, err := BuildCommand(fake, nil, nil)
	if cmd != nil {
		t.Errorf("expected nil cmd on LookPath failure, got %v", cmd)
	}
	assertSystemError(t, err, "exec.LookPath", fake)
}

// TestBuildCommandEnvMapFlattened asserts that the caller-supplied
// env map is flattened into the KEY=VALUE string slice that
// exec.Cmd.Env expects. Order is nondeterministic; use set-based
// assertions.
func TestBuildCommandEnvMapFlattened(t *testing.T) {
	t.Parallel()

	name := knownExecutable(t)
	env := map[string]string{
		"PRAXIS_MCP_TEST_A": "alpha",
		"PRAXIS_MCP_TEST_B": "bravo",
	}
	cmd, err := BuildCommand(name, nil, env)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}
	if len(cmd.Env) != 2 {
		t.Errorf("len(cmd.Env) = %d, want 2", len(cmd.Env))
	}
	got := make(map[string]bool, len(cmd.Env))
	for _, kv := range cmd.Env {
		got[kv] = true
	}
	for _, want := range []string{"PRAXIS_MCP_TEST_A=alpha", "PRAXIS_MCP_TEST_B=bravo"} {
		if !got[want] {
			t.Errorf("missing env entry %q in cmd.Env = %v", want, cmd.Env)
		}
	}
}

// TestBuildCommandNilEnvIsNilSlice pins that a nil/empty env map
// flattens to a nil slice, which the Go runtime interprets as
// "child inherits nothing". This is the T31.4 hygiene default for
// the adapter: no implicit inheritance of the parent's environment.
func TestBuildCommandNilEnvIsNilSlice(t *testing.T) {
	t.Parallel()

	name := knownExecutable(t)

	cmdNil, err := BuildCommand(name, nil, nil)
	if err != nil {
		t.Fatalf("BuildCommand (nil env): %v", err)
	}
	if cmdNil.Env != nil {
		t.Errorf("nil env: cmd.Env = %v, want nil", cmdNil.Env)
	}

	cmdEmpty, err := BuildCommand(name, nil, map[string]string{})
	if err != nil {
		t.Fatalf("BuildCommand (empty env): %v", err)
	}
	if cmdEmpty.Env != nil {
		t.Errorf("empty env: cmd.Env = %v, want nil", cmdEmpty.Env)
	}
}

// TestBuildCommandProcessIsolationUnix asserts that on Unix
// targets the returned *exec.Cmd has SysProcAttr.Setpgid set to
// true, per T31.4 and Phase 7 D119 process-group hardening. On
// non-Unix targets the test skips — applyProcessIsolation is a
// no-op and there is no field to check.
func TestBuildCommandProcessIsolationUnix(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skipf("applyProcessIsolation is a no-op on %s", runtime.GOOS)
	}

	name := knownExecutable(t)
	cmd, err := BuildCommand(name, nil, nil)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil; Unix process isolation not applied")
	}
	// Reflection-free check: the syscall.SysProcAttr struct is
	// platform-specific. On Unix the Setpgid field exists; tests
	// compiled for Unix can reference it directly via a type
	// assertion. But the field lives in a tagged package, so a
	// build-tag-gated helper is the cleanest path.
	if !procIsolationSetpgid(cmd) {
		t.Error("SysProcAttr.Setpgid not set on Unix; T31.4 requirement violated")
	}
}
