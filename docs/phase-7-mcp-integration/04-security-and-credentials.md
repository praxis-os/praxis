# Phase 7 — Security and Credentials

**Decisions:** D116, D117, D118, D119
**Cross-references:** Phase 5 `02-credential-lifecycle.md` (zeroing,
soft-cancel), Phase 5 `03-identity-signing.md` (JWT claim set),
Phase 5 `04-trust-boundaries.md` (untrusted tool output, D77–D79),
Phase 3 `09-credentials-and-identity.md` (frozen Resolver/Signer),
03-integration-model.md (adapter API).

---

## 1. Trust boundary classification (D116)

The MCP transport edge — the connection between the praxis process
and an MCP server — is a **trust boundary**. Tool output received
from an MCP server is **untrusted by contract**, identically to any
other `tools.Invoker` output.

Phase 5 D77 already classifies all `ToolResult.Content` as untrusted.
Phase 7 confirms this classification extends to MCP-sourced content
**without modification**. No new filter or hook interface is needed.
`PostToolFilter` continues to be the seam where caller-supplied
logic inspects and blocks untrusted content.

### 1.1 Why no special-casing for MCP

A superficially reasonable proposal would be to tag MCP-sourced
content as "more untrusted than local tool output" and expose the
tag to `PostToolFilter`. Phase 7 rejects this:

1. **All tool output is untrusted.** The Phase 5 threat model does
   not rank tool outputs by trust level; it treats them all as
   potentially hostile. Ranking introduces a false-hierarchy risk
   where callers who trust "local" tools skip filtering and miss a
   prompt-injection in a compromised local binary.
2. **Filters should be content-based, not source-based.** A good
   `PostToolFilter` looks at the content, not at which invoker
   produced it. A "source = mcp" tag would tempt filter authors to
   write source-gated logic that misses injections from other
   sources.
3. **The adapter cannot certify trustworthiness.** Whether an MCP
   server is trusted is a consumer-side decision. Building a
   two-tier trust model into the framework would require the
   framework to take a position it cannot verify.

### 1.2 Filter implementor guidance (informative)

`PostToolFilter` implementations that run in deployments using the
MCP adapter should, in their own documentation:

- Remind readers that any MCP server — especially third-party — can
  return attacker-crafted content, including prompt-injection
  payloads explicitly constructed to subvert the LLM.
- Recommend that filters not rely on tool-name allowlisting alone
  (because namespaced MCP tool names are caller-configurable).
- Recommend that filter implementations exercise MCP-specific test
  fixtures as part of their CI suite.

This is caller documentation. The framework does not change.

**D116** — The MCP transport edge is a trust boundary. MCP-sourced
content is classified as untrusted by the existing Phase 5 D77
contract without modification. No new filter, hook, or trust tier
is added.

## 2. Credential flow (D117)

Phase 5 D69 establishes "credentials are fetched per tool call and
closed before GC". MCP sessions are long-lived — opening a session
typically requires an auth material once, not per tool call. This
is a **tension**, not a contradiction: the Phase 5 contract governs
the framework's relationship with `credentials.Resolver`, while the
MCP session lifetime is an adapter-internal concern.

The adapter resolves the tension as follows:

### 2.1 Session-credential pattern

1. At `New` time, the adapter does **not** fetch credentials.
   Construction only validates that each `Server.CredentialRef` is
   well-formed; it does not call `Resolver.Fetch`.
2. On the **first** `Invoker.Invoke` call routed to a given server,
   the adapter opens the MCP session:
   - Calls `Resolver.Fetch(ctx, server.CredentialRef)` inside the
     tool-call goroutine.
   - Uses the returned `Credential.Value()` to authenticate the
     session (as a bearer header for HTTP; as an env var for
     stdio).
   - Calls `Credential.Close()` **immediately after the session is
     established**, before the `Invoke` call returns the first tool
     result.
3. Subsequent `Invoker.Invoke` calls to the same server use the
   already-open session. No re-fetch. No credential-handle retention.
4. When the session is torn down (explicit `Invoker.Close`, a
   `ToolSubKindCircuitOpen` cool-down, or process exit), the MCP
   transport-level session closes. There is no credential to zero
   at this point — the credential was zeroed when `Credential.Close`
   was called in step 2.

