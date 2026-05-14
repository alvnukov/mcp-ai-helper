---
id: task-055
title: Спроектировать mutation transaction через lake --server
status: done
priority: critical
tags:
  - tasks
  - lean-registry
  - lake-server
  - mutation-design
  - transactions
  - llm-strong
  - type-design-strong
worktree_path: .worktrees/task-055
created_at: 2026-05-14T09:15:25.480394Z
updated_at: 2026-05-14T09:15:25.480394Z
---

## Body

Design/prototype задача, не массовая миграция. Нужно доказать одну safe mutation transaction, например status transition, с Lean-owned validation, rollback/fail-closed semantics, typed diagnostics и clear ownership. Нельзя реализовывать все mutations сразу. Цель — закрыть самый рискованный invariant: partial registry mutation must not survive failed validation/server error.
