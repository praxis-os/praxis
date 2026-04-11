// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"fmt"
	"os/exec"

	"github.com/praxis-os/praxis/credentials"
)

// InjectEnvCredential appends a `<envName>=<token>` entry to the
// given [*exec.Cmd]'s environment, where <token> is the string
// projection of the caller-supplied credential bytes, and then
// zeros the caller's byte slice via [credentials.ZeroBytes] so
// the adapter does not retain a mutable, mutable-memory copy of
// the secret material beyond the Env assignment.
//
// # Buffer lifecycle contract (T31.3, Phase 7 §4.2)
//
// The `cred` argument MUST be a fresh, adapter-owned byte slice
// made via `append([]byte(nil), credential.Value()...)` or an
// equivalent copy. Callers MUST NOT pass the raw
// `credentials.Credential.Value` slice directly: the Resolver owns
// that buffer and is responsible for zeroing it via
// `Credential.Close`. Zeroing the Resolver-owned buffer from this
// function would corrupt the Resolver's bookkeeping and is a
// protocol violation.
//
// After this function returns successfully, the only in-process
// copies of the credential bytes are:
//
//  1. An immutable Go string in `cmd.Env[len(cmd.Env)-1]` — this
//     is the OI-MCP-1 residual risk (see package doc of
//     github.com/praxis-os/praxis/mcp). The Go runtime holds the
//     string in the heap until the next GC pass collects it; the
//     adapter cannot zero immutable strings.
//  2. Whatever the Go runtime buffers internally during `cmd.Start`
//     (the exact path is implementation-defined — on Unix the
//     runtime serialises the env block into kernel memory via
//     `execve`, at which point the parent-side copy becomes
//     collectible).
//
// Neither residual is under the adapter's direct control. The
// T31.3 acceptance criterion is scoped to the buffer the adapter
// OWNS, which is the `cred` input, and which this function zeros
// before returning.
//
// # When to call
//
// InjectEnvCredential SHOULD be called immediately before the
// session pool passes `cmd` to [sdkmcp.CommandTransport] and then
// to [sdkmcp.Client.Connect]. Zeroing happens inline with this
// function, not deferred — the buffer is no longer needed once
// the Env string has been materialised, which happens on the
// `string(cred)` conversion inside this function body.
//
// # Errors
//
// Returns a typed [praxiserrors.SystemError] with
// [praxiserrors.ErrorKindSystem] if either envName is empty or
// cred is nil/empty. Both are framework/configuration errors that
// indicate the caller (praxis/mcp's session pool) reached this
// helper with a malformed server specification; they should not
// be reachable from well-formed Server values that passed New's
// validation matrix.
func InjectEnvCredential(cmd *exec.Cmd, envName string, cred []byte) error {
	if envName == "" {
		return systemError("transport: credential env var name is empty (TransportStdio.CredentialEnv must be set when Server.CredentialRef is non-empty)")
	}
	if len(cred) == 0 {
		return systemError(fmt.Sprintf(
			"transport: credential bytes for env %q are empty; the resolver returned no material",
			envName,
		))
	}

	// Build the env entry. The `string(cred)` conversion allocates
	// a fresh immutable Go string; after this statement the
	// contents of `cred` are no longer referenced by anything the
	// exec package can observe. The string held by cmd.Env is the
	// OI-MCP-1 residual copy.
	entry := envName + "=" + string(cred)
	cmd.Env = append(cmd.Env, entry)

	// Zero the adapter-owned buffer in place. Any Go runtime code
	// that still holds a slice header pointing at the backing
	// array will observe an all-zero buffer from this point
	// forward. credentials.ZeroBytes uses a runtime.KeepAlive
	// fence internally so the compiler cannot elide the zeroing
	// writes (Phase 5 D67).
	credentials.ZeroBytes(cred)

	return nil
}