### 2.2 Why this preserves the Phase 5 invariants

| Phase 5 invariant | How §2.1 preserves it |
|---|---|
| **Zero-on-close:** `Credential.Close()` zeros secret material | Called after the session is established. The secret bytes are zeroed before the adapter retains the session handle. |
| **No caching across calls inside the framework** | The adapter never retains a `Credential` handle. It retains the MCP session handle, which is not a `Credential`. The MCP session's internal copy of the auth material is owned by the MCP SDK and is outside praxis's zeroing contract. |
| **Per-tool-call fetch semantics** | For the _first_ call to a server, the Resolver is called per Phase 5 contract. For subsequent calls, no Resolver call happens — the session's cached auth handles authentication. |
| **Credential never in InvocationContext** | Unchanged. The adapter reads the credential in its own goroutine and does not place it into any shared state. |

The compromise is that **the MCP SDK's internal auth cache is outside
praxis's zero-on-close guarantee.** This is unavoidable: once the
credential is handed to the MCP SDK for bearer-header construction,
the SDK may retain it for the session's lifetime. The adapter
documents this clearly in godoc: consumers with FIPS-level
in-memory-secret requirements must either use short-lived credentials
or implement a custom `credentials.Resolver` that returns a proxy
token rather than the real secret.

### 2.2.1 Accepted deviation from Phase 5 §3.2 (HTTP transport only)

Phase 5 `02-credential-lifecycle.md` §3.2 states a structural
**goroutine-scope isolation invariant**: "The `Credential` value is
used only within the goroutine that received it from `Resolver.Fetch`.
It is not passed to other goroutines."

The HTTP transport path in the MCP adapter breaches this invariant.
Once the bearer token is handed to the underlying HTTP client, the
client's connection-pool and keep-alive goroutines read the token
during connection reuse, re-authentication, and health-check paths.
This is unavoidable for any HTTP client that supports connection
reuse, and connection reuse is required for latency-sensitive
production use.

The stdio transport path does **not** breach the invariant — the
credential bytes are consumed inside the spawning goroutine and
never cross a goroutine boundary within the praxis process before
being zeroed.

**This is a separate concern from the D67 zero-on-close boundary
discussed elsewhere in this document.** D67 specifies when credential
bytes are erased; Phase 5 §3.2 specifies which goroutines may observe
them while still live. The two invariants are related but distinct.
The MCP adapter preserves D67 (up to the SDK boundary) but
structurally cannot preserve the Phase 5 §3.2 goroutine-scope
invariant for HTTP transport.

**Acceptance rationale.** Closing this gap would require either
dropping HTTP connection reuse (a non-starter for latency) or
adopting a request-scoped auth model that the MCP spec does not
offer. The breach is classified at the same "acceptable risk"
tier as the D67 §4.3 GC-timing statement. Consumers with strict
goroutine-scope isolation requirements either use stdio transport
exclusively or interpose a KMS-backed proxy-token pattern at the
`credentials.Resolver` layer (a caller decision, not a framework
change).

This deviation is formally recorded as an amendment to **D117**
in `01-decisions-log.md`.

### 2.3 Soft-cancel credential resolution

Phase 5 D69 specifies that during a soft-cancel grace window, the
adapter passes a `context.WithoutCancel`-derived context with a
500 ms deadline to `Resolver.Fetch`. The MCP adapter inherits this
behaviour verbatim: if the first-call session-open happens during a
soft-cancel window, the same context-derivation rule applies to
`Resolver.Fetch`.

However: the MCP session handshake itself may take longer than the
500 ms grace window. If the handshake does not complete within
500 ms of the grace window start, the adapter abandons the session
open and returns a `ToolResult{Status: ToolStatusError, Err: ...}`
with `ErrorKindTool`/`ToolSubKindNetwork`. The credential, if
already fetched, is `Close()`d before the return.

**D117** — The adapter fetches credentials via `credentials.Resolver`
on the first `Invoke` call to each server, uses them to open the
session, and calls `Credential.Close()` immediately after the
session is established. Subsequent calls reuse the session without
re-fetching. Soft-cancel rules from Phase 5 D69 apply to the fetch.

