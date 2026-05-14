---
id: task-054
title: Сделать минимальный read-only vertical slice через lake --server
status: done
priority: critical
tags:
  - tasks
  - lean-registry
  - lake-server
  - read-slice
  - llm-strong
  - type-implementation-strong
worktree_path: .worktrees/task-054
created_at: 2026-05-14T09:15:25.480267Z
updated_at: 2026-05-14T09:15:25.480267Z
---

## Body

Implementation spike после task-052/task-053. Реализовать самый маленький read-only путь через server-backed registry service, например task_get для одной задачи, чтобы проверить protocol end-to-end. Не переводить весь task surface. Slice должен вернуть typed structured fields, включая acceptance_criteria и verification_plan, либо дать fail-closed blocker если server contract непригоден.
