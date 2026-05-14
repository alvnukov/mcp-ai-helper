---
id: task-056
title: Закрепить запрет Go-side Lean registry parsing и mutation
status: done
priority: high
tags:
  - tasks
  - lean-registry
  - lake-server
  - hardening
  - quality
  - llm-strong
  - type-verification-strong
worktree_path: .worktrees/task-056
created_at: 2026-05-14T09:15:25.480573Z
updated_at: 2026-05-14T09:15:25.480573Z
---

## Body

Hardening gate после server-backed read/mutation решений. Добавить regression coverage, guidance и planning packet constraints, которые не позволяют вернуть прямой Go parsing/mutation Lean registry source как production path. Это отдельный quality gate перед повторным закрытием task-047, а не часть первого implementation slice.
