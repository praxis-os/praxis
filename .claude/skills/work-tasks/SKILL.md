---
name: work-tasks
description: >
  Implement one or more praxis tasks by Jira key (PRAX-37) or task ID (T1.1).
  Checks Jira "Blocks" links for dependencies, transitions issues through the
  workflow, writes Go code following project conventions, runs tests, and commits
  atomically. Use when ready to implement specific tasks from the Jira backlog.
---

# Work Tasks

Implement praxis tasks end-to-end: dependency check, Jira transitions, Go code,
tests, and atomic commits.

## Input

Space-separated Jira keys or task IDs:
- `/work-tasks PRAX-37 PRAX-42`
- `/work-tasks T1.1 T2.1`
- `/work-tasks PRAX-37 T2.1` (mixed)

## Jira Configuration

- **Cloud ID:** `dbf84607-febc-42ec-87a9-df0f3d926c0b`
- **Project:** `PRAX`
- **Link type:** `Blocks` (inward = "is blocked by")

## Phase 1: Parse and Resolve

### Resolve task IDs to Jira keys

- If argument matches `PRAX-\d+`: use directly
- If argument matches `T\d+\.\d+`: look up in `.claude/skills/work-tasks/jira-map.md`
- If not found in map, search Jira with JQL:
  `project = PRAX AND issuetype = Sub-task AND summary ~ "T<X>.<Y>:"`

### Fetch issue details

For each resolved key, call `mcp__claude_ai_Atlassian_Rovo__getJiraIssue` to get:
- Summary, description, acceptance criteria
- Status (must be "To Do" to proceed)
- Labels (for `area:*` and `milestone:*`)
- Issue links (for dependency check)

If a task is already "Done", skip it with a message. If "In Progress", warn and
ask the user whether to continue.

## Phase 2: Dependency Check

For each task, inspect `issuelinks` of type "is blocked by":
- Fetch each blocker's status
- If ANY blocker is NOT "Done", the task is **blocked**

**If blocked:** Report which blockers are not done. Suggest running those first.
Do NOT proceed with blocked tasks.

**If requesting multiple tasks with inter-dependencies:** Automatically sequence
them. Example: if the user requests T1.1 and T1.2, and T1.2 depends on T1.1,
execute T1.1 first, then T1.2 after T1.1 completes.

## Phase 3: Execution Plan

Before starting implementation, present the plan:

```
## Execution Plan

### Sequential (dependency chain):
1. PRAX-37 (T1.1) — Create go.mod [area:build]

### Parallel (independent):
- PRAX-42 (T2.1) — Define State type [area:state-machine]
- PRAX-52 (T4.1) — TypedError interface [area:errors]

Proceed? [Y/n]
```

Wait for user confirmation before executing.

## Phase 4: Implementation (per task)

### 4.1 Transition to "In Progress"

Use `mcp__claude_ai_Atlassian_Rovo__getTransitionsForJiraIssue` to find the
transition ID for "In Progress", then call
`mcp__claude_ai_Atlassian_Rovo__transitionJiraIssue`.

### 4.2 Gather context

Read these sources for implementation guidance:
1. The Jira issue description and acceptance criteria
2. The task's full specification in `docs/jira-decomposition.md`
3. Relevant phase documents in `docs/phase-*` directories
4. Decision references (D01, D02, etc.) mentioned in the task
5. `CLAUDE.md` for project conventions

### 4.3 Implement

Follow the `golang-pro` agent patterns (`.claude/agents/golang-pro.md`):
- **SPDX header:** `// SPDX-License-Identifier: Apache-2.0` on every `.go` file
- **gofmt** compliance
- **Context propagation** in all APIs
- **Error handling** with wrapping (`fmt.Errorf("...: %w", err)`)
- **Table-driven tests** with subtests
- **Godoc** on every exported type, function, method, and constant
- **Race-free** code (safe for `go test -race`)
- **Accept interfaces, return structs**
- **Package naming:** follow the layout in design docs (e.g., `orchestrator/`,
  `llm/`, `errors/`, `hooks/`, `budget/`, `telemetry/`, `credentials/`,
  `identity/`, `tools/`, `internal/`)

### 4.4 Verify

Run in sequence:
1. `go build ./...` — must succeed
2. `go vet ./...` — must succeed
3. `go test ./...` — must pass
4. `go test -race ./...` — must pass (if applicable)

If any step fails, stop. Do NOT transition to "Done". Report the error and leave
the task "In Progress" for the user to investigate.

### 4.5 Commit

Create an atomic commit with conventional commit format:

```
feat(<area>): <concise summary> [PRAX-XX]

<body: what was implemented and why>

Refs: PRAX-XX
```

Where `<area>` is derived from the task's `area:*` label:
- `area:build` → `build`
- `area:state-machine` → `state-machine`
- `area:orchestrator` → `orchestrator`
- `area:errors` → `errors`
- `area:llm` → `llm`
- `area:defaults` → `defaults`
- `area:docs` → `docs`
- `area:quality` → `quality`
- `area:testing` → `test`
- `area:ci` → `ci`
- `area:streaming` → `streaming`
- `area:hooks` → `hooks`
- `area:filters` → `filters`
- `area:budget` → `budget`
- `area:telemetry` → `telemetry`
- `area:identity` → `identity`
- `area:credentials` → `credentials`
- `area:security` → `security`
- `area:benchmarks` → `bench`
- `area:examples` → `examples`
- `area:governance` → `governance`
- `area:api` → `api`
- `area:cancellation` → `cancel`

### 4.6 Transition to "Done"

Use `mcp__claude_ai_Atlassian_Rovo__getTransitionsForJiraIssue` to find the
transition ID for "Done", then call
`mcp__claude_ai_Atlassian_Rovo__transitionJiraIssue`.

## Phase 5: Parallel Execution

If multiple independent tasks are requested (no dependency between them):
- Launch each task as a separate `Agent` instance using `subagent_type: "golang-pro"`
  with `isolation: "worktree"` for file-level isolation
- Provide each agent with the full task context (Jira description, design docs,
  conventions)
- After all agents complete, merge worktree changes
- Verify no file conflicts

**Important:** Only parallelize tasks that:
1. Have zero "Blocks" links between them
2. Are likely to modify different packages/files
3. Are all in "To Do" status with all dependencies met

When in doubt, run sequentially.

## Guardrails

- **Never mark "Done" without green tests** — if tests fail, leave "In Progress"
- **SPDX header** on every `.go` file — `// SPDX-License-Identifier: Apache-2.0`
- **Decoupling contract** — never introduce banned identifiers (custos, reef,
  governance_event, hardcoded org.id/agent.id/user.id/tenant.id)
- **One commit per task** — atomic, traceable back to Jira key
- **No scope creep** — implement exactly what the task specifies, nothing more
- **Acceptance criteria** — every criterion in the Jira issue must be satisfied
- **Ask before proceeding** — always show the execution plan and wait for
  confirmation
