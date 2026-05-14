---
id: task-102
title: Добавить preflight для task_transition вне main
status: todo
priority: high
model_level: medium
tags:
  - tasks
  - workflow
  - git
  - worktree
  - lean-registry
  - implementation
  - tests
worktree_path: .worktrees/task-102
created_at: 2026-05-14T09:15:25.488226Z
updated_at: 2026-05-14T09:15:25.488226Z
---

## Body

Implement a preflight warning or fail-closed behavior when task_transition finalizes code tasks inside non-main worktrees, with focused tests for branch-local registry mutation risk.