### 2.4 Credential refresh

MCP sessions may require periodic credential refresh (e.g., OAuth
access token rotation). **v1.0.0 does not ship credential refresh.**
If a session's credential expires, the next MCP call fails with a
transport-level auth error. The adapter maps this to
`ToolSubKindCircuitOpen` (since the session is effectively dead) and
on the next cool-down the session is re-opened with a fresh
`Resolver.Fetch`.

This is correct but potentially latency-spiky. v1.x may add explicit
refresh support via an optional `CredentialRefresher` interface. That
is not part of the v1.0 freeze.

## 3. `SignedIdentity` propagation policy (D118)

The core orchestrator populates `tools.InvocationContext.SignedIdentity`
with an Ed25519 JWT produced by `identity.Signer`. For local tools,
forwarding this JWT as a Bearer token to downstream services is the
canonical pattern. For MCP, the question is whether to do the same.

**Decision: No automatic forwarding to MCP servers in v1.0.0.**

### 3.1 Rationale

1. **Trust model mismatch.** An MCP server is an external process
   that the praxis consumer does not necessarily control. Forwarding
   a signed identity JWT to an arbitrary MCP server — which may log,
   replay, or exfiltrate the token — is a credential disclosure risk
   that should require explicit caller opt-in.
2. **Spec silence.** The MCP specification does not define a
   standard header or JSON-RPC field for "invoking agent identity"
   as distinct from session authentication. Shipping a
   praxis-specific convention would create a de-facto standard the
   project has no authority to set.
3. **Filterability.** Phase 5 D79 adds `praxis.signed_identity` to
   the `RedactingHandler` deny-list to catch accidental JWT logging.
   Forwarding the JWT to an MCP server is exactly the kind of
   accidental exposure the deny-list is defence-in-depth against.
4. **Opt-in escape hatch.** Consumers who do want to forward
   `SignedIdentity` build their own `tools.Invoker` that wraps the
   MCP adapter. Since the wrapping invoker has access to
   `InvocationContext.SignedIdentity` and the raw MCP client, this
   is a ~20-line implementation. praxis does not need to ship it.

### 3.2 What the adapter does with `SignedIdentity`

Nothing. The adapter reads `InvocationContext.SignedIdentity` only
to populate the `praxis.mcp.toolcall` span as... nothing, because
span attributes must not carry the JWT (Phase 5 §3.1). The adapter
does not:

- Add the JWT to any HTTP header.
- Add the JWT to any stdio environment variable.
- Add the JWT to any MCP JSON-RPC request field.
- Log the JWT at any level.
- Include the JWT in any metric label.

The JWT remains readable from `InvocationContext` for consumer-built
wrapping invokers, but the framework-shipped MCP adapter ignores it.

**D118** — `tools.InvocationContext.SignedIdentity` is **not**
forwarded to MCP servers by the v1.0.0 adapter. Consumers who need
identity-chain propagation to MCP servers build a wrapping invoker
themselves. The framework's `RedactingHandler` deny-list (Phase 5
D79) protects against accidental logging of the JWT inside the MCP
adapter code path.

## 4. Stdio transport trust properties (D119)

stdio transport has a distinct threat profile from HTTP because the
MCP server runs as a child process of the praxis process. Three
specific security properties govern how the adapter handles stdio:

### 4.1 Command path resolution

`TransportStdio.Command` must be either an **absolute path** or a
name that `exec.LookPath` can resolve at `New` time. `New` calls
`exec.LookPath` and records the resolved absolute path internally;
the resolved path is used for every subsequent child-process launch
(if the session needs restarting after a circuit-open cool-down).

**Rationale.** `$PATH` lookup at every restart opens a TOCTOU
window: an attacker who can write to a directory earlier in `$PATH`
between the initial `New` and a later restart could inject a
malicious binary. Resolving once at `New` and caching the absolute
path closes this window.

### 4.2 Credential delivery via env

When a credential is attached to an stdio server
(`Server.CredentialRef` non-empty), the adapter:

