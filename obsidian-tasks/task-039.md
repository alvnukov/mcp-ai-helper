---
id: task-039
title: Switch MCP task read tools to Lean registry projection
status: done
priority: critical
tags:
  - mcp
  - tasks
  - lean
  - lake
  - canonical
worktree_path: .worktrees/task-039
created_at: 2026-05-14T09:15:25.478004Z
updated_at: 2026-05-14T09:15:25.478004Z
---

## Body

Switch MCP task read paths to use the Lean/Lake registry exporter when a repo-local Lake project is present. Legacy task-store reads become fallback only, not canonical, for repositories that have not migrated.

Required scope:
1. Update task_current and task_get to prefer the task-037 Lean exporter for repos with a valid Lake workspace and exporter target.
2. Keep legacy read fallback only when no Lean workspace/exporter exists, with clear output metadata or logs showing fallback mode.
3. Preserve existing response JSON shapes so existing agents do not break.
4. Ensure task_current filters active statuses consistently with the old behavior.
5. Ensure task_get returns a clear not-found/blocker when the Lean exporter does not contain the id.
6. Add focused tests for Lean-backed read, fallback legacy read, and exporter failure.

Out of scope:
- No task mutation tools in this task.
- No Go-side Lean parsing.
- No removal of legacy task files yet.
