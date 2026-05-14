---
id: task_batch_upsert-advertises-string-tasks-but-handler-expects-task-objects
title: task_batch_upsert advertises string[] tasks but handler expects task objects
status: done
priority: high
model_level: medium
tags:
  - issue
  - feedback
  - mcp-schema
  - tasks
  - surface-mismatch
  - task_batch_upsert
  - corrected
worktree_path: .worktrees/task_batch_upsert-advertises-string-tasks-but-handler-expects-task-objects
created_at: 2026-05-14T09:15:25.48263Z
updated_at: 2026-05-14T09:15:25.483341Z
---

## Body

Corrected on 2026-05-13: current exposed MCP schema for task_batch_upsert advertises an array of task objects, matching the handler contract used by this backlog synchronization. Keep closed unless the callable schema regresses to string[].