1. Fetches the credential via `Resolver.Fetch`.
2. Copies `Credential.Value()` into a fresh byte slice.
3. Sets the `TransportStdio.CredentialEnv` key of the child
   process's environment to the copied value.
4. Spawns the child process.
5. Zeroes the copied byte slice and calls `Credential.Close()`.

The child process inherits the environment block at spawn time.
After the copy is zeroed and `Close()` is called, the praxis-process
memory no longer holds the secret; the child process holds it in
its own address space, which is the caller's trust boundary (the
caller chose to launch the child).

**`os.exec.Cmd.Env`'s backing storage.** Go's `exec.Cmd.Env` is a
`[]string` of `key=value` entries. Once `Cmd.Start` has returned,
the `Env` slice may still reference the secret string. The adapter
zeros its own copy of the byte slice; the `Cmd.Env` string is
unaffected by this zeroing because strings are immutable. The
adapter must **not** construct the env var by concatenating the
credential byte slice into a Go string — once the concatenation
happens, the secret is in an immutable string that cannot be zeroed.

The correct pattern is: build the env var as a new `[]byte` buffer
that the adapter owns, convert to string only in a narrow scope
that is discarded after `Cmd.Start`, and zero the `[]byte` buffer.
This is the intended implementation; it is documented in the Phase
7 non-goals as a known imperfect zeroing boundary (§6 below).

### 4.3 Child process lifetime and stdio privilege

The child process inherits the parent's file descriptors by default
under Go's `os/exec`. The adapter must explicitly:

