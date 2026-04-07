# Security Policy

## Supported Versions

Only the latest release in the v0.x series receives security fixes.
Once v1.0.0 is published, the latest v1.x minor release will be supported.

| Version | Supported |
|---------|-----------|
| v0.x (latest) | Yes |
| older v0.x tags | No |

## Reporting a Vulnerability

Report security vulnerabilities using GitHub's private vulnerability reporting
feature. Navigate to the repository's **Security** tab and click
**"Report a vulnerability"**. GitHub's built-in reporting provides encrypted
communication without requiring a PGP key exchange.

Do not open a public GitHub Issue for security-related findings.

## Response Timeline

After you submit a report you can expect:

- **Acknowledgement** within 48 hours.
- **Triage and severity assessment** within 7 days.
- **Fix or mitigation** within 90 days of the initial report (aligned with
  Google Project Zero and CERT/CC industry standards).
- **Public disclosure** after the fix is released, or after 90 days if no fix
  is available.

If a report is accepted, a GitHub Security Advisory will be published once a fix
is released. If a report is declined, you will receive an explanation of why it
falls outside scope.

## Scope

This policy covers the praxis library itself — the code in this repository.

Security issues in caller-provided implementations (custom `PolicyHook`,
`credentials.Resolver`, `identity.Signer`, `AttributeEnricher`, tool handlers,
and similar caller-owned types) are the responsibility of the caller.

### Known Limitations

The following limitations are documented by design. They are not treated as
undisclosed vulnerabilities, but callers with strict requirements should
review them before deploying.

**OI-1 — Private key in-memory lifetime.**
The `ed25519.PrivateKey` held by the built-in `Ed25519Signer` is not zeroed on
garbage collection. Callers with strict key hygiene requirements should use a
KMS- or HSM-backed `identity.Signer` implementation rather than relying on the
built-in one.

**OI-2 — Enricher attribute log-injection vector.**
Values returned by a caller-provided `AttributeEnricher` are included in OTel
spans and lifecycle events. The framework's `RedactingHandler` redacts fields by
key pattern but cannot redact by value. Callers must ensure that enricher values
do not contain sensitive data that would be exposed if spans are exported to an
untrusted backend.

### Out of Scope

The following are outside the scope of this policy:

- Vulnerabilities in third-party dependencies (report those upstream).
- Security issues arising from misconfiguration of a caller's deployment
  environment.
- Theoretical attacks with no practical exploitation path against a correctly
  integrated consumer.
- Issues in caller-supplied hook, filter, resolver, or signer implementations.
