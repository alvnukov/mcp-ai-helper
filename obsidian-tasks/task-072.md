---
id: task-072
title: Зафиксировать контракты structured file edit operations
status: done
priority: critical
model_level: medium
tags:
    - fileops
    - idempotency
    - safety
    - design
worktree_path: .worktrees/task-072
created_at: "2026-05-14T09:15:25.484242Z"
updated_at: "2026-06-18T09:20:01.591155Z"
---

## Body

Design the safe edit contract for structured patch, create-if-absent, replace-by-marker, append-unique, delete-exact-block, JSON/YAML edits, dry-run diff, hashes, and conflict policy. Output must be an implementation-ready matrix of operations and failure modes.
