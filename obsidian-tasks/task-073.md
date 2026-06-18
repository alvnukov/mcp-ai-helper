---
id: task-073
title: Реализовать безопасный subset идемпотентных file edits
status: done
priority: critical
model_level: medium
tags:
    - fileops
    - idempotency
    - safety
    - implementation
    - tests
worktree_path: .worktrees/task-073
created_at: "2026-05-14T09:15:25.484384Z"
updated_at: "2026-06-18T09:22:34.849286Z"
---

## Body

Implement the smallest useful subset after task-072, with hash/conflict checks and focused tests. Prefer exact block/append unique/create-if-absent before broader structured patch support.
