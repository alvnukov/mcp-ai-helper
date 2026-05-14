---
id: task-042
title: Add end-to-end Lean-backed task layer gate
status: done
priority: critical
tags:
  - tasks
  - lean
  - lake
  - e2e
  - verification
worktree_path: .worktrees/task-042
created_at: 2026-05-14T09:15:25.478407Z
updated_at: 2026-05-14T09:15:25.478407Z
---

## Body

Add the final end-to-end gate that proves the task layer is closed enough for further development. This task should run after task-036 through task-041.

Required scope:
1. Add an end-to-end test or scripted workflow against a fixture or the repo itself that starts from a clean clone-like state.
2. Verify Lake workspace detection, `lake build`, task_current, task_get, task_set_status, task_upsert or task_batch_upsert, and invalid registry diagnostics.
3. Verify that a bad Lean registry fails the gate and a valid registry passes.
4. Verify no task operation silently falls back to legacy storage in this repo once migrated.
5. Document the canonical developer workflow: clone repo, run `lake build`, inspect current tasks through MCP, update a task, rerun validation.

Out of scope:
- No new task features beyond closing the existing layer.
- No provider routing, advanced workflow DSL, git commit hardening, or log retention.
- No visual UI or external tracker sync.
