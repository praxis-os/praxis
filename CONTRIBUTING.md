# Contributing to praxis

Thank you for your interest in contributing. This document covers everything you
need to get started: local setup, coding standards, commit conventions, and the
PR process.

## Prerequisites

- Go 1.23 or later
- `make`
- `golangci-lint` (install: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)
- `commitsar` (install: `go install github.com/aevea/commitsar@latest`)

## Local setup

```sh
git clone https://github.com/praxis-go/praxis.git
cd praxis
go mod download
make check
```

`make check` runs lint, tests (with race detector), banned-identifier check, and
SPDX header verification. All checks must pass before opening a PR.

### Available make targets

| Target | What it does |
|---|---|
| `make test` | `go test -race -count=1 ./...` |
| `make lint` | `golangci-lint run ./...` |
| `make vet` | `go vet ./...` |
| `make fmt` | Verify `gofmt` formatting (fails if files need reformatting) |
| `make bench` | Run benchmarks with `-benchmem -count=5` |
| `make cover` | Generate coverage report (`coverage.out`) |
| `make banned-grep` | Check for banned identifiers (decoupling contract) |
| `make spdx-check` | Verify every `.go` file has an SPDX header |
| `make check` | Run all CI gates: lint + test + banned-grep + spdx-check |

## Code style

- Run `gofmt -w .` before committing. The `make fmt` target fails CI if any files
  need reformatting.
- `golangci-lint` must pass without suppressions (`make lint`).
- Every `.go` file must begin with an SPDX header:
  ```go
  // SPDX-License-Identifier: Apache-2.0
  ```
  The `make spdx-check` target enforces this.
- Follow standard Go idioms: accept interfaces, return concrete types; explicit
  error handling; context propagation in all blocking calls.
- Document all exported symbols with godoc comments.

## Testing requirements

- Write table-driven tests with `t.Run` subtests.
- Coverage gate: 85% per package, including `internal/` packages (enforced in CI).
- Tests that exercise concurrent code must pass the race detector (`make test` uses
  `-race` by default).
- Add benchmarks for hot paths (state machine transitions, filter chain dispatch,
  budget checks). Name them `BenchmarkXxx` and run with `make bench`.
- Use `t.Helper()` in shared test utilities.

## Commit message conventions (D83)

All commits must follow [Conventional Commits](https://www.conventionalcommits.org/)
format:

```
<type>(<scope>): <description>
```

Allowed types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `perf`, `ci`,
`build`.

Scope is optional. When provided, it should be the package name, for example:
`budget`, `orchestrator`, `llm/anthropic`.

Breaking changes must be signalled with a `BREAKING CHANGE:` footer — not with a
`!` suffix:

```
feat(orchestrator): add streaming invoke method

BREAKING CHANGE: Invoke signature changed; callers must update to StreamInvoke.
```

`commitsar` validates commit format as a required CI check. Non-conforming commits
will fail the PR gate.

## DCO sign-off requirement (D92)

Every commit must carry a `Signed-off-by` line. Use `git commit -s` to add it
automatically:

```sh
git commit -s -m "feat(budget): add per-token cost accumulator"
```

This produces:

```
feat(budget): add per-token cost accumulator

Signed-off-by: Your Name <your@email.com>
```

By signing off you certify that you wrote the code or have the right to submit
it under the Apache 2.0 license, per the
[Developer Certificate of Origin v1.1](DCO).

The `probot/dco` GitHub App enforces this as a required check on all PRs
targeting `main`. PRs with unsigned commits will not be merged.

## Pull request process (D93)

1. Fork the repository and create a feature branch from `main`.
2. Make your changes, add tests, and verify `make check` passes locally.
3. Open a PR against `main`. Fill in the PR template, including the interface-change
   checkbox if your PR touches any exported symbol.
4. All required CI checks must be green before review:
   - `lint` — golangci-lint
   - `test` — go test with race detector
   - `coverage` — 85% gate per package
   - `banned-grep` — decoupling contract
   - `spdx-check` — license headers
   - `commitsar` — conventional commits
5. Review requirements:
   - **v0.x:** one maintainer approval is required. PRs that change exported
     symbols have a mandatory 24-hour waiting period before merge.
   - **Post-v1.0:** PRs touching `frozen-v1.0` interfaces require two maintainer
     approvals. All other PRs require one approval. Self-merge is not permitted
     for interface-changing PRs.
6. All PRs are squash-merged into `main`.

## Decoupling contract

praxis is a generic library. The following identifiers are banned from all `.go`
and `.md` files outside of designated disclosure sections (see `CLAUDE.md` for
the complete list). The `make banned-grep` target enforces this automatically.

Banned identifiers include consumer brand names and hardcoded identity attributes
(`org.id`, `agent.id`, `user.id`, `tenant.id`). Attribute enrichment is always
caller-provided via `AttributeEnricher` — never baked into the framework.

## Questions and RFCs

For bugs and small improvements, open a GitHub issue.

Post-v1.0, significant API changes follow the RFC process via GitHub Discussions.
During v0.x, open an issue or start a Discussion to propose changes before
submitting a large PR.