- Set `Cmd.Stdin`, `Cmd.Stdout`, `Cmd.Stderr` to pipes (not inherit
  the parent's stdio), to prevent the child from writing to the
  parent's terminal or receiving the parent's stdin.
- Not set `Cmd.ExtraFiles` — no additional file descriptors are
  passed to the child.
- Set a process group (`Cmd.SysProcAttr.Setpgid = true` on Unix) so
  that the child can be killed as a group on `Invoker.Close`.

### 4.4 SIGPIPE handling

Writes to the child's stdin pipe must handle `EPIPE` cleanly. When
the child process exits unexpectedly and the praxis process then
writes to the now-closed pipe, the OS returns `EPIPE`; Go may also
deliver SIGPIPE to the writing goroutine. For pipes created via
`os/exec`, Go does not install the default SIGPIPE-ignoring
handler that it uses for the main goroutine's stdio, so pipe
writes must be wrapped to convert `EPIPE` / `io.ErrClosedPipe`
into a transport-level error. The adapter maps the resulting
error to `ErrorKindTool`/`ToolSubKindNetwork` and triggers the
session circuit-open cool-down.

### 4.5 Child resource constraints

The adapter does not impose OS-level resource limits
(`setrlimit`, cgroups) on the child process in v1.0.0 — doing so
cleanly across platforms is out of scope for the initial release.
The adapter's godoc warns operators that a misbehaving MCP server
binary can exhaust the parent process's file-descriptor table or
memory if it leaks descriptors or holds unbounded buffers.
Operators running untrusted MCP binaries are advised to execute
them under an external supervisor (systemd, launchd, Docker) that
enforces resource limits. A v1.x amendment may add a
`WithChildRLimits(rl RLimits) Option` once a portable API shape
is agreed.

**D119** — stdio transport requires: absolute command-path
resolution at `New` time (cached for restarts); credential delivery
via a privately-owned env var buffer that is zeroed after
`Cmd.Start`; explicit stdio redirection to pipes; no extra file
descriptors; process-group isolation so that `Invoker.Close`
terminates the child; `EPIPE` / SIGPIPE handling on every pipe
write; and a documented operator obligation to run untrusted MCP
binaries under an external supervisor that enforces resource
limits.

## 5. HTTP transport trust properties

HTTP transport is simpler from a process-isolation standpoint (no
child process) but has its own security surface:

1. **TLS.** The adapter requires `https://` URLs for production use.
   `http://` URLs are accepted (for local testing) but the adapter
   logs a WARN lifecycle event (`EventTypeMCPInsecureTransport`) at
   session-open time. This is a warning, not an error — the spec
   does not prohibit plaintext HTTP and the adapter is not
   opinionated enough to block it.
2. **Certificate validation.** The adapter uses Go's default TLS
   verification. No TOFU, no pinning, no `InsecureSkipVerify`.
   Consumers with pinning requirements supply a custom
   `http.RoundTripper` through... actually, no — v1.0.0 does not
   expose a custom `http.Client` option. Consumers who need
   pinning use a wrapping invoker pattern or wait for a v1.x
   amendment.
3. **Bearer token delivery.** The credential bytes are placed in the
   `Authorization: Bearer <value>` header on every outgoing request.
   The adapter owns the request-construction path and zeros its
   working copy of the credential slice immediately after the
   underlying SDK has consumed it. As with stdio, this zeroing is
   imperfect once the byte slice has been converted to a Go string
   — see §6.

## 6. Known imperfect zeroing boundary

The zero-on-close contract (Phase 5 D67) is enforced by the framework
up to the boundary where the credential is handed to a third-party
library (the MCP Go SDK) or placed into Go-immutable storage (a
string, an `exec.Cmd.Env` entry, or an `http.Header` value).

**Beyond that boundary, zeroing is the third-party library's
responsibility**, which in practice means it does not happen: neither
`net/http` nor typical SDK clients zero header values after use.

The adapter documents this boundary in godoc:

> **Credential zeroing boundary.** The adapter zeros its own copies
> of credential byte slices immediately after the credential has
> been consumed by the underlying MCP client. Credential material
> that has crossed into the `net/http` stack, `os/exec.Cmd.Env`, or
> any SDK-owned storage is not zeroed by praxis. Consumers with
> strict memory-residency requirements should use short-lived
> credentials or route through a KMS-backed `credentials.Resolver`
> that returns short-lived proxy tokens.

This is consistent with the Phase 5 D67 §4.3 "acceptable risk"
statement for GC timing and memory pages.

**No new decision** — this is an informative restatement of the
Phase 5 acceptable-risk position as it applies to the MCP adapter.

## 7. What Phase 5 does not change

Phase 7 introduces no amendments to Phase 5. The following are
preserved verbatim:

- **D67 (zero-on-close).** Adapter code calls `Credential.Close()`
  to trigger zeroing; the Phase 5 `credentials.ZeroBytes` utility is
  available and should be used for any adapter-owned buffers.
- **D68 (`ZeroBytes` utility).** Used by the adapter for its env-var
  buffer.
- **D69 (soft-cancel credential fetch).** Applied to the first-call
  session-open path without modification.
- **D77 (untrusted tool output).** Applied to MCP-sourced content
  without modification.
- **D78 (filter trust classification).** `PostToolFilter` continues
  to be classified as trust-boundary-crossing for MCP content; no
  new classification tier is added.
- **D79 (`RedactingHandler` deny-list additions).** Unchanged;
  `praxis.signed_identity` and `_jwt` suffix deny-list entries
  protect against accidental JWT leakage inside the adapter.

Phase 7 **augments** Phase 5 only insofar as it specifies how the
existing Phase 5 primitives apply to the MCP adapter. No new
interfaces, no new deny-list entries, no new threat categories.

## 8. Decisions (summary)

| ID | Subject | Outcome |
|---|---|---|
| D116 | Trust classification of MCP transport edge | Trust boundary; MCP-sourced content handled by existing Phase 5 D77 without modification |
| D117 | Credential flow for long-lived sessions | First-call fetch + zero + session-reuse; soft-cancel rules from D69 apply; **Phase 5 §3.2 goroutine-scope isolation is explicitly breached for HTTP transport and accepted as a deviation** |
| D118 | `SignedIdentity` propagation to MCP | **Not forwarded** by v1.0.0 adapter; consumer-built wrapping invoker is the escape hatch |
| D119 | stdio transport hardening | Absolute command resolution, env-var buffer zeroing, pipe redirection, process-group isolation, `EPIPE`/SIGPIPE handling, operator obligation to supervise untrusted binaries |

Full decision text in `01-decisions-log.md`.

---

**Next:** `05-non-goals.md` enumerates the items Phase 7 explicitly
declines to own at v1.0.0.
