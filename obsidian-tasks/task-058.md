---
id: task-058
title: task_packet reads legacy tasks files instead of Lean registry
status: done
priority: high
tags:
    - issue
    - feedback
    - tasks
    - lean-registry
    - planning
    - surface-mismatch
worktree_path: .worktrees/task-058
created_at: "2026-05-14T09:15:25.480852Z"
updated_at: "2026-05-14T11:46:09.789184Z"
---

## Body

Observed while researching task-050 on 2026-05-10: task_get successfully returned task-050 from source=lean_registry, but task_packet for task-050 returned `read task file: open <repo:mcp-ai-helper>/tasks/task-050.lean: no such file or directory`. The same happened for task-048. Expected behavior: task_packet should use the same canonical Lean-backed task store as task_get/task_current, or fail with an explicit surface mismatch diagnostic instead of reading legacy task files. Suggested fix: route planning_tools task fetch through the canonical store/exporter path and add a regression test for Lean-backed task_packet on a task that has no legacy tasks/*.lean projection. Secondary issue seen in this same feedback path: issue_add schema says id is optional, but Lean-backed upsert currently fails without id (`task id is required for Lean task upsert`); either make id required in schema or auto-allocate safely.

source_repo_path: <repo:mcp-ai-helper>
