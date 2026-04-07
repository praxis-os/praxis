---
name: ready-tasks
description: >
  Show which praxis tasks have all Jira dependencies satisfied and are ready to
  implement. Queries Jira for subtask statuses and checks "Blocks" issue links.
  Use to decide what to work on next, or filter by milestone (v0.1.0, v0.3.0, etc.).
---

# Ready Tasks

Show which praxis implementation tasks are ready to work — all blocking dependencies
satisfied in Jira.

## Input

Optional milestone filter as argument: `v0.1.0`, `v0.3.0`, `v0.5.0`, `v1.0.0`.
If omitted, show all milestones.

## Jira Configuration

- **Cloud ID:** `dbf84607-febc-42ec-87a9-df0f3d926c0b`
- **Project:** `PRAX`
- **Link type:** `Blocks` (inward = "is blocked by", outward = "blocks")

## Confluence Configuration

- **Cloud ID:** `dbf84607-febc-42ec-87a9-df0f3d926c0b`
- **Space ID:** `196612` (key: `Praxis`)
- **Readiness page ID:** `425985`
- **Readiness page URL:** `https://francescofioredev.atlassian.net/wiki/spaces/Praxis/pages/425985/Task+Readiness+Map+v0.1.0`

## Procedure

### 1. Query open subtasks

Use `mcp__claude_ai_Atlassian_Rovo__searchJiraIssuesUsingJql` with:

```
project = PRAX AND issuetype = Sub-task AND status != Done
```

If a milestone filter is provided, add: `AND labels = "milestone:<version>"`.

### 2. Check dependencies for each task

For each subtask returned, use `mcp__claude_ai_Atlassian_Rovo__getJiraIssue` with
`fields: ["issuelinks", "summary", "status", "priority", "labels"]` to fetch
the issue including its dependency links.

A task is **ready** if ALL of the following are true:
- Status is "To Do" (not already "In Progress")
- Every inward "is blocked by" link points to an issue with status "Done"
- If no "is blocked by" links exist, the task has no blockers and is ready

A task is **blocked** if any inward "is blocked by" link points to an issue that
is NOT "Done".

### 3. Output format

Present results as a markdown table grouped by Story:

```
## Ready Tasks — <milestone or "All Milestones">

### <Story key>: <Story title>
| Task | Key | Priority | Title |
|------|-----|----------|-------|
| T1.1 | PRAX-37 | Highest | Create go.mod with confirmed module path |

### Blocked Tasks (next wave)
| Task | Key | Blocked by | Blocker status |
|------|-----|------------|----------------|
| T1.2 | PRAX-38 | PRAX-37 (T1.1) | To Do |
```

### 4. Suggest execution order

After the tables, suggest which ready tasks to work first based on:
1. **Priority** (Highest first)
2. **Unblock count** — tasks that unblock the most other tasks should go first
3. **Critical path** — tasks on the longest dependency chain

Use the mapping in `.claude/skills/work-tasks/jira-map.md` to resolve Jira keys
back to task IDs (T1.1, T2.1, etc.) for display.

### 5. Update Confluence page

After producing the output, update the Confluence readiness page using
`mcp__claude_ai_Atlassian_Rovo__updateConfluencePage` with the page ID from the
Confluence Configuration section above. This keeps the living document in sync
without requiring a manual copy-paste.

## Guardrails

- Read-only skill on Jira — never modify Jira issues
- If Jira API is unreachable, report the error and stop
- Do not show "Done" tasks unless explicitly requested
