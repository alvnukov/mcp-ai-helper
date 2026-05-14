---
id: task-052
title: Исследовать lake --server как транспорт для task registry
status: done
priority: critical
tags:
  - tasks
  - lean-registry
  - lake-server
  - research
  - protocol
  - llm-strong
  - type-research-strong
worktree_path: .worktrees/task-052
created_at: 2026-05-14T09:15:25.479951Z
updated_at: 2026-05-14T09:15:25.479951Z
---

## Body

Research-only задача. Нужно минимально доказать, какие возможности реально даёт lake --server для repo-local Lean code: session lifecycle, request/response framing, diagnostics, typed command dispatch, state visibility, file mutation safety и ограничения по long-running server process. Не делать production-интеграцию task tools. Результат должен сказать, можно ли строить canonical task registry API поверх lake --server, какие риски остаются, и какие операции нужно держать полностью на стороне Lean.
