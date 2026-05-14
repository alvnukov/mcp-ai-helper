---
id: task-047
title: Сохранять structured acceptance criteria в Lean task registry
status: done
priority: critical
model_level: high
tags:
  - tasks
  - lean-registry
  - acceptance-criteria
  - workflow
  - corrected
worktree_path: .worktrees/task-047
created_at: 2026-05-14T09:15:25.479224Z
updated_at: 2026-05-14T09:15:25.479224Z
---

## Body

Corrected on 2026-05-13: current task_get/task_current expose structured acceptance_criteria and verification_plan for task-069, so the original surface gap is no longer active. Keep closed unless a regression reproduces missing structured fields after task_batch_upsert.
