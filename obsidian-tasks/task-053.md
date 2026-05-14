---
id: task-053
title: Спроектировать Lean-owned task registry server contract
status: done
priority: critical
tags:
  - tasks
  - lean-registry
  - lake-server
  - architecture
  - protocol
  - llm-strong
  - type-design-strong
worktree_path: .worktrees/task-053
created_at: 2026-05-14T09:15:25.48009Z
updated_at: 2026-05-14T09:15:25.48009Z
---

## Body

Design-only задача после task-052. Нужно описать protocol contract для Lean-owned registry service: ADT/commands, stable JSON schema, diagnostics, validation semantics, transaction boundaries, relation/structured fields model, compatibility with existing MCP task tools, and migration constraints. Go boundary: orchestration, command policy, evidence, owned commits; no Lean source parsing or regex mutation. Не писать production implementation до утверждения контракта.
