---
id: task-100
title: Реализовать persisted workflow records/status lookup
status: todo
priority: high
model_level: medium
tags:
  - workflow
  - timeout
  - observability
  - implementation
  - tests
worktree_path: .worktrees/task-100
created_at: 2026-05-14T09:15:25.488008Z
updated_at: 2026-05-14T09:15:25.488008Z
---

## Body

Implement the durable result/status contract from task-099 with a regression test where client wait budget is exceeded but final result remains inspectable without rerunning workflow.
